package config

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper process for mocking exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "hostname":
		if len(args) > 0 && args[0] == "-I" {
			fmt.Print("192.168.1.100 172.17.0.1")
		}
	}
}

// Mock net.Conn
type mockConn struct {
	net.Conn
	localAddr net.Addr
}

func (m *mockConn) LocalAddr() net.Addr {
	return m.localAddr
}
func (m *mockConn) Close() error { return nil }

type mockAddr struct {
	ip string
}

func (m *mockAddr) Network() string { return "udp" }
func (m *mockAddr) String() string  { return m.ip }

func TestDetectPublicURL(t *testing.T) {
	// Backup original vars
	origOsStat := osStat
	origExecCommand := execCommand
	origNetDial := netDial
	origNetInterfaceAddrs := netInterfaceAddrs
	defer func() {
		osStat = origOsStat
		execCommand = origExecCommand
		netDial = origNetDial
		netInterfaceAddrs = origNetInterfaceAddrs
	}()

	t.Run("Docker Environment", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) {
			if name == "/.dockerenv" {
				return nil, nil // Exists
			}
			return nil, os.ErrNotExist
		}

		url := detectPublicURL(8080)
		assert.Equal(t, "", url)
	})

	t.Run("Proxmox Environment (hostname -I)", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) {
			if name == "/etc/pve" {
				return nil, nil // Exists
			}
			return nil, os.ErrNotExist
		}

		execCommand = func(name string, arg ...string) *exec.Cmd {
			cs := []string{"-test.run=TestHelperProcess", "--", name}
			cs = append(cs, arg...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
			return cmd
		}

		url := detectPublicURL(8080)
		assert.Equal(t, "http://192.168.1.100:8080", url)
	})

	t.Run("Outbound (Method 2)", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }

		netDial = func(network, address string) (net.Conn, error) {
			return &mockConn{
				localAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.50")},
			}, nil
		}

		url := detectPublicURL(8080)
		assert.Equal(t, "http://10.0.0.50:8080", url)
	})

	t.Run("Interface Addrs (Method 3 - Private)", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		netDial = func(network, address string) (net.Conn, error) { return nil, fmt.Errorf("fail") }

		netInterfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.ParseIP("192.168.1.200"), Mask: net.CIDRMask(24, 32)},
			}, nil
		}

		url := detectPublicURL(8080)
		assert.Equal(t, "http://192.168.1.200:8080", url)
	})

	t.Run("Interface Addrs (Method 3 - Public)", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		netDial = func(network, address string) (net.Conn, error) { return nil, fmt.Errorf("fail") }

		netInterfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.ParseIP("123.45.67.89"), Mask: net.CIDRMask(24, 32)},
			}, nil
		}

		url := detectPublicURL(8080)
		assert.Equal(t, "http://123.45.67.89:8080", url)
	})

	t.Run("None Found", func(t *testing.T) {
		osStat = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		netDial = func(network, address string) (net.Conn, error) { return nil, fmt.Errorf("fail") }
		netInterfaceAddrs = func() ([]net.Addr, error) { return nil, fmt.Errorf("fail") }

		url := detectPublicURL(8080)
		assert.Equal(t, "", url)
	})
}

func TestGetOutboundIP_Fallback(t *testing.T) {
	// Backup
	origNetDial := netDial
	defer func() { netDial = origNetDial }()

	// Fail first dial, succeed second
	netDial = func(network, address string) (net.Conn, error) {
		if address == "8.8.8.8:80" {
			return nil, fmt.Errorf("fail 1")
		}
		if address == "1.1.1.1:80" {
			return &mockConn{
				localAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.51")},
			}, nil
		}
		return nil, fmt.Errorf("unexpected address")
	}

	ip := getOutboundIP()
	assert.Equal(t, "10.0.0.51", ip)
}

func TestGetOutboundIP_AllFail(t *testing.T) {
	origNetDial := netDial
	defer func() { netDial = origNetDial }()

	netDial = func(network, address string) (net.Conn, error) {
		return nil, fmt.Errorf("fail")
	}

	ip := getOutboundIP()
	assert.Equal(t, "", ip)
}

func TestGetOutboundIP_NonUDPAddr(t *testing.T) {
	origNetDial := netDial
	defer func() { netDial = origNetDial }()

	netDial = func(network, address string) (net.Conn, error) {
		return &mockConn{
			localAddr: &mockAddr{ip: "not-udp"},
		}, nil
	}

	ip := getOutboundIP()
	assert.Equal(t, "", ip)
}
