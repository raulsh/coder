package codersdk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	"golang.org/x/xerrors"
	"nhooyr.io/websocket"

	"github.com/coder/coder/v2/buildinfo"
	"github.com/coder/coder/v2/codersdk/drpc"
	"github.com/coder/coder/v2/inteld/proto"
)

type ServeIntelDaemonRequest struct {
	Organization uuid.UUID `json:"organization" format:"uuid"`
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
