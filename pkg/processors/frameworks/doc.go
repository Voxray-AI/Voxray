// Package frameworks provides processor integrations for external runtimes and the RTVI protocol,
// ported from Pipecat's processors/frameworks (https://github.com/pipecat-ai/pipecat/tree/main/src/pipecat/processors/frameworks).
//
//   - External chain: calls an HTTP endpoint (e.g. a Langchain or Strands sidecar) with the last user
//     message from LLMContextFrame and streams the response back as LLMTextFrame.
//   - RTVI: Real-Time Voice Interface protocol processor for client/server messaging, bot-ready,
//     and send-text handling.
package frameworks
