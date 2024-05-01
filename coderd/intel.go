package coderd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	"github.com/sqlc-dev/pqtype"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"nhooyr.io/websocket"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtime"
	"github.com/coder/coder/v2/coderd/httpapi"
	"github.com/coder/coder/v2/coderd/httpmw"
	"github.com/coder/coder/v2/coderd/inteldserver"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/inteld/proto"
)

// intelReport returns a report of invocations for a set of cohorts.
//
// @Summary Get intel report
// @ID get-intel-report
// @Security CoderSessionToken
// @Produce json
// @Tags Intel
// @Param organization path string true "Organization ID" format(uuid)
// @Param cohort_id query string true "Cohort ID" format(uuid)
// @Param starts_at query string false "Starts at" format(date)
// @Success 200 {object} codersdk.IntelReport
// @Router /organizations/{organization}/intel/report [get]
func (api *API) intelReport(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req codersdk.IntelReportRequest
	q := r.URL.Query()
	rawCohortIDs := q["cohort_id"]
	req.CohortIDs = make([]uuid.UUID, 0, len(rawCohortIDs))
	for _, rawCohortID := range rawCohortIDs {
		cohortID, err := uuid.Parse(rawCohortID)
		if err != nil {
			httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
				Message: "Invalid cohort ID.",
				Detail:  err.Error(),
			})
			return
		}
		req.CohortIDs = append(req.CohortIDs, cohortID)
	}
	// Default to the beginning of time?
	rawStartsAt := q.Get("starts_at")
	if rawStartsAt != "" {
		var err error
		req.StartsAt, err = time.Parse(q.Get("starts_at"), time.DateOnly)
		if err != nil {
			httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
				Message: "Invalid starts_at.",
				Detail:  err.Error(),
			})
			return
		}
	}

	var eg errgroup.Group
	var report codersdk.IntelReport
	eg.Go(func() error {
		rows, err := api.Database.GetIntelReportGitRemotes(ctx, database.GetIntelReportGitRemotesParams{
			StartsAt:  req.StartsAt,
			CohortIds: req.CohortIDs,
		})
		if err != nil {
			return err
		}
		reportByRemote := make(map[string]codersdk.IntelReportGitRemote, len(rows))
		for _, row := range rows {
			gitRemote, ok := reportByRemote[row.GitRemoteUrl]
			if !ok {
				var externalAuthConfigID *string
				for _, extAuth := range api.ExternalAuthConfigs {
					if extAuth.Regex.MatchString(row.GitRemoteUrl) {
						externalAuthConfigID = &extAuth.ID
						break
					}
				}
				gitRemote = codersdk.IntelReportGitRemote{
					URL:                    row.GitRemoteUrl,
					ExternalAuthProviderID: externalAuthConfigID,
				}
			}
			gitRemote.Invocations += row.TotalInvocations
			gitRemote.Intervals = append(gitRemote.Intervals, codersdk.IntelReportInvocationInterval{
				StartsAt:         row.StartsAt,
				EndsAt:           row.EndsAt,
				Invocations:      row.TotalInvocations,
				MedianDurationMS: row.MedianDurationMs,
				CohortID:         row.CohortID,
			})
			reportByRemote[row.GitRemoteUrl] = gitRemote
		}
		for _, gitRemote := range reportByRemote {
			report.GitRemotes = append(report.GitRemotes, gitRemote)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := api.Database.GetIntelReportCommands(ctx, database.GetIntelReportCommandsParams{
			StartsAt:  req.StartsAt,
			CohortIds: req.CohortIDs,
		})
		if err != nil {
			return err
		}
		reportByBinary := make(map[string]codersdk.IntelReportCommand, len(rows))
		for _, row := range rows {
			// Just index by this for simplicity on lookup.
			binaryID := string(append([]byte(row.BinaryName), row.BinaryArgs...))

			command, ok := reportByBinary[binaryID]
			if !ok {
				command = codersdk.IntelReportCommand{
					BinaryName: row.BinaryName,
				}
				err = json.Unmarshal(row.BinaryArgs, &command.BinaryArgs)
				if err != nil {
					return err
				}
			}
			command.Invocations += row.TotalInvocations

			// Merge exit codes
			exitCodes := map[string]int64{}
			for _, exitCodeRaw := range row.AggregatedExitCodes {
				err = json.Unmarshal([]byte(exitCodeRaw), &exitCodes)
				if err != nil {
					return err
				}
				for exitCodeRaw, invocations := range exitCodes {
					exitCode, err := strconv.Atoi(exitCodeRaw)
					if err != nil {
						return err
					}
					command.ExitCodes[exitCode] += invocations
				}
			}
			// Merge binary paths
			binaryPaths := map[string]int64{}
			for _, binaryPathRaw := range row.AggregatedBinaryPaths {
				err = json.Unmarshal([]byte(binaryPathRaw), &binaryPaths)
				if err != nil {
					return err
				}
				for binaryPath, invocations := range binaryPaths {
					command.BinaryPaths[binaryPath] += invocations
				}
			}
			// Merge working directories
			workingDirectories := map[string]int64{}
			for _, workingDirectoryRaw := range row.AggregatedWorkingDirectories {
				err = json.Unmarshal([]byte(workingDirectoryRaw), &workingDirectories)
				if err != nil {
					return err
				}
				for workingDirectory, invocations := range workingDirectories {
					command.WorkingDirectories[workingDirectory] += invocations
				}
			}
			// Merge git remote URLs
			gitRemoteURLs := map[string]int64{}
			for _, gitRemoteURLRaw := range row.AggregatedGitRemoteUrls {
				err = json.Unmarshal([]byte(gitRemoteURLRaw), &gitRemoteURLs)
				if err != nil {
					return err
				}
				for gitRemoteURL, invocations := range gitRemoteURLs {
					command.GitRemoteURLs[gitRemoteURL] += invocations
				}
			}
			command.Intervals = append(command.Intervals, codersdk.IntelReportInvocationInterval{
				StartsAt:         row.StartsAt,
				EndsAt:           row.EndsAt,
				Invocations:      row.TotalInvocations,
				MedianDurationMS: row.MedianDurationMs,
				CohortID:         row.CohortID,
			})
			reportByBinary[binaryID] = command
		}
		for _, command := range reportByBinary {
			report.Commands = append(report.Commands, command)
			report.Invocations += command.Invocations
		}
		return nil
	})
	err := eg.Wait()
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error getting intel report.",
			Detail:  err.Error(),
		})
		return
	}
	httpapi.Write(ctx, rw, http.StatusOK, report)
}

