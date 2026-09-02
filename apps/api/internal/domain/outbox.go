package domain

import (
	"context"
	"encoding/json"
	"time"
)

const (
	OutboxStatusPending    = "PENDING"
	OutboxStatusProcessing = "PROCESSING"
	OutboxStatusProcessed  = "PROCESSED"
	OutboxStatusFailed     = "FAILED"
	OutboxStatusDeadLetter = "DEAD_LETTER"

	MaxOutboxRetryCount = 10

	OutboxEventSettlementApproved = "SETTLEMENT_APPROVED"
	OutboxAggregateSettlement     = "SETTLEMENT"
)

type OutboxEvent struct {
	ID                  string
	IdempotencyKey      string
	AggregateType       string
	AggregateID         string
	EventType           string
	Payload             json.RawMessage
	Status              string
	RetryCount          int
	AvailableAt         time.Time
	ProcessingStartedAt *time.Time
	ProcessedAt         *time.Time
	CreatedAt           time.Time
}

type OutboxEventRepository interface {
	Create(ctx context.Context, event *OutboxEvent) error
	ClaimNext(ctx context.Context) (*OutboxEvent, error)
	MarkProcessed(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, nextAvailableAt time.Time) error
	MarkDeadLetter(ctx context.Context, id string, reason string) error
	ReplayDeadLetter(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*OutboxEvent, error)
	ReclaimExpiredLeases(ctx context.Context, ttl time.Duration) (int64, error)
}
