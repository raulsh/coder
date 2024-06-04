package dispatch

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/coder/coder/v2/codersdk"
	"github.com/google/uuid"
	"golang.org/x/xerrors"
)

var (
	ValidationNoFromAddressErr   = xerrors.New("no 'from' address defined")
	ValidationNoSmarthostHostErr = xerrors.New("smarthost 'host' is not defined, or is invalid")
	ValidationNoSmarthostPortErr = xerrors.New("smarthost 'port' is not defined, or is invalid")
	ValidationNoHelloErr         = xerrors.New("'hello' not defined")
)

const (
	labelFrom    = "from"
	labelTo      = "to"
	labelSubject = "subject"
	labelBody    = "body"
)

type SMTPDispatcher struct {
	cfg codersdk.NotificationsEmailConfig
	log slog.Logger
}

func NewSMTPDispatcher(cfg codersdk.NotificationsEmailConfig, log slog.Logger) *SMTPDispatcher {
	return &SMTPDispatcher{cfg: cfg, log: log}
}

func (s *SMTPDispatcher) Name() string {
	// TODO: don't use database types
	return string(database.NotificationReceiverSmtp)
}

func (s *SMTPDispatcher) Validate(input types.Labels) (bool, []string) {
	missing := input.Missing("to", "subject", "body")
	return len(missing) == 0, missing
}