// intelMachines returns all machines that match the given filters.
//
// @Summary List intel machines
// @ID list-intel-machines
// @Security CoderSessionToken
// @Produce json
// @Tags Intel
// @Param limit query int false "Page limit"
// @Param offset query int false "Page offset"
// @Param operating_system query string false "Regex to match a machine operating system against"
// @Param operating_system_platform query string false "Regex to match a machine operating system platform against"
// @Param operating_system_version query string false "Regex to match a machine operating system version against"
// @Param architecture query string false "Regex to match a machine architecture against"
// @Param instance_id query string false "Regex to match a machine instance ID against"
// @Success 200 {object} codersdk.IntelMachinesResponse
// @Router /insights/daus [get]
func (api *API) intelMachines(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	organization := httpmw.OrganizationParam(r)

	page, ok := parsePagination(rw, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	filters := &codersdk.IntelCohortRegexFilters{
		OperatingSystem:         query.Get("operating_system"),
		OperatingSystemPlatform: query.Get("operating_system_platform"),
		OperatingSystemVersion:  query.Get("operating_system_version"),
		Architecture:            query.Get("architecture"),
		InstanceID:              query.Get("instance_id"),
	}
	filters.Normalize()

	machineRows, err := api.Database.GetIntelMachinesMatchingFilters(ctx, database.GetIntelMachinesMatchingFiltersParams{
		OrganizationID:              organization.ID,
		RegexOperatingSystem:        filters.OperatingSystem,
		RegexOperatingSystemVersion: filters.OperatingSystemVersion,
		RegexArchitecture:           filters.Architecture,
		RegexInstanceID:             filters.InstanceID,
		LimitOpt:                    int32(page.Limit),
		OffsetOpt:                   int32(page.Offset),
	})
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error getting intel machines.",
			Detail:  err.Error(),
		})
		return
	}
	machines := make([]database.IntelMachine, 0, len(machineRows))
	count := 0
	for _, machineRow := range machineRows {
		machines = append(machines, machineRow.IntelMachine)
		count = int(machineRow.Count)
	}
	httpapi.Write(ctx, rw, http.StatusOK, codersdk.IntelMachinesResponse{
		IntelMachines: convertIntelMachines(machines),
		Count:         count,
	})
}

