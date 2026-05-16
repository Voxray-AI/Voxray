// Voxray refreshed UI — voice operations console
// Editorial dark/light, mono telemetry, real waveform, real WebRTC pipeline.

const { useState, useEffect, useRef, useCallback } = React;

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "theme": "ink",
  "accent": "#C5F462",
  "showLog": false,
  "density": "regular",
  "transport": "WebRTC"
}/*EDITMODE-END*/;

function titleCaseProvider(name) {
  if (!name) return "";
  return String(name).replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function formatProviderSub(provider, detail) {
  const p = titleCaseProvider(provider);
  if (!p) return "—";
  return detail ? `${p} · ${detail}` : p;
}

function transportOutLabel(transport) {
  const t = (transport || "").toLowerCase();
  if (t === "websocket") return "WebSocket";
  if (t === "smallwebrtc" || t === "both") return "WebRTC peer";
  return titleCaseProvider(t) || "—";
}

const THEMES = {
  ink: {
    bg: "#0E0F0C",
    bg2: "#161813",
    panel: "#1A1C17",
    line: "rgba(255,255,255,0.08)",
    line2: "rgba(255,255,255,0.14)",
    text: "#F2F1EC",
    dim: "rgba(242,241,236,0.56)",
    dim2: "rgba(242,241,236,0.38)",
    chip: "rgba(255,255,255,0.04)",
  },
  paper: {
    bg: "#F4F2EB",
    bg2: "#EAE7DD",
    panel: "#FBFAF5",
    line: "rgba(14,15,12,0.10)",
    line2: "rgba(14,15,12,0.18)",
    text: "#15170F",
    dim: "rgba(21,23,15,0.60)",
    dim2: "rgba(21,23,15,0.40)",
    chip: "rgba(14,15,12,0.03)",
  },
};

// ─── App ─────────────────────────────────────────────────────────────────────
function App() {
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const theme = THEMES[t.theme] || THEMES.ink;

  // Connection state: idle → requesting → connecting → connected → speaking → listening
  const [state, setState] = useState("idle");
  const [sessionMs, setSessionMs] = useState(0);
  const [audioLevel, setAudioLevel] = useState(0);
  const [backendUrl] = useState("");
  const [pipelineInfo, setPipelineInfo] = useState(null);
  const [logLines, setLogLines] = useState([
    "[boot] github.com/Voxray-AI/Voxray v0.4.2 — pipeline runner ready",
    "[boot] transports: ws, webrtc(opus) — cgo enabled",
  ]);

  const sessionStartRef = useRef(null);
  const canvasRef = useRef(null);

  // WebRTC refs — kept as refs so async callbacks always see the latest value
  const pcRef = useRef(null);
  const localStreamRef = useRef(null);
  const remoteAudioRef = useRef(null);
  const audioCtxRef = useRef(null);
  const analyserRef = useRef(null);
  const waveSourceRef = useRef(null);

  const accent = t.accent;
  const isConnected = state === "connected" || state === "speaking" || state === "listening";
  const isBusy = state === "requesting" || state === "connecting";

  const pushLog = useCallback((s) => {
    const time = new Date().toTimeString().slice(0, 8);
    setLogLines((prev) => [...prev.slice(-40), `[${time}] ${s}`]);
  }, []);

  useEffect(() => {
    const base = backendUrl.trim().replace(/\/$/, "");
    const url = `${base || ""}/api/v1/config`;
    let cancelled = false;
    fetch(url)
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (cancelled) return;
        const payload = data?.data ?? data;
        if (payload && typeof payload === "object") setPipelineInfo(payload);
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [backendUrl]);

  // ─ WebRTC waveform visualizer — feeds real audio data into analyserRef ────
  const startWaveformVisualizer = useCallback((stream) => {
    const AudioCtx = window.AudioContext || window.webkitAudioContext;
    if (!AudioCtx || !stream) return;

    if (!audioCtxRef.current) {
      audioCtxRef.current = new AudioCtx();
    }
    if (audioCtxRef.current.state === "suspended") {
      audioCtxRef.current.resume().catch(() => {});
    }

    if (waveSourceRef.current) {
      try { waveSourceRef.current.disconnect(); } catch (e) {}
      waveSourceRef.current = null;
    }

    if (!analyserRef.current) {
      analyserRef.current = audioCtxRef.current.createAnalyser();
      analyserRef.current.fftSize = 256;
      analyserRef.current.smoothingTimeConstant = 0.8;
    }

    waveSourceRef.current = audioCtxRef.current.createMediaStreamSource(stream);
    waveSourceRef.current.connect(analyserRef.current);
  }, []);

  // ─ Cleanup — tears down WebRTC, audio nodes, and resets all state ─────────
  const cleanup = useCallback(() => {
    if (pcRef.current) {
      pcRef.current.close();
      pcRef.current = null;
    }
    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach((t) => t.stop());
      localStreamRef.current = null;
    }
    if (remoteAudioRef.current) {
      remoteAudioRef.current.srcObject = null;
    }
    if (waveSourceRef.current) {
      try { waveSourceRef.current.disconnect(); } catch (e) {}
      waveSourceRef.current = null;
    }
    if (audioCtxRef.current?.state !== "closed") {
      audioCtxRef.current?.suspend().catch(() => {});
    }
    analyserRef.current = null;
    setState("idle");
    setSessionMs(0);
    sessionStartRef.current = null;
    setAudioLevel(0);
    pushLog("Peer connection closed. Bye.");
  }, [pushLog]);

  // Ref so async connect() can always call the latest cleanup without stale closure
  const cleanupRef = useRef(cleanup);
  useEffect(() => { cleanupRef.current = cleanup; }, [cleanup]);

  // ─ Connect — real WebRTC: getUserMedia → RTCPeerConnection → offer/answer ─
  const connect = useCallback(async () => {
    if (isConnected || isBusy) return;

    const baseUrl = backendUrl.trim().replace(/\/$/, "");
    const offerUrl = (baseUrl || "") + "/webrtc/offer";

    setState("requesting");
    pushLog("Requesting microphone permission…");

    if (!navigator.mediaDevices?.getUserMedia) {
      setState("idle");
      pushLog("Microphone API not available. Use HTTPS or localhost.");
      return;
    }

    let stream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch (e) {
      setState("idle");
      pushLog(`getUserMedia error: ${e.name} ${e.message || ""}`);
      if (e.name === "NotAllowedError" || e.name === "PermissionDeniedError") {
        pushLog("Allow microphone in the browser prompt, or check site permissions.");
      }
      return;
    }
    localStreamRef.current = stream;
    pushLog(`Microphone ready, tracks: ${stream.getTracks().length}`);

    setState("connecting");
    pushLog(`Negotiating WebRTC → ${offerUrl || "(same origin)/webrtc/offer"}`);

    const isLocal = !baseUrl || /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/i.test(baseUrl);
    const iceServers = isLocal ? [] : [{ urls: ["stun:stun.l.google.com:19302"] }];
    const pc = new RTCPeerConnection({ iceServers });
    pcRef.current = pc;

    pc.onconnectionstatechange = () => {
      pushLog(`pc state: ${pc.connectionState}`);
      if (pc.connectionState === "connected") {
        setState("connected");
        sessionStartRef.current = sessionStartRef.current ?? performance.now();
        pushLog("ICE connected. Pipeline armed: STT → LLM → TTS.");
      } else if (["disconnected", "failed", "closed"].includes(pc.connectionState)) {
        cleanupRef.current();
      }
    };

    pc.oniceconnectionstatechange = () => {
      if (["disconnected", "failed", "closed"].includes(pc.iceConnectionState)) {
        cleanupRef.current();
      }
    };

    pc.ontrack = (ev) => {
      pushLog(`Received remote track: ${ev.track?.kind} id=${ev.track?.id}`);
      if (ev.streams?.[0]) {
        if (remoteAudioRef.current) remoteAudioRef.current.srcObject = ev.streams[0];
        setState("connected");
        sessionStartRef.current = sessionStartRef.current ?? performance.now();
        pushLog("Playing remote audio (TTS).");
        try {
          startWaveformVisualizer(ev.streams[0]);
        } catch (e) {
          pushLog(`Waveform error: ${e}`);
        }
      } else {
        pushLog("Remote track has no stream.");
      }
    };

    stream.getTracks().forEach((track) => pc.addTrack(track, stream));
    pushLog(`Added ${stream.getTracks().length} track(s) to peer connection.`);

    try {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
    } catch (e) {
      pushLog(`Create offer failed: ${e.message || e.name}`);
      cleanupRef.current();
      return;
    }

    // Wait for ICE gathering (5 s timeout)
    if (pc.iceGatheringState !== "complete") {
      await new Promise((resolve) => {
        let done = false;
        const finish = () => {
          if (done) return;
          done = true;
          pc.removeEventListener("icegatheringstatechange", onDone);
          resolve();
        };
        const onDone = () => { if (pc.iceGatheringState === "complete") finish(); };
        pc.addEventListener("icegatheringstatechange", onDone);
        if (pc.iceGatheringState === "complete") finish();
        else setTimeout(finish, 5000);
      });
    }
    pushLog("Offer created (ICE gathered), sending to server…");

    let resp;
    try {
      resp = await fetch(offerUrl, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ offer: JSON.stringify(pc.localDescription) }),
      });
    } catch (e) {
      pushLog(`Network error: ${e.message}`);
      cleanupRef.current();
      return;
    }

    if (!resp.ok) {
      const text = await resp.text();
      pushLog(`Offer failed: ${resp.status} ${text}`);
      cleanupRef.current();
      return;
    }

    const data = await resp.json();
    const payload = data?.data ?? data; // server wraps in {data:{answer}} envelope
    if (!payload.answer) {
      pushLog("No answer in response.");
      cleanupRef.current();
      return;
    }

    const answer = typeof payload.answer === "string" ? JSON.parse(payload.answer) : payload.answer;
    pushLog("Answer received, setting remote description…");
    await pc.setRemoteDescription(new RTCSessionDescription(answer));
    pushLog("Signaling complete. Waiting for ICE and remote track.");
  }, [isConnected, isBusy, backendUrl, pushLog, startWaveformVisualizer]);

  const disconnect = useCallback(() => {
    pushLog("Disconnecting…");
    cleanup();
  }, [cleanup, pushLog]);

  // ─ Session clock ──────────────────────────────────────────────────────────
  useEffect(() => {
    if (!isConnected) return;
    const id = setInterval(() => {
      if (sessionStartRef.current != null) {
        setSessionMs(performance.now() - sessionStartRef.current);
      }
    }, 100);
    return () => clearInterval(id);
  }, [isConnected]);

  // ─ Animated waveform canvas ───────────────────────────────────────────────
  // When analyserRef is populated (real WebRTC audio), RMS drives the envelope.
  // Otherwise falls back to deterministic sine-wave animation per state.
  useEffect(() => {
    const cv = canvasRef.current;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    let raf;
    let phase = 0;
    const resize = () => {
      const dpr = window.devicePixelRatio || 1;
      const r = cv.getBoundingClientRect();
      cv.width = r.width * dpr;
      cv.height = r.height * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener("resize", resize);

    const draw = () => {
      const w = cv.clientWidth, h = cv.clientHeight;
      ctx.clearRect(0, 0, w, h);
      phase += 0.04;

      let envelope = 0.05;
      if (analyserRef.current && (state === "connected" || state === "listening" || state === "speaking")) {
        const bufLen = analyserRef.current.frequencyBinCount;
        const data = new Uint8Array(bufLen);
        analyserRef.current.getByteTimeDomainData(data);
        let sum = 0;
        for (let i = 0; i < bufLen; i++) {
          const v = (data[i] - 128) / 128;
          sum += v * v;
        }
        envelope = Math.max(0.05, Math.min(1, Math.sqrt(sum / bufLen) * 4));
      } else {
        if (state === "listening") envelope = 0.55 + Math.sin(phase * 1.7) * 0.12;
        else if (state === "speaking") envelope = 0.75 + Math.sin(phase * 2.3) * 0.18;
        else if (state === "connected") envelope = 0.18 + Math.sin(phase) * 0.04;
        else if (isBusy) envelope = 0.25 + Math.sin(phase * 3) * 0.1;
      }

      setAudioLevel(envelope);

      // Discrete bars — radio-station meter feel
      const bars = 96;
      const gap = 3;
      const bw = Math.max(1, (w - gap * (bars - 1)) / bars);
      const mid = h / 2;

      for (let i = 0; i < bars; i++) {
        const x = i * (bw + gap);
        const n =
          Math.sin(phase * 1.3 + i * 0.27) * 0.5 +
          Math.sin(phase * 0.7 + i * 0.13) * 0.3 +
          Math.sin(phase * 2.1 + i * 0.41) * 0.2;
        const center = (n + 1) / 2;
        const edge = Math.sin((i / (bars - 1)) * Math.PI);
        const amp = envelope * edge * (0.35 + center * 0.65) * h * 0.55;
        const bh = Math.max(2, amp);
        ctx.fillStyle = state === "speaking" ? accent : theme.text;
        ctx.globalAlpha = state === "idle" ? 0.18 : (state === "speaking" ? 0.95 : 0.7);
        ctx.fillRect(x, mid - bh / 2, bw, bh);
      }
      ctx.globalAlpha = 1;

      ctx.strokeStyle = theme.line;
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(0, mid);
      ctx.lineTo(w, mid);
      ctx.stroke();

      raf = requestAnimationFrame(draw);
    };
    draw();
    return () => { cancelAnimationFrame(raf); window.removeEventListener("resize", resize); };
  }, [state, theme.text, theme.line, accent, isBusy]);

  const stateLabel = ({
    idle: "STANDBY",
    requesting: "REQ MIC",
    connecting: "NEGOTIATING",
    connected: "LIVE · IDLE",
    listening: "LIVE · USER SPEAKING",
    speaking: "LIVE · AGENT SPEAKING",
  })[state];

  const stateDot = isConnected ? accent : (isBusy ? "#F0B83A" : theme.dim2);

  const formatClock = (ms) => {
    if (!ms) return "00:00.00";
    const total = Math.floor(ms);
    const m = String(Math.floor(total / 60000)).padStart(2, "0");
    const s = String(Math.floor((total % 60000) / 1000)).padStart(2, "0");
    const cs = String(Math.floor((total % 1000) / 10)).padStart(2, "0");
    return `${m}:${s}.${cs}`;
  };

  const densityPad = t.density === "compact" ? 18 : t.density === "comfy" ? 36 : 26;

  return (
    <div style={{
      "--accent": accent,
      "--bg": theme.bg,
      "--bg2": theme.bg2,
      "--panel": theme.panel,
      "--line": theme.line,
      "--line2": theme.line2,
      "--text": theme.text,
      "--dim": theme.dim,
      "--dim2": theme.dim2,
      "--chip": theme.chip,
      "--pad": `${densityPad}px`,
      background: theme.bg,
      color: theme.text,
      minHeight: "100vh",
      fontFamily: "'Geist', ui-sans-serif, system-ui, sans-serif",
      letterSpacing: "-0.01em",
    }}>
      <TopBar transport={t.transport} setTweak={setTweak} stateDot={stateDot} stateLabel={stateLabel} sessionClock={formatClock(sessionMs)} />

      <main style={{
        borderTop: `1px solid ${theme.line}`,
        minHeight: "calc(100vh - 56px)",
      }}>
        <LeftStage
          theme={theme}
          accent={accent}
          state={state}
          isConnected={isConnected}
          isBusy={isBusy}
          stateLabel={stateLabel}
          connect={connect}
          disconnect={disconnect}
          canvasRef={canvasRef}
          audioLevel={audioLevel}
          pipelineInfo={pipelineInfo}
        />
      </main>

      {/* Hidden audio element — receives the remote WebRTC track */}
      <audio ref={remoteAudioRef} autoPlay playsInline style={{ display: "none" }} />

      {t.showLog && <LogStrip lines={logLines} theme={theme} onClose={() => setTweak("showLog", false)} />}

      <Tweaks t={t} setTweak={setTweak} />
    </div>
  );
}

