# Voila

Voice pipeline server (STT → LLM → TTS) with WebSocket and WebRTC transports.

## Running with voice (WebRTC TTS)

WebRTC outbound audio uses the **Opus encoder**, which is only built when **CGO** is enabled and a **C compiler** is available. Without it, you'll see:

`opus encoder unavailable (build without cgo); TTS audio cannot be sent`

### 1. Install a C compiler (Windows)

CGO needs **gcc** on your PATH. Use one of:

- **WinLibs (winget):**  
  `winget install BrechtSanders.WinLibs.POSIX.UCRT --accept-package-agreements`  
  Restart your terminal (or add the WinLibs `mingw64\bin` folder to PATH), then run `gcc --version` to confirm.

- **MSYS2:**  
  Install [MSYS2](https://www.msys2.org/), open **MSYS2 UCRT64**, run:  
  `pacman -S mingw-w64-ucrt-x86_64-toolchain`  
  Add `C:\msys64\ucrt64\bin` (or your MSYS2 path) to PATH, then verify with `gcc --version`.

### 2. Build and run with CGO

**Windows (PowerShell, from repo root):**

- Build once, then run:  
  `.\scripts\build-voice.ps1`  
  then:  
  `.\voila.exe -config config.json`

- Or run without a separate build:  
  `.\scripts\run-voice.ps1 -config config.json`

**Linux/macOS:**

- `make build-voice` then run the binary, or:  
  `make run-voice ARGS="-config config.json"`

**Manual (any OS):**  
Set `CGO_ENABLED=1` and ensure `gcc` is on PATH, then:

```bash
go build -o voila.exe ./cmd/voila
.\voila.exe -config config.json
```

or:

```bash
CGO_ENABLED=1 go run ./cmd/voila -config config.json
```

After this, WebRTC offers will succeed and TTS audio will be sent over the peer connection.

## Config

Use a `config.json` with your providers and API keys. See `examples/voice/README.md` for sample configs and `tests/frontend/README.md` for the WebRTC voice client.
