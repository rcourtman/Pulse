package models

import (
	"math"
	"testing"
)

func TestPrimaryCPUTemperatureCelsiusTakesTheHottestReading(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *Temperature
		want *float64
	}{
		{name: "nil", in: nil},
		{name: "unavailable", in: &Temperature{Available: false, CPUPackage: 90}},
		{name: "no CPU reading", in: &Temperature{Available: true}},
		{name: "package only", in: &Temperature{Available: true, CPUPackage: 58}, want: ptr(58)},
		{name: "producer max above package", in: &Temperature{Available: true, CPUPackage: 58, CPUMax: 60}, want: ptr(60)},
		{name: "package above max of cores", in: &Temperature{Available: true, CPUPackage: 61, CPUMax: 57}, want: ptr(61)},
		{name: "core above package and max", in: &Temperature{
			Available: true, CPUPackage: 60, CPUMax: 60,
			Cores: []CoreTemp{{Core: 0, Temp: 55}, {Core: 1, Temp: 66}},
		}, want: ptr(66)},
		{name: "cores only", in: &Temperature{
			Available: true,
			Cores:     []CoreTemp{{Core: 0, Temp: 42}, {Core: 1, Temp: 55}},
		}, want: ptr(55)},
		{name: "unusable readings ignored", in: &Temperature{
			Available: true, CPUPackage: math.NaN(), CPUMax: math.Inf(1),
			Cores: []CoreTemp{{Core: 0, Temp: -1}, {Core: 1, Temp: 0}, {Core: 2, Temp: 47}},
		}, want: ptr(47)},
		{name: "only unusable readings", in: &Temperature{
			Available: true, CPUPackage: math.NaN(),
			Cores: []CoreTemp{{Core: 0, Temp: -1}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertTemperature(t, PrimaryCPUTemperatureCelsius(tc.in), tc.want)
		})
	}
}

func TestHostSensorPrimaryCPUTemperatureCelsiusTakesTheHottestCPUKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sensors map[string]float64
		want    *float64
	}{
		{name: "no sensors"},
		{name: "package only", sensors: map[string]float64{"cpu_package": 62.5}, want: ptr(62.5)},
		{name: "package above cores", sensors: map[string]float64{"cpu_package": 61, "cpu_core_0": 55}, want: ptr(61)},
		{name: "core above package", sensors: map[string]float64{"cpu_package": 60, "cpu_core_0": 66}, want: ptr(66)},
		{name: "non-CPU sensors ignored", sensors: map[string]float64{"nvme0": 80, "gpu_edge": 75, "cpu_core_1": 50}, want: ptr(50)},
		{name: "unusable readings ignored", sensors: map[string]float64{"cpu_package": 0, "cpu_core_0": math.NaN(), "cpu_core_1": 44}, want: ptr(44)},
		{name: "no CPU key", sensors: map[string]float64{"nvme0": 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := HostSensorPrimaryCPUTemperatureCelsius(HostSensorSummary{TemperatureCelsius: tc.sensors})
			assertTemperature(t, got, tc.want)
		})
	}
}

func ptr(value float64) *float64 { return &value }

func assertTemperature(t *testing.T, got, want *float64) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("got %v, want nil", *got)
	case want != nil && got == nil:
		t.Fatalf("got nil, want %v", *want)
	case want != nil && *got != *want:
		t.Fatalf("got %v, want %v", *got, *want)
	}
}
