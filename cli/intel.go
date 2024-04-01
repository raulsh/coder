package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"golang.org/x/xerrors"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/sloghuman"
	"github.com/coder/coder/v2/cli/clilog"
	"github.com/coder/coder/v2/cli/cliui"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/inteld"
	"github.com/coder/coder/v2/inteld/proto"
	"github.com/coder/serpent"
)

func (r *RootCmd) intelDaemon() *serpent.Command {
	cmd := &serpent.Command{
		Use:   "inteld",
		Short: "Manage the Intel Daemon",
		Handler: func(i *serpent.Invocation) error {
			return nil
		},
		Children: []*serpent.Command{
			r.intelDaemonStart(),
		},
	}
	return cmd
}

func (r *RootCmd) intelDaemonStart() *serpent.Command {
	var (
		logHuman        string
		logJSON         string
		logStackdriver  string
		logFilter       []string
		invokeDirectory string
		verbose         bool
	)
	client := new(codersdk.Client)
	cmd := &serpent.Command{
		Use:   "start",
		Short: "Start the Intel Daemon",
		Middleware: serpent.Chain(
			r.InitClient(client),
		),
		Handler: func(inv *serpent.Invocation) error {
			ctx, cancel := context.WithCancel(inv.Context())
			defer cancel()

			stopCtx, stopCancel := inv.SignalNotifyContext(ctx, StopSignalsNoInterrupt...)
			defer stopCancel()
			interruptCtx, interruptCancel := inv.SignalNotifyContext(ctx, InterruptSignals...)
			defer interruptCancel()

			logOpts := []clilog.Option{
				clilog.WithFilter(logFilter...),
				clilog.WithHuman(logHuman),
				clilog.WithJSON(logJSON),
				clilog.WithStackdriver(logStackdriver),
			}
			if verbose {
				logOpts = append(logOpts, clilog.WithVerbose())
			}

			logger, closeLogger, err := clilog.New(logOpts...).Build(inv)
			if err != nil {
				// Fall back to a basic logger
				logger = slog.Make(sloghuman.Sink(inv.Stderr))
				logger.Error(ctx, "failed to initialize logger", slog.Error(err))
			} else {
				defer closeLogger()
			}

			logger.Info(ctx, "starting intel daemon")

			srv := inteld.New(inteld.Options{
				Dialer: func(ctx context.Context) (proto.DRPCIntelDaemonClient, error) {
					return client.ServeIntelDaemon(ctx, codersdk.ServeIntelDaemonRequest{})
				},
				Logger:          logger,
				Filesystem:      afero.NewOsFs(),
				InvokeBinary:    "/tmp/bypass",
				InvokeDirectory: invokeDirectory,
			})

			waitForReporting := false
			var exitErr error
			select {
			case <-stopCtx.Done():
				exitErr = stopCtx.Err()
				_, _ = fmt.Fprintln(inv.Stdout, cliui.Bold(
					"Stop caught, waiting for intel to report and gracefully exiting. Use ctrl+\\ to force quit",
				))
				waitForReporting = true
			case <-interruptCtx.Done():
				exitErr = interruptCtx.Err()
				_, _ = fmt.Fprintln(inv.Stdout, cliui.Bold(
					"Interrupt caught, gracefully exiting. Use ctrl+\\ to force quit",
				))
			}
			if exitErr != nil && !xerrors.Is(exitErr, context.Canceled) {
				cliui.Errorf(inv.Stderr, "Unexpected error, shutting down daemon: %s\n", exitErr)
			}
			// TODO: Make this work!
			_ = waitForReporting

			err = srv.Close()
			if err != nil {
				return xerrors.Errorf("shutdown: %w", err)
			}

			cancel()
			if xerrors.Is(exitErr, context.Canceled) {
				return nil
			}
			return exitErr
		},
	}
	cmd.Options = serpent.OptionSet{
		{
			Flag:        "verbose",
			Env:         "CODER_INTEL_DAEMON_VERBOSE",
			Description: "Output debug-level logs.",
			Value:       serpent.BoolOf(&verbose),
			Default:     "false",
		},
		{
			Flag:        "log-human",
			Env:         "CODER_INTEL_DAEMON_LOGGING_HUMAN",
			Description: "Output human-readable logs to a given file.",
			Value:       serpent.StringOf(&logHuman),
			Default:     "/dev/stderr",
		},
		{
			Flag:        "log-json",
			Env:         "CODER_INTEL_DAEMON_LOGGING_JSON",
			Description: "Output JSON logs to a given file.",
			Value:       serpent.StringOf(&logJSON),
			Default:     "",
		},
		{
			Flag:        "log-stackdriver",
			Env:         "CODER_INTEL_DAEMON_LOGGING_STACKDRIVER",
			Description: "Output Stackdriver compatible logs to a given file.",
			Value:       serpent.StringOf(&logStackdriver),
			Default:     "",
		},
		{
			Flag:        "log-filter",
			Env:         "CODER_INTEL_DAEMON_LOG_FILTER",
			Description: "Filter debug logs by matching against a given regex. Use .* to match all debug logs.",
			Value:       serpent.StringArrayOf(&logFilter),
			Default:     "",
		},
		{
			Flag:        "invoke-directory",
			Env:         "CODER_INTEL_DAEMON_INVOKE_DIRECTORY",
			Description: "The directory where binaries are aliased to and overridden in the $PATH so they can be tracked.",
			Value:       serpent.StringOf(&invokeDirectory),
			Default:     defaultInvokeDirectory(),
		},
	}
	return cmd
}

func defaultInvokeDirectory() string {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(homeDir, ".coder-intel", "bin")
	}
	return filepath.Join(os.TempDir(), ".coder-intel", "bin")
}
