package hostagent

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
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
	UnraidStorage(ctx context.Context) (*agentshost.UnraidStorage, error)
	CephStatus(ctx context.Context) (*CephClusterStatus, error)
	SMARTLocal(ctx context.Context, exclude []string, unraid *agentshost.UnraidStorage) ([]DiskSMART, error)
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

type diskFilterMetricsCollector interface {
	MetricsWithDiskFilters(ctx context.Context, exclude, include []string) (hostmetrics.Snapshot, error)
}

func collectMetricsWithDiskFilters(ctx context.Context, collector SystemCollector, exclude, include []string) (hostmetrics.Snapshot, error) {
	if filtered, ok := collector.(diskFilterMetricsCollector); ok {
		return filtered.MetricsWithDiskFilters(ctx, exclude, include)
	}
	return collector.Metrics(ctx, exclude)
}

// NewDefaultCollector returns a SystemCollector that uses real OS calls.
func NewDefaultCollector() SystemCollector {
	return &defaultCollector{}
}

type defaultCollector struct {
	metrics hostmetrics.Collector
}

func (c *defaultCollector) HostInfo(ctx context.Context) (*gohost.InfoStat, error) {
	return gohost.InfoWithContext(ctx)
}

func (c *defaultCollector) HostUptime(ctx context.Context) (uint64, error) {
	return gohost.UptimeWithContext(ctx)
}

func (c *defaultCollector) Metrics(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
	return c.metrics.Collect(ctx, exclude)
}

func (c *defaultCollector) MetricsWithDiskFilters(ctx context.Context, exclude, include []string) (hostmetrics.Snapshot, error) {
	return c.metrics.CollectWithDiskFilters(ctx, exclude, include)
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
	return CollectRAIDArrays(ctx)
}

func (c *defaultCollector) UnraidStorage(ctx context.Context) (*agentshost.UnraidStorage, error) {
	return CollectUnraidStorage(ctx, c)
}

func (c *defaultCollector) CephStatus(ctx context.Context) (*CephClusterStatus, error) {
	return CollectCeph(ctx)
}

func (c *defaultCollector) SMARTLocal(ctx context.Context, exclude []string, unraid *agentshost.UnraidStorage) ([]DiskSMART, error) {
	return CollectSMARTLocalWithUnraid(ctx, exclude, unraid)
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

func (c *defaultCollector) CommandCombinedOutputLimited(
	ctx context.Context,
	maxBytes int,
	name string,
	arg ...string,
) (string, error) {
	if maxBytes <= 0 {
		return "", fmt.Errorf("command output limit must be positive")
	}
	output := &limitedCombinedBuffer{limit: maxBytes}
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	if output.Exceeded() {
		return output.String(), fmt.Errorf("command output exceeds %d bytes", maxBytes)
	}
	return output.String(), err
}

func (c *defaultCollector) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// limitedCombinedBuffer bounds memory while preserving CombinedOutput-like
// capture of stdout and stderr. Writes beyond the limit are accepted and
// discarded so the child can exit normally or be stopped by its context.
type limitedCombinedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedCombinedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	originalLength := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if remaining < len(p) {
			_, _ = b.buffer.Write(p[:remaining])
		} else {
			_, _ = b.buffer.Write(p)
		}
	}
	if originalLength > remaining {
		b.exceeded = true
	}
	return originalLength, nil
}

func (b *limitedCombinedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *limitedCombinedBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}
