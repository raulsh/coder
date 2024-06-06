package system

import (
	"context"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/google/uuid"
)

// EnqueueWorkspaceDeleted notifies the given user that their workspace was deleted.
func EnqueueWorkspaceDeleted(ctx context.Context, userID uuid.UUID, name, reason, createdBy string, targets ...uuid.UUID) {
	// TODO: determine which notification method to use, don't hard-code
	_, _ = notifications.Enqueue(ctx, userID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp,
		types.Labels{
			"name":   name,
			"reason": reason,
		}, createdBy, targets...)
}
