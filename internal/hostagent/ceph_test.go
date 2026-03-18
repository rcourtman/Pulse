package hostagent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const fakeCephBinary = "/usr/bin/ceph"

func withCommandRunner(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, []byte, error)) {
	t.Helper()
	orig := commandRunner
	commandRunner = fn
	t.Cleanup(func() { commandRunner = orig })
}

func withLookPath(t *testing.T, fn func(file string) (string, error)) {
	t.Helper()
	orig := lookPath
	lookPath = fn
	t.Cleanup(func() { lookPath = orig })
}

func TestCommandRunner_Default(t *testing.T) {
	stdout, stderr, err := commandRunner(context.Background(), "sh", "-c", "true")
	if err != nil {
		t.Fatalf("commandRunner error: %v", err)
	}
	if len(stdout) != 0 {
		t.Fatalf("unexpected stdout: %q", string(stdout))
	}
	if len(stderr) != 0 {
		t.Fatalf("unexpected stderr: %q", string(stderr))
	}
}

func TestCommandRunner_DefaultOutputLimit(t *testing.T) {
	stdout, stderr, err := commandRunner(context.Background(), "sh", "-c", "head -c 5000000 /dev/zero")
	if !errors.Is(err, errCephCommandOutputTooLarge) {
		t.Fatalf("expected output limit error, got: %v", err)
	}
	if len(stdout) != maxCephCommandOutputSize {
		t.Fatalf("expected stdout capped at %d bytes, got %d", maxCephCommandOutputSize, len(stdout))
	}
	if len(stderr) != 0 {
		t.Fatalf("unexpected stderr: %q", string(stderr))
	}
}

func TestParseStatus(t *testing.T) {
	data := []byte(`{
	  "fsid":"fsid-123",
	  "health":{
		"status":"HEALTH_WARN",
		"checks":{
		  "OSD_DOWN":{
			"severity":"HEALTH_WARN",
			"summary":{"message":"1 osd down"},
			"detail":[{"message":"osd.1 is down"}]
		  }
		}
	  },
	  "monmap":{"epoch":7,"mons":[{"name":"a","rank":0,"addr":"10.0.0.1"}]},
	  "mgrmap":{"available":true,"active_name":"mgr-a","standbys":[{"name":"mgr-b"}]},
	  "osdmap":{"epoch":3,"num_osds":3,"num_up_osds":2,"num_in_osds":1},
	  "pgmap":{
		"num_pgs":64,
		"bytes_total":1000,
		"bytes_used":250,
		"bytes_avail":750,
		"data_bytes":200,
		"degraded_ratio":0.1,
		"misplaced_ratio":0.2,
		"read_bytes_sec":1,
		"write_bytes_sec":2,
		"read_op_per_sec":3,
		"write_op_per_sec":4
	  }
	}`)

	status, err := parseCephStatus(data)
	if err != nil {
		t.Fatalf("parseCephStatus returned error: %v", err)
	}

	if status.FSID != "fsid-123" {
		t.Fatalf("expected FSID fsid-123, got %q", status.FSID)
	}
	if status.Health.Status != "HEALTH_WARN" {
		t.Fatalf("expected HEALTH_WARN, got %q", status.Health.Status)
	}
	check, ok := status.Health.Checks["OSD_DOWN"]
	if !ok || check.Severity != "HEALTH_WARN" || check.Message != "1 osd down" || len(check.Detail) != 1 {
		t.Fatalf("unexpected parsed health checks: %+v", status.Health.Checks)
	}

	if status.MonMap.NumMons != 1 || len(status.MonMap.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %+v", status.MonMap)
	}
	if status.OSDMap.NumOSDs != 3 || status.OSDMap.NumUp != 2 || status.OSDMap.NumIn != 1 {
		t.Fatalf("unexpected OSD map: %+v", status.OSDMap)
	}
	if status.OSDMap.NumDown != 1 || status.OSDMap.NumOut != 2 {
		t.Fatalf("expected computed down/out counts, got %+v", status.OSDMap)
	}

	if status.PGMap.UsagePercent != 25.0 {
		t.Fatalf("expected usage percent 25.0, got %v", status.PGMap.UsagePercent)
	}

	if len(status.Services) != 3 {
		t.Fatalf("expected 3 service summaries, got %d", len(status.Services))
	}
	if status.Services[0].Type != "mon" || status.Services[1].Type != "mgr" || status.Services[2].Type != "osd" {
		t.Fatalf("unexpected service types: %+v", status.Services)
	}
}

