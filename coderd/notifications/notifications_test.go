package notifications_test

import (
	"context"
	"database/sql"
	"testing"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtestutil"
	"github.com/coder/coder/v2/coderd/notifications"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/coder/coder/v2/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestBasicNotificationRoundtrip enqueues a message to the store, waits for it to be acquired by a notifier,
// and passes it off to a fake dispatcher.
func TestBasicNotificationRoundtrip(t *testing.T) {
	// setup
	if !dbtestutil.WillUsePostgres() {
		t.Skip("This test requires postgres")
	}
	connectionURL, closeFunc, err := dbtestutil.Open()
	require.NoError(t, err)
	t.Cleanup(closeFunc)

	sqlDB, err := sql.Open("postgres", connectionURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitSuperLong)
	t.Cleanup(cancel)
	db := database.New(sqlDB)

	logger := slogtest.Make(t, &slogtest.Options{IgnoreErrors: true, IgnoredErrorIs: []error{}}).Leveled(slog.LevelDebug)

	// given
	dispatcher := &fakeDispatcher{}
	fakeDispatchers, err := notifications.NewProviderRegistry[notifications.Dispatcher](dispatcher)
	require.NoError(t, err)

	manager := notifications.NewManager(db, logger, notifications.DefaultRenderers(), fakeDispatchers)
	manager.StartNotifiers(ctx, 1)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(ctx))
	})

	// when
	id, err := manager.Enqueue(ctx, notifications.TemplateWorkspaceDeleted, database.NotificationReceiverSmtp, types.Labels{}, "test")
	require.NoError(t, err)

	// then
	require.Eventually(t, func() bool { return dispatcher.sent == id }, testutil.WaitLong, testutil.IntervalMedium)
}

type fakeDispatcher struct {
	sent uuid.UUID
}

func (f *fakeDispatcher) Name() string {
	return string(database.NotificationReceiverSmtp)
}

func (f *fakeDispatcher) Validate(_ types.Labels) (bool, []string) {
	return true, nil
}

func (f *fakeDispatcher) Send(_ context.Context, msgID uuid.UUID, _ types.Labels) error {
	f.sent = msgID
	return nil
}
