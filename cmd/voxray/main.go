// Package main is the Voxray Go server entrypoint.
//
// @title Voxray API
// @version 1.0
// @description Voxray voice pipeline server: WebSocket and WebRTC transport endpoints.
// @host localhost:8080
// @BasePath /api/v1
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	audiointerruptions "github.com/Voxray-AI/Voxray/pkg/audio/interruptions"
	"github.com/Voxray-AI/Voxray/pkg/audio/turn"
	"github.com/Voxray-AI/Voxray/pkg/audio/vad"
	"github.com/Voxray-AI/Voxray/pkg/config"
	"github.com/Voxray-AI/Voxray/pkg/frames"
	"github.com/Voxray-AI/Voxray/pkg/logger"
	"github.com/Voxray-AI/Voxray/pkg/mcp"
	"github.com/Voxray-AI/Voxray/pkg/observers"
	"github.com/Voxray-AI/Voxray/pkg/pipeline"
	"github.com/Voxray-AI/Voxray/pkg/processors"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregator"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/dtmf"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/gated"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/gatedcontext"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/llmcontextsummarizer"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/llmfullresponse"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/llmtext"
	"github.com/Voxray-AI/Voxray/pkg/processors/aggregators/userresponse"
	"github.com/Voxray-AI/Voxray/pkg/processors/audio"
	"github.com/Voxray-AI/Voxray/pkg/processors/echo"
	_ "github.com/Voxray-AI/Voxray/pkg/processors/filters"
	_ "github.com/Voxray-AI/Voxray/pkg/processors/frameworks"
	_ "github.com/Voxray-AI/Voxray/pkg/processors/frameworks/rtvi"
	proclog "github.com/Voxray-AI/Voxray/pkg/processors/logger"
	"github.com/Voxray-AI/Voxray/pkg/processors/voice"
	"github.com/Voxray-AI/Voxray/pkg/recording"
	"github.com/Voxray-AI/Voxray/pkg/server"
	"github.com/Voxray-AI/Voxray/pkg/services"
	"github.com/Voxray-AI/Voxray/pkg/transcripts"
	"github.com/Voxray-AI/Voxray/pkg/transport"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