// Send delivers a notification via SMTP.
// The following global configuration values can be overridden via labels:
//   - "from" overrides notifications.email.from / CODER_NOTIFICATIONS_EMAIL_FROM
//
// NOTE: this is heavily inspired by Alertmanager's email notifier:
//
//	https://github.com/prometheus/alertmanager/blob/342f6a599ce16c138663f18ed0b880e777c3017d/notify/email/email.go
func (s *SMTPDispatcher) Send(ctx context.Context, msgID uuid.UUID, input types.Labels) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	var (
		c    *smtp.Client
		conn net.Conn
		err  error
	)

	s.log.Debug(ctx, "dispatching via SMTP", slog.F("msgID", msgID))

	// Dial the smarthost to establish a connection.
	smarthost, smarthostPort, err := s.smarthost()
	if err != nil {
		return false, xerrors.Errorf("'smarthost' validation: %w", err)
	}
	if smarthostPort == "465" {
		// TODO: implement TLS
		return false, xerrors.New("TLS is not currently supported")
	} else {
		var d net.Dialer
		conn, err = d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", smarthost, smarthostPort))
		if err != nil {
			return true, xerrors.Errorf("establish connection to server: %w", err)
		}
	}

	// Create an SMTP client.
	c, err = smtp.NewClient(conn, smarthost)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			s.log.Warn(ctx, "failed to close connection", slog.Error(cerr))
		}
		return true, xerrors.Errorf("create client: %w", err)
	}

	// Cleanup.
	defer func() {
		if err := c.Quit(); err != nil {
			s.log.Warn(ctx, "failed to close SMTP connection", slog.Error(err))
		}
	}()

	// Server handshake.
	hello, err := s.hello()
	if err != nil {
		return false, xerrors.Errorf("'hello' validation: %w", err)
	}
	err = c.Hello(hello)
	if err != nil {
		return false, xerrors.Errorf("server handshake: %w", err)
	}

	// Check for authentication capabilities.
	// TODO: implement authentication
	//if ok, mech := c.Extension("AUTH"); ok {
	//	auth, err := s.auth(mech)
	//	if err != nil {
	//		return true, xerrors.Errorf("find auth mechanism: %w", err)
	//	}
	//	if auth != nil {
	//		if err := c.Auth(auth); err != nil {
	//			return true, xerrors.Errorf("%T auth: %w", auth, err)
	//		}
	//	}
	//}

	// Sender identification.
	from, err := s.fromAddr(input)
	if err != nil {
		return false, xerrors.Errorf("'from' validation: %w", err)
	}
	err = c.Mail(from)
	if err != nil {
		// This is retryable because the server may be temporarily down.
		return true, xerrors.Errorf("sender identification: %w", err)
	}

	// Recipient designation.
	to, err := s.toAddrs(input)
	if err != nil {
		return false, xerrors.Errorf("'to' validation: %w", err)
	}
	for _, addr := range to {
		err = c.Rcpt(addr)
		if err != nil {
			// This is a retryable case because the server may be temporarily down.
			// The addresses are already validated, although it is possible that the server might disagree - in which case
			// this will lead to some spurious retries, but that's not a big deal.
			return true, xerrors.Errorf("recipient designation: %w", err)
		}
	}

	// Start message transmission.
	message, err := c.Data()
	if err != nil {
		return true, fmt.Errorf("message transmission: %w", err)
	}
	defer message.Close()

	// Transmit message headers.
	msg := &bytes.Buffer{}
	multipartBuffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(multipartBuffer)
	fmt.Fprintf(msg, "From: %s\r\n", from)
	fmt.Fprintf(msg, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(msg, "Subject: %s\r\n", s.subject(input))
	fmt.Fprintf(msg, "Message-Id: %s@%s\r\n", msgID, s.hostname())
	fmt.Fprintf(msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(msg, "Content-Type: multipart/alternative;  boundary=%s\r\n", multipartWriter.Boundary())
	fmt.Fprintf(msg, "MIME-Version: 1.0\r\n\r\n")
	_, err = message.Write(msg.Bytes())
	if err != nil {
		return false, fmt.Errorf("write headers: %w", err)
	}

	// Transmit message body.
	// TODO: implement text-only body?
	//		 If implemented, keep HTML message last since preferred alternative is placed last per section 5.1.4 of RFC 2046
	// 		 https://www.ietf.org/rfc/rfc2046.txt
	w, err := multipartWriter.CreatePart(textproto.MIMEHeader{
		"Content-Transfer-Encoding": {"quoted-printable"},
		"Content-Type":              {"text/html; charset=UTF-8"},
	})
	if err != nil {
		return false, fmt.Errorf("create part for HTML body: %w", err)
	}
	qw := quotedprintable.NewWriter(w)
	_, err = qw.Write([]byte(s.body(input)))
	if err != nil {
		return true, fmt.Errorf("write HTML part: %w", err)
	}
	err = qw.Close()
	if err != nil {
		return true, fmt.Errorf("close HTML part: %w", err)
	}

	err = multipartWriter.Close()
	if err != nil {
		return false, fmt.Errorf("close multipartWriter: %w", err)
	}

	_, err = message.Write(multipartBuffer.Bytes())
	if err != nil {
		return false, fmt.Errorf("write body buffer: %w", err)
	}

	// Returning false, nil indicates successful send (i.e. non-retryable non-error)
	return false, nil
}

// auth returns a value which implements the smtp.Auth based on the available auth mechanism.
func (s *SMTPDispatcher) auth(mechs string) (smtp.Auth, error) {
	// TODO
	return nil, nil
}

// fromAddr retrieves the "From" address and validates it.
// Allows overriding via the "from" label.
func (s *SMTPDispatcher) fromAddr(input types.Labels) (string, error) {
	from := s.cfg.From.String()
	// Handle overrides.
	if val, ok := input.GetStrict(labelFrom); ok {
		from = val
	}
	addrs, err := mail.ParseAddressList(from)
	if err != nil {
		return "", xerrors.Errorf("parse 'from' address: %w", err)
	}
	if len(addrs) != 1 {
		return "", ValidationNoFromAddressErr
	}
	return from, nil
}

// toAddrs retrieves the "To" address(es) and validates them.
func (s *SMTPDispatcher) toAddrs(input types.Labels) ([]string, error) {
	to, ok := input.GetStrict(labelTo)
	if !ok {
		// This should never happen because Validate should catch this.
		return nil, xerrors.New("no 'to' address(es) found")
	}
	addrs, err := mail.ParseAddressList(to)
	if err != nil {
		return nil, xerrors.Errorf("parse 'to' addresses: %w", err)
	}
	if len(addrs) <= 0 {
		// The addresses can be non-zero but invalid.
		return nil, xerrors.Errorf("no valid 'to' address(es) found, given %+v", to)
	}

	var out []string
	for _, addr := range addrs {
		out = append(out, addr.Address)
	}

	return out, nil
}

// smarthost retrieves the host/port defined and validates them.
// Does not allow overriding.
func (s *SMTPDispatcher) smarthost() (string, string, error) {
	host := s.cfg.Smarthost.Host
	port := s.cfg.Smarthost.Port

	// We don't validate the contents themselves; this will be done by the underlying SMTP library.
	if host == "" {
		return "", "", ValidationNoSmarthostHostErr
	}
	if port == "" {
		return "", "", ValidationNoSmarthostPortErr
	}

	return host, port, nil
}

// hello retrieves the hostname identifying the SMTP server.
// Does not allow overriding.
func (s *SMTPDispatcher) hello() (string, error) {
	val := s.cfg.Hello.String()
	if val == "" {
		return "", ValidationNoHelloErr
	}
	return val, nil
}

// subject returns the value of the "subject" label.
func (s *SMTPDispatcher) subject(input types.Labels) string {
	return input.Get(labelSubject)
}

// body returns the value of the "body" label.
func (s *SMTPDispatcher) body(input types.Labels) string {
	return input.Get(labelBody)
}

func (s *SMTPDispatcher) hostname() string {
	h, err := os.Hostname()
	// If we can't get the hostname, we'll use localhost
	if err != nil {
		h = "localhost.localdomain"
	}
	return h
}
