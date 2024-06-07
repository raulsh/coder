package notifications_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/v2/coderd/coderdtest"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtestutil"
	"github.com/coder/coder/v2/coderd/database/pubsub"
	"github.com/coder/coder/v2/coderd/notifications"
	"github.com/coder/coder/v2/coderd/notifications/dispatch"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/testutil"
	"github.com/coder/serpent"
	"github.com/google/uuid"
	smtpmock "github.com/mocktools/go-smtp-mock/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestBasicNotificationRoundtrip enqueues a message to the store, waits for it to be acquired by a notifier,
// and passes it off to a fake dispatcher.
// TODO: split this test up into table tests or separate tests.
// TODO: implement retries, validate final statuses
func TestBasicNotificationRoundtrip(t *testing.T) {
	t.Parallel()

	// setup
	if !dbtestutil.WillUsePostgres() {
		t.Skip("This test requires postgres")
	}
	ctx, logger, db, ps := setup(t)

	// given
	dispatcher := &fakeDispatcher{}
	fakeDispatchers, err := notifications.NewHandlerRegistry(dispatcher)
	require.NoError(t, err)

	cfg := codersdk.NotificationsConfig{}
	manager := notifications.NewManager(cfg, db, logger, fakeDispatchers)
	notifications.RegisterInstance(manager)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(ctx))
	})

	client := coderdtest.New(t, &coderdtest.Options{Database: db, Pubsub: ps})
	user := coderdtest.CreateFirstUser(t, client)

	// when
	sid, err := manager.Enqueue(ctx, user.UserID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, types.Labels{"type": "success"}, "test")
	require.NoError(t, err)
	fid, err := manager.Enqueue(ctx, user.UserID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, types.Labels{"type": "failure"}, "test")
	require.NoError(t, err)

	// TODO: can be reordered once tick interval is configurable so we don't have to wait long if no messages are enqueued yet.
	manager.StartNotifiers(ctx, 1)

	// then
	require.Eventually(t, func() bool { return dispatcher.succeeded == sid.String() }, testutil.WaitLong, testutil.IntervalMedium)
	require.Eventually(t, func() bool { return dispatcher.failed == fid.String() }, testutil.WaitLong, testutil.IntervalMedium)
}

func TestSMTPDispatch(t *testing.T) {
	t.Parallel()

	// setup
	if !dbtestutil.WillUsePostgres() {
		t.Skip("This test requires postgres")
	}
	ctx, logger, db, ps := setup(t)

	// start mock SMTP server
	mockSMTPSrv := smtpmock.New(smtpmock.ConfigurationAttr{
		LogToStdout:       true,
		LogServerActivity: true,
	})
	require.NoError(t, mockSMTPSrv.Start())
	t.Cleanup(func() {
		require.NoError(t, mockSMTPSrv.Stop())
	})

	// given
	const from = "danny@coder.com"
	cfg := codersdk.NotificationsConfig{
		SMTP: codersdk.NotificationsEmailConfig{
			From:      from,
			Smarthost: serpent.HostPort{Host: "localhost", Port: fmt.Sprintf("%d", mockSMTPSrv.PortNumber())},
			Hello:     "localhost",
		},
	}
	dispatcher := &interceptingSMTPDispatcher{SMTPDispatcher: dispatch.NewSMTPDispatcher(cfg.SMTP, logger)}
	fakeDispatchers, err := notifications.NewHandlerRegistry(dispatcher)
	require.NoError(t, err)

	manager := notifications.NewManager(cfg, db, logger, fakeDispatchers)
	notifications.RegisterInstance(manager)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(ctx))
	})

	client := coderdtest.New(t, &coderdtest.Options{Database: db, Pubsub: ps})
	first := coderdtest.CreateFirstUser(t, client)
	_, user := coderdtest.CreateAnotherUserMutators(t, client, first.OrganizationID, nil, func(r *codersdk.CreateUserRequest) {
		r.Email = "bob@coder.com"
		r.Username = "bob"
	})

	// when
	msgID, err := manager.Enqueue(ctx, user.ID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodSmtp, types.Labels{}, "test")
	require.NoError(t, err)

	manager.StartNotifiers(ctx, 1)

	// then
	require.Eventually(t, func() bool {
		require.NoError(t, dispatcher.err)
		require.False(t, dispatcher.retryable)
		return dispatcher.sent
	}, testutil.WaitLong, testutil.IntervalMedium)

	msgs := mockSMTPSrv.MessagesAndPurge()
	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0].MsgRequest(), fmt.Sprintf("From: %s", from))
	require.Contains(t, msgs[0].MsgRequest(), fmt.Sprintf("To: %s", user.Email))
	require.Contains(t, msgs[0].MsgRequest(), fmt.Sprintf("Message-Id: %s", msgID))
}