func main() {
	defaultConfig := "config.json"
	if e := os.Getenv("VOXRAY_CONFIG"); e != "" {
		defaultConfig = e
	}
	configPath := flag.String("config", defaultConfig, "Path to config file")
	initCmd := flag.Bool("init", false, "Scaffold config.json and dirs (plugins, logs) then exit")
	runnerTransport := flag.String("transport", "", "Runner transport type: webrtc, daily, twilio, telnyx, plivo, exotel (overrides config)")
	runnerPort := flag.Int("port", 0, "Server port (overrides config; default 8080)")
	proxyHost := flag.String("proxy", "", "Public proxy hostname for telephony webhook XML (e.g. mybot.ngrok.io)")
	dialin := flag.Bool("dialin", false, "Enable Daily PSTN dial-in webhook (requires transport=daily)")
	flag.Parse()
	// Support subcommand: Voxray init [config path]
	if len(flag.Args()) >= 1 && flag.Arg(0) == "init" {
		path := *configPath
		if len(flag.Args()) >= 2 {
			path = flag.Arg(1)
		}
		if err := runInit(path, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *initCmd {
		if err := runInit(*configPath, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	forced, err := run(*configPath, runFlags{
		transport: *runnerTransport,
		port:      *runnerPort,
		proxy:     *proxyHost,
		dialin:    *dialin,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if forced {
		os.Exit(1)
	}
}

type runFlags struct {
	transport string
	port      int
	proxy     string
	dialin    bool
}

func run(configPath string, flags runFlags) (forced bool, err error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	// Validate after LoadConfig (LoadConfig already applies ApplyEnvOverrides).
	if errs := config.Validate(cfg); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "config: %s\n", e)
		}
		return false, fmt.Errorf("config validation failed: %d error(s)", len(errs))
	}
	logger.ConfigureFromConfig(cfg.LogLevelOrDefault(), cfg.LogFormatOrDefault())
	// Apply runner flags over config
	if flags.transport != "" {
		cfg.RunnerTransport = flags.transport
	}
	if flags.port > 0 {
		cfg.Port = flags.port
	}
	if flags.proxy != "" {
		cfg.ProxyHost = flags.proxy
	}
	if flags.dialin {
		cfg.Dialin = true
	}

	// Global recording uploader (async S3 worker pool), shared across sessions.
	var recUploader *recording.Uploader
	// Global transcript store, shared across sessions.
	var transcriptStore transcripts.Store
	if cfg.Recording.Enable && cfg.Recording.Bucket != "" {
		workerCount := cfg.Recording.WorkerCount
		queueCap := cfg.Recording.QueueCap
		if queueCap <= 0 {
			queueCap = 32
		}
		uploader, err := recording.NewUploader(context.Background(), workerCount, queueCap, cfg.Recording.MaxRetries)
		if err != nil {
			return false, fmt.Errorf("create recording uploader: %w", err)
		}
		recUploader = uploader
	}
	if cfg.Transcripts.Enable && cfg.Transcripts.Driver != "" && cfg.Transcripts.DSN != "" {
		store, err := transcripts.NewSQLStore(cfg.Transcripts.Driver, cfg.Transcripts.DSN, cfg.Transcripts.TableName)
		if err != nil {
			return false, fmt.Errorf("create transcript store: %w", err)
		}
		transcriptStore = store
	}

	// Register built-in processors for plugin registry (dynamic loading from config)
	pipeline.RegisterProcessor("echo", func(name string, _ json.RawMessage) processors.Processor { return echo.New(name) })
	pipeline.RegisterProcessor("logger", func(name string, _ json.RawMessage) processors.Processor { return proclog.New(name) })
	pipeline.RegisterProcessor("aggregator", func(name string, _ json.RawMessage) processors.Processor { return aggregator.New(name, "", 0) })
	pipeline.RegisterProcessor("dtmf_aggregator", func(name string, _ json.RawMessage) processors.Processor { return dtmf.New(name, 0, "", "") })
	pipeline.RegisterProcessor("gated", func(name string, _ json.RawMessage) processors.Processor {
		return gated.New(name, nil, nil, true, processors.Downstream)
	})
	pipeline.RegisterProcessor("llmfullresponse", func(name string, _ json.RawMessage) processors.Processor { return llmfullresponse.New(name, nil) })
	pipeline.RegisterProcessor("llmtext", func(name string, _ json.RawMessage) processors.Processor { return llmtext.New(name, nil) })
	pipeline.RegisterProcessor("userresponse", func(name string, _ json.RawMessage) processors.Processor { return userresponse.New(name) })
	pipeline.RegisterProcessor("gated_llm_context", func(name string, _ json.RawMessage) processors.Processor { return gatedcontext.New(name, nil, false) })
	pipeline.RegisterProcessor("llmcontextsummarizer", func(name string, _ json.RawMessage) processors.Processor {
		return llmcontextsummarizer.New(name, nil, llmcontextsummarizer.DefaultConfig())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down")
		cancel()
	}()

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	// Prefer voice pipeline if config has provider/model (LLM/STT/TTS); otherwise echo pipeline from plugins.
	useVoice := cfg.Provider != "" && cfg.Model != ""

	buildPipeline := func(ctx context.Context, tr pipeline.Transport, sessionID string) (*pipeline.Pipeline, recording.ConversationRecorder) {
		var (
			pl      *pipeline.Pipeline
			convRec recording.ConversationRecorder
		)
		if useVoice {
			llm, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)
			if cfg.MCP != nil {
				if withTools, ok := llm.(services.LLMServiceWithTools); ok {
					mcpClient := mcp.NewClient(mcp.StdioServerParams{Command: cfg.MCP.Command, Args: cfg.MCP.Args}, cfg.MCP.ToolsFilter, nil)
					if _, err := mcpClient.RegisterTools(ctx, withTools); err != nil {
						logger.Error("MCP RegisterTools: %v", err)
					}
				}
			}
			pl = pipeline.New()
			// Composite observer for turn tracking, latency, and metrics.
			metrics := observers.NewMetrics()
			turnObs := observers.NewTurnTrackingObserver()
			latencyObs := observers.NewUserBotLatencyObserver()
			observersList := []observers.Observer{turnObs, latencyObs}
			if transcriptStore != nil {
				observersList = append(observersList, observers.NewTranscriptObserver(transcriptStore, sessionID))
			}
			withMetrics := observers.NewObserverWithMetrics(
				observers.NewCompositeObserver(observersList...),
				metrics,
			)
			wrap := func(proc processors.Processor) processors.Processor {
				return observers.WrapWithObserver(proc, withMetrics)
			}

			// Optional conversation-wide audio recorder (mixed user+bot) per session.
			if cfg.Recording.Enable && cfg.Recording.Bucket != "" && recUploader != nil {
				rec, err := recording.NewFileRecorder(os.TempDir(), sessionID, 16000)
				if err != nil {
					logger.Error("recording: create recorder failed: %v", err)
				} else {
					convRec = rec
				}
			}

			var audioBuf *audio.AudioBufferProcessor
			if convRec != nil {
				buf := audio.NewAudioBufferProcessor("recording_buffer", 16000, 1, 3200, false)
				buf.OnAudioData = func(m []byte, sampleRate, _ int) {
					if len(m) < 2 {
						return
					}
					samples := make([]int16, len(m)/2)
					for i := range samples {
						lo := uint8(m[2*i])
						hi := uint8(m[2*i+1])
						samples[i] = int16(uint16(lo) | uint16(hi)<<8)
					}
					_ = convRec.WriteSamples(samples, sampleRate)
				}
				buf.StartRecording()
				audioBuf = buf
			}
			if cfg.TurnEnabled() {
				// Construct VAD based on config.
				var vadDetector vad.Detector
				switch cfg.VADBackendOrDefault() {
				case "silero":
					if a, err := vad.NewSileroAnalyzer(vad.Params{
						Confidence: cfg.VADParams().Confidence,
						StartSecs:  cfg.VADParams().StartSecs,
						StopSecs:   cfg.VADParams().StopSecs,
						MinVolume:  cfg.VADParams().MinVolume,
					}, 16000); err == nil {
						vadDetector = &vad.AnalyzerDetector{Analyzer: a}
					} else {
						logger.Info("silero VAD unavailable (%v), falling back to energy VAD", err)
					}
				}
				if vadDetector == nil {
					vp := cfg.VADParams()
					// Energy VAD: volume (smoothed RMS) from typical mics often stays well below 0.2,
					// so cap MinVolume at 0.05 so speech is detected when confidence is high.
					conf, minVol := vp.Confidence, vp.MinVolume
					if conf > 0.5 {
						conf = 0.5
					}
					if minVol > 0.05 {
						minVol = 0.05
					}
					// Cap RMS threshold at 0.01 so that conf = rms/threshold reaches 0.5 at rms >= 0.005;
					// otherwise vad_threshold 0.02 requires rms >= 0.01 and many mics never trigger speech.
					thresh := cfg.VadThreshold
					if thresh <= 0 {
						thresh = 0.02
					} else if thresh > 0.01 {
						thresh = 0.01
					}
					vadParams := vad.Params{
						Confidence: conf,
						StartSecs:  vp.StartSecs,
						StopSecs:   vp.StopSecs,
						MinVolume:  minVol,
						Threshold:  thresh,
					}
					ed := vad.NewEnergyDetectorWithParams(vadParams)
					vadDetector = ed
				}
				vadDetector.SetSampleRate(16000)
				turnParams := turn.Params{
					StopSecs:        cfg.TurnStopSecs,
					PreSpeechMs:     cfg.TurnPreSpeechMs,
					MaxDurationSecs: cfg.TurnMaxDurationSecs,
				}
				if turnParams.StopSecs <= 0 {
					turnParams.StopSecs = turn.DefaultStopSecs
				}
				if turnParams.PreSpeechMs <= 0 {
					turnParams.PreSpeechMs = turn.DefaultPreSpeechMs
				}
				if turnParams.MaxDurationSecs <= 0 {
					turnParams.MaxDurationSecs = turn.DefaultMaxDurationSecs
				}
				analyzer := turn.NewSilenceTurnAnalyzer(turnParams)
				if cfg.VADStartSecs != 0 {
					analyzer.UpdateVADStartSecs(cfg.VADStartSecs)
				}
				// Derive user turn/idle timeouts, falling back to sensible defaults.
				userTurnStopTimeout := cfg.UserTurnStopTimeoutSecs
				if userTurnStopTimeout <= 0 {
					if cfg.TurnStopSecs > 0 {
						userTurnStopTimeout = cfg.TurnStopSecs
					} else {
						userTurnStopTimeout = 5 // seconds
					}
				}
				userIdleTimeout := cfg.UserIdleTimeoutSecs
				pl.Add(wrap(voice.NewTurnProcessorWithUserTurn(
					"turn",
					vadDetector,
					analyzer,
					16000,
					1,
					cfg.TurnAsync,
					userTurnStopTimeout,
					userIdleTimeout,
					cfg.TurnMaxDurationSecs,
				)))
			}
			if audioBuf != nil {
				pl.Add(wrap(audioBuf))
			}
			pl.Add(wrap(voice.NewSTTProcessor("stt", sttSvc, 16000, 1)))
			pl.Add(wrap(voice.NewLLMProcessorWithSystemPrompt("llm", llm, "You are a helpful voice assistant. Reply briefly and naturally in the same language as the user.")))
			pl.Add(wrap(voice.NewTTSProcessor("tts", ttsSvc, 24000)))

			// Optional interruption controller for barge-in behavior. When
			// AllowInterruptions is true and a supported strategy is configured,
			// insert an InterruptionController between TTS and the sink so it
			// can observe both user transcripts and bot speech events.
			if cfg.AllowInterruptions {
				strategy := audiointerruptions.NewStrategy(cfg.InterruptionStrategy, cfg.MinWords)
				if strategy != nil {
					pl.Add(wrap(voice.NewInterruptionController("interruptions", strategy)))
				}
			}

			pl.Add(wrap(pipeline.NewSink("sink", tr.Output())))
			_ = metrics // metrics available for OTELExport or logging if needed
		} else {
			pl = pipeline.New()
			if err := pl.AddFromConfig(cfg); err != nil {
				// Fallback to echo if plugin names unknown
				pl.Add(echo.New("echo"))
			}
			pl.Add(pipeline.NewSink("sink", tr.Output()))
		}
		return pl, convRec
	}

	sessionRegistry := server.NewSessionRegistry()
	pipelineStore := server.NewPipelineStore()
	_ = pipelineStore // used by API routes when added

	onTransport := func(parentCtx context.Context, tr transport.Transport) {
		sessionCtx, cancelSession := context.WithCancel(parentCtx)
		sessionID := uuid.New().String()
		sessionCtx = logger.WithSession(sessionCtx, sessionID)
		sessionRegistry.Register(sessionID, server.SessionMeta{
			SessionID: sessionID,
			CreatedAt: time.Now(),
		}, cancelSession)
		pl, convRec := buildPipeline(sessionCtx, tr, sessionID)
		startFrame := frames.NewStartFrame()
		startFrame.AllowInterruptions = cfg.AllowInterruptions
		runner := pipeline.NewRunner(pl, tr, startFrame)
		if cfg.PipelineInputQueueCap > 0 {
			runner.InputQueueCap = cfg.PipelineInputQueueCap
		}
		go func() {
			defer sessionRegistry.Unregister(sessionID)
			defer cancelSession()
			defer func() {
				if convRec != nil && recUploader != nil && cfg.Recording.Bucket != "" {
					info, err := convRec.Close()
					if err != nil {
						logger.Error("recording: close failed: %v", err)
						return
					}
					key := recording.BuildS3Key(cfg.Recording.BasePath, sessionID, cfg.Recording.Format, info.StartedAt)
					job := recording.RecordingJob{
						LocalPath: info.Path,
						Bucket:    cfg.Recording.Bucket,
						Key:       key,
					}
					if err := recUploader.Enqueue(job); err != nil {
						logger.Error("recording: enqueue failed: %v", err)
					}
				}
			}()
			_ = runner.Run(sessionCtx)
		}()
	}

	var transcriptFetcher transcripts.Fetcher
	if transcriptStore != nil {
		if f, ok := transcriptStore.(transcripts.Fetcher); ok {
			transcriptFetcher = f
		}
	}
	var shutdownForced bool
	err = server.StartServers(ctx, cfg, onTransport, sessionRegistry, pipelineStore, transcriptFetcher)
	if err != nil {
		return false, err
	}
	// recUploader.Shutdown with its own timeout (main context already cancelled).
	uploadTimeout := 10 * time.Second
	if recUploader != nil {
		uploadCtx, cancelUpload := context.WithTimeout(context.Background(), uploadTimeout)
		recUploader.Shutdown(uploadCtx)
		cancelUpload()
	}
	// transcriptStore.Close with 2s budget (synchronous Close; allow up to 2s).
	if transcriptStore != nil {
		done := make(chan struct{})
		go func() {
			_ = transcriptStore.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			logger.Info("transcript store close timed out after 2s; flush may be incomplete")
		}
	}
	return shutdownForced, nil
}
