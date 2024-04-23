package inteld_test

import (
	"context"
	"testing"

	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"

	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/codersdk/drpc"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/inteld"
	"github.com/coder/coder/v2/inteld/proto"
)

func TestInteld(t *testing.T) {
	t.Parallel()
	t.Run("InstantClose", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		t.Cleanup(func() {
			close(done)
		})
		daemon := inteld.New(inteld.Options{
			Dialer: func(ctx context.Context, _ codersdk.IntelDaemonHostInfo) (proto.DRPCIntelDaemonClient, error) {
				return createIntelDaemonClient(t, done, inteldServer{}), nil
			},
		})
		require.NoError(t, daemon.Close())
	})
	t.Run("InstantlyListens", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		listened := make(chan struct{})
		t.Cleanup(func() {
			close(done)
		})
		daemon := inteld.New(inteld.Options{
			Dialer: func(ctx context.Context, _ codersdk.IntelDaemonHostInfo) (proto.DRPCIntelDaemonClient, error) {
				return createIntelDaemonClient(t, done, inteldServer{
					listen: func(_ *proto.ListenRequest, stream proto.DRPCIntelDaemon_ListenStream) error {
						listened <- struct{}{}
						<-ctx.Done()
						return nil
					},
				}), nil
			},
		})
		select {
		case <-listened:
		case <-done:
			t.Error("test ended before registration")
		}
		require.NoError(t, daemon.Close())
	})
}

type inteldServer struct {
	listen           func(req *proto.ListenRequest, stream proto.DRPCIntelDaemon_ListenStream) error
	recordInvocation func(context.Context, *proto.RecordInvocationRequest) (*proto.Empty, error)
	reportPath       func(context.Context, *proto.ReportPathRequest) (*proto.Empty, error)
}

func (i *inteldServer) Listen(req *proto.ListenRequest, stream proto.DRPCIntelDaemon_ListenStream) error {
	if i.listen == nil {
		return nil
	}
	return i.listen(req, stream)
}

func (i *inteldServer) RecordInvocation(ctx context.Context, inv *proto.RecordInvocationRequest) (*proto.Empty, error) {
	if i.recordInvocation == nil {
		return &proto.Empty{}, nil
	}
	return i.recordInvocation(ctx, inv)
}

func (i *inteldServer) ReportPath(ctx context.Context, req *proto.ReportPathRequest) (*proto.Empty, error) {
	if i.reportPath == nil {
		return &proto.Empty{}, nil
	}
	return i.reportPath(ctx, req)
}

func createIntelDaemonClient(t *testing.T, done <-chan struct{}, server inteldServer) proto.DRPCIntelDaemonClient {
	t.Helper()
	clientPipe, serverPipe := drpc.MemTransportPipe()
	t.Cleanup(func() {
		_ = clientPipe.Close()
		_ = serverPipe.Close()
	})
	mux := drpcmux.New()
	err := proto.DRPCRegisterIntelDaemon(mux, &server)
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
			t.Error("createIntelDaemonClient cleanup after test was done!")
		default:
		}
	})
	select {
	case <-done:
		t.Error("called createIntelDaemonClient after test was done!")
	default:
	}
	return proto.NewDRPCIntelDaemonClient(clientPipe)
}
