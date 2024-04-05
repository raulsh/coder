package inteldserver

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/pubsub"
	"github.com/coder/coder/v2/inteld/proto"
)

type Options struct {
	Database database.Store
}

func New(ctx context.Context, opts Options) (proto.DRPCIntelDaemonServer, error) {
	return &server{}, nil
}

type server struct {
	Database database.Store
	Pubsub   pubsub.Pubsub

	invocationQueue *invocationQueue
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

func (s *server) RecordInvocation(ctx context.Context, req *proto.RecordInvocationRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *server) ReportPath(_ context.Context, _ *proto.ReportPathRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *server) Close() error {
	return nil
}

type invocationQueue struct {
	*sync.Cond
	flushInterval  time.Duration
	queue          []*proto.Invocation
	flushRequested bool
	lastFlush      time.Time
	logger         slog.Logger
}

func (i *invocationQueue) enqueue(req *proto.RecordInvocationRequest) {
	i.L.Lock()
	defer i.L.Unlock()
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
