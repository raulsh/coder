package insightd_test

import (
	"context"
	"testing"

	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"

	"github.com/coder/coder/v2/codersdk/drpc"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/insightd"
	"github.com/coder/coder/v2/insightd/proto"
)

func TestInsightd(t *testing.T) {
	t.Parallel()
	t.Run("InstantClose", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		t.Cleanup(func() {
			close(done)
		})
		daemon := insightd.New(insightd.Options{
			Dialer: func(ctx context.Context) (proto.DRPCInsightDaemonClient, error) {
				return createInsightDaemonClient(t, done, insightdServer{}), nil
			},
		})
		require.NoError(t, daemon.Close())
	})
	t.Run("InstantlyRegisters", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		t.Cleanup(func() {
			close(done)
		})
		registered := make(chan struct{})
		daemon := insightd.New(insightd.Options{
			Dialer: func(ctx context.Context) (proto.DRPCInsightDaemonClient, error) {
				return createInsightDaemonClient(t, done, insightdServer{
					register: func(req *proto.RegisterRequest, _ proto.DRPCInsightDaemon_RegisterStream) error {
						close(registered)
						return nil
					},
				}), nil
			},
		})
		select {
		case <-registered:
		case <-done:
			t.Error("test ended before registration")
		}
		require.NoError(t, daemon.Close())
	})
}

type insightdServer struct {
	register         func(req *proto.RegisterRequest, stream proto.DRPCInsightDaemon_RegisterStream) error
	recordInvocation func(context.Context, *proto.ReportInvocationRequest) (*proto.Empty, error)
	reportPath       func(context.Context, *proto.ReportPathRequest) (*proto.Empty, error)
}

func (i *insightdServer) Register(req *proto.RegisterRequest, stream proto.DRPCInsightDaemon_RegisterStream) error {
	if i.register == nil {
		return nil
	}
	return i.register(req, stream)
}

func (i *insightdServer) RecordInvocation(ctx context.Context, inv *proto.ReportInvocationRequest) (*proto.Empty, error) {
	if i.recordInvocation == nil {
		return &proto.Empty{}, nil
	}
	return i.recordInvocation(ctx, inv)
}

func (i *insightdServer) ReportPath(ctx context.Context, req *proto.ReportPathRequest) (*proto.Empty, error) {
	if i.reportPath == nil {
		return &proto.Empty{}, nil
	}
	return i.reportPath(ctx, req)
}

func createInsightDaemonClient(t *testing.T, done <-chan struct{}, server insightdServer) proto.DRPCInsightDaemonClient {
	t.Helper()
	clientPipe, serverPipe := drpc.MemTransportPipe()
	t.Cleanup(func() {
		_ = clientPipe.Close()
		_ = serverPipe.Close()
	})
	mux := drpcmux.New()
	err := proto.DRPCRegisterInsightDaemon(mux, &server)
	require.NoError(t, err)
	srv := drpcserver.New(mux)
	ctx, cancelFunc := context.WithCancel(context.Background())
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		_ = srv.Serve(ctx, serverPipe)
	}()
	t.Cleanup(func() {
		cancelFunc()
		<-closed
		select {
		case <-done:
			t.Error("createInsightDaemonClient cleanup after test was done!")
		default:
		}
	})
	select {
	case <-done:
		t.Error("called createInsightDaemonClient after test was done!")
	default:
	}
	return proto.NewDRPCInsightDaemonClient(clientPipe)
}
