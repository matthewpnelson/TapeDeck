package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/Chuntttttt/tapedeck/internal/middleware"
)

// WebhookHandler handles incoming webhook requests from NFC readers
type WebhookHandler struct {
	pairingHandler *PairingHandler
	webhookSecret  string // Optional secret for validating webhook requests
}

// WebhookRequest represents the expected webhook payload
type WebhookRequest struct {
	TagID string `json:"tag_id"`
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(pairingHandler *PairingHandler, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{
		pairingHandler: pairingHandler,
		webhookSecret:  webhookSecret,
	}
}

// HandleNFCWebhook handles POST /api/webhook/nfc
func (h *WebhookHandler) HandleNFCWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqLogger := middleware.GetLogger(ctx)

	// Validate webhook secret if configured
	if h.webhookSecret != "" {
		providedSecret := r.Header.Get("X-Webhook-Secret")
		if providedSecret == "" {
			reqLogger.Warn("Webhook request missing secret header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(providedSecret), []byte(h.webhookSecret)) != 1 {
			reqLogger.Warn("Webhook request has invalid secret")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Parse request body
	var req WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reqLogger.Warn("Failed to parse webhook request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate tag_id
	if req.TagID == "" {
		reqLogger.Warn("Webhook request missing tag_id")
		http.Error(w, "Missing tag_id field", http.StatusBadRequest)
		return
	}

	reqLogger.Info("Received NFC webhook", "tag_id", req.TagID)

	// Trigger tag scanned handler (same flow as WebSocket events)
	h.pairingHandler.HandleTagScanned(ctx, req.TagID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"tag_id": req.TagID,
	}); err != nil {
		reqLogger.Warn("Failed to encode response", "error", err)
	}
}
