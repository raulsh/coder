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
	Icon               string   `json:"icon"`
	Description        string   `json:"description"`
	TrackedExecutables []string `json:"tracked_executables"`
}

type IntelCohort struct {
	ID             uuid.UUID               `json:"id" format:"uuid"`
	OrganizationID uuid.UUID               `json:"organization_id" format:"uuid"`
	CreatedBy      uuid.UUID               `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at" format:"date-time"`
	UpdatedAt      time.Time               `json:"updated_at" format:"date-time"`
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
	InstanceID string `json:"instance_id"`
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

func (c *Client) IntelCohorts(ctx context.Context, organizationID uuid.UUID) ([]IntelCohort, error) {
	orgParam := organizationID.String()
	if organizationID == uuid.Nil {
		orgParam = DefaultOrganization
	}
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/organizations/%s/intel/cohorts", orgParam), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, ReadBodyAsError(res)
	}
	var cohorts []IntelCohort
	return cohorts, json.NewDecoder(res.Body).Decode(&cohorts)
}

// CreateIntelCohortRequest is the request to create a new cohort.
type CreateIntelCohortRequest struct {
	Name               string                   `json:"name" validate:"required"`
	Icon               string                   `json:"icon"`
	Description        string                   `json:"description"`
	TrackedExecutables []string                 `json:"tracked_executables"`
	RegexFilters       *IntelCohortRegexFilters `json:"regex_filters"`
}

// CreateIntelCohort creates a new cohort.
func (c *Client) CreateIntelCohort(ctx context.Context, organizationID uuid.UUID, req CreateIntelCohortRequest) (IntelCohort, error) {
	orgParam := organizationID.String()
	if organizationID == uuid.Nil {
		orgParam = DefaultOrganization
	}
	res, err := c.Request(ctx, http.MethodPost, fmt.Sprintf("/api/v2/organizations/%s/intel/cohorts", orgParam), req)
	if err != nil {
		return IntelCohort{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return IntelCohort{}, ReadBodyAsError(res)
	}
	var cohort IntelCohort
	return cohort, json.NewDecoder(res.Body).Decode(&cohort)
}

type IntelMachinesRequest struct {
	RegexFilters IntelCohortRegexFilters `json:"regex_filters"`
	Offset       int                     `json:"offset,omitempty" typescript:"-"`
	Limit        int                     `json:"limit,omitempty" typescript:"-"`
}

type IntelMachinesResponse struct {
	IntelMachines []IntelMachine `json:"intel_machines"`
	Count         int            `json:"count"`
}

// IntelMachines returns a set of machines that matches the filters provided.
// It will return all machines if no filters are provided.
func (c *Client) IntelMachines(ctx context.Context, organizationID uuid.UUID, req IntelMachinesRequest) (IntelMachinesResponse, error) {
	orgParam := organizationID.String()
	if organizationID == uuid.Nil {
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
func (c *Client) ServeIntelDaemon(ctx context.Context, organizationID uuid.UUID, req ServeIntelDaemonRequest) (proto.DRPCIntelDaemonClient, error) {
	orgParam := organizationID.String()
	if organizationID == uuid.Nil {
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

// IntelReportRequest returns a report of invocations for a cohort.
type IntelReportRequest struct {
	StartsAt time.Time `json:"starts_at" format:"date-time"`
	// CohortIDs is a list of cohort IDs to report on.
	// If empty, all cohorts will be reported on.
	CohortIDs []uuid.UUID `json:"cohort_ids"`
}

type IntelReport struct {
	Invocations int64 `json:"invocations"`

	Commands   []IntelReportCommand   `json:"commands"`
	GitRemotes []IntelReportGitRemote `json:"git_remotes"`
}

// IntelReportInvocationInterval reports the invocation interval for a duration.
type IntelReportInvocationInterval struct {
	CohortID         uuid.UUID `json:"cohort_id" format:"uuid"`
	StartsAt         time.Time `json:"starts_at" format:"date-time"`
	EndsAt           time.Time `json:"ends_at" format:"date-time"`
	Invocations      int64     `json:"invocations"`
	MedianDurationMS float64   `json:"median_duration_ms"`
}

// IntelReportGitRemote reports the Git remote URL execution time
// across all invocations.
type IntelReportGitRemote struct {
	URL                    string                          `json:"url"`
	ExternalAuthProviderID *string                         `json:"external_auth_provider_id"`
	Invocations            int64                           `json:"invocations"`
	Intervals              []IntelReportInvocationInterval `json:"intervals"`
}

type IntelReportCommand struct {
	BinaryName string   `json:"binary_name"`
	BinaryArgs []string `json:"binary_args"`

	Invocations int64                           `json:"invocations"`
	Intervals   []IntelReportInvocationInterval `json:"intervals"`
	// ExitCodes maps exit codes to the number of invocations.
	ExitCodes          map[int]int64    `json:"exit_codes"`
	GitRemoteURLs      map[string]int64 `json:"git_remote_urls"`
	WorkingDirectories map[string]int64 `json:"working_directories"`
	BinaryPaths        map[string]int64 `json:"binary_paths"`
}

// IntelReport returns a report of invocations for a cohort.
func (c *Client) IntelReport(ctx context.Context, organizationID uuid.UUID, req IntelReportRequest) (IntelReport, error) {
	orgParam := organizationID.String()
	if organizationID == uuid.Nil {
		orgParam = DefaultOrganization
	}
	serverURL, err := c.URL.Parse(fmt.Sprintf("/api/v2/organizations/%s/intel/report", orgParam))
	if err != nil {
		return IntelReport{}, xerrors.Errorf("parse url: %w", err)
	}
	q := serverURL.Query()
	if !req.StartsAt.IsZero() {
		q.Set("starts_at", req.StartsAt.Format(time.DateOnly))
	}
	for _, cohortID := range req.CohortIDs {
		q.Add("cohort_id", cohortID.String())
	}
	serverURL.RawQuery = q.Encode()
	res, err := c.Request(ctx, http.MethodGet, serverURL.String(), req)
	if err != nil {
		return IntelReport{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return IntelReport{}, ReadBodyAsError(res)
	}
	var report IntelReport
	return report, json.NewDecoder(res.Body).Decode(&report)
}
