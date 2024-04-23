package coderd

import (
	"context"
	"database/sql"
	"io"
	"net"
	"net/http"
	"strconv"

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

func (api *API) intelDaemonServe(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiKey := httpmw.APIKey(r)
	organization := httpmw.OrganizationParam(r)

	query := r.URL.Query()
	cpuCores, err := strconv.ParseUint(query.Get("cpu_cores"), 10, 16)
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "Invalid CPU cores.",
			Detail:  err.Error(),
		})
		return
	}
	memoryTotalMB, err := strconv.ParseUint(query.Get("memory_total_mb"), 10, 64)
	if err != nil {
		httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
			Message: "Invalid memory total MB.",
			Detail:  err.Error(),
		})
		return
	}
	hostInfo := codersdk.IntelDaemonHostInfo{
		InstanceID:             query.Get("instance_id"),
		Hostname:               query.Get("hostname"),
		OperatingSystem:        query.Get("operating_system"),
		OperatingSystemVersion: query.Get("operating_system_version"),
		Architecture:           query.Get("architecture"),
		Tags:                   query["tags"],
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
		ID:             uuid.New(),
		CreatedAt:      dbtime.Now(),
		UpdatedAt:      dbtime.Now(),
		UserID:         apiKey.UserID,
		OrganizationID: organization.ID,
		IPAddress:      ipAddress,

		InstanceID:      hostInfo.InstanceID,
		Hostname:        hostInfo.Hostname,
		OperatingSystem: hostInfo.OperatingSystem,
		OperatingSystemVersion: sql.NullString{
			String: hostInfo.OperatingSystemVersion,
			Valid:  hostInfo.OperatingSystemVersion != "",
		},
		CpuCores:      int32(hostInfo.CPUCores),
		MemoryMbTotal: int32(hostInfo.MemoryTotalMB),
		Architecture:  hostInfo.Architecture,
		DaemonVersion: "",
		Tags:          hostInfo.Tags,
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
	logger.Info(ctx, "provisioner daemon disconnected", slog.Error(err))
	if err != nil && !xerrors.Is(err, io.EOF) {
		_ = conn.Close(websocket.StatusInternalError, httpapi.WebsocketCloseSprintf("serve: %s", err))
		return
	}
	_ = conn.Close(websocket.StatusGoingAway, "")
}
