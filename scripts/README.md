# Scripts

Index of script groups and standalone scripts for building, running, and maintaining the Voxray Go server.

## Table of scripts

| Script / Dir | Description | How to run |
|--------------|-------------|------------|
| **build.ps1** | Build voxray (no CGO; no WebRTC TTS/Opus) | `.\scripts\build.ps1` (Windows, from repo root) |
| **build-voice.ps1** | Build with CGO for WebRTC TTS and Opus | `.\scripts\build-voice.ps1` (requires gcc on PATH) |
| **run.ps1** | Run voxray with config (default build) | `.\scripts\run.ps1 -config config.json` |
| **run-voice.ps1** | Run with voice build (CGO) | `.\scripts\run-voice.ps1 -config config.json` |
| **pre-commit.sh** | Go format + vet check (fails if not formatted or vet fails) | `./scripts/pre-commit.sh` (from repo root) |
| **fix-lint.sh** | Fix formatting and optional golangci-lint --fix | `./scripts/fix-lint.sh` |
| **mem-watch.sh** | Watch process RSS in GB (Linux/macOS) | `./scripts/mem-watch.sh <PID>` |
| **mem-watch.ps1** | Same on Windows (PowerShell) | `.\scripts\mem-watch.ps1 -PID <pid>` |
| **daily/** | Daily.co transport and dial-in | See [daily/README.md](daily/README.md) |
| **dtmf/** | Generate DTMF WAV files (ffmpeg or Go) | See [dtmf/README.md](dtmf/README.md); `./scripts/dtmf/generate_dtmf.sh` or `go run ./cmd/generate-dtmf [out_dir]` |
| **evals/** | Go-native eval runner (LLM scenarios) | See [evals/README.md](evals/README.md); `go run ./cmd/evals -config scripts/evals/config/scenarios.json -voxray-config config.json` |
| **krisp/** | Krisp Viva (upstream only) | See [krisp/README.md](krisp/README.md) |

## Makefile

From repo root, `make lint` runs pre-commit checks, `make lint-fix` runs fix-lint, and `make evals` runs the eval runner with default config. Use `make build` / `make build-voice` and `make run` / `make run-voice` on Linux/macOS.

## See also

- [../README.md](../README.md) — Root README, installation, quick start
- [../docs/README.md](../docs/README.md) — Documentation index
