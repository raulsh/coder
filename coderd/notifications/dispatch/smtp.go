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

func (s *SMTPDispatcher) Validate(input types.Labels) (bool, []string) {
	missing := input.Missing("to", "from", "subject", "body")
	return len(missing) == 0, missing
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
