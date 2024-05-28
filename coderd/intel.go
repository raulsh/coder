package coderd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	"github.com/sqlc-dev/pqtype"
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
	summaries, err := api.Database.GetIntelInvocationSummaries(ctx, database.GetIntelInvocationSummariesParams{
		StartsAt:        req.StartsAt,
		MachineMetadata: []byte("{}"),
	})
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error getting intel invocation summaries.",
			Detail:  err.Error(),
		})
		return
	}
	report := codersdk.IntelReport{
		GitAuthProviders: map[string]*string{},
		Intervals:        make([]codersdk.IntelInvocationSummary, 0, len(summaries)),
	}
	for _, summary := range summaries {
		machineMetadata := map[string]map[string]int64{}
		for k, v := range summary.MachineMetadata {
			machineMetadata[k] = map[string]int64(v)
		}
		report.Intervals = append(report.Intervals, codersdk.IntelInvocationSummary{
			ID:                 summary.ID,
			StartsAt:           summary.StartsAt,
			EndsAt:             summary.EndsAt,
			BinaryName:         summary.BinaryName,
			BinaryArgs:         summary.BinaryArgs,
			ExitCodes:          summary.ExitCodes,
			GitRemoteURLs:      summary.GitRemoteUrls,
			WorkingDirectories: summary.WorkingDirectories,
			BinaryPaths:        summary.BinaryPaths,
			MachineMetadata:    machineMetadata,
			UniqueMachines:     summary.UniqueMachines,
			TotalInvocations:   summary.TotalInvocations,
			MedianDurationMS:   summary.MedianDurationMs,
		})
		report.Invocations += summary.TotalInvocations
		for url := range summary.GitRemoteUrls {
			_, exists := report.GitAuthProviders[url]
			if exists {
				continue
			}
			for _, extAuth := range api.ExternalAuthConfigs {
				if !extAuth.Regex.MatchString(url) {
					continue
				}
				report.GitAuthProviders[url] = &extAuth.ID
				break
			}
		}
	}
	httpapi.Write(ctx, rw, http.StatusOK, report)
}

// postIntelReport updates intel invocation summaries which will
// enable the intel report to be up-to-date.
func (api *API) postIntelReport(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := api.Database.UpsertIntelInvocationSummaries(ctx)
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusInternalServerError, codersdk.Response{
			Message: "Internal error upserting intel invocation summaries.",
			Detail:  err.Error(),
		})
		return
	}
	httpapi.Write(ctx, rw, http.StatusNoContent, nil)
}

// intelMachines returns all machines that match the given filters.
//
// @Summary List intel machines
// @ID list-intel-machines
// @Security CoderSessionToken
// @Produce json
// @Tags Intel
// @Param organization path string true "Organization ID" format(uuid)
// @Param limit query int false "Page limit"
// @Param offset query int false "Page offset"
// @Param metadata query string false "A JSON object to match machine metadata against"
// @Success 200 {object} codersdk.IntelMachinesResponse
// @Router /organizations/{organization}/intel/machines [get]
func (api *API) intelMachines(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	organization := httpmw.OrganizationParam(r)

	page, ok := parsePagination(rw, r)
	if !ok {
		return
	}
	metadataRaw := r.URL.Query().Get("metadata")
	if metadataRaw != "" {
		// Just for validation!
		var mapping database.StringMapOfRegex
		err := json.Unmarshal([]byte(metadataRaw), &mapping)
		if err != nil {
			httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
				Message: "Invalid metadata.",
				Detail:  err.Error(),
			})
			return
		}
	} else {
		metadataRaw = "{}"
	}

	machineRows, err := api.Database.GetIntelMachinesMatchingFilters(ctx, database.GetIntelMachinesMatchingFiltersParams{
		OrganizationID: organization.ID,
		Metadata:       []byte(metadataRaw),
		LimitOpt:       int32(page.Limit),
		OffsetOpt:      int32(page.Offset),
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
	if req.TrackedExecutables == nil {
		req.TrackedExecutables = []string{}
	}
	cohort, err := api.Database.UpsertIntelCohort(ctx, database.UpsertIntelCohortParams{
		ID:                 uuid.New(),
		OrganizationID:     organization.ID,
		CreatedBy:          apiKey.UserID,
		CreatedAt:          dbtime.Now(),
		UpdatedAt:          dbtime.Now(),
		Name:               req.Name,
		Icon:               req.Icon,
		Description:        req.Description,
		TrackedExecutables: req.TrackedExecutables,
		MachineMetadata: database.NullStringMapOfRegex{
			StringMapOfRegex: req.MetadataMatch,
			Valid:            req.MetadataMatch != nil,
		},
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
	instanceID := query.Get("instance_id")
	metadataRaw := query.Get("metadata")
	var metadata map[string]string
	if metadataRaw != "" {
		err := json.Unmarshal([]byte(metadataRaw), &metadata)
		if err != nil {
			httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
				Message: "Invalid metadata.",
				Detail:  err.Error(),
			})
			return
		}
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
		ID:             uuid.New(),
		CreatedAt:      dbtime.Now(),
		UpdatedAt:      dbtime.Now(),
		UserID:         apiKey.UserID,
		OrganizationID: organization.ID,
		IPAddress:      ipAddress,
		InstanceID:     instanceID,
		Metadata:       metadata,
		DaemonVersion:  "",
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
		Database:                api.Database,
		Pubsub:                  api.Pubsub,
		Logger:                  logger.Named("inteldserver"),
		MachineID:               machine.ID,
		UserID:                  apiKey.UserID,
		InvocationFlushInterval: api.IntelServerInvocationFlushInterval,
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
			if xerrors.Is(err, io.EOF) || xerrors.Is(err, context.Canceled) {
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
			ID:             machine.ID,
			UserID:         machine.UserID,
			OrganizationID: machine.OrganizationID,
			CreatedAt:      machine.CreatedAt,
			UpdatedAt:      machine.UpdatedAt,
			InstanceID:     machine.InstanceID,
			Metadata:       machine.Metadata,
		}
	}
	return converted
}

func convertIntelCohorts(cohorts []database.IntelCohort) []codersdk.IntelCohort {
	converted := make([]codersdk.IntelCohort, len(cohorts))
	for i, cohort := range cohorts {
		converted[i] = codersdk.IntelCohort{
			ID:              cohort.ID,
			OrganizationID:  cohort.OrganizationID,
			CreatedBy:       cohort.CreatedBy,
			UpdatedAt:       cohort.UpdatedAt,
			MachineMetadata: cohort.MachineMetadata.StringMapOfRegex,
			CreatedAt:       cohort.CreatedAt,
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
