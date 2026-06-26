package tlsutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// subprocessConn wraps a subprocess (nc) as a net.Conn so that Go's HTTP/TLS
// stack can run transparently over a connection that bypasses VPN/NECP routing
// captures affecting the host process.
type subprocessConn struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	addr    net.Addr
	closeMu sync.Once

	deadlineMu sync.Mutex
	readTimer  *time.Timer
	writeTimer *time.Timer
}

func (c *subprocessConn) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *subprocessConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }

func (c *subprocessConn) Close() error {
	c.closeMu.Do(func() {
		c.stdin.Close()
		c.stdout.Close()
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
		c.cancelTimers()
	})
	return nil
}

func (c *subprocessConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (c *subprocessConn) RemoteAddr() net.Addr { return c.addr }

func (c *subprocessConn) SetDeadline(t time.Time) error {
	c.setReadDeadline(t)
	c.setWriteDeadline(t)
	return nil
}

func (c *subprocessConn) SetReadDeadline(t time.Time) error {
	c.setReadDeadline(t)
	return nil
}

func (c *subprocessConn) SetWriteDeadline(t time.Time) error {
	c.setWriteDeadline(t)
	return nil
}

func (c *subprocessConn) setReadDeadline(t time.Time) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}
	if t.IsZero() {
		return
	}
	d := time.Until(t)
	if d <= 0 {
		go c.Close()
		return
	}
	c.readTimer = time.AfterFunc(d, func() { c.Close() })
}

func (c *subprocessConn) setWriteDeadline(t time.Time) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}
	if t.IsZero() {
		return
	}
	d := time.Until(t)
	if d <= 0 {
		go c.Close()
		return
	}
	c.writeTimer = time.AfterFunc(d, func() { c.Close() })
}

func (c *subprocessConn) cancelTimers() {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}
	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}
}

// dialViaSubprocess spawns nc to establish a TCP connection to address, then
// wraps its stdin/stdout as a net.Conn.  The subprocess is not subject to the
// NECP routing policies that capture the host process's direct sockets, so it
// can reach RFC 1918 addresses on macOS even when a VPN system extension is
// active.
func dialViaSubprocess(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "nc", "-w", "30", host, port)
	cmd.Stderr = nil

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("nc stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("nc stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("nc start for %s: %w", address, err)
	}

	conn := &subprocessConn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		addr:   &net.TCPAddr{IP: net.ParseIP(host)},
	}

	// Detect immediate exit (connection refused, no route, etc.).
	// If nc is still running after a brief grace period, the connection
	// succeeded and data can flow through the pipes.
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		stdin.Close()
		stdout.Close()
		return nil, &net.OpError{
			Op:   "dial",
			Net:  network,
			Addr: &net.TCPAddr{IP: net.ParseIP(host)},
			Err:  fmt.Errorf("nc exited immediately for %s: %w", address, err),
		}
	case <-time.After(50 * time.Millisecond):
		log.Debug().Str("addr", address).Msg("subprocess TCP relay established")
		go func() {
			if err := <-waitCh; err != nil && !strings.Contains(err.Error(), "signal: killed") {
				log.Debug().Str("addr", address).Err(err).Msg("subprocess TCP relay exited")
			}
		}()
		return conn, nil
	case <-ctx.Done():
		conn.Close()
		return nil, ctx.Err()
	}
}
