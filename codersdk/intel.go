package codersdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	"golang.org/x/xerrors"
	"nhooyr.io/websocket"

	"github.com/coder/coder/v2/buildinfo"
	"github.com/coder/coder/v2/codersdk/drpc"
	"github.com/coder/coder/v2/inteld/proto"
)

type IntelCohortRegexFilters struct {
	OperatingSystem         string `json:"operating_system"`
	OperatingSystemPlatform string `json:"operating_system_platform"`
	OperatingSystemVersion  string `json:"operating_system_version"`
	Architecture            string `json:"architecture"`
	InstanceID              string `json:"instance_id"`
}

func (i *IntelCohortRegexFilters) Normalize() {
	if i.OperatingSystem == "" {
		i.OperatingSystem = ".*"
	}
	if i.OperatingSystemPlatform == "" {
		i.OperatingSystemPlatform = ".*"
	}
	if i.OperatingSystemVersion == "" {
		i.OperatingSystemVersion = ".*"
	}
	if i.Architecture == "" {
		i.Architecture = ".*"
	}
	if i.InstanceID == "" {
		i.InstanceID = ".*"
	}
}

type IntelCohortMetadata struct {
	Name               string   `json:"name"`
	DisplayName        string   `json:"display_name"`
	Icon               string   `json:"icon"`
	Description        string   `json:"description"`
	TrackedExecutables []string `json:"tracked_executables"`
}

type IntelCohort struct {
	ID             uuid.UUID               `json:"id" format:"uuid"`
	OrganizationID uuid.UUID               `json:"organization_id" format:"uuid"`
	CreatedBy      uuid.UUID               `json:"created_by"`
	CreatedAt      int64                   `json:"created_at" format:"date-time"`
	UpdatedAt      int64                   `json:"updated_at" format:"date-time"`
	RegexFilters   IntelCohortRegexFilters `json:"regex_filters"`

	IntelCohortMetadata
}

type IntelDaemonHostInfo struct {
	Hostname                string `json:"hostname"`
	OperatingSystem         string `json:"operating_system"`
	OperatingSystemPlatform string `json:"operating_system_platform"`
	OperatingSystemVersion  string `json:"operating_system_version"`
	Architecture            string `json:"architecture"`
	CPUCores                uint16 `json:"cpu_cores"`
	MemoryTotalMB           uint64 `json:"memory_total_mb"`
}

type ServeIntelDaemonRequest struct {
	IntelDaemonHostInfo
	InstanceID   string    `json:"instance_id"`
	Organization uuid.UUID `json:"organization" format:"uuid"`
}

type IntelMachine struct {
	ID                      uuid.UUID `json:"id" format:"uuid"`
	CreatedAt               time.Time `json:"created_at" format:"date-time"`
	UpdatedAt               time.Time `json:"updated_at" format:"date-time"`
	UserID                  uuid.UUID `json:"user_id" format:"uuid"`
	OrganizationID          uuid.UUID `json:"organization_id" format:"uuid"`
	InstanceID              string    `json:"instance_id"`
	Hostname                string    `json:"hostname"`
	OperatingSystem         string    `json:"operating_system"`
	OperatingSystemPlatform string    `json:"operating_system_platform"`
	OperatingSystemVersion  string    `json:"operating_system_version"`
	CPUCores                uint16    `json:"cpu_cores"`
	MemoryMBTotal           uint64    `json:"memory_mb_total"`
	Architecture            string    `json:"architecture"`
}

type IntelMachinesRequest struct {
	OrganizationID uuid.UUID               `json:"organization_id" format:"uuid"`
	RegexFilters   IntelCohortRegexFilters `json:"regex_filters"`
	Offset         int                     `json:"offset,omitempty" typescript:"-"`
	Limit          int                     `json:"limit,omitempty" typescript:"-"`
}

type IntelMachinesResponse struct {
	IntelMachines []IntelMachine `json:"intel_machines"`
	Count         int            `json:"count"`
}

// IntelMachines returns a set of machines that matches the filters provided.
// It will return all machines if no filters are provided.
func (c *Client) IntelMachines(ctx context.Context, req IntelMachinesRequest) (IntelMachinesResponse, error) {
	orgParam := req.OrganizationID.String()
	if req.OrganizationID == uuid.Nil {
		orgParam = DefaultOrganization
	}
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/organizations/%s/intel/machines", orgParam), nil,
		WithQueryParam("operating_system", req.RegexFilters.OperatingSystem),
		WithQueryParam("operating_system_platform", req.RegexFilters.OperatingSystemPlatform),
		WithQueryParam("operating_system_version", req.RegexFilters.OperatingSystemVersion),
		WithQueryParam("architecture", req.RegexFilters.Architecture),
		WithQueryParam("instance_id", req.RegexFilters.InstanceID),
		WithQueryParam("offset", strconv.Itoa(req.Offset)),
		WithQueryParam("limit", strconv.Itoa(req.Limit)),
	)
	if err != nil {
		return IntelMachinesResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return IntelMachinesResponse{}, ReadBodyAsError(res)
	}
	var machines IntelMachinesResponse
	return machines, json.NewDecoder(res.Body).Decode(&machines)
}

// ServeIntelDaemon returns the gRPC service for an intel daemon.
func (c *Client) ServeIntelDaemon(ctx context.Context, req ServeIntelDaemonRequest) (proto.DRPCIntelDaemonClient, error) {
	orgParam := req.Organization.String()
	if req.Organization == uuid.Nil {
		orgParam = DefaultOrganization
	}
	serverURL, err := c.URL.Parse(fmt.Sprintf("/api/v2/organizations/%s/intel/serve", orgParam))
	if err != nil {
		return nil, xerrors.Errorf("parse url: %w", err)
	}
	query := serverURL.Query()
	query.Set("instance_id", req.InstanceID)
	query.Set("hostname", req.Hostname)
	query.Set("operating_system", req.OperatingSystem)
	query.Set("operating_system_version", req.OperatingSystemVersion)
	query.Set("architecture", req.Architecture)
	query.Set("cpu_cores", strconv.Itoa(int(req.CPUCores)))
	query.Set("memory_total_mb", strconv.Itoa(int(req.MemoryTotalMB)))
	serverURL.RawQuery = query.Encode()
	httpClient := &http.Client{
		Transport: c.HTTPClient.Transport,
	}
	headers := http.Header{}
	headers.Set(BuildVersionHeader, buildinfo.Version())
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, xerrors.Errorf("create cookie jar: %w", err)
	}
	jar.SetCookies(serverURL, []*http.Cookie{{
		Name:  SessionTokenCookie,
		Value: c.SessionToken(),
	}})
	httpClient.Jar = jar
	conn, res, err := websocket.Dial(ctx, serverURL.String(), &websocket.DialOptions{
		HTTPClient: httpClient,
		// Need to disable compression to avoid a data-race.
		CompressionMode: websocket.CompressionDisabled,
		HTTPHeader:      headers,
	})
	if err != nil {
		if res == nil {
			return nil, err
		}
		return nil, ReadBodyAsError(res)
	}
	// Align with the frame size of yamux.
	conn.SetReadLimit(256 * 1024)

	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard

	// Use background context because caller should close the client.
	_, wsNetConn := WebsocketNetConn(context.Background(), conn, websocket.MessageBinary)
	session, err := yamux.Client(wsNetConn, config)
	if err != nil {
		_ = conn.Close(websocket.StatusGoingAway, "")
		_ = wsNetConn.Close()
		return nil, xerrors.Errorf("multiplex client: %w", err)
	}
	return proto.NewDRPCIntelDaemonClient(drpc.MultiplexedConn(session)), nil
}
