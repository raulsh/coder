package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/inteld/proto"
)

func main() {
	runtime.LockOSThread()
	runtime.GOMAXPROCS(1)
	err := run(context.Background())
	if err != nil && os.Getenv("CODER_INTEL_INVOKE_DEBUG") != "" {
		_, _ = fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	pathParts := filepath.SplitList(os.Getenv("PATH"))
	baseName := filepath.Base(os.Args[0])
	currentPath, err := exec.LookPath(baseName)
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
	err = os.Setenv("PATH", strings.Join(pathParts, string(filepath.ListSeparator)))
	if err != nil {
		return err
	}
	currentPath, err = exec.LookPath(baseName)
	if err != nil {
		return err
	}
	currentPath, err = filepath.Abs(currentPath)
	if err != nil {
		return err
	}
	currentExec, err := os.Executable()
	if err != nil {
		return err
	}
	if currentPath == currentExec {
		return xerrors.New("supposed to be linked")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	//nolint:gosec
	cmd := exec.CommandContext(ctx, baseName, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	start := time.Now()
	err = cmd.Run()
	end := time.Now()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
	}

	return proto.ReportInvocation(&proto.ReportInvocationRequest{
		ExecutablePath:   currentPath,
		Arguments:        os.Args[1:],
		DurationMs:       end.Sub(start).Milliseconds(),
		ExitCode:         int32(exitCode),
		WorkingDirectory: wd,
	})
}
