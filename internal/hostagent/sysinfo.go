package hostagent

import (
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ceph"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mdadm"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	"github.com/rcourtman/pulse-go-rewrite/internal/smartctl"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// SystemCollector abstracts system-level information gathering for testability.
type SystemCollector interface {
	HostInfo(ctx context.Context) (*gohost.InfoStat, error)
	HostUptime(ctx context.Context) (uint64, error)
	Metrics(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error)
	SensorsLocal(ctx context.Context) (string, error)
	SensorsParse(jsonStr string) (*sensors.TemperatureData, error)
	SensorsPower(ctx context.Context) (*sensors.PowerData, error)
	RAIDArrays(ctx context.Context) ([]agentshost.RAIDArray, error)
	CephStatus(ctx context.Context) (*ceph.ClusterStatus, error)
	SMARTLocal(ctx context.Context, exclude []string) ([]smartctl.DiskSMART, error)
	Now() time.Time
	GOOS() string
	ReadFile(name string) ([]byte, error)
	NetInterfaces() ([]net.Interface, error)
	Hostname() (string, error)
	LookupIP(host string) ([]net.IP, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Chmod(name string, mode os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	CommandCombinedOutput(ctx context.Context, name string, arg ...string) (string, error)
	LookPath(file string) (string, error)
}

// NewDefaultCollector returns a SystemCollector that uses real OS calls.
func NewDefaultCollector() SystemCollector {
	return &defaultCollector{}
}

type defaultCollector struct{}

func (c *defaultCollector) HostInfo(ctx context.Context) (*gohost.InfoStat, error) {
	return gohost.InfoWithContext(ctx)
}

func (c *defaultCollector) HostUptime(ctx context.Context) (uint64, error) {
	return gohost.UptimeWithContext(ctx)
}

func (c *defaultCollector) Metrics(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
	return hostmetrics.Collect(ctx, exclude)
}

func (c *defaultCollector) SensorsLocal(ctx context.Context) (string, error) {
	return sensors.CollectLocal(ctx)
}

func (c *defaultCollector) SensorsParse(jsonStr string) (*sensors.TemperatureData, error) {
	return sensors.Parse(jsonStr)
}

func (c *defaultCollector) SensorsPower(ctx context.Context) (*sensors.PowerData, error) {
	return sensors.CollectPower(ctx)
}

func (c *defaultCollector) RAIDArrays(ctx context.Context) ([]agentshost.RAIDArray, error) {
	return mdadm.CollectArrays(ctx)
}

func (c *defaultCollector) CephStatus(ctx context.Context) (*ceph.ClusterStatus, error) {
	return ceph.Collect(ctx)
}

func (c *defaultCollector) SMARTLocal(ctx context.Context, exclude []string) ([]smartctl.DiskSMART, error) {
	return smartctl.CollectLocal(ctx, exclude)
}

func (c *defaultCollector) Now() time.Time {
	return time.Now().UTC()
}

func (c *defaultCollector) GOOS() string {
	return runtime.GOOS
}

func (c *defaultCollector) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (c *defaultCollector) NetInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (c *defaultCollector) Hostname() (string, error) {
	return os.Hostname()
}

func (c *defaultCollector) LookupIP(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

func (c *defaultCollector) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

func (c *defaultCollector) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (c *defaultCollector) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (c *defaultCollector) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (c *defaultCollector) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (c *defaultCollector) CommandCombinedOutput(ctx context.Context, name string, arg ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, arg...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (c *defaultCollector) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
