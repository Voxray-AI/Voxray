## Voxray Concurrency & Synchronization Skill

This file defines **non‑obvious concurrency rules** for Voxray. Follow these patterns exactly when adding goroutines, channels, locks, atomics, or worker pools.

---

### Session Goroutine Topology

- **Rule: One runner per session; transports own I/O goroutines.**
  - `pipeline.Runner.Run` runs in **one goroutine per connection**.
  - Transports (`websocket.ConnTransport`, `smallwebrtc.Transport`, memory/telephony transports) own their **read/write loops** and `Done()` lifecycle.

**Wrong (spawning an extra per‑frame goroutine in a processor):**

```go
// BAD: starts a goroutine per frame, bypassing pipeline back-pressure.
func (p *MyProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	go func() {
		_ = p.Next().ProcessFrame(ctx, f, dir)
	}()
	return nil
}
```

**Right (keep topology: Runner + transport + pipeline only):**

```go
// GOOD: let the pipeline run in the runner goroutine.
func (p *MyProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if p.Next() != nil {
		return p.Next().ProcessFrame(ctx, f, dir)
	}
	return nil
}
```

---

- **Rule: Runner owns the “transport → pipeline” bridge via a bounded queue.**
  - See `pipeline.Runner.Run` in `pkg/pipeline/runner.go`.
  - The runner launches **exactly two goroutines** when there is input:
    - Reader: reads from `Transport.Input()` into a buffered `queueCh`.
    - Worker: drains `queueCh` into `Pipeline.Push`.

**Wrong (directly pushing from transport goroutine, blocking WebSocket read):**

```go
// BAD: called from websocket readLoop.
for {
	_, data, _ := conn.ReadMessage()
	f, _ := serializer.Deserialize(data)
	// This may block on slow pipeline → stalls network reads.
	_ = runner.Pipeline.Push(ctx, f)
}
```

**Right (let Runner own a separate buffered queue):**

```go
// GOOD: websocket.ConnTransport only writes to t.inCh; Runner owns Push.
func (t *ConnTransport) readLoop() {
	for {
		_, data, err := t.conn.ReadMessage()
		// ...
		f, _ := t.serializer.Deserialize(data)
		select {
		case <-t.closed:
			return
		case t.inCh <- f: // buffered; Runner drains via queueCh
		}
	}
}
```

---

- **Rule: When adding a new stage that needs goroutines, tie them to the session context.**
  - Use `context.Context` passed into `Setup`/`Run`/`ProcessFrame`.
  - Never use `context.Background()` for long‑lived goroutines; cancel on session end.

**Wrong (goroutine outlives session):**

```go
// BAD: leaks if session ends; ignores cancellation.
go func() {
	for f := range ch {
		_ = doWork(context.Background(), f)
	}
}()
```

**Right (cancellable worker owned by processor/runner):**

```go
// GOOD: worker stops when ctx is cancelled.
go func() {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-ch:
			if !ok {
				return
			}
			_ = doWork(ctx, f)
		}
	}
}()
```

When adding a new pipeline stage that needs a worker, follow the patterns in:
- `pipeline.Sink.Setup` / `Cleanup` (single writer goroutine).
- `pipeline.PipelineTask.Run` (one drain goroutine).

---

### Channel Rules

- **Rule: Use bounded, buffered channels for transport ↔ pipeline; never unbounded.**
  - `pipeline.Runner` uses `inputQueueCap = 256` for mic frames.
  - `pipeline.PipelineTask` uses `DefaultPipelineTaskQueueSize = 64`.
  - Transports like `websocket.ConnTransport` and `smallwebrtc.Transport` use `64`‑sized `inCh`/`outCh`.

**Wrong (unbuffered channel between transport and pipeline):**

```go
// BAD: any slow processor stalls websocket readLoop.
inCh := make(chan frames.Frame) // unbuffered
go func() {
	for f := range inCh {
		_ = pl.Push(ctx, f)
	}
}()
```

**Right (buffered with explicit back‑pressure semantics):**

