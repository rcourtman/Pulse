package hostagent

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	gohost "github.com/shirou/gopsutil/v4/host"
)

type mockCollector struct {
	hostInfoFn      func(ctx context.Context) (*gohost.InfoStat, error)
	hostUptimeFn    func(ctx context.Context) (uint64, error)
	metricsFn       func(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error)
	sensorsLocalFn  func(ctx context.Context) (string, error)
	sensorsParseFn  func(jsonStr string) (*sensors.TemperatureData, error)
	sensorsPowerFn  func(ctx context.Context) (*sensors.PowerData, error)
	raidArraysFn    func(ctx context.Context) ([]agentshost.RAIDArray, error)
	cephStatusFn    func(ctx context.Context) (*CephClusterStatus, error)
	smartLocalFn    func(ctx context.Context, exclude []string) ([]DiskSMART, error)
	nowFn           func() time.Time
	goos            string
	readFileFn      func(name string) ([]byte, error)
	netInterfacesFn func() ([]net.Interface, error)

	hostnameFn              func() (string, error)
	lookupIPFn              func(host string) ([]net.IP, error)
	dialTimeoutFn           func(network, address string, timeout time.Duration) (net.Conn, error)
	statFn                  func(name string) (os.FileInfo, error)
	mkdirAllFn              func(path string, perm os.FileMode) error
	writeFileFn             func(filename string, data []byte, perm os.FileMode) error
	commandCombinedOutputFn func(ctx context.Context, name string, arg ...string) (string, error)
	lookPathFn              func(file string) (string, error)
}

func (m *mockCollector) HostInfo(ctx context.Context) (*gohost.InfoStat, error) {
	if m.hostInfoFn != nil {
		return m.hostInfoFn(ctx)
	}
	return &gohost.InfoStat{}, nil
}

func (m *mockCollector) HostUptime(ctx context.Context) (uint64, error) {
	if m.hostUptimeFn != nil {
		return m.hostUptimeFn(ctx)
	}
	return 0, nil
}

func (m *mockCollector) Metrics(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
	if m.metricsFn != nil {
		return m.metricsFn(ctx, exclude)
	}
	return hostmetrics.Snapshot{}, nil
}

func (m *mockCollector) SensorsLocal(ctx context.Context) (string, error) {
	if m.sensorsLocalFn != nil {
		return m.sensorsLocalFn(ctx)
	}
	return "", nil
}

func (m *mockCollector) SensorsParse(jsonStr string) (*sensors.TemperatureData, error) {
	if m.sensorsParseFn != nil {
		return m.sensorsParseFn(jsonStr)
	}
	return &sensors.TemperatureData{}, nil
}

func (m *mockCollector) SensorsPower(ctx context.Context) (*sensors.PowerData, error) {
	if m.sensorsPowerFn != nil {
		return m.sensorsPowerFn(ctx)
	}
	return &sensors.PowerData{}, nil
}

func (m *mockCollector) RAIDArrays(ctx context.Context) ([]agentshost.RAIDArray, error) {
	if m.raidArraysFn != nil {
		return m.raidArraysFn(ctx)
	}
	return nil, nil
}

func (m *mockCollector) CephStatus(ctx context.Context) (*CephClusterStatus, error) {
	if m.cephStatusFn != nil {
		return m.cephStatusFn(ctx)
	}
	return nil, nil
}

func (m *mockCollector) SMARTLocal(ctx context.Context, exclude []string) ([]DiskSMART, error) {
	if m.smartLocalFn != nil {
		return m.smartLocalFn(ctx, exclude)
	}
	return nil, nil
}

func (m *mockCollector) Now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func (m *mockCollector) GOOS() string {
	if m.goos != "" {
		return m.goos
	}
	return "linux" // Default to linux for testing Linux-specific paths
}

func (m *mockCollector) ReadFile(name string) ([]byte, error) {
	if m.readFileFn != nil {
		return m.readFileFn(name)
	}
	return nil, os.ErrNotExist
}

func (m *mockCollector) NetInterfaces() ([]net.Interface, error) {
	if m.netInterfacesFn != nil {
		return m.netInterfacesFn()
	}
	return nil, nil
}

func (m *mockCollector) Hostname() (string, error) {
	if m.hostnameFn != nil {
		return m.hostnameFn()
	}
	return "mock-host", nil
}

func (m *mockCollector) LookupIP(host string) ([]net.IP, error) {
	if m.lookupIPFn != nil {
		return m.lookupIPFn(host)
	}
	return nil, nil
}

func (m *mockCollector) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if m.dialTimeoutFn != nil {
		return m.dialTimeoutFn(network, address, timeout)
	}
	return nil, nil
}

func (m *mockCollector) Stat(name string) (os.FileInfo, error) {
	if m.statFn != nil {
		return m.statFn(name)
	}
	return nil, os.ErrNotExist
}

func (m *mockCollector) MkdirAll(path string, perm os.FileMode) error {
	if m.mkdirAllFn != nil {
		return m.mkdirAllFn(path, perm)
	}
	return nil
}

func (m *mockCollector) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if m.writeFileFn != nil {
		return m.writeFileFn(filename, data, perm)
	}
	return nil
}

func (m *mockCollector) CommandCombinedOutput(ctx context.Context, name string, arg ...string) (string, error) {
	if m.commandCombinedOutputFn != nil {
		return m.commandCombinedOutputFn(ctx, name, arg...)
	}
	return "", nil
}

func (m *mockCollector) LookPath(file string) (string, error) {
	if m.lookPathFn != nil {
		return m.lookPathFn(file)
	}
	return "", os.ErrNotExist
}
