package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications/dispatch"
	"github.com/coder/coder/v2/coderd/notifications/render"
	"github.com/coder/coder/v2/coderd/notifications/types"

	"cdr.dev/slog"
)

const (
	MaxAttempts          = 5           // TODO: configurable
	BulkUpdateBufferSize = 50          // TODO: configurable
	BulkUpdateInterval   = time.Second // TODO: configurable
)

var (
	TemplateUserRegistration   = uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	TemplatePasswordReset      = uuid.MustParse("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a14")
	TemplateWorkspaceUnhealthy = uuid.MustParse("c2eebc99-9c0b-4ef8-bb6d-6bb9bd380a15")
)

// Manager manages all notifications being enqueued and dispatched.
//
// Manager maintains a group of notifiers: these consume the queue of notification messages in the store.
//
// Notifiers dequeue messages from the store _n_ at a time and concurrently "dispatch" these messages, meaning they are
// sent to their respective receivers (email, webhook, etc).
//
// To reduce load on the store, successful and failed dispatches are accumulated in two separate buffers (success/failure)
// in the Manager, and updates are sent to the store about which messages succeeded or failed every _n_ seconds.
// These buffers are limited in size, naturally introduces some backpressure; if there are hundreds of messages to be
// sent but they start failing too quickly, the buffers (receive channels) will fill up and block senders, which will
// slow down the dispatch rate.
//
// NOTE: The above backpressure mechanism only works if all notifiers live within the same process, which may not be true
// forever, such as if we split notifiers out into separate targets for greater processing throughput; in this case we
// will need an alternative mechanism for handling backpressure.
type Manager struct {
	log   slog.Logger
	store Store

	notifiers  []*notifier
	notifierMu sync.Mutex

	rendererProvider    *ProviderRegistry[Renderer]
	dispatchersProvider *ProviderRegistry[Dispatcher]

	stopMu sync.Mutex
	stop   chan any
	done   chan any
}

func NewManager(ctx context.Context, notifiers int, store Store, log slog.Logger, rp *ProviderRegistry[Renderer], dp *ProviderRegistry[Dispatcher]) *Manager {
	man := &Manager{store: store, stop: make(chan any), done: make(chan any), log: log, rendererProvider: rp, dispatchersProvider: dp}

	// Closes when Stop() is called or context is canceled.
	go func() {
		err := man.loop(ctx, notifiers)
		if err != nil {
			log.Error(ctx, "notification manager exited with error", slog.Error(err))
		}
	}()
	return man
}

// DefaultRenderers builds a set of known renderers and panics if any error occurs.
func DefaultRenderers() *ProviderRegistry[Renderer] {
	reg, err := NewProviderRegistry[Renderer](&render.TextRenderer{}, &render.HTMLRenderer{})
	if err != nil {
		panic(err)
	}
	return reg
}

// DefaultDispatchers builds a set of known dispatchers and panics if any error occurs.
func DefaultDispatchers() *ProviderRegistry[Dispatcher] {
	reg, err := NewProviderRegistry[Dispatcher](&dispatch.SMTPDispatcher{})
	if err != nil {
		panic(err)
	}
	return reg
}

func (m *Manager) loop(ctx context.Context, nc int) error {
	defer func() {
		close(m.done)
		m.log.Info(context.Background(), "notification manager loop exited")
	}()

	// TODO: ensure process stop gracefully shuts down and communicates well
	// Caught a terminal signal before notifiers were created, exit immediately.
	select {
	case <-m.stop:
		m.log.Warn(ctx, "gracefully stopped")
		return xerrors.Errorf("gracefully stopped")
	case <-ctx.Done():
		m.log.Error(ctx, "ungracefully stopped", slog.Error(ctx.Err()))
		return xerrors.Errorf("notifications: %w", ctx.Err())
	default:
	}

	var eg errgroup.Group

	var (
		success = make(chan dispatchResult, BulkUpdateBufferSize)
		failure = make(chan dispatchResult, BulkUpdateBufferSize)
	)

	for i := 0; i < nc; i++ {
		eg.Go(func() error {
			i := i
			m.notifierMu.Lock()
			n := newNotifier(ctx, i+1, m.log, m.store, m.rendererProvider, m.dispatchersProvider)
			m.notifiers = append(m.notifiers, n)
			m.notifierMu.Unlock()
			return n.run(ctx, success, failure)
		})
	}

	go func() {
		// Every interval, collect the messages in the channels and bulk update them in the database.
		tick := time.NewTicker(BulkUpdateInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				// Nothing we can do in this scenario except bail out; after the message lease expires, the messages will
				// be requeued and users will receive duplicates.
				// This is an explicit trade-off between keeping the database load light (by bulk-updating records) and
				// exactly-once delivery.
				//
				// The current assumption is that duplicate delivery of these messages is, at worst, slightly annoying.
				// If these notifications are triggering external actions (e.g. via webhooks) this could be more
				// consequential, and we may need a more sophisticated mechanism.
				//
				// TODO: mention the above tradeoff in documentation.
				if len(success)+len(failure) > 0 {
					m.log.Warn(ctx, "content canceled with pending updates in buffer, these messages will be sent again after lease expires",
						slog.F("success_count", len(success)), slog.F("failure_count", len(failure)))
				}
				return
			case <-m.stop:
				m.log.Warn(ctx, "graceful stop initiated; updating messages before stop",
					slog.F("success_count", len(success)), slog.F("failure_count", len(failure)))
				m.bulkUpdate(ctx, success, failure)
				return
			case <-tick.C:
				m.bulkUpdate(ctx, success, failure)
			}
		}
	}()

	err := eg.Wait()
	m.log.Info(ctx, "loop exited", slog.F("err", err))
	return err
}