```go
// GOOD: bounded buffer; reader never blocks on pipeline.
const inputQueueCap = 256
queueCh := make(chan frames.Frame, inputQueueCap)
go func() { // reader
	for {
		select {
		case <-ctx.Done():
			close(queueCh)
			return
		case f, ok := <-inCh:
			if !ok {
				close(queueCh)
				return
			}
			select {
			case <-ctx.Done():
				close(queueCh)
				return
			case queueCh <- f:
			}
		}
	}
}()
```

---

- **Rule: When adding new queues, choose capacity by analogy.**
  - **Mic/transport → pipeline**: 256 (`inputQueueCap`) to absorb STT/LLM/TTS bursts.
  - **PipelineTask queues**: 64 for generic frame pipelines.
  - **SmallWebRTC branch outCh**: 64 for parallel pipelines.
  - **Recording uploader jobs**: 32 queue, workers configurable (`Recording.WorkerCount`).

For a new per‑session frame queue, default to **64** unless you have clear evidence of needed depth. For cross‑session global queues (S3 uploads), use a smaller capacity and explicit “queue full” error like `recording.Uploader.Enqueue`.

---

- **Rule: Always include a `ctx.Done()` arm on blocking sends/receives that depend on session lifetime.**

**Wrong (send can block forever when session cancelled):**

```go
// BAD: if outCh is full, this goroutine may leak.
outCh <- f
```

**Right (respect cancellation):**

```go
// GOOD: used in Sink and transports.
select {
case outCh <- f:
case <-ctx.Done():
	return
}
```

Follow this pattern in:
- `ConnTransport.readLoop` (send to `t.inCh`).
- `pipeline.Sink.ProcessFrame` (send to `sendCh`).
- `smallwebrtc.handleInboundTrack` (send `AudioRawFrame` to `t.inCh`).

---

- **Rule: Use channel direction to express ownership at function boundaries.**
  - Transports expose:
    - `Input() <-chan frames.Frame` (read‑only to callers).
    - `Output() chan<- frames.Frame` (write‑only to callers).
  - `pipeline.Source` and `Sink` embed channels with explicit directions.

**Wrong (function takes a bidirectional channel and both sends/receives):**

```go
// BAD: caller can accidentally both send and receive.
func attach(ch chan frames.Frame) {
	go func() { ch <- frames.NewStartFrame() }()
	go func() { <-ch }()
}
```

**Right (directional channels clarify intent):**

```go
// GOOD: only send; can't accidentally receive.
func emitStart(out chan<- frames.Frame) {
	go func() {
		select {
		case out <- frames.NewStartFrame():
		default:
		}
	}()
}
```

When introducing new helpers around transports or pipelines, mirror the `Transport` and `Sink` signatures to keep ownership clear.

---

### Mutex & Atomic Rules

- **Rule: Prefer `sync.Mutex` for per‑session state; use `atomic` for hot counters and flags.**
  - `pipeline.ParallelPipeline` and `SyncParallelPipeline` use `sync.Mutex` for coordinating branch state (maps, counters).
  - `observers.TranscriptObserver` uses an internal mutex plus `atomic.Int32` for worker/closed flags.
  - `server.buildSessionCap` uses `atomic.Int64` for `activeCount` and `metrics.ActiveSessions`.

**Wrong (using atomic for structured state):**

```go
// BAD: composite state split into multiple atomics → races.
type Bad struct {
	count atomic.Int64
	data  map[string]string
}
```

**Right (mutex guards structured state; atomic for independent counters):**

```go
// GOOD: mirrors ParallelPipeline / TranscriptObserver.
type Good struct {
	mu    sync.Mutex
	data  map[string]string
	count int64
}

// For pure counters hot on the critical path, use atomic.
var activeSessions atomic.Int64
```

---

- **Rule: Never perform blocking I/O or provider calls while holding a mutex.**
  - This avoids priority inversion and global stalls in high‑concurrency runs.

**Wrong (lock held across network I/O):**

```go
// BAD: holds mu while calling external HTTP client.
func (s *Service) Do(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp, err := s.client.Do(req.WithContext(ctx)) // external I/O under lock
	// ...
	return err
}
```

**Right (copy minimal state under lock, then release before I/O):**

```go
// GOOD: pattern from observers and services.
func (s *Service) Do(ctx context.Context) error {
	s.mu.Lock()
	cfg := s.cfg // copy needed fields
	s.mu.Unlock()

	resp, err := s.client.Do(reqWithConfig(ctx, cfg))
	if err != nil {
		return err
	}
	// Process response without holding s.mu unless mutating shared state.
	return nil
}
```

