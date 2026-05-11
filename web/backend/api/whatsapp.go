package api

import (
	"encoding/json"
	"net/http"

	whatsappqr "github.com/sipeed/picoclaw/pkg/channels/whatsapp_qr"
)

// registerWhatsAppRoutes binds WhatsApp QR login endpoints to the mux.
func (h *Handler) registerWhatsAppRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/whatsapp/qr", h.handleWhatsAppQRStatus)
}

// handleWhatsAppQRStatus returns the current WhatsApp QR login state.
//
//	GET /api/whatsapp/qr
func (h *Handler) handleWhatsAppQRStatus(w http.ResponseWriter, r *http.Request) {
	state := whatsappqr.Get()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}
