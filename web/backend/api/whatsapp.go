package api

import (
	"encoding/json"
	"net/http"

	whatsappqr "github.com/sipeed/picoclaw/pkg/channels/whatsapp_qr"
)

// registerWhatsAppRoutes binds WhatsApp QR login endpoints to the mux.
func (h *Handler) registerWhatsAppRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/whatsapp/qr", h.handleWhatsAppQRStatus)
	mux.HandleFunc("POST /api/whatsapp/pause", h.handleWhatsAppPause)
}

// handleWhatsAppQRStatus returns the current WhatsApp QR login state.
//
//	GET /api/whatsapp/qr
func (h *Handler) handleWhatsAppQRStatus(w http.ResponseWriter, r *http.Request) {
	state := whatsappqr.Get()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// handleWhatsAppPause pauses or resumes the WhatsApp bot.
//
//	POST /api/whatsapp/pause  { "paused": true }
func (h *Handler) handleWhatsAppPause(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	// Write pause state to file so gateway can read it
	if req.Paused {
		whatsappqr.SetError("Bot is paused by admin")
	} else {
		whatsappqr.SetConnected("")
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "paused": req.Paused})
}
