// Package whatsapp_qr provides a process-level store for sharing WhatsApp
// QR codes between the whatsapp_native channel and the web dashboard API.
// This package has no build constraints so it can be imported by both
// the gateway (which may or may not include whatsapp_native) and the
// web backend API without creating an import cycle.
package whatsapp_qr

import (
	"sync"
	"time"
)

// Status values for the WhatsApp login flow.
const (
	StatusWaiting   = "waiting"   // QR code ready, waiting for scan
	StatusScanned   = "scanned"   // QR scanned, waiting for confirmation
	StatusConnected = "connected" // Login successful
	StatusExpired   = "expired"   // QR code expired
	StatusError     = "error"     // Error occurred
)

// State holds the current WhatsApp QR login state.
type State struct {
	Status    string    `json:"status"`
	QRCode    string    `json:"qr_code,omitempty"`    // raw QR string
	QRDataURI string    `json:"qr_data_uri,omitempty"` // base64 PNG data URI
	Phone     string    `json:"phone,omitempty"`       // connected phone number
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	mu    sync.RWMutex
	state = State{Status: StatusWaiting}
)

// SetQRCode updates the QR code and data URI.
func SetQRCode(code, dataURI string) {
	mu.Lock()
	defer mu.Unlock()
	state = State{
		Status:    StatusWaiting,
		QRCode:    code,
		QRDataURI: dataURI,
		UpdatedAt: time.Now(),
	}
}

// SetConnected marks the login as successful.
func SetConnected(phone string) {
	mu.Lock()
	defer mu.Unlock()
	state = State{
		Status:    StatusConnected,
		Phone:     phone,
		UpdatedAt: time.Now(),
	}
}

// SetExpired marks the QR code as expired.
func SetExpired() {
	mu.Lock()
	defer mu.Unlock()
	state = State{
		Status:    StatusExpired,
		UpdatedAt: time.Now(),
	}
}

// SetError records an error state.
func SetError(msg string) {
	mu.Lock()
	defer mu.Unlock()
	state = State{
		Status:    StatusError,
		Error:     msg,
		UpdatedAt: time.Now(),
	}
}

// Get returns a snapshot of the current state.
func Get() State {
	mu.RLock()
	defer mu.RUnlock()
	return state
}

// Reset clears the state back to waiting.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	state = State{Status: StatusWaiting, UpdatedAt: time.Now()}
}
