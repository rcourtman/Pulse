export interface ChartPoint {
  timestamp: number;
  value: number;
}

export interface MetricData {
  cpu: ChartPoint[];
  memory: ChartPoint[];
  disk: ChartPoint[];
  diskread: ChartPoint[];
  diskwrite: ChartPoint[];
  netin: ChartPoint[];
  netout: ChartPoint[];
  [key: string]: ChartPoint[];
}

export interface ChartData {
  [guestId: string]: MetricData;
}

export interface NodeMetricData {
  cpu: ChartPoint[];
  memory: ChartPoint[];
  disk: ChartPoint[];
  [key: string]: ChartPoint[];
}

export interface NodeChartData {
  [nodeId: string]: NodeMetricData;
}

export interface StorageMetricData {
  disk: ChartPoint[];
  [key: string]: ChartPoint[];
}

export interface StorageChartData {
  [storageId: string]: StorageMetricData;
}

export interface ChartStats {
  oldestDataTimestamp: number;
}