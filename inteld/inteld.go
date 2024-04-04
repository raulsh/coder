package inteld

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ammario/tlru"
	"github.com/elastic/go-sysinfo"
	"github.com/hashicorp/yamux"
	"github.com/spf13/afero"
	"golang.org/x/xerrors"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/inteld/pathman"
	"github.com/coder/coder/v2/inteld/proto"
	"github.com/coder/retry"

	"github.com/kalafut/imohash"
)

type Dialer func(ctx context.Context) (proto.DRPCIntelDaemonClient, error)

type Options struct {
	// Dialer connects the daemon to a client.
	Dialer Dialer

	Filesystem afero.Fs

	// InvokeBinary is the path to the binary that will be
	// associated with aliased commands.
	InvokeBinary string

	// InvokeDirectory is the directory where binaries are aliased
	// to and overridden in the $PATH so they can be man-in-the-middled.
	InvokeDirectory string

	// InvocationFlushInterval is the interval at which invocations
	// are flushed to the server.
	InvocationFlushInterval time.Duration

	Logger slog.Logger
}

func New(opts Options) *API {
	if opts.Dialer == nil {
		panic("Dialer is required")
	}
	if opts.InvocationFlushInterval == 0 {
		opts.InvocationFlushInterval = 5 * time.Second
	}
	if opts.Filesystem == nil {
		opts.Filesystem = afero.NewOsFs()
	}
	closeContext, closeCancel := context.WithCancel(context.Background())
	invocationQueue := newInvocationQueue(opts.InvocationFlushInterval)
	api := &API{
		clientDialer:    opts.Dialer,
		clientChan:      make(chan proto.DRPCIntelDaemonClient),
		closeContext:    closeContext,
		closeCancel:     closeCancel,
		filesystem:      opts.Filesystem,
		logger:          opts.Logger,
		invokeDirectory: opts.InvokeDirectory,
		invokeBinary:    opts.InvokeBinary,
		invocationQueue: invocationQueue,
	}
	api.closeWaitGroup.Add(3)
	go api.invocationQueueLoop()
	go api.connectLoop()
	go api.registerLoop()
	return api
}

// API serves an instance of the intel daemon.
type API struct {
	filesystem      afero.Fs
	invokeBinary    string
	invokeDirectory string
	invocationQueue *invocationQueue

	clientDialer   Dialer
	clientChan     chan proto.DRPCIntelDaemonClient
	closeContext   context.Context
	closeCancel    context.CancelFunc
	closed         bool
	closeMutex     sync.Mutex
	closeWaitGroup sync.WaitGroup
	logger         slog.Logger
}

// invocationQueueLoop ensures that invocations are sent to the server
// at a regular interval.
func (a *API) invocationQueueLoop() {
	defer a.logger.Debug(a.closeContext, "invocation queue loop exited")
	defer a.closeWaitGroup.Done()
	for {
		err := a.invocationQueue.startSendLoop(a.closeContext, func(i []*proto.Invocation) error {
			client, ok := a.client()
			if !ok {
				return xerrors.New("no client available")
			}
			_, err := client.RecordInvocation(a.closeContext, &proto.RecordInvocationRequest{
				Invocations: i,
			})
			return err
		})
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, yamux.ErrSessionShutdown) {
				return
			}
			a.logger.Warn(a.closeContext, "unable to receive a message", slog.Error(err))
		}
	}
}

