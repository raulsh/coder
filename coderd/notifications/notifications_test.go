package notifications_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/xerrors"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestStuff(t *testing.T) {
	//sqlDB, err := sql.Open("postgres", "postgres://coder:secret42!@localhost/coder?sslmode=disable")
	//require.NoError(t, err)
	//t.Cleanup(func() { _ = sqlDB.Close() })
	////
	//// u1, _ := uuid.Parse("e4eebc99-9c0b-4ef8-bb6d-6bb9bd380a24")
	////u2, _ := uuid.Parse("f9d5cea0-d919-47d1-a035-129931daab0d")
	////u3, _ := uuid.Parse("f9d5cea0-d919-47d1-a035-129931daab0e")
	////
	//tid, _ := uuid.Parse("c2eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
	//input := map[string]string{
	//	"order_id": "12345",
	//}
	//inputBytes, _ := json.Marshal(input)
	//
	//db := database.New(sqlDB)
	//nm, err := db.EnqueueNotificationMessage(context.Background(), database.EnqueueNotificationMessageParams{
	//	ID:                     uuid.New(),
	//	NotificationTemplateID: tid,
	//	CreatedBy:              "danny",
	//	Receiver:               database.NotificationReceiverSmtp,
	//	Input:                  inputBytes,
	//	Targets:                []uuid.UUID{},
	//})
	//_ = nm
	//require.NoError(t, err)

	// nr, err := db.BulkMarkNotificationMessagesSent(context.Background(), database.BulkMarkNotificationMessagesSentParams{
	//	IDs:     []uuid.UUID{u1, u2, u3},
	//	SentAts: []time.Time{time.Now(), time.Now().Add(-1 * time.Second), time.Now().Add(-1 * time.Minute)},
	//})
	//
	//require.Greater(t, nr, int64(0))
	//require.NoError(t, err)
	//
	//t.Fail()

	dp, err := notifications.NewProviderRegistry[notifications.Dispatcher](slowFailingSMTPProvider{
		delayFn: func() time.Duration {
			return time.Second + time.Duration(rand.IntN(100))*time.Millisecond
		},
	})
	require.NoError(t, err)
	logger := slogtest.Make(t, &slogtest.Options{
		IgnoreErrors: true, IgnoredErrorIs: []error{},
	}).Leveled(slog.LevelDebug)
	n := notifications.NewManager(
		fakeDB{}, logger,
		notifications.DefaultRenderers(),
		dp,
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.ErrorIs(t, n.loop(ctx, 3), context.Canceled)
	}()

	select {
	case <-ctx.Done():
		return
		// case <-time.After(time.Second * 1):
		//	t.Logf("\n\n\n\nCANCELED\n\n\n\n")
		//	cancel()
	// case <-time.After(time.Millisecond * 200):
	//	t.Logf("\n\n\n\nSTOPPED\n\n\n\n")
	//	n.Stop(ctx)
	default:
	}

	wg.Wait()
}

type fakeDB struct{}

func (f fakeDB) AcquireNotificationMessages(ctx context.Context, params database.AcquireNotificationMessagesParams) ([]database.AcquireNotificationMessagesRow, error) {
	out := make([]database.AcquireNotificationMessagesRow, 2)
	for i := 0; i < cap(out); i++ {
		out[i] = database.AcquireNotificationMessagesRow{
			ID:                     uuid.New(),
			Status:                 database.NotificationMessageStatusEnqueued,
			NotificationTemplateID: uuid.New(),
			BodyTemplate:           "body with {{.variable}}",
			TitleTemplate:          "title with {{.variable}}",
			Receiver:               database.NotificationReceiverSmtp,
			Input: map[string]string{
				"id":       fmt.Sprintf("%d", i),
				"variable": fmt.Sprintf("ITEM %d", i+1),
			},
		}
	}
	return out, nil
}

func (f fakeDB) BulkMarkNotificationMessagesSent(ctx context.Context, arg database.BulkMarkNotificationMessagesSentParams) (int64, error) {
	return int64(len(arg.IDs)), nil
}

func (f fakeDB) BulkMarkNotificationMessagesFailed(ctx context.Context, arg database.BulkMarkNotificationMessagesFailedParams) (int64, error) {
	return int64(len(arg.IDs)), nil
}

type slowFailingSMTPProvider struct {
	delayFn func() time.Duration
}

func (f slowFailingSMTPProvider) Name() string {
	return string(database.NotificationReceiverSmtp)
}

func (f slowFailingSMTPProvider) Validate(input types.Labels) (bool, []string) {
	return true, nil
}

func (f slowFailingSMTPProvider) Send(ctx context.Context, input types.Labels) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(f.delayFn()):
		break
	}

	// Fail half of requests.
	if rand.IntN(10) < 5 {
		return xerrors.New(fmt.Sprintf("oops"))
	}
	return nil
}