Apply this rule when adding new logic to:
- `observers.TranscriptObserver` (never call `SaveMessage` with locks held beyond enqueue).
- Provider services in `pkg/services/*`.

---

- **Rule: Use `sync.Pool` for hot audio buffers instead of allocating in tight loops.**
  - `smallwebrtc` uses `sync.Pool` for:
    - `outboundSamplesPool` (int16 slices for Opus encoding).
    - `inboundDecodePool` (byte buffers for decoding).

**Wrong (allocating `[]byte` or `[]int16` for every encode/decode):**

```go
// BAD: allocates on every packet.
for len(pcm) >= opusFrameSize {
	frame := pcm[:opusFrameSize]
	pcm = pcm[opusFrameSize:]
	samples := bytesToSamples(frame) // allocates new []int16
	encoded, _ := enc.Encode(samples, opusFrameSamples, 1500)
	_ = track.WriteSample(media.Sample{Data: encoded, Duration: frameDuration})
}
```

**Right (borrow from `sync.Pool`, return after use):**

```go
// GOOD: pattern from runOutboundEncode.
func withPooledSamples(b []byte, fn func(samples []int16)) {
	n := len(b) / 2
	ptr := outboundSamplesPool.Get().(*[]int16)
	out := *ptr
	if cap(out) < n {
		out = make([]int16, n)
		*ptr = out
	} else {
		out = out[:n]
	}
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	fn(out)
	outboundSamplesPool.Put(ptr)
}
```

When you add new audio hot paths (e.g. in transports or audio processors), follow this pattern instead of allocating per packet.

---

### Worker Pool Usage

- **Rule: Recording uploads use a fixed worker pool with a bounded job queue; never spawn per‑upload goroutines.**
  - `recording.Uploader` starts `workerCount` goroutines, each looping on `jobs <-chan RecordingJob`.
  - `Enqueue` is **non‑blocking** only while the queue has capacity; otherwise it fails with `"recording queue full"`.

**Wrong (one goroutine per upload, unbounded growth):**

```go
// BAD: leaks goroutines under load.
func (u *Uploader) Enqueue(job RecordingJob) {
	go func() {
		_ = u.putOne(ctx, job)
	}()
}
```

**Right (fixed worker pool, back‑pressure via bounded channel):**

```go
// GOOD: mirrors recording.Uploader.
func NewUploader(ctx context.Context, workerCount, queueSize int) (*Uploader, error) {
	u := &Uploader{
		client: s3.NewFromConfig(awsCfg),
		jobs:   make(chan RecordingJob, queueSize),
	}
	for i := 0; i < workerCount; i++ {
		u.wg.Add(1)
		go u.worker(ctx)
	}
	return u, nil
}

func (u *Uploader) Enqueue(job RecordingJob) error {
	select {
	case u.jobs <- job:
		return nil
	default:
		return fmt.Errorf("recording queue full")
	}
}
```

---

- **Rule: When `Submit`/`Enqueue` fails for a worker pool, log and degrade gracefully; never panic.**
  - Example: session end tries to enqueue a recording job; if queue is full, it logs and continues.

**Wrong (ignoring pool‑full error):**

```go
// BAD: silently drops failed work.
_ = recUploader.Enqueue(job) // error ignored
```

**Right (log and keep serving sessions):**

```go
// GOOD: from cmd/voxray/main.go.
if err := recUploader.Enqueue(job); err != nil {
	logger.Error("recording: enqueue failed: %v", err)
}
```

Apply the same pattern when you introduce any new worker pool (e.g. DB writers, HTTP fan‑out).

---

- **Rule: Size new pools based on cores and expected latency.**
  - For CPU‑light, I/O‑bound tasks (S3 uploads, HTTP providers), start with:
    - `workers = 2 * NumCPU` for heavy shared pools.
    - `workers = 2–4` for single‑feature pools.
  - For CPU‑heavy tasks (local audio processing), keep `workers <= NumCPU`.
  - Express this in config (like `Recording.WorkerCount`) instead of hard‑coding.

