package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/coder/v2/codersdk"
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
	// TODO: configurable
	MaxAttempts          = 5
	RetryInterval        = time.Minute * 5
	BulkUpdateBufferSize = 50
	BulkUpdateInterval   = time.Second
	DeletionPeriod       = time.Hour * 24 * 7
)

var (
	singleton *Manager

	SingletonNotRegisteredErr = xerrors.New("singleton not registered")
)

// RegisterInstance receives a Manager reference to act as a Singleton.
// We use a Singleton to centralize the logic around enqueueing notifications, instead of requiring that an instance
// of the Manager be passed around the codebase.
func RegisterInstance(m *Manager) {
	singleton = m
}

// Manager manages all notifications being enqueued and dispatched.
//
// Manager maintains a group of notifiers: these consume the queue of notification messages in the store.
//
// Notifiers dequeue messages from the store _n_ at a time and concurrently "dispatch" these messages, meaning they are
// sent by their respective methods (email, webhook, etc).
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
	cfg codersdk.NotificationsConfig

	store Store
	log   slog.Logger

	notifiers  []*notifier
	notifierMu sync.Mutex

	rendererProvider    *ProviderRegistry[Renderer]
	dispatchersProvider *ProviderRegistry[Dispatcher]

	stopOnce sync.Once
	stop     chan any
	done     chan any
}

func NewManager(cfg codersdk.NotificationsConfig, store Store, log slog.Logger, rp *ProviderRegistry[Renderer], dp *ProviderRegistry[Dispatcher]) *Manager {
	if rp == nil {
		rp = DefaultRenderers(cfg, log)
	}
	if dp == nil {
		dp = DefaultDispatchers(cfg, log)
	}
	return &Manager{cfg: cfg, store: store, stop: make(chan any), done: make(chan any), log: log, rendererProvider: rp, dispatchersProvider: dp}
}

// DefaultRenderers builds a set of known renderers and panics if any error occurs.
func DefaultRenderers(cfg codersdk.NotificationsConfig, log slog.Logger) *ProviderRegistry[Renderer] {
	reg, err := NewProviderRegistry[Renderer](
		&render.TextRenderer{},
		&render.HTMLRenderer{})
	if err != nil {
		panic(err)
	}
	return reg
}

// DefaultDispatchers builds a set of known dispatchers and panics if any error occurs.
func DefaultDispatchers(cfg codersdk.NotificationsConfig, log slog.Logger) *ProviderRegistry[Dispatcher] {
	reg, err := NewProviderRegistry[Dispatcher](
		dispatch.NewSMTPDispatcher(cfg.SMTP, log.Named("dispatcher.smtp")),
		dispatch.NewWebhookDispatcher(cfg.Webhook, log.Named("dispatcher.webhook")),
	)
	if err != nil {
		panic(err)
	}
	return reg
}

// StartNotifiers initiates the control loop in the background, which spawns a given number of notifier goroutines.
func (m *Manager) StartNotifiers(ctx context.Context, notifiers int) {
	// Closes when Stop() is called or context is canceled.
	go func() {
		err := m.loop(ctx, notifiers)
		if err != nil {
			m.log.Error(ctx, "notification manager stopped with error", slog.Error(err))
		}
	}()
}

// loop contains the main business logic of the notification manager. It is responsible for subscribing to notification
// events, creating notifiers, and publishing bulk dispatch result updates to the store.
func (m *Manager) loop(ctx context.Context, notifiers int) error {
	defer func() {
		close(m.done)
		m.log.Info(context.Background(), "notification manager stopped")
	}()

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

	var (
		// Buffer successful/failed notification dispatches in memory to reduce load on the store.
		//
		// We keep separate buffered for success/failure right now because the bulk updates are already a bit janky,
		// see BulkMarkNotificationMessagesSent/BulkMarkNotificationMessagesFailed. If we had the ability to batch updates,
		// like is offered in https://docs.sqlc.dev/en/stable/reference/query-annotations.html#batchmany, we'd have a cleaner
		// approach to this - but for now this will work fine.
		success = make(chan dispatchResult, BulkUpdateBufferSize)
		failure = make(chan dispatchResult, BulkUpdateBufferSize)
	)

	// Create a specific number of notifiers to run concurrently.
	var eg errgroup.Group
	for i := 0; i < notifiers; i++ {
		eg.Go(func() error {
			m.notifierMu.Lock()
			n := newNotifier(ctx, i+1, m.log, m.store, m.rendererProvider, m.dispatchersProvider)
			m.notifiers = append(m.notifiers, n)
			m.notifierMu.Unlock()
			return n.run(ctx, success, failure)
		})
	}

	eg.Go(func() error {
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
				m.log.Warn(ctx, "exiting ungracefully", slog.Error(ctx.Err()))

				if len(success)+len(failure) > 0 {
					m.log.Warn(ctx, "content canceled with pending updates in buffer, these messages will be sent again after lease expires",
						slog.F("success_count", len(success)), slog.F("failure_count", len(failure)))
				}
				return ctx.Err()
			case <-m.stop:
				if len(success)+len(failure) > 0 {
					m.log.Warn(ctx, "flushing buffered updates before stop",
						slog.F("success_count", len(success)), slog.F("failure_count", len(failure)))
					m.bulkUpdate(ctx, success, failure)
					m.log.Warn(ctx, "flushing updates done")
				}
				return nil
			case <-tick.C:
				m.bulkUpdate(ctx, success, failure)
			}
		}
	})

	err := eg.Wait()
	if err != nil {
		m.log.Error(ctx, "manager loop exited with error", slog.Error(err))
	}
	return err
}

