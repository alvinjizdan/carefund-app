package domain

import "context"

// WebhookNotification represents an agnostic payment notification from a provider.
type WebhookNotification struct {
	Provider        string
	EventSource     string
	ProviderEventID string
	OrderID         string
	TransactionID   string
	GrossAmount     int64
	ProviderStatus  string // The raw status from provider (e.g. "capture", "settlement")
	FraudStatus     string // E.g. "accept", "challenge"
	RawPayload      string
	IdempotencyKey  string
}

// WebhookService processes incoming provider notifications.
type WebhookService interface {
	ProcessNotification(ctx context.Context, notif *WebhookNotification) error
}
