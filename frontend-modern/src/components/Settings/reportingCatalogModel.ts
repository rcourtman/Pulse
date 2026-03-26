import {
  parseVMInventoryExportDefinition,
  type ReportingInventoryExportDefinition,
} from '@/components/Settings/reportingInventoryExportModel';

export type ReportingFormat = 'pdf' | 'csv';
export type ReportingFilenameDateStyle = 'utc_yyyymmdd';
export type ReportingFilenameSubject = 'resource_id';

export interface ReportingFormatDefinition {
  value: ReportingFormat;
  label: string;
}

export interface ReportingRangeDefinition {
  key: string;
  label: string;
  description: string;
  windowHours: number;
}

export interface ReportingPerformanceReportDefinition {
  id: string;
  title: string;
  description: string;
  singleResourceEndpoint: string;
  multiResourceEndpoint: string;
  singleFilenamePrefix: string;
  singleFilenameSubject: ReportingFilenameSubject;
  multiFilenamePrefix: string;
  filenameDateStyle: ReportingFilenameDateStyle;
  formats: ReportingFormatDefinition[];
  defaultFormat: ReportingFormat;
  ranges: ReportingRangeDefinition[];
  defaultRange: string;
  multiResourceMax: number;
  supportsMetricFilter: boolean;
  supportsCustomTitle: boolean;
}

export interface ReportingLockedStateDefinition {
  title: string;
  description: string;
}

export interface ReportingGuidanceDefinition {
  title: string;
  description: string;
}

export interface ReportingCatalog {
  id: string;
  title: string;
  description: string;
  lockedState: ReportingLockedStateDefinition;
  guidance: ReportingGuidanceDefinition;
  performanceReport: ReportingPerformanceReportDefinition;
  vmInventoryExport: ReportingInventoryExportDefinition | null;
}

export function buildReportingCatalogRequest(): { url: string } {
  return {
    url: '/api/admin/reports/catalog',
  };
}

export function buildLegacyReportingCatalogFallback(): ReportingCatalog {
  return {
    id: 'advanced_reporting',
    title: 'Detailed Reporting',
    description: 'Generate performance reports across infrastructure and workloads.',
    lockedState: {
      title: 'Advanced Reporting (Pro)',
      description: 'Generate PDF and CSV performance reports across infrastructure and workload resources.',
    },
    guidance: {
      title: 'Advanced Insights',
      description: 'Older Pulse servers expose the legacy reporting transport directly. Performance reports remain available here, but newer catalog-driven reporting surfaces require a newer backend.',
    },
    performanceReport: {
      id: 'performance_reports',
      title: 'Performance Reports',
      description: 'Generate PDF summaries or CSV metric exports from historical monitoring data for one or more selected resources.',
      singleResourceEndpoint: '/api/reporting',
      multiResourceEndpoint: '/api/reporting/generate-multi',
      singleFilenamePrefix: 'report',
      singleFilenameSubject: 'resource_id',
      multiFilenamePrefix: 'fleet-report',
      filenameDateStyle: 'utc_yyyymmdd',
      formats: [
        { value: 'pdf', label: 'PDF Report' },
        { value: 'csv', label: 'CSV Data' },
      ],
      defaultFormat: 'pdf',
      ranges: [
        {
          key: '24h',
          label: 'Last 24 Hours',
          description: 'Current-day operational summary for short-term regressions.',
          windowHours: 24,
        },
        {
          key: '7d',
          label: 'Last 7 Days',
          description: 'Weekly trend window for recent performance changes.',
          windowHours: 168,
        },
        {
          key: '30d',
          label: 'Last 30 Days',
          description: 'Monthly review window for sustained capacity or reliability shifts.',
          windowHours: 720,
        },
      ],
      defaultRange: '24h',
      multiResourceMax: 50,
      supportsMetricFilter: true,
      supportsCustomTitle: true,
    },
    vmInventoryExport: null,
  };
}

function parseReportingFormatDefinition(input: unknown): ReportingFormatDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingFormatDefinition>;
  if (
    (candidate.value !== 'pdf' && candidate.value !== 'csv') ||
    typeof candidate.label !== 'string'
  ) {
    throw new Error('Invalid reporting catalog payload');
  }
  return {
    value: candidate.value,
    label: candidate.label,
  };
}

