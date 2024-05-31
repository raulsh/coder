package dispatch

import (
	"context"
	"fmt"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications/types"
)

type SMTPDispatcher struct{}

func (s *SMTPDispatcher) Name() string {
	// TODO: don't use database types
	return string(database.NotificationReceiverSmtp)
}

func (s *SMTPDispatcher) Validate(input types.Labels) bool {
	return input.Contains("to", "from", "subject", "body")
}

func (s *SMTPDispatcher) Send(ctx context.Context, input types.Labels) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fmt.Printf("would've sent %v\n", input)
	return nil
}
