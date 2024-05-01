package coderd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/coderd/coderdtest"
	"github.com/coder/coder/v2/codersdk"
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
			Name: "example",
		})
		require.NoError(t, err)
		report, err := client.IntelReport(ctx, user.OrganizationID, codersdk.IntelReportRequest{
			CohortIDs: []uuid.UUID{cohort.ID},
		})
		require.NoError(t, err)
		// require.Equal(t, )
		fmt.Printf("%+v\n", report)
	})
}
