package coderd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/coderd/coderdtest"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/inteld/proto"
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
			IntelDaemonHostInfo: codersdk.IntelDaemonHostInfo{
				OperatingSystem: "linux",
			},
			InstanceID: "test",
		})
		require.NoError(t, err)
		defer firstClient.DRPCConn().Close()
		secondClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			IntelDaemonHostInfo: codersdk.IntelDaemonHostInfo{
				OperatingSystem: "windows",
			},
			InstanceID: "test",
		})
		require.NoError(t, err)
		defer secondClient.DRPCConn().Close()

		res, err := client.IntelMachines(context.Background(), user.OrganizationID, codersdk.IntelMachinesRequest{
			RegexFilters: codersdk.IntelCohortRegexFilters{
				OperatingSystem: "windows",
			},
		})
		require.NoError(t, err)
		require.Len(t, res.IntelMachines, 1)
		require.Equal(t, res.Count, 1)
		require.Equal(t, res.IntelMachines[0].OperatingSystem, "windows")
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
}

func TestIntelReport(t *testing.T) {
	t.Parallel()
	t.Run("Empty", func(t *testing.T) {
		t.Parallel()
		client := coderdtest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		ctx := context.Background()
		cohort, err := client.CreateIntelCohort(ctx, user.OrganizationID, codersdk.CreateIntelCohortRequest{
			Name: "Everyone",
		})
		require.NoError(t, err)
		report, err := client.IntelReport(ctx, user.OrganizationID, codersdk.IntelReportRequest{
			CohortIDs: []uuid.UUID{cohort.ID},
		})
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
		cohort, err := client.CreateIntelCohort(ctx, user.OrganizationID, codersdk.CreateIntelCohortRequest{
			Name: "Everyone",
		})
		require.NoError(t, err)
		firstClient, err := client.ServeIntelDaemon(ctx, user.OrganizationID, codersdk.ServeIntelDaemonRequest{
			IntelDaemonHostInfo: codersdk.IntelDaemonHostInfo{
				OperatingSystem: "linux",
			},
			InstanceID: "test",
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
		report, err := client.IntelReport(ctx, user.OrganizationID, codersdk.IntelReportRequest{
			CohortIDs: []uuid.UUID{cohort.ID},
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), report.Invocations)
		fmt.Printf("%+v\n", report)
	})
}