func TestWebhookDispatch(t *testing.T) {
	t.Parallel()

	// setup
	if !dbtestutil.WillUsePostgres() {
		t.Skip("This test requires postgres")
	}
	ctx, logger, db, ps := setup(t)

	var (
		msgID *uuid.UUID
		input types.Labels
	)

	sent := make(chan bool, 1)
	// Mock server to simulate webhook endpoint.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload dispatch.WebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.EqualValues(t, 1, payload.Version)
		require.Equal(t, *msgID, payload.MsgID)
		require.Equal(t, payload.Payload.Labels, input)
		require.Equal(t, payload.Payload.UserEmail, "bob@coder.com")
		require.Equal(t, payload.Payload.UserName, "bob")
		require.Equal(t, payload.Payload.NotificationName, "Workspace Deleted")

		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte("noted."))
		require.NoError(t, err)
		sent <- true
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)

	// given
	cfg := codersdk.NotificationsConfig{
		Webhook: codersdk.NotificationsWebhookConfig{
			Endpoint: *serpent.URLOf(endpoint),
		},
	}
	manager := notifications.NewManager(cfg, db, logger, nil)
	notifications.RegisterInstance(manager)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(ctx))
	})

	client := coderdtest.New(t, &coderdtest.Options{Database: db, Pubsub: ps})
	first := coderdtest.CreateFirstUser(t, client)
	_, user := coderdtest.CreateAnotherUserMutators(t, client, first.OrganizationID, nil, func(r *codersdk.CreateUserRequest) {
		r.Email = "bob@coder.com"
		r.Username = "bob"
	})

	// when
	input = types.Labels{
		"a": "b",
		"c": "d",
	}
	msgID, err = manager.Enqueue(ctx, user.ID, notifications.TemplateWorkspaceDeleted, database.NotificationMethodWebhook, input, "test")
	require.NoError(t, err)

	manager.StartNotifiers(ctx, 1)

	// then
	require.Eventually(t, func() bool { return <-sent }, testutil.WaitShort, testutil.IntervalFast)
}

func setup(t *testing.T) (context.Context, slog.Logger, database.Store, *pubsub.PGPubsub) {
	t.Helper()

	connectionURL, closeFunc, err := dbtestutil.Open()
	require.NoError(t, err)
	t.Cleanup(closeFunc)

	ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitSuperLong)
	t.Cleanup(cancel)
	logger := slogtest.Make(t, &slogtest.Options{IgnoreErrors: true, IgnoredErrorIs: []error{}}).Leveled(slog.LevelDebug)

	sqlDB, err := sql.Open("postgres", connectionURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	db := database.New(sqlDB)
	ps, err := pubsub.New(ctx, logger, sqlDB, connectionURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ps.Close())
	})

	return ctx, logger, db, ps
}

type fakeDispatcher struct {
	succeeded string
	failed    string
}

func (f *fakeDispatcher) NotificationMethod() database.NotificationMethod {
	return database.NotificationMethodSmtp
}

func (f *fakeDispatcher) Dispatcher(payload types.MessagePayload, _, _ string) (dispatch.DeliveryFunc, error) {
	return func(ctx context.Context, msgID uuid.UUID) (retryable bool, err error) {
		if payload.Labels.Get("type") == "success" {
			f.succeeded = msgID.String()
		} else {
			f.failed = msgID.String()
		}
		return false, nil
	}, nil
}

type interceptingSMTPDispatcher struct {
	*dispatch.SMTPDispatcher

	sent      bool
	retryable bool
	err       error
}

func (i *interceptingSMTPDispatcher) Dispatcher(payload types.MessagePayload, title, body string) (dispatch.DeliveryFunc, error) {
	return func(ctx context.Context, msgID uuid.UUID) (retryable bool, err error) {
		deliveryFn, err := i.SMTPDispatcher.Dispatcher(payload, title, body)
		if err != nil {
			return false, err
		}

		i.retryable, i.err = deliveryFn(ctx, msgID)
		i.sent = true
		return i.retryable, i.err
	}, nil
}