func (api *API) intelCohorts(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	organization := httpmw.OrganizationParam(r)

	cohorts, err := api.Database.GetIntelCohortsByOrganizationID(ctx, database.GetIntelCohortsByOrganizationIDParams{
		OrganizationID: organization.ID,
	})
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error getting intel cohorts.",
			Detail:  err.Error(),
		})
		return
	}
	httpapi.Write(ctx, rw, http.StatusOK, convertIntelCohorts(cohorts))
}

// postIntelCohorts creates a new intel cohort.
//
// @Summary Create intel cohort
// @ID create-intel-cohort
// @Security CoderSessionToken
// @Tags Intel
// @Param organization path string true "Organization ID" format(uuid)
// @Param request body codersdk.CreateIntelCohortRequest true "Create intel cohort request"
// @Success 201 {object} codersdk.IntelCohort
// @Router /organizations/{organization}/intel/cohorts [post]
func (api *API) postIntelCohorts(rw http.ResponseWriter, r *http.Request) {
	organization := httpmw.OrganizationParam(r)
	apiKey := httpmw.APIKey(r)
	ctx := r.Context()

	var req codersdk.CreateIntelCohortRequest
	if !httpapi.Read(ctx, rw, r, &req) {
		return
	}
	if req.RegexFilters == nil {
		req.RegexFilters = &codersdk.IntelCohortRegexFilters{}
	}
	if req.TrackedExecutables == nil {
		req.TrackedExecutables = []string{}
	}
	// Ensure defaults to match any exist!
	req.RegexFilters.Normalize()

	cohort, err := api.Database.UpsertIntelCohort(ctx, database.UpsertIntelCohortParams{
		ID:                           uuid.New(),
		OrganizationID:               organization.ID,
		CreatedBy:                    apiKey.UserID,
		CreatedAt:                    dbtime.Now(),
		UpdatedAt:                    dbtime.Now(),
		Name:                         req.Name,
		Icon:                         req.Icon,
		Description:                  req.Description,
		TrackedExecutables:           req.TrackedExecutables,
		RegexOperatingSystem:         req.RegexFilters.OperatingSystem,
		RegexOperatingSystemPlatform: req.RegexFilters.OperatingSystemPlatform,
		RegexOperatingSystemVersion:  req.RegexFilters.OperatingSystemVersion,
		RegexArchitecture:            req.RegexFilters.Architecture,
		RegexInstanceID:              req.RegexFilters.InstanceID,
	})
	if err != nil {
		if database.IsUniqueViolation(err, database.UniqueIntelCohortsOrganizationIDNameKey) {
			httpapi.Write(ctx, rw, http.StatusConflict, codersdk.Response{
				Message: fmt.Sprintf("A cohort with name %q already exists.", req.Name),
				Validations: []codersdk.ValidationError{{
					Field:  "name",
					Detail: "This value is already in use and should be unique.",
				}},
			})
			return
		}
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error creating intel cohort.",
			Detail:  err.Error(),
		})
		return
	}
	httpapi.Write(ctx, rw, http.StatusCreated, convertIntelCohorts([]database.IntelCohort{cohort})[0])
}

