package notifications

import (
	"context"

	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/google/uuid"
)

type NoopManager struct{}

// NewNoopManager builds a NoopManager which is used to fulfil the contract for enqueuing notifications, if ExperimentNotifications is not set.
func NewNoopManager() *NoopManager {
	return &NoopManager{}
}

func (n *NoopManager) Enqueue(context.Context, uuid.UUID, uuid.UUID, types.Labels, string, ...uuid.UUID) (*uuid.UUID, error) {
	return nil, nil
}
