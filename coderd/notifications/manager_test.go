package notifications_test

import (
	"context"
	"testing"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/coderdtest"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbmem"
	"github.com/coder/coder/v2/coderd/database/pubsub"
	"github.com/coder/coder/v2/coderd/notifications"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
)

// TestSingletonRegistration tests that a Manager which has been instantiated but not registered will error.
func TestSingletonRegistration(t *testing.T) {
	ctx := context.Background()
	logger := slogtest.Make(t, &slogtest.Options{IgnoreErrors: true, IgnoredErrorIs: []error{}}).Leveled(slog.LevelDebug)

	mgr := notifications.NewManager(codersdk.NotificationsConfig{}, dbmem.New(), logger, nil, nil)
	t.Cleanup(func() {
		require.NoError(t, mgr.Stop(ctx))
	})

	// Not registered yet.
	_, err := notifications.Enqueue(ctx, uuid.New(), notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, nil, "")
	require.ErrorIs(t, err, notifications.SingletonNotRegisteredErr)

	// Works after registering.
	notifications.RegisterInstance(mgr)
	_, err = notifications.Enqueue(ctx, uuid.New(), notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, nil, "")
	require.NoError(t, err)
}

func TestBufferedUpdates(t *testing.T) {
	// setup
	ctx := context.Background()
	logger := slogtest.Make(t, &slogtest.Options{IgnoreErrors: true, IgnoredErrorIs: []error{}}).Leveled(slog.LevelDebug)

	db := dbmem.New()
	interceptor := &bulkUpdateInterceptor{Store: db}

	santa := &santaDispatcher{}
	dispatchers, err := notifications.NewProviderRegistry[notifications.Dispatcher](santa)
	require.NoError(t, err)
	mgr := notifications.NewManager(codersdk.NotificationsConfig{}, interceptor, logger.Named("notifications"), nil, dispatchers)
	notifications.RegisterInstance(mgr)

	client := coderdtest.New(t, &coderdtest.Options{Database: db, Pubsub: pubsub.NewInMemory()})
	user := coderdtest.CreateFirstUser(t, client)

	// given
	if _, err := notifications.Enqueue(ctx, user.UserID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, types.Labels{"nice": "true"}, ""); true {
		require.NoError(t, err)
	}
	if _, err := notifications.Enqueue(ctx, user.UserID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, types.Labels{"nice": "false"}, ""); true {
		require.NoError(t, err)
	}

	// when
	mgr.StartNotifiers(ctx, 1)

	// then

	// Wait for messages to be dispatched.
	require.Eventually(t, func() bool { return len(santa.naughty) == 1 && len(santa.nice) == 1 }, testutil.WaitMedium, testutil.IntervalFast)

	// Stop the manager which forces an update of buffered updates.
	require.NoError(t, mgr.Stop(ctx))

	// Wait until both success & failure updates have been sent to the store.
	require.Eventually(t, func() bool { return len(interceptor.failed) == 1 && len(interceptor.sent) == 1 }, testutil.WaitMedium, testutil.IntervalFast)
}

type bulkUpdateInterceptor struct {
	notifications.Store

	sent   []uuid.UUID
	failed []uuid.UUID
}

func (b *bulkUpdateInterceptor) BulkMarkNotificationMessagesSent(ctx context.Context, arg database.BulkMarkNotificationMessagesSentParams) (int64, error) {
	b.sent = append(b.sent, arg.IDs...)
	return 1, nil
}

func (b *bulkUpdateInterceptor) BulkMarkNotificationMessagesFailed(ctx context.Context, arg database.BulkMarkNotificationMessagesFailedParams) (int64, error) {
	b.failed = append(b.failed, arg.IDs...)
	return 1, nil
}

// santaDispatcher only dispatches nice messages.
type santaDispatcher struct {
	naughty []uuid.UUID
	nice    []uuid.UUID
}

func (s *santaDispatcher) Name() string {
	return string(database.NotificationMethodSmtp)
}

func (s *santaDispatcher) Validate(input types.Labels) (bool, []string) {
	missing := input.Missing("nice")
	return len(missing) == 0, missing
}

func (s *santaDispatcher) Send(ctx context.Context, msgID uuid.UUID, input types.Labels) (bool, error) {
	if input.Get("nice") != "true" {
		s.naughty = append(s.naughty, msgID)
		return false, xerrors.New("be nice")
	}

	s.nice = append(s.nice, msgID)
	return false, nil
}