func (api *API) deleteIntelCohort(rw http.ResponseWriter, r *http.Request) {

}

func (api *API) patchIntelCohort(rw http.ResponseWriter, r *http.Request) {

}

// Serves the intel daemon protobuf API over a WebSocket.
//
// @Summary Serve intel daemon
// @ID serve-intel-daemon
// @Security CoderSessionToken
// @Tags Intel
// @Param organization path string true "Organization ID" format(uuid)
// @Param instance_id query string true "Instance ID"
// @Param cpu_cores query int false "Number of CPU cores"
// @Param memory_total_mb query int false "Total memory in MB"
// @Param hostname query string false "Hostname"
// @Param operating_system query string false "Operating system"
// @Param operating_system_version query string false "Operating system version"
// @Param architecture query string false "Architecture"
// @success 101
// @Router /organizations/{organization}/intel/serve [get]
func (api *API) intelDaemonServe(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiKey := httpmw.APIKey(r)
	organization := httpmw.OrganizationParam(r)

	query := r.URL.Query()
	// It's fine if this is unset!
	cpuCores, _ := strconv.ParseUint(query.Get("cpu_cores"), 10, 16)
	// It's fine if this is unset as well!
	memoryTotalMB, _ := strconv.ParseUint(query.Get("memory_total_mb"), 10, 64)
	instanceID := query.Get("instance_id")
	hostInfo := codersdk.IntelDaemonHostInfo{
		Hostname:               query.Get("hostname"),
		OperatingSystem:        query.Get("operating_system"),
		OperatingSystemVersion: query.Get("operating_system_version"),
		Architecture:           query.Get("architecture"),
		CPUCores:               uint16(cpuCores),
		MemoryTotalMB:          memoryTotalMB,
	}
	remoteIP := net.ParseIP(r.RemoteAddr)
	if remoteIP == nil {
		remoteIP = net.IPv4(0, 0, 0, 0)
	}
	bitlen := len(remoteIP) * 8
	ipAddress := pqtype.Inet{
		IPNet: net.IPNet{
			IP:   remoteIP,
			Mask: net.CIDRMask(bitlen, bitlen),
		},
		Valid: true,
	}

	machine, err := api.Database.UpsertIntelMachine(ctx, database.UpsertIntelMachineParams{
		ID:                      uuid.New(),
		CreatedAt:               dbtime.Now(),
		UpdatedAt:               dbtime.Now(),
		UserID:                  apiKey.UserID,
		OrganizationID:          organization.ID,
		IPAddress:               ipAddress,
		InstanceID:              instanceID,
		Hostname:                hostInfo.Hostname,
		OperatingSystem:         hostInfo.OperatingSystem,
		OperatingSystemVersion:  hostInfo.OperatingSystemVersion,
		OperatingSystemPlatform: hostInfo.OperatingSystemPlatform,
		CPUCores:                int32(hostInfo.CPUCores),
		MemoryMBTotal:           int32(hostInfo.MemoryTotalMB),
		Architecture:            hostInfo.Architecture,
		DaemonVersion:           "",
	})
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error creating intel machine.",
			Detail:  err.Error(),
		})
		return
	}

	api.WebsocketWaitMutex.Lock()
	api.WebsocketWaitGroup.Add(1)
	api.WebsocketWaitMutex.Unlock()
	defer api.WebsocketWaitGroup.Done()

	conn, err := websocket.Accept(rw, r, &websocket.AcceptOptions{
		// Need to disable compression to avoid a data-race.
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		if !xerrors.Is(err, context.Canceled) {
			api.Logger.Error(ctx, "accept intel daemon websocket conn", slog.Error(err))
		}
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "Internal error accepting websocket connection.",
			Detail:  err.Error(),
		})
		return
	}
	// Align with the frame size of yamux.
	conn.SetReadLimit(256 * 1024)

	// Multiplexes the incoming connection using yamux.
	// This allows multiple function calls to occur over
	// the same connection.
	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard
	ctx, wsNetConn := codersdk.WebsocketNetConn(ctx, conn, websocket.MessageBinary)
	defer wsNetConn.Close()
	session, err := yamux.Server(wsNetConn, config)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, httpapi.WebsocketCloseSprintf("multiplex server: %s", err))
		return
	}
	mux := drpcmux.New()
	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()
	logger := api.Logger

	srv, err := inteldserver.New(srvCtx, inteldserver.Options{
		Database:  api.Database,
		Pubsub:    api.Pubsub,
		Logger:    logger.Named("intel_server"),
		MachineID: machine.ID,
		UserID:    apiKey.UserID,
	})
	if err != nil {
		if !xerrors.Is(err, context.Canceled) {
			api.Logger.Error(ctx, "create intel daemon server", slog.Error(err))
		}
		_ = conn.Close(websocket.StatusInternalError, httpapi.WebsocketCloseSprintf("create intel daemon server: %s", err))
		return
	}
	err = proto.DRPCRegisterIntelDaemon(mux, srv)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, httpapi.WebsocketCloseSprintf("drpc register intel daemon: %s", err))
		return
	}
	server := drpcserver.NewWithOptions(mux, drpcserver.Options{
		Log: func(err error) {
			if xerrors.Is(err, io.EOF) {
				return
			}
			logger.Debug(ctx, "drpc server error", slog.Error(err))
		},
	})
	err = server.Serve(ctx, session)
	srvCancel()
	logger.Info(ctx, "intel daemon disconnected", slog.Error(err))
	if err != nil && !xerrors.Is(err, io.EOF) {
		_ = conn.Close(websocket.StatusInternalError, httpapi.WebsocketCloseSprintf("serve: %s", err))
		return
	}
	_ = conn.Close(websocket.StatusGoingAway, "")
}

