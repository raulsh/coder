package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/coderd/notifications/render"
	"github.com/coder/coder/v2/coderd/notifications/types"

	"github.com/coder/coder/v2/coderd/database"
)

const (
	// TODO: configurable?
	NotifierQueueSize       = 10
	NotifierLeasePeriod     = time.Minute * 10
	NotifierFetchInterval   = time.Second * 15
	NotifierDispatchTimeout = time.Second * 30
)

// notifier is a consumer of the notifications_messages queue. It dequeues messages from that table and processes them
// through a pipeline of fetch -> render -> dispatch.
type notifier struct {
	id    int
	ctx   context.Context
	log   slog.Logger
	store Store

	tick        *time.Ticker
	stopOnce    sync.Once
	quit        chan any
	done        chan any
	renderers   *ProviderRegistry[Renderer]
	dispatchers *ProviderRegistry[Dispatcher]
}

func newNotifier(ctx context.Context, id int, log slog.Logger, db Store, rp *ProviderRegistry[Renderer], dp *ProviderRegistry[Dispatcher]) *notifier {
	return &notifier{
		id:          id,
		ctx:         ctx,
		log:         log.Named("notifier").With(slog.F("id", id)),
		quit:        make(chan any),
		done:        make(chan any),
		tick:        time.NewTicker(NotifierFetchInterval),
		store:       db,
		renderers:   rp,
		dispatchers: dp,
	}
}

// run is the main loop of the notifier.
func (n *notifier) run(ctx context.Context, success chan<- dispatchResult, failure chan<- dispatchResult) error {
	defer func() {
		close(n.done)
		n.log.Info(context.Background(), "gracefully stopped")
	}()

	// TODO: idea from Cian: instead of querying the database on a short interval, we could wait for pubsub notifications.
	//		 if 100 notifications are enqueued, we shouldn't activate this routine for each one; so how to debounce these?
	//		 PLUS we should also have an interval (but a longer one, maybe 1m) to account for retries (those will not get
	//		 triggered by a code path, but rather by a timeout expiring which makes the message retryable)
	for {
		err := n.process(ctx, success, failure)
		if err != nil {
			n.log.Error(ctx, "failed to process messages", slog.Error(err))
		}

		select {
		case <-ctx.Done():
			return xerrors.Errorf("notifier %d context canceled: %w", n.id, ctx.Err())
		case <-n.quit:
			return nil
		case <-n.tick.C:
			// sleep until next invocation
		}
	}
}

// process is responsible for coordinating the retrieval, processing, and delivery of messages.
func (n *notifier) process(ctx context.Context, success chan<- dispatchResult, failure chan<- dispatchResult) error {
	n.log.Debug(ctx, "attempting to dequeue messages")

	msgs, err := n.fetch(ctx)
	if err != nil {
		return xerrors.Errorf("fetch messages: %w", err)
	}

	n.log.Debug(ctx, "dequeued messages", slog.F("count", len(msgs)))
	if len(msgs) == 0 {
		return nil
	}

	var eg errgroup.Group
	for _, msg := range msgs {
		// A message failing to be prepared correctly should not affect other messages.
		input, err := n.prepare(ctx, msg)
		if err != nil {
			n.log.Warn(ctx, "message preparation failed", slog.F("msg_id", msg.ID), slog.Error(err))
			failure <- newFailedDispatch(n.id, msg.ID, err)
			continue
		}

		eg.Go(func() error {
			// Dispatch must only return an error for exceptional cases, NOT for failed messages.
			// The first message to return an error cancels the errgroup's context, and all in-flight dispatches will be canceled.
			// TODO: validate this
			return n.dispatch(ctx, msg.ID, string(msg.Method), input, success, failure)
		})
	}

	if err = eg.Wait(); err != nil {
		n.log.Debug(ctx, "dispatch failed", slog.Error(err))
		return xerrors.Errorf("dispatch failed: %w", err)
	}

	n.log.Debug(ctx, "dispatch completed", slog.F("count", len(msgs)))
	return nil
}