When adding a new worker pool, follow the `recording.Uploader` pattern: constructor takes `workerCount` and `queueSize`, with safe defaults when zero.

---

### Common Mistakes (and How to Avoid Them)

1. **Launching goroutines without passing the session context.**

   **Wrong:**

   ```go
   go func() {
   	_ = runner.Run(context.Background()) // never cancelled
   }()
   ```

   **Right (from `cmd/voxray/main.go`):**

   ```go
   go func() {
   	_ = runner.Run(ctx) // ctx cancelled on SIGINT/SIGTERM
   }()
   ```

2. **Writing to `websocket.Conn` from multiple goroutines.**

   The only allowed writing goroutine is `ConnTransport.writeLoop`.

   **Wrong:**

   ```go
   // BAD: another goroutine calling WriteMessage directly.
   go func() {
   	_ = conn.WriteMessage(websocket.TextMessage, data)
   }()
   ```

   **Right:**

   ```go
   // GOOD: send frames into ConnTransport.Out(), single writer loop owns conn.
   tr := websocket.NewConnTransport(conn, 64, 64, ser)
   out := tr.Output()
   go func() {
   	select {
   	case out <- frames.NewTextFrame("hello"):
   	case <-ctx.Done():
   	}
   }()
   ```

3. **Using `context.Background()` instead of the session context in providers or observers.**

   **Wrong:**

   ```go
   // BAD: DB write keeps running after session cancelled.
   _ = store.SaveMessage(context.Background(), sessionID, role, text, time.Now().UTC(), seq)
   ```

   **Right (from `TranscriptObserver.runWriter`):**

   ```go
   _ = o.store.SaveMessage(o.ctx, o.sessionID, m.role, m.text, m.at, m.seq)
   ```

4. **Creating an Opus encoder per packet instead of per session.**

   **Wrong:**

   ```go
   // BAD: inside loop over TTSAudioRawFrame chunks.
   enc, _ := gopus.NewEncoder(opusSampleRate, 1, gopus.Audio)
   encoded, _ := enc.Encode(samples, opusFrameSamples, 1500)
   ```

   **Right (from `runOutboundEncode`):**

   ```go
   enc, err := gopus.NewEncoder(opusSampleRate, 1, gopus.Audio) // once per transport
   // reuse enc for all frames in the loop
   ```

5. **Allocating `[]byte` inside the audio read loop.**

   **Wrong:**

   ```go
   // BAD: allocate fresh PCM buffer every RTP packet.
   decoded, _ := decoder.Decode(pkt.Payload)
   pcm := make([]byte, len(decoded))
   copy(pcm, decoded)
   ```

   **Right (reuse/resample buffers as in `Resample16Mono` and SmallWebRTC):**

   ```go
   var pcmAccum []byte
   decoded, _ := decoder.Decode(pkt.Payload)
   resampled := audio.Resample16MonoAlloc(decoded, 48000, 16000) // allocates once per packet
   pcmAccum = append(pcmAccum, resampled...)
   ```

   For hot paths, prefer `Resample16Mono` with a reused `out []byte` or `sync.Pool` patterns.

6. **Blocking channel send without a `ctx.Done()` select arm.**

   See “Channel Rules” above. Always wrap blocking sends in a `select` with `<-ctx.Done()` or `<-closed`.

7. **Holding a mutex while calling an external HTTP provider.**

   See “Mutex & Atomic Rules”. Always copy config under lock, then unlock before network I/O (e.g. in OpenAI/Groq services or future providers).

8. **Forgetting to call `wg.Done()` in deferred cleanup when adding workers.**

   **Wrong:**

   ```go
   u.wg.Add(1)
   go func() {
   	for job := range u.jobs {
   		_ = u.uploadOnce(ctx, job)
   	}
   	// missing wg.Done()
   }()
   ```

   **Right (from `recording.Uploader.worker`):**

   ```go
   func (u *Uploader) worker(ctx context.Context) {
   	for job := range u.jobs {
   		_ = u.uploadOnce(ctx, job)
   	}
   	u.wg.Done()
   }
   ```

   When you add any new worker tied to a `sync.WaitGroup`, ensure:
   - `wg.Add(1)` before the goroutine starts.
   - `defer wg.Done()` or explicit `wg.Done()` on all exit paths.

