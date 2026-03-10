# Voxray-AI

Config-driven framework for building real-time AI voice agents.

STT → LLM → TTS pipelines with WebSocket and WebRTC streaming.

Built for low-latency conversational systems and industrial-scale adoption.

Voxray-AI is the Go server (`voxray-go`) that runs configurable voice pipelines and exposes **WebSocket** (`/ws`) and **SmallWebRTC** (`/webrtc/offer`) transports for **voice-ai** and **real-time-ai** agents built in **Go/golang**. For architecture and pipeline details, see [Architecture](docs/ARCHITECTURE.md).

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

## Table of contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Examples](#examples)
- [Use cases](#use-cases)
- [Roadmap](#roadmap)
- [Documentation](#documentation)
- [License](#license)
- [Contributing](#contributing)

## Overview

Voxray-AI is a **config-driven Go server** for building **real-time voice agents** over **WebSocket** and **WebRTC**. It wires together **speech-to-text (STT)**, **LLM**, and **text-to-speech (TTS)** providers into low-latency streaming pipelines, so you can build production-ready **voice-ai**, **voice-agents**, and **conversational-ai** systems without hand-rolling audio plumbing. Pipelines, providers, and transports are defined via JSON config, making it easy to swap services and deploy to your own infrastructure.

## Features

- **Pipelines:** Low-latency STT → LLM → TTS voice pipeline with configurable providers and models
- **Transports:** WebSocket and WebRTC (SmallWebRTC) support for real-time streaming audio
- **Providers:** Multiple STT, LLM, and TTS backends (e.g. OpenAI, Groq, Sarvam, AWS, Google, Anthropic)
- **Framework & plugins:** Plugin system for custom processors and aggregators; built as an extensible **ai-framework**
- **Config-driven:** JSON configuration for pipelines and transports; API keys via config or environment variables
- **Voice over WebRTC:** Optional CGO build for Opus encoding and WebRTC TTS audio, tuned for low-latency conversational systems

## Architecture

At a high level, Voxray-AI receives audio from Web or native clients over **WebSocket** or **WebRTC**, runs it through a configurable **STT → LLM → TTS** pipeline, and streams audio responses back over the same transport. Each stage (STT, LLM, TTS) is pluggable, so you can mix and match providers while keeping a consistent, low-latency real-time pipeline.

```mermaid
flowchart LR
  client["Client (Web/Native)"] --> ws[WebSocket]
  client --> webrtc[WebRTC]
  ws & webrtc --> stt[STT]
  stt --> llm[LLM]
  llm --> tts[TTS]
  tts --> ws
  tts --> webrtc
```

For a deeper dive into the internals and pipeline design, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md).


## Requirements

- **Go 1.25+** (see [go.mod](go.mod))
- For **voice over WebRTC (TTS)** and Opus: **CGO** enabled and a **C compiler** (e.g. `gcc`) on PATH

### C compiler (Windows)

CGO needs **gcc** on your PATH. Use one of:

- **WinLibs (winget):**  
  `winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements`  
  Restart your terminal (or add the WinLibs `mingw64\bin` folder to PATH), then run `gcc --version` to confirm.

- **MSYS2:**  
  Install [MSYS2](https://www.msys2.org/), open **MSYS2 UCRT64**, run:  
  `pacman -S mingw-w64-ucrt-x86_64-toolchain`  
  Add `C:\msys64\ucrt64\bin` (or your MSYS2 path) to PATH, then verify with `gcc --version`.

Without CGO, WebRTC TTS will report *opus encoder unavailable (build without cgo); TTS audio cannot be sent* and the server may return **503** for WebRTC offers. The server also returns **503** when at capacity (session or memory cap; see [Session capacity and admission control](#session-capacity-and-admission-control)).

## Installation

Clone the repository, then build and run as below.

### Default build (no WebRTC TTS / no Opus)

```bash
go build -o voxray ./cmd/voxray
```

Or with Make (Linux/macOS):

```bash
make build
make run
```

### Build with voice (WebRTC TTS, Opus)

Requires **CGO** and **gcc** on PATH (see [Requirements](#requirements)).

**Windows (PowerShell, from repo root):**

- Build once, then run:
  ```powershell
  .\scripts\build-voice.ps1
  .\voxray.exe -config config.json
  ```
- Or run without a separate build:
  ```powershell
  .\scripts\run-voice.ps1 -config config.json
  ```

**Linux/macOS:**

```bash
make build-voice
./voxray -config config.json
```

Or in one step:

```bash
make run-voice ARGS="-config config.json"
```

**Manual (any OS):** Set `CGO_ENABLED=1` and ensure `gcc` is on PATH, then:

```bash
CGO_ENABLED=1 go build -o voxray ./cmd/voxray
./voxray -config config.json
```

or:

```bash
CGO_ENABLED=1 go run ./cmd/voxray -config config.json
```

After a voice build, WebRTC offers succeed and TTS audio is sent over the peer connection.

## Quick start

1. Copy the example config and set your API keys (or use env vars):
   ```bash
   cp config.example.json config.json
   ```
2. Run the server:
   ```bash
   ./voxray -config config.json
   ```
   On Windows: `.\voxray.exe -config config.json`
3. **Endpoints:** WebSocket at `/ws`, WebRTC at `/webrtc/offer`.

For sample configs and provider/model examples see [examples/voice/README.md](examples/voice/README.md). For a WebRTC voice client see [tests/frontend/README.md](tests/frontend/README.md).

## Configuration

Configuration is JSON. Copy [config.example.json](config.example.json) to `config.json` and set providers, models, and API keys. Unknown keys (e.g. `_comment`) are ignored; keys can often be overridden via environment variables.

- **[config.example.json](config.example.json)** — structure and available options
- **[examples/voice/README.md](examples/voice/README.md)** — provider/model examples, `transport: "both"`, `webrtc_ice_servers`
- **[tests/frontend/README.md](tests/frontend/README.md)** — WebRTC voice client usage

### Conversation recording to S3

Voxray can record the **entire mixed conversation audio per session** and upload it **asynchronously** to an **S3 bucket** using a simple worker pool.

- **Config block (`config.json`)**:
  ```json
  "recording": {
    "enable": true,
    "bucket": "your-recordings-bucket",
    "base_path": "recordings/",
    "format": "wav",
    "worker_count": 4
  }
  ```
  - **`enable`**: turn recording on for all sessions.
  - **`bucket`**: S3 bucket name where recordings are stored.
  - **`base_path`**: key prefix inside the bucket (default `recordings/`).
  - **`format`**: file format/extension (currently `wav` for 16‑bit PCM mono).
  - **`worker_count`**: number of background uploader workers (thread pool size).

- **Environment overrides** (optional):
  - `VOXRAY_RECORDING_ENABLE=true`
  - `VOXRAY_RECORDING_BUCKET=your-recordings-bucket`
  - `VOXRAY_RECORDING_BASE_PATH=recordings/`
  - `VOXRAY_RECORDING_FORMAT=wav`
  - `VOXRAY_RECORDING_WORKER_COUNT=4`

Each call/session is written to a local WAV file and, when the session ends, a background job enqueues an S3 upload using the configured bucket and base path with a key like:

```text
<base_path>/yyyy/mm/dd/<session-id>.wav
```

AWS credentials and region are resolved via the standard AWS SDK v2 configuration (environment variables, shared config/credentials files, IAM role, etc.).

### Transcript logging to Postgres/MySQL

Voxray can persist **per-message text transcripts** (user and assistant) for each session to a **Postgres** or **MySQL** database.

- **Config block (`config.json`)**:
  ```json
  "transcripts": {
    "enable": true,
    "driver": "postgres",
    "dsn": "postgres://user:pass@localhost:5432/voxray?sslmode=disable",
    "table_name": "call_transcripts"
  }
  ```
  or:
  ```json
  "transcripts": {
    "enable": true,
    "driver": "mysql",
    "dsn": "user:pass@tcp(localhost:3306)/voxray?parseTime=true",
    "table_name": "call_transcripts"
  }
  ```
- **Expected schema** (Postgres example):
  ```sql
  CREATE TABLE call_transcripts (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL,
    role        TEXT NOT NULL, -- "user" or "assistant"
    text        TEXT NOT NULL,
    seq         BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  ```

Environment overrides:

- `VOXRAY_TRANSCRIPTS_ENABLE=true`
- `VOXRAY_TRANSCRIPTS_DRIVER=postgres` (or `mysql`)
- `VOXRAY_TRANSCRIPTS_DSN=...`
- `VOXRAY_TRANSCRIPTS_TABLE=call_transcripts`

### Session capacity and admission control

You can limit concurrent voice sessions and reject new connections when limits are reached (HTTP 503 and `{"error":"server at capacity"}`).

- **Fixed cap**: Set `max_concurrent_sessions` (integer). When the limit is reached, new connections are rejected.
- **Memory-based caps** (optional):
  - `session_cap_memory_percent`: reject when system memory used % is at or above this (e.g. 80). Uses hysteresis so acceptance resumes when usage drops below (threshold − `session_cap_memory_hysteresis_percent`).
  - `session_cap_process_memory_mb`: reject when process heap (MB) is at or above this.
  - `session_cap_memory_hysteresis_percent`: default 5; only applies when `session_cap_memory_percent` is set.
- **Environment overrides**: `VOXRAY_SESSION_CAP_MEMORY_PERCENT`, `VOXRAY_SESSION_CAP_PROCESS_MEMORY_MB`, `VOXRAY_SESSION_CAP_MEMORY_HYSTERESIS_PERCENT`.
- **Scope**: Applies to WebSocket (`/ws`), WebRTC (`/webrtc/offer`), telephony (`/telephony/ws`), runner (`/start`, `/sessions/{id}/api/offer`), and Daily flows.

See [config.example.json](config.example.json) for the `_comment_cap` and the full list of keys.

### Prometheus metrics

- **Endpoint**: the server exposes a Prometheus-compatible scrape endpoint at `/metrics` on the same host/port as `/ws` and `/webrtc/offer`.
- **Config flag**: metrics collection is controlled by `metrics_enabled` in `config.json`:
  - `"metrics_enabled": true` (default when omitted) enables recording of HTTP, WebRTC, STT, LLM, TTS, and session capacity metrics and exports them at `/metrics`.
  - `"metrics_enabled": false` disables recording; `/metrics` remains reachable but returns `204 No Content` so Prometheus scrape configs do not break.
- **Metric areas**: HTTP, WebRTC, STT, LLM, TTS, recording queue, and **session capacity** (`active_sessions` gauge, `sessions_rejected_total` counter with label `reason`: `fixed_cap`, `memory_system`, `memory_process`).
- **Scalability**: metrics are process-local (per instance); Prometheus aggregates across instances using its own `instance`/`pod` labels, and high-cardinality labels like `session_id` are safely handled via hashing/sampling.

You can set the config path with the `-config` flag or the `VOXRAY_CONFIG` environment variable.

## Examples

- **Minimal voice pipeline**: see [examples/voice/README.md](examples/voice/README.md) for sample `config.json` files that wire STT, LLM, and TTS providers into end-to-end voice pipelines.
- **WebRTC voice client**: see [tests/frontend/README.md](tests/frontend/README.md) for a browser-based WebRTC client that connects to Voxray-AI and streams audio in real time.

Example `config.json` snippet for a simple voice agent:

```json
{
  "transport": "both",
  "stt": { "provider": "openai", "model": "gpt-4o-mini-transcribe" },
  "llm": { "provider": "openai", "model": "gpt-4.1-mini" },
  "tts": { "provider": "openai", "voice": "alloy" }
}
```

## Use cases

- **AI call centers / IVR**: conversational-ai agents that handle inbound and outbound calls with low latency.
- **In-app voice copilots**: embed real-time voice-agents inside SaaS or productivity apps using WebSocket or WebRTC.
- **Operations and support bots**: agentic voicebots for support, ops, and internal tooling that run in your own infra.
- **Realtime monitoring and control**: voice interfaces for dashboards, observability tools, and control systems.
- **On-prem / VPC assistants**: self-hosted voice-ai stacks where data must stay within your cloud or datacenter.

## Roadmap

- More built-in STT/LLM/TTS providers and opinionated presets for common stacks.
- Deeper observability, tracing, and debugging tools for real-time pipelines.
- Additional starter templates and example agents for popular voice-agent scenarios.
- Expanded documentation on scaling, deployment patterns, and production hardening.

## Documentation

- [docs/README.md](docs/README.md) — documentation index and reading order
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — high-level architecture and pipeline
- [docs/SYSTEM_ARCHITECTURE.md](docs/SYSTEM_ARCHITECTURE.md) — system view and entry points
- [examples/voice/README.md](examples/voice/README.md) — minimal voice pipeline and config samples
- [tests/frontend/README.md](tests/frontend/README.md) — WebRTC voice client
- [docs/CONNECTIVITY.md](docs/CONNECTIVITY.md) — connectivity and transports
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — deployment notes
- [docs/EXTENSIONS.md](docs/EXTENSIONS.md) — extensions and plugins
- [docs/FRAMEWORKS.md](docs/FRAMEWORKS.md) — framework integration
- [docs/WEBSOCKET_SERVICES.md](docs/WEBSOCKET_SERVICES.md) — WebSocket service reconnection

## License

License: see repository.

## Contributing

Contributions are welcome. Open an issue or pull request to get started.