function parseReportingRangeDefinition(input: unknown): ReportingRangeDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingRangeDefinition>;
  if (
    typeof candidate.key !== 'string' ||
    typeof candidate.label !== 'string' ||
    typeof candidate.description !== 'string' ||
    typeof candidate.windowHours !== 'number' ||
    !Number.isFinite(candidate.windowHours) ||
    candidate.windowHours <= 0
  ) {
    throw new Error('Invalid reporting catalog payload');
  }
  return {
    key: candidate.key,
    label: candidate.label,
    description: candidate.description,
    windowHours: candidate.windowHours,
  };
}

function parseReportingPerformanceReportDefinition(
  input: unknown,
): ReportingPerformanceReportDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingPerformanceReportDefinition>;
  if (
    typeof candidate.id !== 'string' ||
    typeof candidate.title !== 'string' ||
    typeof candidate.description !== 'string' ||
    typeof candidate.singleResourceEndpoint !== 'string' ||
    typeof candidate.multiResourceEndpoint !== 'string' ||
    typeof candidate.singleFilenamePrefix !== 'string' ||
    candidate.singleFilenameSubject !== 'resource_id' ||
    typeof candidate.multiFilenamePrefix !== 'string' ||
    candidate.filenameDateStyle !== 'utc_yyyymmdd' ||
    !Array.isArray(candidate.formats) ||
    (candidate.defaultFormat !== 'pdf' && candidate.defaultFormat !== 'csv') ||
    !Array.isArray(candidate.ranges) ||
    typeof candidate.defaultRange !== 'string' ||
    typeof candidate.multiResourceMax !== 'number' ||
    !Number.isFinite(candidate.multiResourceMax) ||
    candidate.multiResourceMax <= 0 ||
    typeof candidate.supportsMetricFilter !== 'boolean' ||
    typeof candidate.supportsCustomTitle !== 'boolean'
  ) {
    throw new Error('Invalid reporting catalog payload');
  }

  const formats = candidate.formats.map(parseReportingFormatDefinition);
  const ranges = candidate.ranges.map(parseReportingRangeDefinition);
  if (formats.length === 0 || ranges.length === 0) {
    throw new Error('Invalid reporting catalog payload');
  }
  if (!formats.some((format) => format.value === candidate.defaultFormat)) {
    throw new Error('Invalid reporting catalog payload');
  }
  if (!ranges.some((range) => range.key === candidate.defaultRange)) {
    throw new Error('Invalid reporting catalog payload');
  }

  return {
    id: candidate.id,
    title: candidate.title,
    description: candidate.description,
    singleResourceEndpoint: candidate.singleResourceEndpoint,
    multiResourceEndpoint: candidate.multiResourceEndpoint,
    singleFilenamePrefix: candidate.singleFilenamePrefix,
    singleFilenameSubject: candidate.singleFilenameSubject,
    multiFilenamePrefix: candidate.multiFilenamePrefix,
    filenameDateStyle: candidate.filenameDateStyle,
    formats,
    defaultFormat: candidate.defaultFormat,
    ranges,
    defaultRange: candidate.defaultRange,
    multiResourceMax: candidate.multiResourceMax,
    supportsMetricFilter: candidate.supportsMetricFilter,
    supportsCustomTitle: candidate.supportsCustomTitle,
  };
}

function parseReportingLockedStateDefinition(input: unknown): ReportingLockedStateDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingLockedStateDefinition>;
  if (typeof candidate.title !== 'string' || typeof candidate.description !== 'string') {
    throw new Error('Invalid reporting catalog payload');
  }

  return {
    title: candidate.title,
    description: candidate.description,
  };
}

function parseReportingGuidanceDefinition(input: unknown): ReportingGuidanceDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingGuidanceDefinition>;
  if (typeof candidate.title !== 'string' || typeof candidate.description !== 'string') {
    throw new Error('Invalid reporting catalog payload');
  }

  return {
    title: candidate.title,
    description: candidate.description,
  };
}

export function parseReportingCatalog(input: unknown): ReportingCatalog {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid reporting catalog payload');
  }
  const candidate = input as Partial<ReportingCatalog>;
  if (
    typeof candidate.id !== 'string' ||
    typeof candidate.title !== 'string' ||
    typeof candidate.description !== 'string'
  ) {
    throw new Error('Invalid reporting catalog payload');
  }

  return {
    id: candidate.id,
    title: candidate.title,
    description: candidate.description,
    lockedState: parseReportingLockedStateDefinition(candidate.lockedState),
    guidance: parseReportingGuidanceDefinition(candidate.guidance),
    performanceReport: parseReportingPerformanceReportDefinition(candidate.performanceReport),
    vmInventoryExport: parseVMInventoryExportDefinition(candidate.vmInventoryExport),
  };
}
