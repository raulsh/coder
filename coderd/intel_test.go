package coderd_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"cdr.dev/slog/sloggers/slogtest"

	"github.com/coder/coder/v2/coderd/coderdtest"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/inteld"
	"github.com/coder/coder/v2/inteld/proto"
	"github.com/coder/coder/v2/testutil"
)

func TestIntelMachines(t *testing.T) {
	t.Parallel()
	t.Run("Empty", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)

		res, err := client.IntelMachines(context.Background(), user.OrganizationID, codersdk.IntelMachinesRequest{})
		require.NoError(t, err)
		require.Len(t, res.IntelMachines, 0)
		require.Equal(t, res.Count, 0)
	})
	t.Run("Single", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		intelClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			InstanceID: "test",
		})
		require.NoError(t, err)
		defer intelClient.DRPCConn().Close()

		res, err := client.IntelMachines(context.Background(), user.OrganizationID, codersdk.IntelMachinesRequest{})
		require.NoError(t, err)
		require.Len(t, res.IntelMachines, 1)
		require.Equal(t, res.Count, 1)
	})
	t.Run("Filtered", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		firstClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			Metadata: map[string]string{
				"operating_system": "linux",
			},
			InstanceID: "test",
		})
		require.NoError(t, err)
		defer firstClient.DRPCConn().Close()
		secondClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			Metadata: map[string]string{
				"operating_system": "windows",
			},
			InstanceID: "test",
		})
		require.NoError(t, err)
		defer secondClient.DRPCConn().Close()

		res, err := client.IntelMachines(context.Background(), user.OrganizationID, codersdk.IntelMachinesRequest{
			MetadataMatch: map[string]*regexp.Regexp{
				"operating_system": regexp.MustCompile("windows"),
			},
		})
		require.NoError(t, err)
		require.Len(t, res.IntelMachines, 1)
		require.Equal(t, res.Count, 1)
		require.Equal(t, res.IntelMachines[0].Metadata["operating_system"], "windows")
	})
}

func TestIntelCohorts(t *testing.T) {
	t.Parallel()
	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		cohort, err := client.CreateIntelCohort(ctx, user.OrganizationID, codersdk.CreateIntelCohortRequest{
			Name: "example",
		})
		require.NoError(t, err)
		require.Equal(t, cohort.Name, "example")
	})
	t.Run("TrackedBinaries", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		// Create a cohort that matches the machine.
		_, err := client.CreateIntelCohort(ctx, user.OrganizationID, codersdk.CreateIntelCohortRequest{
			Name:               "example",
			TrackedExecutables: []string{"go"},
		})
		require.NoError(t, err)
		fs := &linkerFS{
			Fs:    afero.NewMemMapFs(),
			links: map[string]string{},
		}
		closer := inteld.New(inteld.Options{
			Dialer: func(ctx context.Context, metadata map[string]string) (proto.DRPCIntelDaemonClient, error) {
				return client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
					Metadata:   metadata,
					InstanceID: "example",
				})
			},
			Logger:          slogtest.Make(t, nil).Named("inteld"),
			Filesystem:      fs,
			InvokeDirectory: "/tmp",
		})
		defer closer.Close()
		require.Eventually(t, func() bool {
			return fs.links["/tmp/go"] == "/tmp/coder-intel-invoke"
		}, testutil.WaitShort, testutil.IntervalFast)
	})
}

func TestIntelReport(t *testing.T) {
	t.Parallel()
	t.Run("Empty", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		report, err := client.IntelReport(ctx, user.OrganizationID, codersdk.IntelReportRequest{})
		require.NoError(t, err)
		// No invocations of course!
		require.Equal(t, int64(0), report.Invocations)
	})
	t.Run("Basic", func(t *testing.T) {
		t.Parallel()
		flushInterval := 25 * time.Millisecond
		client := coderdtest.New(t, &coderdtest.Options{
			IntelServerInvocationFlushInterval: flushInterval,
		})
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		firstClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			InstanceID: "test",
			Metadata: map[string]string{
				"operating_system": "linux",
			},
		})
		require.NoError(t, err)
		defer firstClient.DRPCConn().Close()
		_, err = firstClient.RecordInvocation(ctx, &proto.RecordInvocationRequest{
			Invocations: []*proto.Invocation{{
				Executable: &proto.Executable{
					Hash:     "hash",
					Basename: "go",
					Path:     "/usr/bin/go",
					Version:  "1.0.0",
				},
				Arguments:        []string{"run", "main.go"},
				DurationMs:       354,
				ExitCode:         1,
				WorkingDirectory: "/home/coder",
				GitRemoteUrl:     "https://github.com/coder/coder",
			}},
		})
		require.NoError(t, err)

		// TODO: @kylecarbs this is obviously a racey piece of code...
		// Wait for invocations to flush
		<-time.After(flushInterval * 2)

		err = client.RefreshIntelReport(ctx, user.OrganizationID)
		require.NoError(t, err)
		report, err := client.IntelReport(ctx, user.OrganizationID, codersdk.IntelReportRequest{})
		require.NoError(t, err)
		require.Equal(t, int64(1), report.Invocations)
		fmt.Printf("%+v\n", report)
	})
}

type linkerFS struct {
	afero.Fs
	links map[string]string
}

func (fs *linkerFS) SymlinkIfPossible(oldname, newname string) error {
	fs.links[newname] = oldname
	return nil
}
