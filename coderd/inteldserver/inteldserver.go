package inteldserver

import (
	"context"
	"time"

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

func (s *server) RecordInvocation(ctx context.Context, req *proto.ReportInvocationRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *server) ReportPath(_ context.Context, _ *proto.ReportPathRequest) (*proto.Empty, error) {
	return &proto.Empty{}, nil
}

func (s *server) Close() error {
	return nil
}
