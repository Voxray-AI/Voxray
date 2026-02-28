// Package frames defines VAD/turn-related control frames.
package frames

// VADParamsUpdateFrame updates VAD/turn parameters (e.g. stop_secs for IVR mode).
// Processors such as TurnProcessor apply these to their Analyzer when received.
type VADParamsUpdateFrame struct {
	ControlFrame
	// StopSecs is silence duration in seconds to end turn (e.g. 2.0 for IVR).
	StopSecs float64 `json:"stop_secs"`
	// StartSecs is VAD start trigger time in seconds (pre-speech padding).
	StartSecs float64 `json:"start_secs,omitempty"`
}

func (*VADParamsUpdateFrame) FrameType() string { return "VADParamsUpdateFrame" }

// NewVADParamsUpdateFrame creates a VADParamsUpdateFrame.
func NewVADParamsUpdateFrame(stopSecs, startSecs float64) *VADParamsUpdateFrame {
	return &VADParamsUpdateFrame{
		ControlFrame: ControlFrame{Base: NewBase()},
		StopSecs:     stopSecs,
		StartSecs:    startSecs,
	}
}