// registerLoop starts a loop that registers the system with the server.
func (a *API) registerLoop() {
	defer a.logger.Debug(a.closeContext, "system loop exited")
	defer a.closeWaitGroup.Done()
	for {
		client, ok := a.client()
		if !ok {
			a.logger.Debug(a.closeContext, "shut down before client (re) connected")
			return
		}
		err := pathman.Prepend(a.closeContext, a.filesystem, a.invokeDirectory)
		if err != nil {
			a.logger.Error(a.closeContext, "unable to prepend invoke directory to PATH", slog.Error(err))
		}
		userEmail, err := fetchFromGitConfig("user.email")
		if err != nil {
			a.logger.Warn(a.closeContext, "unable to fetch user.email from git config", slog.Error(err))
		}
		userName, err := fetchFromGitConfig("user.name")
		if err != nil {
			a.logger.Warn(a.closeContext, "unable to fetch user.name from git config", slog.Error(err))
		}
		var (
			hostname    string
			osVersion   string
			memoryTotal uint64
		)
		sysInfoHost, err := sysinfo.Host()
		if err == nil {
			info := sysInfoHost.Info()
			osVersion = info.OS.Version
			hostname = info.Hostname
			mem, err := sysInfoHost.Memory()
			if err == nil {
				memoryTotal = mem.Total
			}
		} else {
			a.logger.Warn(a.closeContext, "unable to fetch machine information", slog.Error(err))
		}
		system, err := client.Register(a.closeContext, &proto.RegisterRequest{
			Hostname:               hostname,
			OperatingSystem:        runtime.GOOS,
			Architecture:           runtime.GOARCH,
			OperatingSystemVersion: osVersion,
			CpuCores:               uint32(runtime.NumCPU()),
			MemoryTotal:            memoryTotal,
			GitConfigEmail:         userEmail,
			GitConfigName:          userName,
			InstalledSoftware:      &proto.InstalledSoftware{
				// TODO: Make this valid!
			},
		})
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, yamux.ErrSessionShutdown) {
				continue
			}
		}
		a.systemRecvLoop(system)
	}
}

func (a *API) systemRecvLoop(client proto.DRPCIntelDaemon_RegisterClient) {
	ctx := a.closeContext
	for {
		resp, err := client.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, yamux.ErrSessionShutdown) {
				return
			}

			a.logger.Warn(ctx, "unable to receive a message", slog.Error(err))
			return
		}

		switch m := resp.Message.(type) {
		case *proto.SystemResponse_TrackExecutables:
			err = a.trackExecutables(m.TrackExecutables.BinaryName)
			if err != nil {
				// TODO: send an error back to the server
				a.logger.Warn(ctx, "unable to track executables", slog.Error(err))
			}
			a.logger.Info(ctx, "tracked executables", slog.F("binary_names", m.TrackExecutables.BinaryName))
		}
	}
}

// trackExecutables creates symlinks in the invoke directory for the
// given binary names.
func (a *API) trackExecutables(binaryNames []string) error {
	// Clear out any existing symlinks so we're only tracking the
	// executables we're told to track.
	files, err := afero.ReadDir(a.filesystem, a.invokeDirectory)
	if errors.Is(err, os.ErrNotExist) {
		err = a.filesystem.MkdirAll(a.invokeDirectory, 0755)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	for _, file := range files {
		// Clear out the directory to remove old filenames.
		// Don't do this for the global dir because it makes
		// debugging harder.
		err = a.filesystem.Remove(filepath.Join(a.invokeDirectory, file.Name()))
		if err != nil {
			return err
		}
	}
	err = a.filesystem.MkdirAll(a.invokeDirectory, 0755)
	if err != nil {
		return err
	}
	linker, ok := a.filesystem.(afero.Linker)
	if !ok {
		return xerrors.New("filesystem does not support symlinks")
	}
	for _, binaryName := range binaryNames {
		err = linker.SymlinkIfPossible(a.invokeBinary, filepath.Join(a.invokeDirectory, binaryName))
		if err != nil {
			return err
		}
	}
	return nil
}

// connectLoop starts a loop that attempts to connect to coderd.
func (a *API) connectLoop() {
	defer a.logger.Debug(a.closeContext, "connect loop exited")
	defer a.closeWaitGroup.Done()
connectLoop:
	for retrier := retry.New(50*time.Millisecond, 10*time.Second); retrier.Wait(a.closeContext); {
		a.logger.Debug(a.closeContext, "dialing coderd")
		client, err := a.clientDialer(a.closeContext)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			var sdkErr *codersdk.Error
			// If something is wrong with our auth, stop trying to connect.
			if errors.As(err, &sdkErr) && sdkErr.StatusCode() == http.StatusForbidden {
				a.logger.Error(a.closeContext, "not authorized to dial coderd", slog.Error(err))
				return
			}
			if a.isClosed() {
				return
			}
			a.logger.Warn(a.closeContext, "coderd client failed to dial", slog.Error(err))
			continue
		}
		a.logger.Info(a.closeContext, "successfully connected to coderd")
		retrier.Reset()

		// serve the client until we are closed or it disconnects
		for {
			select {
			case <-a.closeContext.Done():
				client.DRPCConn().Close()
				return
			case <-client.DRPCConn().Closed():
				a.logger.Info(a.closeContext, "connection to coderd closed")
				continue connectLoop
			case a.clientChan <- client:
				continue
			}
		}
	}
}

