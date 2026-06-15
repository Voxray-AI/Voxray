# Voxray

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/Voxray-AI/Voxray.svg)](https://pkg.go.dev/github.com/Voxray-AI/Voxray)
[![Go Report Card](https://goreportcard.com/badge/github.com/Voxray-AI/Voxray)](https://goreportcard.com/report/github.com/Voxray-AI/Voxray)
[![codecov](https://codecov.io/gh/Voxray-AI/Voxray/branch/main/graph/badge.svg)](https://codecov.io/gh/Voxray-AI/Voxray)
[![Docs](https://img.shields.io/badge/docs-online-blue)](https://voxray-cac3ed72.mintlify.app/get-started/introduction)
[![Version](https://img.shields.io/badge/version-v0.2.0-green)](https://github.com/Voxray-AI/Voxray/releases)

**Real-time voice agent platform. Build and deploy speech-to-speech AI in minutes.**

[Quick Start](#quick-start) · [Features](#features) · [Docs](https://voxray-cac3ed72.mintlify.app/get-started/introduction) · [Roadmap](#roadmap) · [Contributing](#contributing)

</div>

---

## What is Voxray?

Voxray is a **config-driven Go server** for building production-ready **real-time voice AI agents**. It wires together **STT → LLM → TTS** providers into a single low-latency streaming pipeline delivered over **WebSocket** or **WebRTC** — no audio plumbing required.

Define your pipeline in a single JSON file, point it at any combination of speech and language providers, and ship a voice agent to production in under 5 minutes.

---

## Demo

> The Voxray web console shows a live pipeline visualization with real-time audio waveform, transport toggle (WebRTC / WebSocket), and stage-by-stage status indicators.

![Voxray Console — real-time voice pipeline](docs/assets/console-demo.png)

*Pipeline shown: MIC (48 kHz · Opus) → VAD (Energy) → STT (Sarvam · saarika:v2.5) → LLM (Groq · llama-3.1-8b-instant) → TTS (Sarvam · bulbul:v2) → OUT (WebRTC peer)*

---

## Features

- **End-to-end voice pipeline** — MIC → VAD → STT → LLM → TTS → speaker, fully streamed and low-latency
- **Dual transport** — WebSocket (`/ws`) and WebRTC via SmallWebRTC (`/webrtc/offer`); switchable at runtime
- **Telephony support** — Twilio, Telnyx, Plivo, Exotel, and Daily.co (rooms + optional PSTN dial-in)
- **Wide provider coverage** — 10+ STT, 20+ LLM, and 15+ TTS providers out of the box (see [Supported Providers](#supported-providers))
- **MCP tool integration** — LLM can call external tools via a configurable MCP server
- **Barge-in / interruption** — configurable interruption strategy so users can cut the agent mid-sentence
- **Conversation recording** — mixed-audio WAV per session, uploaded asynchronously to S3
- **Transcript logging** — per-message text persisted to Postgres or MySQL
- **Observability** — Prometheus metrics at `/metrics`, structured JSON logs, configurable log level
- **Plugin system** — extend the pipeline with custom processors and aggregators
- **Config-driven** — one JSON file to define providers, models, voices, turn detection, recording, and more
- **Self-hostable** — no vendor lock-in; deploy to your own infra or VPC

---

## Architecture

Audio enters from a browser, native client, or telephony provider, passes through a configurable processing chain, and streams back to the caller — all within a single low-latency pipeline.

```
┌─────────────────────────────────────────────────────────────────────┐
│                            CLIENT                                   │
│          Browser / Mobile app / Telephony / Daily.co                │
└─────────────┬───────────────────────────────────────┬───────────────┘
              │  WebSocket / WebRTC / Telephony WS     │
              ▼                                         ▲
┌─────────────────────────────────────────────────────────────────────┐
│                         VOXRAY SERVER                               │
│   HTTP server — /ws  /webrtc/offer  /start  /metrics  /swagger/    │
│                                                                     │
│  ┌───────┐    ┌──────────────────────────────────────────────────┐  │
│  │Runner │───▶│           PIPELINE (per session)                 │  │
│  └───────┘    │  MIC → VAD → STT → LLM → TTS → Transport Sink   │  │
│               └──────┬──────────┬──────────┬─────────────────────┘  │
└──────────────────────┼──────────┼──────────┼────────────────────────┘
                       ▼          ▼          ▼
               ┌──────────┐ ┌─────────┐ ┌─────────┐
               │ STT API  │ │ LLM API │ │ TTS API │
               │(Sarvam,  │ │(OpenAI, │ │(ElevenL,│
               │ OpenAI,  │ │ Groq,   │ │ Sarvam, │
               │ Groq...) │ │ Claude…)│ │ Google…)│
               └──────────┘ └─────────┘ └─────────┘
```

Each stage is independently pluggable. Swap any provider by changing a single field in `config.json`.

For detailed design docs, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md).

---

## Supported Providers

| Stage   | Provider        | Notes                                           |
| :------ | :-------------- | :---------------------------------------------- |
| **STT** | OpenAI          | Whisper (`gpt-4o-mini-transcribe`, etc.)        |
|         | Groq            |                                                 |
|         | Sarvam          | Indian languages (`saarika:v2.5`)               |
|         | ElevenLabs      |                                                 |
|         | AWS             | Amazon Transcribe                               |
|         | Google          | Cloud Speech-to-Text                            |
|         | Whisper         | Direct Whisper integration                      |
|         | Camb / Gradium / Soniox |                                         |
| **LLM** | OpenAI          | GPT-4.1, GPT-4o, etc.                          |
|         | Anthropic       | Claude (Sonnet, Haiku, Opus)                    |
|         | Groq            | llama-3.1-8b-instant, etc.                     |
|         | Google          | Gemini (+ Vertex AI with ADC auth)              |
|         | AWS             | Amazon Bedrock                                  |
|         | Mistral / DeepSeek / Cerebras / Grok / Ollama / Qwen | |
|         | AsyncAI / Fish / Inworld / Minimax / Moondream / OpenPipe | |
| **TTS** | OpenAI          | `alloy`, `nova`, `shimmer`, etc.                |
|         | ElevenLabs      |                                                 |
|         | Sarvam          | Indian languages (`bulbul:v2`)                  |
|         | Google          | Cloud Text-to-Speech                            |
|         | AWS             | Amazon Polly                                    |
|         | Groq / Hume / Inworld / Minimax / Neuphonic / XTTS | |

Full capability matrix: [pkg/services/README.md](pkg/services/README.md)

---

## Requirements

**Go 1.25+** is the only hard dependency for the default WebSocket build.

```bash
go version   # 1.25+ required
```

For **WebRTC + Opus TTS audio**, CGO and `gcc` must be on your PATH:

```bash
gcc --version
```

### Installing gcc on Windows

**Option A — WinLibs (winget):**
```powershell
winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements
# Restart terminal, then verify:
gcc --version
```

**Option B — MSYS2:**
Install [MSYS2](https://www.msys2.org/), open MSYS2 UCRT64, then:
```bash
pacman -S mingw-w64-ucrt-x86_64-toolchain
```
Add `C:\msys64\ucrt64\bin` to PATH and verify with `gcc --version`.

> Without CGO, WebRTC TTS reports *opus encoder unavailable (build without cgo)* and the server returns **503** on WebRTC offers.

### API Keys

You will need API keys for whichever providers you configure:

| Provider   | Where to get it                          |
| :--------- | :--------------------------------------- |
| OpenAI     | https://platform.openai.com/api-keys     |
| Anthropic  | https://console.anthropic.com/           |
| Groq       | https://console.groq.com/keys            |
| Sarvam     | https://dashboard.sarvam.ai/             |
| ElevenLabs | https://elevenlabs.io/app/settings/api-keys |
| Google     | https://aistudio.google.com/apikey       |
| AWS        | IAM credentials via standard AWS SDK chain |

---

## Installation

### 1. Clone

```bash
git clone https://github.com/Voxray-AI/Voxray.git
cd Voxray
```

### 2. Build

**WebSocket only (no CGO required):**
```bash
go build -o voxray ./cmd/voxray
# or:
make build
```

**With WebRTC + Opus TTS (CGO required):**

Linux / macOS:
```bash
make build-voice
```

Windows (PowerShell):
```powershell
.\scripts\build-voice.ps1
```

Manual (any OS):
```bash
CGO_ENABLED=1 go build -o voxray ./cmd/voxray
```

### 3. Configure

```bash
cp config.example.json config.json
# Edit config.json — fill in your API keys and choose providers
```

Or use `-init` to scaffold the config and required directories:
```bash
./voxray -init
# Windows:
.\voxray.exe -init
```

### 4. Run

```bash
./voxray -config config.json
# Windows:
.\voxray.exe -config config.json
```

One-step build and run with voice:
```bash
# Linux / macOS:
make run-voice ARGS="-config config.json"

# Windows:
.\scripts\run-voice.ps1 -config config.json
```

---

## Configuration

Set the config path with `-config` or the `VOXRAY_CONFIG` environment variable.

### Minimal config

```json
{
  "transport": "both",
  "host": "0.0.0.0",
  "port": 8080,

  "stt_provider": "openai",
  "stt_model": "gpt-4o-mini-transcribe",

  "llm_provider": "openai",
  "model": "gpt-4.1-mini",
  "system_prompt": "You are a helpful voice assistant. Keep replies brief and conversational.",

  "tts_provider": "openai",
  "tts_voice": "alloy",

  "api_keys": {
    "openai": "YOUR_OPENAI_API_KEY"
  },

  "webrtc_ice_servers": [
    "stun:stun.l.google.com:19302"
  ]
}
```

### Key config options

| Key | Type | Default | Description |
| :-- | :--- | :------ | :---------- |
| `transport` | string | `"websocket"` | `"websocket"`, `"smallwebrtc"`, or `"both"` |
| `host` / `port` | string / int | `"0.0.0.0"` / `8080` | Bind address |
| `stt_provider` | string | — | STT provider (`"openai"`, `"sarvam"`, `"groq"`, …) |
| `stt_model` | string | — | STT model name |
| `llm_provider` | string | — | LLM provider (`"openai"`, `"anthropic"`, `"groq"`, …) |
| `model` | string | — | LLM model name |
| `system_prompt` | string | — | System prompt for the LLM |
| `tts_provider` | string | — | TTS provider (`"openai"`, `"elevenlabs"`, `"sarvam"`, …) |
| `tts_voice` | string | — | Voice name / ID |
| `api_keys` | object | — | Map of `provider → API key` |
| `turn_detection` | string | `"energy"` | VAD mode (`"energy"`, `"silero"`) |
| `allow_interruptions` | bool | `true` | Enable barge-in |
| `metrics_enabled` | bool | `true` | Expose `/metrics` |
| `runner_transport` | string | `""` | `"webrtc"`, `"daily"`, `"twilio"`, `"telnyx"`, `"plivo"`, `"exotel"` |

### Swapping providers (example)

```json
{
  "stt_provider": "sarvam",
  "stt_model": "saarika:v2.5",

  "llm_provider": "groq",
  "model": "llama-3.1-8b-instant",

  "tts_provider": "sarvam",
  "tts_voice": "bulbul:v2",

  "api_keys": {
    "sarvam": "YOUR_SARVAM_KEY",
    "groq": "YOUR_GROQ_KEY"
  }
}
```

### Recording (S3)

```json
"recording": {
  "enable": true,
  "bucket": "your-recordings-bucket",
  "base_path": "recordings/",
  "format": "wav",
  "worker_count": 4
}
```

### Transcript logging (Postgres / MySQL)

```json
"transcripts": {
  "enable": true,
  "driver": "postgres",
  "dsn": "postgres://user:pass@localhost:5432/voxray?sslmode=disable",
  "table_name": "call_transcripts"
}
```

See [config.example.json](config.example.json) and [examples/voice/README.md](examples/voice/README.md) for all options.

---

## Environment Variables

All config values can be overridden via environment variables.

| Variable | Description |
| :------- | :---------- |
| `VOXRAY_CONFIG` | Path to config file |
| `VOXRAY_HOST` / `VOXRAY_PORT` | Bind host / port |
| `VOXRAY_LOG_LEVEL` | `debug`, `info`, `warn`, `error` |
| `VOXRAY_JSON_LOGS` | `true` for structured JSON logs |
| `VOXRAY_CORS_ORIGINS` | Comma-separated allowed CORS origins |
| `VOXRAY_SERVER_API_KEY` | Enable server-level API key auth |
| `VOXRAY_RECORDING_ENABLE` | `true` to enable S3 recording |
| `VOXRAY_RECORDING_BUCKET` | S3 bucket name |
| `VOXRAY_TRANSCRIPTS_ENABLE` | `true` to enable transcript logging |
| `VOXRAY_TRANSCRIPTS_DRIVER` | `postgres` or `mysql` |
| `VOXRAY_TRANSCRIPTS_DSN` | Database connection string |

---

## Usage

### Start the server

```bash
./voxray -config config.json
```

Override specific values without changing the config file:

```bash
./voxray -config config.json -port 9090 -transport webrtc
```

### Connect

| Endpoint | Method | Description |
| :------- | :----- | :---------- |
| `/ws` | GET | WebSocket voice transport (upgrade) |
| `/webrtc/offer` | POST | WebRTC SDP offer/answer |
| `/start` | POST | Create a runner-style session |
| `/sessions/:id/offer` | POST / PATCH | SDP offer for an existing session |
| `/telephony/ws` | GET | Telephony media WebSocket |
| `/health` | GET | Liveness probe |
| `/ready` | GET | Readiness probe |
| `/metrics` | GET | Prometheus scrape endpoint |
| `/swagger/` | GET | Swagger UI |

### Try the browser WebRTC client

```bash
cd tests/frontend && python -m http.server 3000
# Open http://localhost:3000/webrtc-voice.html
# Set Server URL → http://localhost:8080 and click Start
```

See [tests/frontend/README.md](tests/frontend/README.md) for details.

### Scaffold config and directories

```bash
./voxray init                    # scaffold config.json in current dir
./voxray init /path/to/config.json  # scaffold at a custom path
```

---

## Project Structure

```
Voxray/
├── cmd/
│   └── voxray/         # Main entrypoint (main.go, flags, init)
├── pkg/
│   ├── audio/          # VAD, turn detection, codecs, resampling
│   ├── config/         # JSON config parsing and env overrides
│   ├── frames/         # Frame types and serialization (audio, text, control)
│   ├── metrics/        # Prometheus metric definitions and helpers
│   ├── pipeline/       # Runner, processor chain, source/sink, task registry
│   ├── processors/     # Voice, echo, filters, aggregators
│   ├── recording/      # Conversation recording and S3 upload
│   ├── runner/         # Session store, runner args, telephony runner
│   ├── services/       # LLM, STT, TTS interfaces and provider factory
│   ├── transport/      # WebSocket, SmallWebRTC, in-memory transports
│   └── utils/          # Backoff, notifier, sentence splitter, aggregators
├── docs/               # Architecture, connectivity, API, deployment guides
├── examples/
│   └── voice/          # Minimal config samples per provider
├── scripts/            # Build, run, and maintenance scripts (PS1 + shell)
├── tests/
│   └── frontend/       # Browser-based WebRTC voice client (HTML/JS)
├── config.example.json # Annotated starter config
├── Makefile            # build, run, lint, swagger, evals targets
└── go.mod / go.sum
```

---

## Use Cases

| Use Case | Description |
| :------- | :---------- |
| **AI call centers / IVR** | Conversational agents for inbound and outbound calls |
| **In-app voice copilots** | Embed voice AI inside SaaS or productivity apps |
| **Support and ops bots** | Voicebots for customer support and internal tooling |
| **Realtime monitoring** | Voice interfaces for dashboards and control systems |
| **On-prem / VPC deployment** | Self-hosted voice AI where data must stay in-network |
| **Multilingual agents** | Mix providers like Sarvam (Indian languages) for local-language pipelines |

---

## API Reference

Full OpenAPI spec is generated with `make swagger` (requires [swag](https://github.com/swaggo/swag)). Swagger UI is served at `/swagger/` when built.

For client integration — REST, WebSocket framing, auth headers, and WebRTC signaling — see [docs/API_CLIENT.md](docs/API_CLIENT.md).

---

## Roadmap

**Near-term**
- [ ] Additional STT / LLM / TTS providers and opinionated starter presets
- [ ] Deeper observability — distributed tracing and per-stage latency breakdown
- [ ] Docker and Kubernetes deployment templates

**Planned**
- [ ] Additional starter agent examples (customer support, appointment booking, IVR)
- [ ] Expanded docs on scaling, load balancing, and production hardening
- [ ] WebRTC SFU support for multi-party voice sessions
- [ ] Streaming LLM function calling with real-time tool results

---

## Documentation

| Doc | Description |
| :-- | :---------- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | High-level architecture and pipeline design |
| [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md) | System view, entry points, and module map |
| [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md) | Transport layer, runner modes, telephony |
| [docs/API_CLIENT.md](docs/API_CLIENT.md) | Client integration — REST, WebSocket, WebRTC, auth |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Deployment notes and environment setup |
| [docs/EXTENSIONS.md](docs/EXTENSIONS.md) | Plugin system and custom processors |
| [docs/FRAMEWORKS.md](docs/FRAMEWORKS.md) | Framework integration patterns |
| [examples/voice/README.md](examples/voice/README.md) | Provider-specific config samples |
| [tests/frontend/README.md](tests/frontend/README.md) | Browser WebRTC voice client |

Package-level READMEs: [pipeline](pkg/pipeline/README.md) · [transport](pkg/transport/README.md) · [services](pkg/services/README.md) · [audio](pkg/audio/README.md) · [config](pkg/config/README.md) · [recording](pkg/recording/README.md) · [metrics](pkg/metrics/README.md) · [processors](pkg/processors/README.md) · [runner](pkg/runner/README.md) · [utils](pkg/utils/README.md) · [frames](pkg/frames/README.md) · [scripts](scripts/README.md)

---

## Contributing

Contributions are welcome. Quick setup:

```bash
git clone https://github.com/Voxray-AI/Voxray.git
cd Voxray
go test ./...          # run all tests
make lint              # lint (or: ./scripts/pre-commit.sh)
make swagger           # regenerate API docs (requires swag)
make evals             # run eval scenarios (optional)
```

**Before opening a PR:**
1. Run `go test ./...` — all tests must pass
2. Run `make lint` — no new lint errors
3. For new providers, add an entry to the capability matrix in [pkg/services/README.md](pkg/services/README.md)
4. For config changes, update [config.example.json](config.example.json) and [pkg/config/README.md](pkg/config/README.md)

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide — setup, testing conventions, style, and pull request guidelines.

---

## License

Licensed under the [Apache License 2.0](LICENSE).
Attribution details for distribution are in [NOTICE](NOTICE).

---

<div align="center">

Built with Go · WebRTC · WebSocket · STT + LLM + TTS

[Documentation](https://voxray-cac3ed72.mintlify.app/get-started/introduction) · [Report a bug](https://github.com/Voxray-AI/Voxray/issues) · [Request a feature](https://github.com/Voxray-AI/Voxray/issues)

</div>
