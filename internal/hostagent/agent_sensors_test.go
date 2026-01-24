package hostagent

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	"github.com/rs/zerolog"
)

func TestAgent_collectTemperatures_MapsKeys(t *testing.T) {
	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return "{}", nil },
		sensorsParseFn: func(string) (*sensors.TemperatureData, error) {
			return &sensors.TemperatureData{
				Available:  true,
				CPUPackage: 55.5,
				Cores: map[string]float64{
					"Core 0": 44,
					"Core 1": 45,
				},
				NVMe: map[string]float64{
					"nvme0": 40,
				},
				GPU: map[string]float64{
					"amdgpu-pci-0100": 60,
				},
			}, nil
		},
	}

	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	want := map[string]float64{
		"cpu_package":     55.5,
		"cpu_core_0":      44,
		"cpu_core_1":      45,
		"nvme0":           40,
		"amdgpu-pci-0100": 60,
	}

	if got.TemperatureCelsius == nil {
		t.Fatalf("expected TemperatureCelsius map to be initialised")
	}
	if len(got.TemperatureCelsius) != len(want) {
		t.Fatalf("temperature keys = %d, want %d", len(got.TemperatureCelsius), len(want))
	}
	for k, v := range want {
		if gotVal, ok := got.TemperatureCelsius[k]; !ok || gotVal != v {
			t.Fatalf("TemperatureCelsius[%q] = (%v, %v), want (%v, %v)", k, gotVal, ok, v, true)
		}
	}
}

func TestAgent_collectTemperatures_BestEffortFailuresReturnEmpty(t *testing.T) {
	mc := &mockCollector{goos: "linux"}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	mc.sensorsLocalFn = func(context.Context) (string, error) { return "", errors.New("no sensors") }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on collect error, got %#v", got.TemperatureCelsius)
	}

	mc.sensorsLocalFn = func(context.Context) (string, error) { return "{}", nil }
	mc.sensorsParseFn = func(string) (*sensors.TemperatureData, error) { return nil, errors.New("bad json") }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on parse error, got %#v", got.TemperatureCelsius)
	}

	mc.sensorsParseFn = func(string) (*sensors.TemperatureData, error) { return &sensors.TemperatureData{Available: false}, nil }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors when unavailable, got %#v", got.TemperatureCelsius)
	}
}

func TestAgent_collectTemperatures_SkipsNonLinux(t *testing.T) {
	mc := &mockCollector{goos: "darwin"}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on non-linux, got %#v", got.TemperatureCelsius)
	}
}
