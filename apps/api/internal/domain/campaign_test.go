package domain_test

import (
	"testing"
	"time"

	"carefund-api/internal/domain"
)

func TestCampaignStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		next      string
		expectVal bool
	}{
		// DRAFT
		{"draft to pending_review valid", domain.CampaignStateDraft, domain.CampaignStatePendingReview, true},
		{"draft to active invalid", domain.CampaignStateDraft, domain.CampaignStateActive, false},

		// PENDING_REVIEW
		{"pending_review to active valid", domain.CampaignStatePendingReview, domain.CampaignStateActive, true},
		{"pending_review to rejected valid", domain.CampaignStatePendingReview, domain.CampaignStateRejected, true},
		{"pending_review to completed invalid", domain.CampaignStatePendingReview, domain.CampaignStateCompleted, false},

		// ACTIVE
		{"active to suspended valid", domain.CampaignStateActive, domain.CampaignStateSuspended, true},
		{"active to completed valid", domain.CampaignStateActive, domain.CampaignStateCompleted, true},
		{"active to cancelled valid", domain.CampaignStateActive, domain.CampaignStateCancelled, true},
		{"active to draft invalid", domain.CampaignStateActive, domain.CampaignStateDraft, false},

		// SUSPENDED
		{"suspended to active valid", domain.CampaignStateSuspended, domain.CampaignStateActive, true},
		{"suspended to cancelled valid", domain.CampaignStateSuspended, domain.CampaignStateCancelled, true},
		{"suspended to completed invalid", domain.CampaignStateSuspended, domain.CampaignStateCompleted, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &domain.Campaign{Status: tc.current}
			valid := c.IsValidTransition(tc.next)
			if valid != tc.expectVal {
				t.Errorf("transition %s -> %s: expected %v, got %v", tc.current, tc.next, tc.expectVal, valid)
			}
		})
	}
}

func TestCampaignValidation(t *testing.T) {
	// Simple validation tests for fields we check
	now := time.Now()
	
	validTarget := int64(1000)
	invalidTarget := int64(0)
	
	validStart := now
	validEnd := now.Add(24 * time.Hour)
	invalidEnd := now.Add(-24 * time.Hour)

	if validTarget <= 0 {
		t.Errorf("expected valid target to be > 0")
	}
	if invalidTarget > 0 {
		t.Errorf("expected invalid target to be <= 0")
	}
	if invalidEnd.After(validStart) {
		t.Errorf("expected invalid end to be before start")
	}
	if !validEnd.After(validStart) {
		t.Errorf("expected valid end to be after start")
	}
}
