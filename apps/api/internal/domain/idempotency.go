package domain

import (
	"context"
	"time"
)

// IdempotencyStatus represents the state of an idempotency record.
// PENDING:   Key reserved inside the financial DB transaction; Midtrans not yet called.
//            Response fields (code, body) are NULL.
// COMPLETED: Midtrans responded successfully; response_code and response_body are populated.
// FAILED:    Midtrans returned a definitive (non-ambiguous) rejection.
const (
	IdempotencyStatusPending   = "PENDING"
	IdempotencyStatusCompleted = "COMPLETED"
	IdempotencyStatusFailed    = "FAILED"
)

type IdempotencyRecord struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	RequestHash    string    `json:"request_hash"`
	Status         string    `json:"status"`
	ResponseCode   *int      `json:"response_code,omitempty"`
	ResponseBody   []byte    `json:"response_body,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// ErrIdempotencyConflict is returned when two concurrent requests attempt the
// same (user_id, idempotency_key) and the current request loses the race.
// The caller must look up the existing record and return its result.
var ErrIdempotencyConflict = newSentinel("idempotency_conflict")

type IdempotencyRepository interface {
	// Get returns an existing record or ErrNotFound.
	// It does NOT filter by expires_at, allowing safe handling of expired operations.
	Get(ctx context.Context, userID, idempotencyKey string) (*IdempotencyRecord, error)

	// Reserve atomically inserts a PENDING reservation row.
	// Includes orderID to create a durable link to the financial operation for recovery.
	Reserve(ctx context.Context, userID, idempotencyKey, requestHash, orderID string, expiresAt time.Time) error

	// Complete marks the reservation COMPLETED and stores the cached response.
	Complete(ctx context.Context, userID, idempotencyKey string, responseCode int, responseBody []byte) error

	// Fail marks the reservation FAILED (definitive provider rejection).
	Fail(ctx context.Context, userID, idempotencyKey string) error

	// RecoverCompletedByOrderID allows a background worker or webhook to durably recover a PENDING idempotency record.
	RecoverCompletedByOrderID(ctx context.Context, orderID string, responseCode int, responseBody []byte) error

	// RecoverFailedByOrderID allows a background worker or webhook to transition a PENDING record to FAILED.
	RecoverFailedByOrderID(ctx context.Context, orderID string) error
}


// sentinelError is a simple unexported error type for domain sentinel values.
type sentinelError struct{ s string }

func (e sentinelError) Error() string { return e.s }
func newSentinel(s string) error      { return sentinelError{s: s} }
