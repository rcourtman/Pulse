// Agent-backed hosts share one metrics-history vocabulary regardless of
// whether they are presented as standalone machines, Proxmox nodes, or
// Docker/Podman hosts. Keep the group definition here so those surfaces cannot
// silently diverge when host telemetry grows.
export const HOST_METRICS_HISTORY_GROUPS = [
  {
    id: 'utilization',
    label: 'Utilization',
    unit: '%',
    series: [
      { metric: 'cpu', label: 'CPU', unit: '%', color: '#8b5cf6' },
      { metric: 'memory', label: 'Memory', unit: '%', color: '#f59e0b' },
      { metric: 'disk', label: 'Disk', unit: '%', color: '#10b981' },
    ],
  },
  {
    id: 'network',
    label: 'Network I/O',
    unit: 'B/s',
    series: [
      { metric: 'netin', label: 'In', unit: 'B/s', color: '#10b981' },
      { metric: 'netout', label: 'Out', unit: 'B/s', color: '#fb923c' },
    ],
  },
  {
    id: 'disk-io',
    label: 'Disk I/O',
    unit: 'B/s',
    series: [
      { metric: 'diskread', label: 'Read', unit: 'B/s', color: '#3b82f6' },
      { metric: 'diskwrite', label: 'Write', unit: 'B/s', color: '#f59e0b' },
    ],
  },
  {
    id: 'thermals',
    label: 'Thermals',
    unit: 'C',
    series: [{ metric: 'temperature', label: 'CPU', unit: 'C', color: '#ef4444' }],
  },
];

// GPU groups are conditional because many hosts do not report a GPU. Keep the
// vocabulary beside the base host catalog so every host presentation appends
// the same persisted metric series when typed GPU telemetry is available.
export const GPU_METRICS_HISTORY_GROUPS = [
  {
    id: 'gpu-utilization',
    label: 'GPU Utilization',
    unit: '%',
    series: [
      { metric: 'gpu', label: 'Core', unit: '%', color: '#06b6d4' },
      { metric: 'gpu_memory', label: 'VRAM', unit: '%', color: '#6366f1' },
    ],
  },
  {
    id: 'gpu-thermal',
    label: 'GPU Thermal',
    unit: 'C',
    series: [{ metric: 'gpu_temperature', label: 'GPU', unit: 'C', color: '#ef4444' }],
  },
];