// fetch retrieves messages from the queue by "acquiring a lease" whereby this notifier is the exclusive handler of these
// messages until they are dispatched - or until the lease expires (in exceptional cases).
func (n *notifier) fetch(ctx context.Context) ([]database.AcquireNotificationMessagesRow, error) {
	ctx, cancel := context.WithTimeout(ctx, NotifierLeasePeriod)
	deadline, _ := ctx.Deadline()
	defer cancel()

	msgs, err := n.store.AcquireNotificationMessages(ctx, database.AcquireNotificationMessagesParams{
		Count:           NotifierQueueSize,
		MaxAttemptCount: MaxAttempts,
		NotifierID:      int32(n.id),
		LeasedUntil:     deadline,
		UserID:          uuid.New(), // TODO: use real user ID
		OrgID:           uuid.New(), // TODO: use real org ID
	})
	if err != nil {
		return nil, xerrors.Errorf("acquire messages: %w", err)
	}

	return msgs, nil
}

// prepare renders the given message's templates and modifies the input for dispatcher-specific implementations.
func (n *notifier) prepare(ctx context.Context, msg database.AcquireNotificationMessagesRow) (types.Labels, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	input := types.Labels(msg.Input)
	if input == nil {
		input = types.Labels{}
	}
	// Debug information.
	input.Set("notifier_id", fmt.Sprintf("%d", n.id))
	input.SetValue("msg_id", msg.ID)

	// Additional information.
	input.SetValue("user_id", msg.UserID)
	input.Set("user_email", msg.UserEmail)
	input.Set("user_name", msg.UserName)

	var err error
	switch msg.Method {
	case database.NotificationMethodSmtp:
		var subject, body string

		if subject, err = n.render(ctx, render.Text, msg.TitleTemplate, input); err != nil {
			return nil, xerrors.Errorf("SMTP render subject: %w", err)
		}
		if body, err = n.render(ctx, render.HTML, msg.BodyTemplate, input); err != nil {
			return nil, xerrors.Errorf("SMTP render body: %w", err)
		}

		// Set required labels.
		input.Merge(types.Labels{
			"subject": subject,
			"body":    body,
			"to":      msg.UserEmail,
		})
	default:
		err = xerrors.Errorf("unrecognized method: %s", msg.Method)
	}

	return input, err
}

// dispatch sends a given notification message to its defined method.
// This method *only* returns an error when a context error occurs; any other error is interpreted as a failure to
// deliver the notification and as such the message will be marked as failed (to later be optionally retried).
func (n *notifier) dispatch(ctx context.Context, msgID uuid.UUID, provider string, input types.Labels, success, failure chan<- dispatchResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	ctx, cancel := context.WithTimeout(ctx, NotifierDispatchTimeout)
	defer cancel()

	d, err := n.dispatchers.Resolve(provider)
	if err != nil {
		return xerrors.Errorf("resolve dispatch provider: %w", err)
	}

	logger := n.log.With(slog.F("msg_id", msgID), slog.F("method", provider))

	if ok, missing := d.Validate(input); !ok {
		logger.Warn(ctx, "message failed dispatcher validation", slog.F("missing_labels", strings.Join(missing, ", ")))
		failure <- newFailedDispatch(n.id, msgID, xerrors.Errorf("failed validation, missing %v labels", missing))
		return nil
	}

	if _, err = d.Send(ctx, msgID, input); err != nil {
		// Don't try to accumulate message responses if the context has been canceled.
		// This message's lease will expire in the store and will be requeued.
		// It's possible this will lead to a message being delivered more than once, and that is why Stop() is preferable
		// instead of canceling the context.
		if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
			return err
		}

		logger.Warn(ctx, "message dispatch failed", slog.Error(err))
		failure <- newFailedDispatch(n.id, msgID, err)
	} else {
		logger.Debug(ctx, "message dispatch succeeded")
		success <- newSuccessfulDispatch(n.id, msgID)
	}

	return nil
}

// render renders a given template using the given Renderer using the given input labels, and returns the resulting
// rendered string or an error.
func (n *notifier) render(ctx context.Context, provider string, template string, input types.Labels) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	r, err := n.renderers.Resolve(provider)
	if err != nil {
		return "", xerrors.Errorf("resolve render provider: %w", err)
	}

	// TODO: pass context down
	return r.Render(template, input)
}

// stop stops the notifier from processing any new notifications.
// This is a graceful stop, so any in-flight notifications will be completed before the notifier stops.
// Once a notifier has stopped, it cannot be restarted.
func (n *notifier) stop() {
	n.stopOnce.Do(func() {
		n.log.Info(context.Background(), "graceful stop requested")

		n.tick.Stop()
		close(n.quit)
		<-n.done
	})
}
