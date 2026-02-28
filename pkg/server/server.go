// Package server provides transport servers (e.g. WebSocket, SmallWebRTC) for Voila.
package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/swaggo/http-swagger"
	_ "voila-go/docs" // register generated Swagger spec
	"voila-go/pkg/config"
	"voila-go/pkg/logger"
	"voila-go/pkg/transport"
	"voila-go/pkg/transport/smallwebrtc"
	ws "voila-go/pkg/transport/websocket"
)

// webrtcOfferResponse is the JSON response for POST /webrtc/offer.
type webrtcOfferResponse struct {
	Answer string `json:"answer"`
}

// WebrtcOfferDoc documents the WebRTC offer endpoint for Swagger.
//
// @Summary Submit WebRTC offer
// @Description Accepts a WebRTC SDP offer and returns an SDP answer. Available when transport is smallwebrtc or both.
// @Tags webrtc
// @Accept json
// @Produce json
// @Param body body object true "JSON body with 'offer' (SDP offer string)"
// @Success 200 {object} webrtcOfferResponse
// @Failure 400 {string} string "Invalid offer or handling failed"
// @Failure 405 {string} string "Method not allowed"
// @Router /webrtc/offer [post]
func WebrtcOfferDoc() {}

// StartServers starts the HTTP server that hosts the WebSocket endpoint (/ws)
// and, optionally, the SmallWebRTC signaling endpoint (/webrtc/offer).
// The onTransport callback is invoked for every new transport (WebSocket or WebRTC)
// so the caller can attach it to a pipeline runner.
func StartServers(ctx context.Context, cfg *config.Config, onTransport func(ctx context.Context, tr transport.Transport)) error {
	if cfg == nil {
		return nil
	}

	mode := cfg.Transport
	if mode == "" {
		mode = "websocket"
	}
	enableWebRTC := mode == "smallwebrtc" || mode == "both"

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	server := &ws.Server{
		Host:           cfg.Host,
		Port:           port,
		SessionTimeout: ws.DefaultSessionTimeout,
		OnConn: func(c context.Context, tr *ws.ConnTransport) {
			if onTransport != nil {
				onTransport(c, tr)
			}
		},
		RegisterHandlers: func(mux *http.ServeMux) {
			mux.Handle("/swagger/", httpSwagger.Handler(
				httpSwagger.URL("/swagger/doc.json"),
				httpSwagger.DeepLinking(true),
			))
			if !enableWebRTC {
				return
			}
			mux.HandleFunc("/webrtc/offer", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				defer r.Body.Close()

				var req struct {
					Offer string `json:"offer"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Offer == "" {
					http.Error(w, "invalid offer payload", http.StatusBadRequest)
					return
				}

				tr := smallwebrtc.NewTransport(&smallwebrtc.Config{
					ICEServers: cfg.WebRTCICEServers,
				})
				answer, err := tr.HandleOffer(req.Offer)
				if err != nil {
					logger.Error("smallwebrtc handle offer: %v", err)
					http.Error(w, "failed to handle offer", http.StatusBadRequest)
					return
				}
				if err := tr.Start(ctx); err != nil {
					logger.Error("smallwebrtc start: %v", err)
					http.Error(w, "failed to start transport", http.StatusInternalServerError)
					return
				}

				if onTransport != nil {
					go onTransport(ctx, tr)
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(struct {
					Answer string `json:"answer"`
				}{
					Answer: answer,
				}); err != nil {
					logger.Error("encode webrtc answer: %v", err)
				}
			})
		},
	}

	logger.Info("starting server on %s:%d (transport=%s)", cfg.Host, port, mode)
	return server.ListenAndServe(ctx)
}
