package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"storj.io/drpc/drpcconn"
	"tailscale.com/safesocket"

	"github.com/coder/coder/v2/inteld/proto"
)

func main() {
	runtime.LockOSThread()
	runtime.GOMAXPROCS(1)
	_ = run(context.Background())
}

func run(ctx context.Context) error {
	// go = os.Args[0]
	//

	// start := time.Now()

	// addr := os.Getenv("CODER_INTEL_DAEMON_ADDRESS")
	// if addr == "" {
	// 	addr = "localhost:13337"
	// }

	safesocket.Connect(safesocket.DefaultConnectionStrategy("asdasd"))

	c, _ := net.Dial("tcp", "localhost:3000")

	// ctx, cancelFunc := context.WithCancel(context.Background())

	pathParts := filepath.SplitList(os.Getenv("PATH"))
	currentPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}
	toRemove := filepath.Dir(currentPath)
	for i, part := range pathParts {
		if part == toRemove {
			pathParts = append(pathParts[:i], pathParts[i+1:]...)
			break
		}
	}
	os.Setenv("PATH", strings.Join(pathParts, string(filepath.ListSeparator)))
	currentPath, err = exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			exitCode = e.ExitCode()
		}
	}

	client := proto.NewDRPCIntelClientClient(drpcconn.New(c))
	_, err = client.ReportInvocation(ctx, &proto.ReportInvocationRequest{
		ExecutablePath:   currentPath,
		Arguments:        os.Args[1:],
		DurationMs:       time.Since(start).Milliseconds(),
		ExitCode:         int32(exitCode),
		WorkingDirectory: wd,
	})

	return nil
}