// Enqueue queues a notification message for later delivery.
// This is a delegator for the underlying notifications singleton.
func Enqueue(ctx context.Context, userID, templateID uuid.UUID, method database.NotificationMethod, labels types.Labels, createdBy string, targets ...uuid.UUID) (*uuid.UUID, error) {
	if singleton == nil {
		return nil, SingletonNotRegisteredErr
	}

	return singleton.Enqueue(ctx, userID, templateID, method, labels, createdBy, targets...)
}

// Enqueue queues a notification message for later delivery.
// Messages will be dequeued by a notifier later and dispatched.
// TODO: don't accept method here; determine which method to use from notification_preferences.
func (m *Manager) Enqueue(ctx context.Context, userID, templateID uuid.UUID, method database.NotificationMethod, labels types.Labels, createdBy string, targets ...uuid.UUID) (*uuid.UUID, error) {
	input, err := json.Marshal(labels)
	if err != nil {
		return nil, xerrors.Errorf("failed encoding input labels: %w", err)
	}

	id := uuid.New()
	msg, err := m.store.EnqueueNotificationMessage(ctx, database.EnqueueNotificationMessageParams{
		ID:                     id,
		UserID:                 userID,
		NotificationTemplateID: templateID,
		Method:                 method,
		Input:                  input,
		Targets:                targets,
		CreatedBy:              createdBy,
	})
	if err != nil {
		m.log.Warn(ctx, "enqueue notification", slog.F("template", templateID), slog.F("input", input), slog.Error(err))
		return nil, xerrors.Errorf("failed to enqueue notification: %w", err)
	}

	m.log.Debug(ctx, "enqueued notification", slog.F("msg_id", msg.ID))
	return &id, nil
}

// bulkUpdate updates messages in the store based on the given successful and failed message dispatch results.
func (m *Manager) bulkUpdate(ctx context.Context, success, failure <-chan dispatchResult) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	nSuccess := len(success)
	nFailure := len(failure)

	// Nothing to do.
	if nSuccess+nFailure == 0 {
		return
	}

	var (
		successParams database.BulkMarkNotificationMessagesSentParams
		failureParams database.BulkMarkNotificationMessagesFailedParams
	)

	// Read all the existing messages due for update from the channel, but don't range over the channels because they
	// block until they are closed.
	//
	// This is vulnerable to TOCTOU.
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

	// Execute bulk updates for success/failure concurrently.
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

		failureParams.MaxAttempts = MaxAttempts
		failureParams.RetryInterval = int32(RetryInterval.Seconds())
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

// Stop stops all notifiers and waits until they have stopped.
func (m *Manager) Stop(ctx context.Context) error {
	var err error
	m.stopOnce.Do(func() {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
		}

		m.log.Info(context.Background(), "graceful stop requested")

		// If the notifiers haven't been started, we don't need to wait for anything.
		// This is only really during testing when we want to enqueue messages only but not deliver them.
		if len(m.notifiers) == 0 {
			close(m.done)
		}

		// Stop all notifiers.
		var eg errgroup.Group
		for _, n := range m.notifiers {
			eg.Go(func() error {
				n.stop()
				return nil
			})
		}
		_ = eg.Wait()

		// Signal the stop channel to cause loop to exit.
		close(m.stop)

		// Wait for the manager loop to exit or the context to be canceled, whichever comes first.
		select {
		case <-ctx.Done():
			var errStr string
			if ctx.Err() != nil {
				errStr = ctx.Err().Error()
			}
			// For some reason, slog.Error returns {} for a context error.
			m.log.Error(context.Background(), "graceful stop failed", slog.F("err", errStr))
			err = ctx.Err()
			return
		case <-m.done:
			m.log.Info(context.Background(), "gracefully stopped")
			return
		}
	})

	return err
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