func convertIntelMachines(machines []database.IntelMachine) []codersdk.IntelMachine {
	converted := make([]codersdk.IntelMachine, len(machines))
	for i, machine := range machines {
		converted[i] = codersdk.IntelMachine{
			ID:                      machine.ID,
			UserID:                  machine.UserID,
			OrganizationID:          machine.OrganizationID,
			CreatedAt:               machine.CreatedAt,
			UpdatedAt:               machine.UpdatedAt,
			InstanceID:              machine.InstanceID,
			Hostname:                machine.Hostname,
			OperatingSystem:         machine.OperatingSystem,
			OperatingSystemPlatform: machine.OperatingSystemPlatform,
			OperatingSystemVersion:  machine.OperatingSystemVersion,
			CPUCores:                uint16(machine.CPUCores),
			MemoryMBTotal:           uint64(machine.MemoryMBTotal),
			Architecture:            machine.Architecture,
		}
	}
	return converted
}

func convertIntelCohorts(cohorts []database.IntelCohort) []codersdk.IntelCohort {
	converted := make([]codersdk.IntelCohort, len(cohorts))
	for i, cohort := range cohorts {
		converted[i] = codersdk.IntelCohort{
			ID:             cohort.ID,
			OrganizationID: cohort.OrganizationID,
			CreatedBy:      cohort.CreatedBy,
			UpdatedAt:      cohort.UpdatedAt,
			RegexFilters: codersdk.IntelCohortRegexFilters{
				OperatingSystem:         cohort.RegexOperatingSystem,
				OperatingSystemPlatform: cohort.RegexOperatingSystemPlatform,
				OperatingSystemVersion:  cohort.RegexOperatingSystemVersion,
				Architecture:            cohort.RegexArchitecture,
				InstanceID:              cohort.RegexInstanceID,
			},
			CreatedAt: cohort.CreatedAt,
			IntelCohortMetadata: codersdk.IntelCohortMetadata{
				Name:               cohort.Name,
				Icon:               cohort.Icon,
				Description:        cohort.Description,
				TrackedExecutables: cohort.TrackedExecutables,
			},
		}
	}
	return converted
}
