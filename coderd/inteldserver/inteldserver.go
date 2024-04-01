package inteldserver

import (
	"context"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/inteld/proto"
)

type Options struct {
	Database database.Store
}

func New(ctx context.Context, opts Options) (proto.DRPCIntelDaemonServer, error) {
	return nil, nil
}

type server struct {
}

func (s *server) Close() error {
	return nil
}
