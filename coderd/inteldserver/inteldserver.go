package inteldserver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"cdr.dev/slog"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtime"
	"github.com/coder/coder/v2/coderd/database/pubsub"
	"github.com/coder/coder/v2/inteld/proto"
)

type Options struct {
	Database database.Store

	Pubsub pubsub.Pubsub

	Logger slog.Logger

	// MachineID is the unique identifier for an intel
	// machine that will be used to record invocations.
	MachineID uuid.UUID

	// UserID is the unique identifier for the machine owner.
	UserID uuid.UUID

	// InvocationFlushInterval is the interval at which invocations
	// are flushed to the database.
	InvocationFlushInterval time.Duration

	// InvocationQueueLimit is the maximum number of invocations that
	// can be queued before they are dropped.
	InvocationQueueLimit int
}

func New(ctx context.Context, opts Options) (proto.DRPCIntelDaemonServer, error) {
	if opts.InvocationFlushInterval == 0 {
		opts.InvocationFlushInterval = time.Minute
	}
	if opts.InvocationQueueLimit == 0 {
		opts.InvocationQueueLimit = 1000
	}

	opts.Logger = opts.Logger.With(slog.F("machine_id", opts.MachineID), slog.F("user_id", opts.UserID))

	ctx, cancelFunc := context.WithCancel(ctx)
	srv := &server{
		Options: opts,

		invocationQueue: &invocationQueue{
			Cond:          sync.NewCond(&sync.Mutex{}),
			flushInterval: opts.InvocationFlushInterval,
			queue:         make([]*proto.Invocation, 0, opts.InvocationQueueLimit),
			queueLimit:    opts.InvocationQueueLimit,
			logger:        opts.Logger.Named("invocation_queue"),
		},
		closeContext: ctx,
		closeCancel:  cancelFunc,
	}

	srv.closeWaitGroup.Add(1)
	go srv.invocationQueueLoop()

	return srv, nil
}

type server struct {
	Options

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
			ids := make([]uuid.UUID, 0)
			binaryNames := make([]string, 0, len(i))
			binaryHashes := make([]string, 0, len(i))
			binaryPaths := make([]string, 0, len(i))
			binaryArgs := make([]json.RawMessage, 0, len(i))
			binaryVersions := make([]string, 0, len(i))
			workingDirs := make([]string, 0, len(i))
			gitRemoteURLs := make([]string, 0, len(i))
			exitCodes := make([]int32, 0, len(i))
			durationsMS := make([]float64, 0, len(i))

			for _, invocation := range i {
				ids = append(ids, uuid.New())

				binaryNames = append(binaryNames, filepath.Base(invocation.Executable.Path))
				binaryHashes = append(binaryHashes, invocation.Executable.Hash)
				binaryPaths = append(binaryPaths, invocation.Executable.Path)
				argsData, _ := json.Marshal(invocation.Arguments)
				binaryArgs = append(binaryArgs, argsData)
				binaryVersions = append(binaryVersions, invocation.Executable.Version)
				workingDirs = append(workingDirs, invocation.WorkingDirectory)
				gitRemoteURLs = append(gitRemoteURLs, invocation.GitRemoteUrl)
				exitCodes = append(exitCodes, invocation.ExitCode)
				durationsMS = append(durationsMS, float64(invocation.DurationMs))
			}

			binaryArgsData, _ := json.Marshal(binaryArgs)
			err := s.Database.InsertIntelInvocations(s.closeContext, database.InsertIntelInvocationsParams{
				ID:               ids,
				CreatedAt:        dbtime.Now(),
				MachineID:        s.MachineID,
				UserID:           s.UserID,
				BinaryName:       binaryNames,
				BinaryHash:       binaryHashes,
				BinaryPath:       binaryPaths,
				BinaryArgs:       binaryArgsData,
				BinaryVersion:    binaryVersions,
				WorkingDirectory: workingDirs,
				GitRemoteUrl:     gitRemoteURLs,
				ExitCode:         exitCodes,
				DurationMs:       durationsMS,
			})
			if err != nil {
				s.Logger.Error(s.closeContext, "write invocations", slog.Error(err))
				// Just ignore the failure and ignore the invocations.
				// It's not a big deal... the bigger deal is keeping
				// too big of a queue and failing.
				return nil
			}
			s.Logger.Info(s.closeContext, "invocations flushed", slog.F("count", len(i)))
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, yamux.ErrSessionShutdown) {
				return
			}
			s.Logger.Warn(s.closeContext, "failed to write invocations", slog.Error(err))
		}
	}
}

func (s *server) Listen(req *proto.ListenRequest, stream proto.DRPCIntelDaemon_ListenStream) error {
	// TODO: Move this centrally so on update a single query is fired instead!
	cohorts, err := s.Database.GetIntelCohortsMatchedByMachineIDs(stream.Context(), []uuid.UUID{s.MachineID})
	if err != nil {
		return xerrors.Errorf("get intel cohorts: %w", err)
	}
	executablesToTrack := []string{}
	for _, cohort := range cohorts {
		if cohort.MachineID != s.MachineID {
			continue
		}
		executablesToTrack = append(executablesToTrack, cohort.TrackedExecutables...)
	}
	err = stream.Send(&proto.SystemResponse{
		Message: &proto.SystemResponse_TrackExecutables{
			TrackExecutables: &proto.TrackExecutables{
				BinaryName: executablesToTrack,
			},
		},
	})
	if err != nil {
		return xerrors.Errorf("send track executables: %w", err)
	}
	for {
		select {
		// TODO: Listen for updates here!
		case <-stream.Context().Done():
			return nil
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
