// Package whatsapp_qr provides a process-level store for sharing WhatsApp
// QR codes between the whatsapp_native channel and the web dashboard API.
//
// Because the gateway and launcher run as separate processes, the QR state
// is persisted to a JSON file so both processes can access it.
package whatsapp_qr

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	QRCode    string    `json:"qr_code,omitempty"`     // raw QR string
	QRDataURI string    `json:"qr_data_uri,omitempty"` // base64 PNG data URI
	Phone     string    `json:"phone,omitempty"`       // connected phone number
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	mu       sync.RWMutex
	stateDir string // set via SetDir
)

const stateFile = "whatsapp_qr_state.json"

// SetDir sets the directory where the QR state file is stored.
// Must be called before any Set/Get operations.
func SetDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	stateDir = dir
	_ = os.MkdirAll(dir, 0o755)
}

func statePath() string {
	if stateDir == "" {
		home := os.Getenv("PICOCLAW_HOME")
		if home == "" {
			home, _ = os.UserHomeDir()
			home = filepath.Join(home, ".picoclaw")
		}
		stateDir = home
	}
	return filepath.Join(stateDir, stateFile)
}

func writeState(s State) {
	s.UpdatedAt = time.Now()
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(statePath(), data, 0o644)
}

// SetQRCode updates the QR code and data URI.
func SetQRCode(code, dataURI string) {
	mu.Lock()
	defer mu.Unlock()
	writeState(State{
		Status:    StatusWaiting,
		QRCode:    code,
		QRDataURI: dataURI,
	})
}

// SetConnected marks the login as successful.
func SetConnected(phone string) {
	mu.Lock()
	defer mu.Unlock()
	writeState(State{
		Status: StatusConnected,
		Phone:  phone,
	})
}

// SetExpired marks the QR code as expired.
func SetExpired() {
	mu.Lock()
	defer mu.Unlock()
	writeState(State{Status: StatusExpired})
}

// SetError records an error state.
func SetError(msg string) {
	mu.Lock()
	defer mu.Unlock()
	writeState(State{Status: StatusError, Error: msg})
}

// Get returns a snapshot of the current state by reading the file.
func Get() State {
	mu.RLock()
	defer mu.RUnlock()

	data, err := os.ReadFile(statePath())
	if err != nil {
		return State{Status: StatusWaiting, UpdatedAt: time.Now()}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{Status: StatusWaiting, UpdatedAt: time.Now()}
	}
	return s
}

// Reset clears the state back to waiting.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	writeState(State{Status: StatusWaiting})
}