func (m *Manager) Enqueue(ctx context.Context, templateId uuid.UUID, r database.NotificationReceiver, labels types.Labels, createdBy string, targets ...uuid.UUID) error {
	input, err := json.Marshal(labels)
	if err != nil {
		return xerrors.Errorf("failed encoding input labels: %w", err)
	}

	msg, err := m.store.EnqueueNotificationMessage(ctx, database.EnqueueNotificationMessageParams{
		ID:                     uuid.New(),
		NotificationTemplateID: templateId,
		Receiver:               r,
		Input:                  input,
		Targets:                targets,
		CreatedBy:              createdBy,
	})
	if err != nil {
		return xerrors.Errorf("failed to enqueue notification: %w", err)
	}

	m.log.Debug(ctx, "enqueued notification", slog.F("msg_id", msg.ID))
	return nil
}

// bulkUpdate updates messages in the store based on the given successful and failed message dispatch results.
func (m *Manager) bulkUpdate(ctx context.Context, success, failure <-chan dispatchResult) {
	// Nothing to do.
	nSuccess := len(success)
	nFailure := len(failure)
	if nSuccess+nFailure == 0 {
		return
	}

	var (
		successParams database.BulkMarkNotificationMessagesSentParams
		failureParams database.BulkMarkNotificationMessagesFailedParams
	)

	// Read all the existing messages due for update from the channel, but don't range over the channels because they
	// block until they are closed.
	// If more items are added to the success or failure channels between measuring their lengths and now, those items
	// will be processed on the next bulk update.

	for i := 0; i < nSuccess; i++ {
		res := <-success
		successParams.IDs = append(successParams.IDs, res.msg)
		successParams.SentAts = append(successParams.SentAts, res.ts)
	}
	for i := 0; i < nFailure; i++ {
		res := <-failure
		failureParams.IDs = append(failureParams.IDs, res.msg)
		failureParams.FailedAts = append(failureParams.FailedAts, res.ts)
		failureParams.Statuses = append(failureParams.Statuses, database.NotificationMessageStatusFailed)
		var reason string
		if res.err != nil {
			reason = res.err.Error()
		}
		failureParams.StatusReasons = append(failureParams.StatusReasons, reason)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logger := m.log.With(slog.F("type", "update_sent"))

		n, err := m.store.BulkMarkNotificationMessagesSent(ctx, successParams)
		if err != nil {
			logger.Error(ctx, "bulk update failed", slog.Error(err))
			return
		}

		if int(n) == nSuccess {
			logger.Debug(ctx, "bulk update completed", slog.F("updated", n))
		} else {
			logger.Warn(ctx, "bulk update completed with discrepancy", slog.F("input", nSuccess), slog.F("updated", n))
		}
	}()

	go func() {
		defer wg.Done()
		logger := m.log.With(slog.F("type", "update_failed"))

		n, err := m.store.BulkMarkNotificationMessagesFailed(ctx, failureParams)
		if err != nil {
			logger.Error(ctx, "bulk update failed", slog.Error(err))
			return
		}

		if int(n) == nFailure {
			logger.Debug(ctx, "bulk update completed", slog.F("updated", n))
		} else {
			logger.Warn(ctx, "bulk update completed with discrepancy", slog.F("input", nFailure), slog.F("updated", n))
		}
	}()

	wg.Wait()
}

// Stop stops all notifiers; waits until all notifiers have stopped.
//
// TODO: by the time this is called, the notifiers have already had their context canceled
func (m *Manager) Stop(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.stopMu.Lock()
	defer m.stopMu.Unlock()

	m.log.Info(context.Background(), "graceful stop requested")

	var wg sync.WaitGroup
	wg.Add(len(m.notifiers))
	for _, n := range m.notifiers {
		time.Sleep(time.Second * 7)
		m.log.Info(ctx, "stopping notifier", slog.F("id", n.id))
		n.stop()
		wg.Done()
	}
	wg.Wait()
	close(m.stop)

	select {
	case <-ctx.Done():
		var errStr string
		if ctx.Err() != nil {
			errStr = ctx.Err().Error()
		}
		// For some reason, slog.Error returns {} for a context error.
		m.log.Error(context.Background(), "graceful stop failed", slog.F("err", errStr))
		return ctx.Err()
	case <-m.done:
		m.log.Info(context.Background(), "gracefully stopped")
		return nil
	}
}

type dispatchResult struct {
	notifier int
	msg      uuid.UUID
	ts       time.Time
	err      error
}

func newSuccessfulDispatch(notifier int, msg uuid.UUID) dispatchResult {
	return dispatchResult{
		notifier: notifier,
		msg:      msg,
		ts:       time.Now(),
	}
}

func newFailedDispatch(notifier int, msg uuid.UUID, err error) dispatchResult {
	return dispatchResult{
		notifier: notifier,
		msg:      msg,
		ts:       time.Now(),
		err:      err,
	}
}