// client returns the current client or nil if the API is closed
func (a *API) client() (proto.DRPCIntelDaemonClient, bool) {
	select {
	case <-a.closeContext.Done():
		return nil, false
	case client := <-a.clientChan:
		return client, true
	}
}

// isClosed returns whether the API is closed or not.
func (a *API) isClosed() bool {
	select {
	case <-a.closeContext.Done():
		return true
	default:
		return false
	}
}

func (a *API) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	a.closeCancel()
	a.closeWaitGroup.Wait()
	return nil
}

// fetchFromGitConfig returns the value of a property from the git config.
// If the property is not found, it returns an empty string.
// If git is not installed, it returns an empty string.
func fetchFromGitConfig(property string) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", nil
	}
	cmd := exec.Command(gitPath, "config", "--get", property)
	output, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

var _ proto.DRPCIntelClientServer = (*API)(nil)

// ReportInvocation is called by the client to report an invocation.
func (a *API) ReportInvocation(_ context.Context, req *proto.ReportInvocationRequest) (*proto.Empty, error) {
	a.invocationQueue.enqueue(req)
	return &proto.Empty{}, nil
}

func newInvocationQueue(flushInterval time.Duration) *invocationQueue {
	return &invocationQueue{
		Cond:           sync.NewCond(&sync.Mutex{}),
		flushInterval:  flushInterval,
		binaryCache:    tlru.New[string, *proto.Executable](tlru.ConstantCost, 1000),
		gitRemoteCache: tlru.New[string, string](tlru.ConstantCost, 1000),
	}
}

type invocationQueue struct {
	*sync.Cond
	flushInterval  time.Duration
	queue          []*proto.Invocation
	flushRequested bool
	lastFlush      time.Time
	binaryCache    *tlru.Cache[string, *proto.Executable]
	gitRemoteCache *tlru.Cache[string, string]
	logger         slog.Logger
}

func (i *invocationQueue) enqueue(req *proto.ReportInvocationRequest) {
	inv := &proto.Invocation{
		Arguments:        req.Arguments,
		DurationMs:       req.DurationMs,
		ExitCode:         req.ExitCode,
		WorkingDirectory: req.WorkingDirectory,
	}

	var err error
	// We check if this is non-empty purely for testing. It's
	// expected in production that this is always set.
	if req.ExecutablePath != "" {
		inv.Executable, err = i.binaryCache.Do(req.ExecutablePath, func() (*proto.Executable, error) {
			rawHash, err := imohash.SumFile(req.ExecutablePath)
			if err != nil {
				return nil, err
			}
			hash := fmt.Sprintf("%X", rawHash)

			return &proto.Executable{
				Hash:     hash,
				Basename: filepath.Base(req.ExecutablePath),
				Path:     req.ExecutablePath,
			}, nil
		}, 24*time.Hour)
		if err != nil {
			i.logger.Error(context.Background(), "failed to inspect executable", slog.Error(err))
			return
		}
	}

	// We check if this is non-empty purely for testing. It's
	// expected in production that this is always set.
	if req.WorkingDirectory != "" {
		inv.GitRemoteUrl, err = i.gitRemoteCache.Do(req.WorkingDirectory, func() (string, error) {
			cmd := exec.Command("git", "remote", "get-url", "origin")
			cmd.Dir = req.WorkingDirectory
			out, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(out)), nil
		}, time.Hour)
		if err != nil {
			// This isn't worth failing the execution on, but is an issue
			// that is worth reporting!
			i.logger.Error(context.Background(), "failed to inspect git remote", slog.Error(err))
		}
	}

	i.L.Lock()
	defer i.L.Unlock()
	i.queue = append(i.queue, inv)
	i.Broadcast()
}

func (i *invocationQueue) startSendLoop(ctx context.Context, flush func([]*proto.Invocation) error) error {
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