func TestIsCephAvailable(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		withLookPath(t, func(file string) (string, error) {
			if file != "ceph" {
				t.Fatalf("unexpected lookup: %s", file)
			}
			return fakeCephBinary, nil
		})

		if !IsCephAvailable(context.Background()) {
			t.Fatalf("expected available")
		}
	})

	t.Run("missing", func(t *testing.T) {
		withLookPath(t, func(file string) (string, error) {
			return "", errors.New("missing")
		})

		if IsCephAvailable(context.Background()) {
			t.Fatalf("expected unavailable")
		}
	})
}

func TestRunCephCommand(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name != fakeCephBinary {
			t.Fatalf("unexpected command name: %s", name)
		}
		if len(args) < 1 || args[0] != "status" {
			t.Fatalf("unexpected args: %v", args)
		}
		return []byte(`{"ok":true}`), nil, nil
	})

	out, err := runCephCommand(context.Background(), "status", "--format", "json")
	if err != nil {
		t.Fatalf("runCephCommand error: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestRunCephCommandError(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("bad"), errors.New("boom")
	})

	_, err := runCephCommand(context.Background(), "status", "--format", "json")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "ceph status --format json failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "stderr: bad") {
		t.Fatalf("expected stderr in error: %v", err)
	}
}

func TestRunCephCommandOutputTooLarge(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return make([]byte, maxCephCommandOutputSize), nil, errCephCommandOutputTooLarge
	})

	_, err := runCephCommand(context.Background(), "status", "--format", "json")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "output exceeded") {
		t.Fatalf("expected output limit error, got: %v", err)
	}
	if strings.Contains(err.Error(), "stderr:") {
		t.Fatalf("unexpected stderr in output limit error: %v", err)
	}
}

