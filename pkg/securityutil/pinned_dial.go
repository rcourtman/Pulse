package securityutil

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

const pinnedFallbackDelay = 300 * time.Millisecond

// DialPinnedIPs connects only to the supplied, already approved IP addresses.
// It never resolves a hostname. Two staggered workers allow a healthy address
// to win while another connect stalls, under one positive overall timeout.
// Callers retain responsibility for destination policy and TLS verification.
func DialPinnedIPs(ctx context.Context, network, port string, ips []net.IP, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{}
	return dialPinnedIPs(ctx, network, port, ips, timeout, dialer.DialContext)
}

type pinnedDialFunc func(context.Context, string, string) (net.Conn, error)

func dialPinnedIPs(ctx context.Context, network, port string, ips []net.IP, timeout time.Duration, dial pinnedDialFunc) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported pinned network %q", network)
	}
	numericPort, err := strconv.Atoi(port)
	if err != nil || numericPort < 1 || numericPort > 65535 {
		return nil, fmt.Errorf("invalid pinned TCP port %q", port)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("pinned connection timeout must be positive")
	}

	// Normalize and copy before starting any I/O. In particular, nil or invalid
	// IP bytes must never become a hostname for the underlying dialer.
	var preferred, alternate []string
	seen := make(map[string]bool, len(ips))
	preferredIPv4 := false
	for _, ip := range ips {
		if ip.To16() == nil {
			return nil, fmt.Errorf("invalid pinned IP address")
		}
		ipv4 := ip.To4() != nil
		if network == "tcp4" && !ipv4 || network == "tcp6" && ipv4 {
			continue
		}
		address := net.JoinHostPort(ip.String(), port)
		if seen[address] {
			continue
		}
		seen[address] = true
		if len(preferred) == 0 {
			preferredIPv4 = ipv4
		}
		if ipv4 == preferredIPv4 {
			preferred = append(preferred, address)
		} else {
			alternate = append(alternate, address)
		}
	}
	if len(preferred) == 0 {
		return nil, fmt.Errorf("no pinned IP addresses for %s", network)
	}

	// Keep each family's resolver order in its own worker. For a single
	// family, split alternating addresses so a stalled leading A/AAAA record
	// still has a concurrent fallback.
	lanes := [2][]string{preferred, alternate}
	if len(alternate) == 0 {
		lanes = [2][]string{}
		for index, address := range preferred {
			lanes[index%2] = append(lanes[index%2], address)
		}
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type result struct {
		lane int
		conn net.Conn
		err  error
	}
	results := make(chan result)
	start := func(lane int) {
		go func() {
			var firstErr error
			for index, address := range lanes[lane] {
				if dialCtx.Err() != nil {
					break
				}
				// Share the remaining budget so two stalled addresses cannot
				// starve a healthy address later in either worker's list.
				deadline, _ := dialCtx.Deadline()
				attemptCtx, attemptCancel := context.WithTimeout(dialCtx, time.Until(deadline)/time.Duration(len(lanes[lane])-index))
				conn, err := dial(attemptCtx, network, address)
				attemptCancel()
				if err == nil {
					select {
					case results <- result{lane: lane, conn: conn}:
					case <-dialCtx.Done():
						_ = conn.Close()
					}
					return
				}
				if conn != nil {
					_ = conn.Close()
				}
				if firstErr == nil {
					firstErr = err
				}
			}
			select {
			case results <- result{lane: lane, err: firstErr}:
			case <-dialCtx.Done():
			}
		}()
	}
	start(0)
	pending := 1
	var fallback <-chan time.Time
	var timer *time.Timer
	if len(lanes[1]) > 0 {
		deadline, _ := dialCtx.Deadline()
		// Preserve fallback opportunity even for a caller deadline shorter
		// than the normal stagger.
		timer = time.NewTimer(min(pinnedFallbackDelay, time.Until(deadline)/2))
		defer timer.Stop()
		fallback = timer.C
	}
	var failures [2]error
	for {
		select {
		case <-dialCtx.Done():
			return nil, dialCtx.Err()
		case <-fallback:
			fallback = nil
			start(1)
			pending++
		case outcome := <-results:
			if outcome.conn != nil {
				if err := dialCtx.Err(); err != nil {
					_ = outcome.conn.Close()
					return nil, err
				}
				return outcome.conn, nil
			}
			failures[outcome.lane] = outcome.err
			pending--
			if fallback != nil {
				timer.Stop()
				fallback = nil
				start(1)
				pending++
			}
			if pending == 0 {
				if err := dialCtx.Err(); err != nil {
					return nil, err
				}
				return nil, failures[0]
			}
		}
	}
}
