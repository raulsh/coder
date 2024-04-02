package main

import (
	"fmt"
	"io"

	"github.com/hashicorp/yamux"

	"github.com/coder/coder/v2/codersdk/drpc"
	"github.com/coder/coder/v2/inteld/proto"
)

func main() {
	fmt.Printf("Hello!\n")

	// drpc.MultiplexedConn()

}

func run() error {

	// Multiplexes the incoming connection using yamux.
	// This allows multiple function calls to occur over
	// the same connection.
	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard

	session, err := yamux.Client(nil, config)
	if err != nil {
		return err
	}
	proto.NewDRPCIntelClientClient(drpc.MultiplexedConn(session))

	return nil
}
