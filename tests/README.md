### Test layout and conventions

- **Unit tests**
  - Co-located next to implementation files as `*_test.go` inside `pkg/**` and `cmd/**`.
  - Use Go's standard `testing` package plus `testify` for assertions and helpers.

- **Integration tests**
  - Live under `tests/integration/` as standalone Go packages.
  - Exercise interactions between multiple packages (e.g. `pipeline`, `processors`, `frames`).

- **End-to-end (e2e) tests**
  - Live under `tests/e2e/`.
  - Intended for CLI- or service-level flows (e.g. wrapping `cmd/realtime-demo` in a harness).

- **Test data**
  - Shared fixtures go under `tests/testdata/`.
  - Package-specific fixtures can also use local `testdata/` folders next to the code when they are not shared.
  - The Groq voice E2E pipeline test (`tests/pkg/pipeline/groq_voice_e2e_test.go`) expects a small spoken-phrase WAV file at `tests/testdata/hello.wav` and a valid Groq API key (`GROQ_API_KEY` or `config.json`); it is skipped automatically if these are not present.

- **Config (provider-agnostic, multi-provider)**
  - Optional per-task providers: `stt_provider`, `llm_provider`, `tts_provider` override the default `provider` for that task only (e.g. OpenAI for STT, Groq for LLM/TTS, Sarvam for STT/TTS).
  - Optional per-task model/voice: `stt_model`, `tts_model`, `tts_voice`; `model` is the chat/LLM model. Omitted values use each provider’s defaults.

Example Groq-centric config:

```json
{
  "host": "localhost",
  "port": 8080,
  "provider": "groq",
  "stt_provider": "openai",
  "llm_provider": "groq",
  "tts_provider": "groq",
  "model": "llama-3.1-8b-instant",
  "stt_model": "whisper-1",
  "tts_model": "canopylabs/orpheus-v1-english",
  "tts_voice": "alloy",
  "plugins": ["echo"],
  "api_keys": { "openai": "<openai_key>", "groq": "<groq_key>" }
}
```

Example Sarvam-centric config:

```json
{
  "host": "localhost",
  "port": 8080,
  "provider": "sarvam",
  "stt_provider": "sarvam",
  "llm_provider": "openai",
  "tts_provider": "sarvam",
  "model": "gpt-3.5-turbo",
  "stt_model": "saarika:v2.5",
  "tts_model": "bulbul:v2",
  "tts_voice": "anushka",
  "plugins": ["echo"],
  "api_keys": {
    "openai": "<openai_key>",
    "sarvam": "<sarvam_api_key>"
  }
}
```
