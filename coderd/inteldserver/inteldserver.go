package inteldserver

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"cdr.dev/slog"

	"github.com/hashicorp/yamux"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/pubsub"
	"github.com/coder/coder/v2/inteld/proto"
)

type Options struct {
	Database database.Store

	Pubsub pubsub.Pubsub

	Logger slog.Logger

	// InvocationFlushInterval is the interval at which invocations
	// are flushed to the database.
	InvocationFlushInterval time.Duration

	// InvocationQueueLimit is the maximum number of invocations that
	// can be queued before they are dropped.
	InvocationQueueLimit int
}

func New(ctx context.Context, opts Options) (proto.DRPCIntelDaemonServer, error) {
	srv := &server{
		Database: opts.Database,
		Pubsub:   opts.Pubsub,

		invocationQueue: &invocationQueue{
			Cond:          sync.NewCond(&sync.Mutex{}),
			flushInterval: opts.InvocationFlushInterval,
			queue:         make([]*proto.Invocation, 0, opts.InvocationQueueLimit),
			queueLimit:    opts.InvocationQueueLimit,
			logger:        opts.Logger.Named("invocation_queue"),
		},
	}

	srv.closeWaitGroup.Add(1)
	go srv.invocationQueueLoop()

	return srv, nil
}

type server struct {
	Database database.Store
	Pubsub   pubsub.Pubsub
	Logger   slog.Logger

	closeContext    context.Context
	closeCancel     context.CancelFunc
	closed          bool
	closeMutex      sync.Mutex
	closeWaitGroup  sync.WaitGroup
	invocationQueue *invocationQueue
}

func (s *server) invocationQueueLoop() {
	defer s.Logger.Debug(s.closeContext, "invocation queue loop exited")
	defer s.closeWaitGroup.Done()
	for {
		err := s.invocationQueue.startFlushLoop(s.closeContext, func(i []*proto.Invocation) error {
			s.Logger.Info(s.closeContext, "invocations flushed", slog.F("count", len(i)))
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, yamux.ErrSessionShutdown) {
				return
			}
			s.Logger.Warn(s.closeContext, "failed to write invocations", slog.Error(err))
			return
		}
	}
}

func (s *server) Register(req *proto.RegisterRequest, stream proto.DRPCIntelDaemon_RegisterStream) error {
	didIt := false
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(time.Second):
			if !didIt {
				stream.Send(&proto.SystemResponse{
					Message: &proto.SystemResponse_TrackExecutables{
						TrackExecutables: &proto.TrackExecutables{
							BinaryName: []string{
								"go",
								"node",
							},
						},
					},
				})
			}
		}
	}

}

func (s *server) RecordInvocation(_ context.Context, req *proto.RecordInvocationRequest) (*proto.Empty, error) {
	// inteld will continue sending invocations if they fail, so we
	// don't want to return an error here even if the queue is full.
	s.invocationQueue.enqueue(req)
	return &proto.Empty{}, nil
}

func (s *server) ReportPath(_ context.Context, _ *proto.ReportPathRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *server) Close() error {
	s.closeMutex.Lock()
	defer s.closeMutex.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.closeCancel()
	s.closeWaitGroup.Wait()
	return nil
}

type invocationQueue struct {
	*sync.Cond
	flushInterval  time.Duration
	queue          []*proto.Invocation
	queueLimit     int
	flushRequested bool
	lastFlush      time.Time
	logger         slog.Logger
}

func (i *invocationQueue) enqueue(req *proto.RecordInvocationRequest) {
	i.L.Lock()
	defer i.L.Unlock()
	if len(i.queue) > i.queueLimit {
		i.logger.Warn(context.Background(), "invocation queue is full, dropping invocations")
		return
	}
	i.queue = append(i.queue, req.Invocations...)
	i.Broadcast()
}

func (i *invocationQueue) startFlushLoop(ctx context.Context, flush func([]*proto.Invocation) error) error {
	i.L.Lock()
	defer i.L.Unlock()

	ctxDone := false

	// wake 4 times per Flush interval to check if anything needs to be flushed
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		tkr := time.NewTicker(i.flushInterval / 4)
		defer tkr.Stop()
		for {
			select {
			// also monitor the context here, so we notice immediately, rather
			// than waiting for the next tick or logs
			case <-ctx.Done():
				i.L.Lock()
				ctxDone = true
				i.L.Unlock()
				i.Broadcast()
				return
			case <-tkr.C:
				i.Broadcast()
			}
		}
	}()

	for {
		for !ctxDone && !i.hasPendingWorkLocked() {
			i.Wait()
		}
		if ctxDone {
			return ctx.Err()
		}
		queue := i.queue[:]
		i.flushRequested = false
		i.L.Unlock()
		err := flush(queue)
		i.L.Lock()
		if err != nil {
			return xerrors.Errorf("failed to flush invocations: %w", err)
		}
		i.queue = i.queue[len(queue):]
		i.lastFlush = time.Now()
	}
}

func (i *invocationQueue) hasPendingWorkLocked() bool {
	if len(i.queue) == 0 {
		return false
	}
	if time.Since(i.lastFlush) > i.flushInterval {
		return true
	}
	if i.flushRequested {
		return true
	}
	return false
}
