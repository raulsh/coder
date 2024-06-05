package notifications

import (
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/google/uuid"
)

// EnqueueNotifyMessage is the message transmitted between parts of the system to the notification Manager for enqueuing
// of messages into the store.
type EnqueueNotifyMessage struct {
	UserID    uuid.UUID
	Template  uuid.UUID
	Input     types.Labels
	CreatedBy string
	TargetIDs []uuid.UUID
}
