import {
  parseVMInventoryExportDefinition,
  type ReportingInventoryExportDefinition,
} from '@/components/Settings/reportingInventoryExportModel';

export type ReportingFormat = 'pdf' | 'csv';
export type ReportingFilenameDateStyle = 'utc_yyyymmdd';

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
  vmInventoryExport: ReportingInventoryExportDefinition;
}

export function buildReportingCatalogRequest(): { url: string } {
  return {
    url: '/api/admin/reports/catalog',
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