// ─── Top bar ─────────────────────────────────────────────────────────────────
function TopBar({ transport, setTweak, stateDot, stateLabel, sessionClock }) {
  return (
    <header style={{
      height: 56,
      display: "grid",
      gridTemplateColumns: "1fr auto 1fr",
      alignItems: "center",
      padding: "0 24px",
      borderBottom: "1px solid var(--line)",
      background: "var(--bg)",
      position: "sticky", top: 0, zIndex: 5,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
        <Wordmark />
        <span style={{ ...mono(10.5), color: "var(--dim)", letterSpacing: "0.16em", whiteSpace: "nowrap" }}>
          v0.4.2 · GO 1.25 · APACHE-2.0
        </span>
      </div>
      <nav style={{ display: "flex", gap: 22, ...mono(11), color: "var(--dim)", textTransform: "uppercase", letterSpacing: "0.12em", whiteSpace: "nowrap" }}>
        <span style={{ color: "var(--text)" }}>Console</span>
        <span>Docs</span>
      </nav>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 12 }}>
        <Segmented
          value={transport}
          options={["WebRTC", "WebSocket"]}
          onChange={(v) => setTweak("transport", v)}
        />
        <div style={{
          ...mono(11), color: "var(--dim)", display: "flex", alignItems: "center", gap: 8,
          padding: "6px 10px", border: "1px solid var(--line)", borderRadius: 999,
        }}>
          <span style={{ width: 7, height: 7, borderRadius: 99, background: stateDot, boxShadow: stateDot !== "rgba(242,241,236,0.38)" ? `0 0 0 3px ${stateDot}22` : "none" }} />
          <span style={{ color: "var(--text)", letterSpacing: "0.06em" }}>{stateLabel}</span>
          <span style={{ color: "var(--dim2)" }}>·</span>
          <span style={{ fontVariantNumeric: "tabular-nums" }}>{sessionClock}</span>
        </div>
      </div>
    </header>
  );
}

