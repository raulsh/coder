package proto

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	gproto "google.golang.org/protobuf/proto"
)

const (
	DialTimeout = 100 * time.Millisecond
)

var (
	daemonSocket = filepath.Join(os.TempDir(), ".coder-intel.sock")
)

// ReportInvocation reports an invocation to a daemon
// if it is running.
func ReportInvocation(inv *ReportInvocationRequest) error {
	data, err := gproto.Marshal(inv)
	if err != nil {
		return err
	}
	var w io.Writer
	// Don't bother closing either of these.
	// It's a waste of a syscall, because we're exiting
	// immediately after reporting anyways!
	if shouldUnixgram() {
		w, err = net.DialUnix("unixgram", nil, &net.UnixAddr{
			Name: daemonSocket,
			Net:  "unixgram",
		})
	} else {
		daemonDialTimeoutRaw, exists := os.LookupEnv("CODER_INTEL_DAEMON_TIMEOUT")
		timeout := DialTimeout
		if exists {
			dur, err := time.ParseDuration(daemonDialTimeoutRaw)
			if err != nil {
				return err
			}
			timeout = dur
		}
		w, err = net.DialTimeout("tcp", daemonAddress(), timeout)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ListenForInvocations starts a listener that listens for invocation requests
// from the intel client in the most efficient way possible.
func ListenForInvocations(sendFunc func(inv *ReportInvocationRequest)) (io.Closer, error) {
	overrideAddress := os.Getenv("CODER_INTEL_DAEMON_ADDRESS")
	if overrideAddress != "" {
		return net.Listen("tcp", overrideAddress)
	}
	unmarshalAndSend := func(data []byte, count int) {
		var inv ReportInvocationRequest
		err := gproto.Unmarshal(data[:count], &inv)
		if err != nil {
			return
		}
		go sendFunc(&inv)
	}
	var closer io.Closer
	if shouldUnixgram() {
		// Remove the socket first!
		_ = os.Remove(daemonSocket)
		unixConn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
			Name: daemonSocket,
			Net:  "unixgram",
		})
		if err != nil {
			return nil, err
		}
		go func() {
			data := make([]byte, 1024)
			for {
				count, _, err := unixConn.ReadFromUnix(data)
				if err != nil {
					return
				}
				unmarshalAndSend(data, count)
			}
		}()
		closer = unixConn
	} else {
		listener, err := net.Listen("tcp", daemonAddress())
		if err != nil {
			return nil, err
		}
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go func() {
					data := make([]byte, 1024)
					read, err := conn.Read(data)
					if err != nil {
						return
					}
					unmarshalAndSend(data, read)
					_ = conn.Close()
				}()
			}
		}()
		closer = listener
	}
	return closer, nil
}

// shouldUnixgram returns whether the fast unixgram method
// of transmission should be used!
func shouldUnixgram() bool {
	return os.Getenv("CODER_INTEL_DAEMON_ADDRESS") == "" && runtime.GOOS != "windows"
}

func daemonAddress() string {
	overrideAddress := os.Getenv("CODER_INTEL_DAEMON_ADDRESS")
	if overrideAddress != "" {
		return overrideAddress
	}
	return "127.0.0.1:13657"
}