func TestParseStatusInvalidJSON(t *testing.T) {
	_, err := parseCephStatus([]byte(`{not-json}`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestParseCephDF(t *testing.T) {
	data := []byte(`{
	  "stats":{"total_bytes":1000,"total_used_bytes":123,"percent_used":0.1234},
	  "pools":[
		{"id":1,"name":"pool-a","stats":{"bytes_used":10,"max_avail":90,"objects":7,"percent_used":0.2}}
	  ]
	}`)

	pools, usagePercent, err := parseCephDF(data)
	if err != nil {
		t.Fatalf("parseCephDF returned error: %v", err)
	}
	if usagePercent != 12.34 {
		t.Fatalf("expected percent_used 12.34, got %v", usagePercent)
	}
	if len(pools) != 1 || pools[0].Name != "pool-a" || pools[0].PercentUsed != 20.0 {
		t.Fatalf("unexpected pools parsed: %+v", pools)
	}
}

func TestParseCephDFSupportsPercentValueFormat(t *testing.T) {
	data := []byte(`{
	  "stats":{"total_bytes":1000,"total_used_bytes":123,"percent_used":12.34},
	  "pools":[
		{"id":1,"name":"pool-a","stats":{"bytes_used":10,"max_avail":90,"objects":7,"percent_used":20.5}}
	  ]
	}`)

	pools, usagePercent, err := parseCephDF(data)
	if err != nil {
		t.Fatalf("parseCephDF returned error: %v", err)
	}
	if usagePercent != 12.34 {
		t.Fatalf("expected percent_used 12.34, got %v", usagePercent)
	}
	if len(pools) != 1 || pools[0].Name != "pool-a" || pools[0].PercentUsed != 20.5 {
		t.Fatalf("unexpected pools parsed: %+v", pools)
	}
}

func TestParseCephDFInvalidJSON(t *testing.T) {
	_, _, err := parseCephDF([]byte(`{not-json}`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestCollect_NotAvailable(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		return "", errors.New("missing")
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status")
	}
}

func TestCollect_StatusError(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == fakeCephBinary && len(args) > 0 && args[0] == "status" {
			return nil, []byte("boom"), errors.New("status failed")
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status")
	}
}

func TestCollect_ParseStatusError(t *testing.T) {
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == fakeCephBinary && len(args) > 0 && args[0] == "status" {
			return []byte(`{not-json}`), nil, nil
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	_, err := CollectCeph(context.Background())
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestCollectCeph_DFCommandError(t *testing.T) {
	statusJSON := []byte(`{
	  "fsid":"fsid-1",
	  "health":{"status":"HEALTH_OK","checks":{}},
	  "monmap":{"epoch":1,"mons":[]},
	  "mgrmap":{"available":false,"active_name":"","standbys":[]},
	  "osdmap":{"epoch":1,"num_osds":0,"num_up_osds":0,"num_in_osds":0},
	  "pgmap":{"num_pgs":0,"bytes_total":100,"bytes_used":50,"bytes_avail":50,
		"data_bytes":0,"degraded_ratio":0,"misplaced_ratio":0,
		"read_bytes_sec":0,"write_bytes_sec":0,"read_op_per_sec":0,"write_op_per_sec":0}
	}`)
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == fakeCephBinary && len(args) > 0 && args[0] == "status" {
			return statusJSON, nil, nil
		}
		if name == fakeCephBinary && len(args) > 0 && args[0] == "df" {
			return nil, []byte("df failed"), errors.New("df error")
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatalf("expected status")
	}
	if status.PGMap.UsagePercent == 0 {
		t.Fatalf("expected usage percent from status")
	}
}

func TestCollectCeph_DFParseError(t *testing.T) {
	statusJSON := []byte(`{
	  "fsid":"fsid-1",
	  "health":{"status":"HEALTH_OK","checks":{}},
	  "monmap":{"epoch":1,"mons":[]},
	  "mgrmap":{"available":false,"active_name":"","standbys":[]},
	  "osdmap":{"epoch":1,"num_osds":0,"num_up_osds":0,"num_in_osds":0},
	  "pgmap":{"num_pgs":0,"bytes_total":0,"bytes_used":0,"bytes_avail":0,
		"data_bytes":0,"degraded_ratio":0,"misplaced_ratio":0,
		"read_bytes_sec":0,"write_bytes_sec":0,"read_op_per_sec":0,"write_op_per_sec":0}
	}`)
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == fakeCephBinary && len(args) > 0 && args[0] == "status" {
			return statusJSON, nil, nil
		}
		if name == fakeCephBinary && len(args) > 0 && args[0] == "df" {
			return []byte(`{not-json}`), nil, nil
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatalf("expected status")
	}
	if status.PGMap.UsagePercent != 0 {
		t.Fatalf("expected usage percent to remain 0, got %v", status.PGMap.UsagePercent)
	}
}

func TestCollectCeph_UsagePercentFromDF(t *testing.T) {
	statusJSON := []byte(`{
	  "fsid":"fsid-1",
	  "health":{"status":"HEALTH_OK","checks":{}},
	  "monmap":{"epoch":1,"mons":[]},
	  "mgrmap":{"available":false,"active_name":"","standbys":[]},
	  "osdmap":{"epoch":1,"num_osds":0,"num_up_osds":0,"num_in_osds":0},
	  "pgmap":{"num_pgs":0,"bytes_total":0,"bytes_used":0,"bytes_avail":0,
		"data_bytes":0,"degraded_ratio":0,"misplaced_ratio":0,
		"read_bytes_sec":0,"write_bytes_sec":0,"read_op_per_sec":0,"write_op_per_sec":0}
	}`)
	dfJSON := []byte(`{
	  "stats":{"total_bytes":1000,"total_used_bytes":500,"percent_used":0.5},
	  "pools":[]
	}`)
	withLookPath(t, func(file string) (string, error) {
		if file != "ceph" {
			t.Fatalf("unexpected lookup: %s", file)
		}
		return fakeCephBinary, nil
	})

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == fakeCephBinary && len(args) > 0 && args[0] == "status" {
			return statusJSON, nil, nil
		}
		if name == fakeCephBinary && len(args) > 0 && args[0] == "df" {
			return dfJSON, nil, nil
		}
		t.Fatalf("unexpected command: %s %v", name, args)
		return nil, nil, nil
	})

	status, err := CollectCeph(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status == nil {
		t.Fatalf("expected status")
	}
	if status.PGMap.UsagePercent != 50 {
		t.Fatalf("expected usage percent from df, got %v", status.PGMap.UsagePercent)
	}
	if status.CollectedAt.IsZero() {
		t.Fatalf("expected collected timestamp set")
	}
}

func TestCephBoolToInt(t *testing.T) {
	if cephBoolToInt(true) != 1 {
		t.Fatalf("expected cephBoolToInt(true)=1")
	}
	if cephBoolToInt(false) != 0 {
		t.Fatalf("expected cephBoolToInt(false)=0")
	}
}