function Wordmark() {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
      <svg width="22" height="22" viewBox="0 0 22 22" style={{ display: "block" }}>
        <rect x="0.5" y="0.5" width="21" height="21" rx="4" fill="none" stroke="var(--text)" />
        <rect x="4" y="9" width="2" height="4" fill="var(--text)" />
        <rect x="7" y="6" width="2" height="10" fill="var(--text)" />
        <rect x="10" y="3" width="2" height="16" fill="var(--accent)" />
        <rect x="13" y="6" width="2" height="10" fill="var(--text)" />
        <rect x="16" y="9" width="2" height="4" fill="var(--text)" />
      </svg>
      <span style={{ fontSize: 18, fontWeight: 600, letterSpacing: "-0.02em" }}>Voxray</span>
    </div>
  );
}

// ─── Left stage (hero + mic + waveform) ──────────────────────────────────────
function LeftStage({ theme, accent, state, isConnected, isBusy, stateLabel, connect, disconnect, canvasRef, audioLevel, pipelineInfo }) {
  return (
    <section style={{
      padding: "var(--pad)",
      paddingRight: "calc(var(--pad) + 12px)",
      display: "flex",
      flexDirection: "column",
      gap: 24,
      background: theme.bg,
      position: "relative",
    }}>
      {/* Eyebrow + headline */}
      <div>
        <div style={{ ...mono(11), color: "var(--dim)", letterSpacing: "0.16em", textTransform: "uppercase", marginBottom: 18 }}>
          <span style={{ color: accent }}>●</span>&nbsp;&nbsp;Real-time voice pipeline · STT → LLM → TTS
        </div>
        <h1 style={{
          margin: 0,
          fontSize: "clamp(40px, 5.4vw, 76px)",
          lineHeight: 0.92,
          letterSpacing: "-0.035em",
          fontWeight: 500,
          textWrap: "balance",
        }}>
          Talk to your agent.<br />
          <span style={{ color: "var(--dim)" }}>It listens, thinks,</span>
          <br />
          <span style={{ color: "var(--dim)" }}>and answers — </span>
          <em style={{ fontStyle: "normal", color: accent, fontWeight: 500 }}>under 400&thinsp;ms.</em>
        </h1>
      </div>

      {/* Pipeline visualization */}
      <PipelineStrip state={state} accent={accent} theme={theme} pipelineInfo={pipelineInfo} />

      {/* Waveform stage + mic */}
      <div style={{
        position: "relative",
        border: `1px solid ${theme.line}`,
        borderRadius: 12,
        padding: 22,
        background: theme.panel,
        display: "grid",
        gridTemplateColumns: "auto 1fr",
        alignItems: "center",
        gap: 28,
        minHeight: 220,
      }}>
        <MicButton state={state} isConnected={isConnected} isBusy={isBusy} accent={accent} theme={theme} onClick={isConnected || isBusy ? disconnect : connect} />
        <div style={{ display: "flex", flexDirection: "column", gap: 10, minWidth: 0 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
            <div style={{ ...mono(10.5), color: "var(--dim)", letterSpacing: "0.16em", textTransform: "uppercase" }}>
              Live audio · channel 01
            </div>
            <div style={{ ...mono(11), color: "var(--dim)", letterSpacing: "0.04em" }}>
              {state === "idle" ? "—" : `${Math.round(audioLevel * 100)}% lvl · 48kHz · opus`}
            </div>
          </div>
          <canvas ref={canvasRef} style={{ width: "100%", height: 110, display: "block" }} />
          <div style={{
            display: "flex", justifyContent: "space-between", alignItems: "center",
            ...mono(10.5), color: "var(--dim2)", letterSpacing: "0.06em",
          }}>
            <span>-∞ db</span>
            <span style={{ color: "var(--dim)" }}>
              {state === "idle" && "Press to begin a session"}
              {state === "requesting" && "Awaiting microphone permission…"}
              {state === "connecting" && "Exchanging SDP · gathering ICE candidates"}
              {state === "connected" && "Pipeline armed · waiting for input"}
              {state === "listening" && "Capturing user audio · streaming to STT"}
              {state === "speaking" && "Streaming TTS · agent speaking"}
            </span>
            <span>0 db</span>
          </div>
        </div>
      </div>

      {/* Quick stats row */}
      <div style={{
        display: "grid",
        gridTemplateColumns: "repeat(4, 1fr)",
        borderTop: `1px solid ${theme.line}`,
        marginTop: "auto",
        paddingTop: 18,
      }}>
        <Stat kpi="< 400 ms" label="Round-trip target" theme={theme} />
        <Stat kpi="18" label="Provider plugins" theme={theme} />
        <Stat kpi="2" label="Transports" theme={theme} accent={accent} sub="WS · WebRTC" />
        <Stat kpi="∞" label="Sessions / process" theme={theme} sub="Worker pool" />
      </div>
    </section>
  );
}

function Stat({ kpi, label, sub, theme, accent }) {
  return (
    <div style={{ paddingRight: 18 }}>
      <div style={{ fontSize: 28, letterSpacing: "-0.03em", fontWeight: 500, color: accent || "var(--text)" }}>{kpi}</div>
      <div style={{ ...mono(10.5), color: "var(--dim)", letterSpacing: "0.12em", textTransform: "uppercase", marginTop: 4 }}>{label}</div>
      {sub && <div style={{ ...mono(10.5), color: "var(--dim2)", marginTop: 2 }}>{sub}</div>}
    </div>
  );
}

function PipelineStrip({ state, accent, theme, pipelineInfo }) {
  const p = pipelineInfo || {};
  const stages = [
    { id: "mic",  label: "MIC",  sub: "48kHz · opus", active: state !== "idle" },
    { id: "vad",  label: "VAD",  sub: formatProviderSub(p.vad_type), active: state === "listening" || state === "connected" || state === "speaking" },
    { id: "stt",  label: "STT",  sub: formatProviderSub(p.stt_provider, p.stt_model || p.stt_language), active: state === "listening" },
    { id: "llm",  label: "LLM",  sub: formatProviderSub(p.llm_provider, p.llm_model), active: state === "listening" || state === "speaking" },
    { id: "tts",  label: "TTS",  sub: formatProviderSub(p.tts_provider, p.tts_model || p.tts_voice), active: state === "speaking" },
    { id: "out",  label: "OUT",  sub: transportOutLabel(p.transport), active: state === "speaking" },
  ];
  return (
    <div style={{
      display: "grid",
      gridTemplateColumns: `repeat(${stages.length}, 1fr)`,
      border: `1px solid ${theme.line}`,
      borderRadius: 10,
      background: theme.panel,
      overflow: "hidden",
    }}>
      {stages.map((s, i) => (
        <div key={s.id} style={{
          padding: "12px 14px",
          borderRight: i < stages.length - 1 ? `1px solid ${theme.line}` : "none",
          position: "relative",
          display: "flex", flexDirection: "column", gap: 4,
        }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{
              width: 6, height: 6, borderRadius: 99,
              background: s.active ? accent : "var(--dim2)",
              boxShadow: s.active ? `0 0 0 4px ${accent}22` : "none",
              transition: "background .2s, box-shadow .2s",
            }} />
            <span style={{ ...mono(10.5), letterSpacing: "0.14em", color: s.active ? "var(--text)" : "var(--dim)" }}>{s.label}</span>
            {i < stages.length - 1 && (
              <span style={{ ...mono(10.5), color: "var(--dim2)", marginLeft: "auto" }}>→</span>
            )}
          </div>
          <div style={{ ...mono(10.5), color: "var(--dim2)" }}>{s.sub}</div>
        </div>
      ))}
    </div>
  );
}

// ─── Mic button ──────────────────────────────────────────────────────────────
function MicButton({ state, isConnected, isBusy, accent, theme, onClick }) {
  const active = isConnected || isBusy;
  return (
    <button
      onClick={onClick}
      style={{
        all: "unset",
        cursor: "pointer",
        width: 130, height: 130, borderRadius: "50%",
        background: active ? accent : "transparent",
        border: `1px solid ${active ? accent : theme.line2}`,
        boxShadow: active ? `0 0 0 8px ${accent}22, 0 0 0 16px ${accent}10` : "none",
        display: "flex", alignItems: "center", justifyContent: "center",
        transition: "all .25s ease",
        position: "relative",
      }}
      aria-pressed={active}
    >
      {active && (
        <>
          <span className="ring r1" style={{ borderColor: accent }} />
          <span className="ring r2" style={{ borderColor: accent }} />
        </>
      )}
      <svg viewBox="0 0 32 32" width="38" height="38">
        {state === "idle" || state === "requesting" ? (
          <g fill="none" stroke={state === "requesting" ? accent : theme.text} strokeWidth="1.75" strokeLinecap="round">
            <rect x="12.25" y="5.25" width="7.5" height="14.5" rx="3.75" />
            <path d="M8 15.5a8 8 0 0 0 16 0" />
            <path d="M16 23.5v3.5" />
            <path d="M11.5 27h9" />
          </g>
        ) : state === "speaking" ? (
          <g fill={theme.bg} stroke={theme.bg} strokeWidth="2" strokeLinecap="round">
            <line x1="8" y1="16" x2="8" y2="16.01" />
            <line x1="12" y1="11" x2="12" y2="21" />
            <line x1="16" y1="7" x2="16" y2="25" />
            <line x1="20" y1="11" x2="20" y2="21" />
            <line x1="24" y1="16" x2="24" y2="16.01" />
          </g>
        ) : (
          <g fill={active ? theme.bg : theme.text} stroke="none">
            <rect x="10" y="10" width="12" height="12" rx="1.5" />
          </g>
        )}
      </svg>

      <style>{`
        .ring{position:absolute;inset:-1px;border-radius:50%;border:1px solid;opacity:.4;animation:r 2.2s ease-out infinite}
        .ring.r2{animation-delay:1.1s}
        @keyframes r{0%{transform:scale(1);opacity:.45}100%{transform:scale(1.55);opacity:0}}
      `}</style>
    </button>
  );
}

// ─── Log strip (toggleable) ──────────────────────────────────────────────────
function LogStrip({ lines, theme, onClose }) {
  return (
    <div style={{
      position: "fixed", left: 0, right: 0, bottom: 0,
      borderTop: `1px solid ${theme.line2}`,
      background: theme.bg2,
      padding: "10px 24px",
      maxHeight: 200,
      overflowY: "auto",
      zIndex: 4,
      ...mono(11.5),
      color: "var(--dim)",
    }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
        <span style={{ letterSpacing: "0.16em", color: "var(--text)" }}>RUNNER LOG</span>
        <button onClick={onClose} style={{ all: "unset", cursor: "pointer", color: "var(--dim)" }}>close ✕</button>
      </div>
      {lines.map((l, i) => <div key={i} style={{ whiteSpace: "pre-wrap" }}>{l}</div>)}
    </div>
  );
}

// ─── Reusable bits ───────────────────────────────────────────────────────────
function Segmented({ value, options, onChange }) {
  return (
    <div style={{
      display: "inline-flex",
      padding: 3,
      border: "1px solid var(--line)",
      borderRadius: 999,
      background: "var(--chip)",
    }}>
      {options.map((o) => {
        const sel = o === value;
        return (
          <button
            key={o}
            onClick={() => onChange(o)}
            style={{
              all: "unset", cursor: "pointer",
              padding: "5px 12px", borderRadius: 999,
              ...mono(11), letterSpacing: "0.08em",
              color: sel ? "var(--bg)" : "var(--dim)",
              background: sel ? "var(--text)" : "transparent",
              transition: "all .15s",
            }}
          >
            {o}
          </button>
        );
      })}
    </div>
  );
}

function mono(size) {
  return {
    fontFamily: "'JetBrains Mono', ui-monospace, monospace",
    fontSize: size,
  };
}

// ─── Tweaks ──────────────────────────────────────────────────────────────────
function Tweaks({ t, setTweak }) {
  return (
    <TweaksPanel>
      <TweakSection label="Theme" />
      <TweakRadio
        label="Surface"
        value={t.theme}
        options={["ink", "paper"]}
        onChange={(v) => setTweak("theme", v)}
      />
      <TweakColor
        label="Accent"
        value={t.accent}
        options={["#C5F462", "#FF6B3D", "#7AA7FF", "#E8E1D0", "#FF3366"]}
        onChange={(v) => setTweak("accent", v)}
      />
      <TweakRadio
        label="Density"
        value={t.density}
        options={["compact", "regular", "comfy"]}
        onChange={(v) => setTweak("density", v)}
      />

      <TweakSection label="Display" />
      <TweakToggle
        label="Show runner log"
        value={t.showLog}
        onChange={(v) => setTweak("showLog", v)}
      />
    </TweaksPanel>
  );
}

// Mount
const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(<App />);
