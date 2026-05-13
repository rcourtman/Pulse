package alerts

import "testing"

func TestInferDiskHardwareType_NVMeDevice(t *testing.T) {
	got := inferDiskHardwareType("/dev/nvme0n1p1")
	if got != "nvme" {
		t.Fatalf("inferDiskHardwareType(/dev/nvme0n1p1) = %q, want %q", got, "nvme")
	}
}

func TestInferDiskHardwareType_SATADevice(t *testing.T) {
	got := inferDiskHardwareType("/dev/sda1")
	if got != "" {
		t.Fatalf("inferDiskHardwareType(/dev/sda1) = %q, want empty", got)
	}
}

func TestInferDiskHardwareType_EmptyDevice(t *testing.T) {
	got := inferDiskHardwareType("")
	if got != "" {
		t.Fatalf("inferDiskHardwareType(\"\") = %q, want empty", got)
	}
}

func TestInferDiskHardwareType_CaseInsensitive(t *testing.T) {
	got := inferDiskHardwareType("/dev/NVMe0n1")
	if got != "nvme" {
		t.Fatalf("inferDiskHardwareType(/dev/NVMe0n1) = %q, want %q", got, "nvme")
	}
}
