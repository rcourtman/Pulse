import { toReportingResourceType } from '@/utils/reportingResourceTypes';
import type { SelectedResource } from '@/components/Settings/ResourcePicker';
import type {
  ReportingFormat,
  ReportingPerformanceReportDefinition,
} from '@/components/Settings/reportingCatalogModel';

export type ReportingRangeValue = string;

export interface ReportingRequestContext {
  end: string;
  format: ReportingFormat;
  metricType: string;
  now: Date;
  resources: SelectedResource[];
  start: string;
  title: string;
}

export interface ReportingRequestDefinition {
  filename: string;
  request: {
    init?: {
      body?: string;
      headers?: Record<string, string>;
      method?: string;
    };
    url: string;
  };
}

export function getReportingRangeStart(
  range: ReportingRangeValue,
  now: Date,
  definition?: Pick<ReportingPerformanceReportDefinition, 'defaultRange' | 'ranges'> | null,
): Date {
  const start = new Date(now);
  const resolvedRange =
    definition?.ranges.find((candidate) => candidate.key === range) ??
    definition?.ranges.find((candidate) => candidate.key === definition.defaultRange) ??
    null;
  start.setHours(start.getHours() - (resolvedRange?.windowHours ?? 24));
  return start;
}

export function buildSingleReportTitle(title: string, resourceName: string): string {
  return title || `Pulse Report - ${resourceName}`;
}

export function buildFleetReportTitle(title: string): string {
  return title || 'Pulse Fleet Report';
}

export function buildReportingFilename(
  format: ReportingFormat,
  resourceName: string | null,
  now: Date,
  definition?: Pick<
    ReportingPerformanceReportDefinition,
    'multiFilenamePrefix' | 'singleFilenamePrefix'
  > | null,
): string {
  const date = now.toISOString().split('T')[0];
  if (resourceName) {
    return `${definition?.singleFilenamePrefix ?? 'report'}-${resourceName}-${date}.${format}`;
  }
  return `${definition?.multiFilenamePrefix ?? 'fleet-report'}-${date}.${format}`;
}

export function buildReportingRequest(
  context: ReportingRequestContext,
  definition?: Pick<
    ReportingPerformanceReportDefinition,
    | 'multiFilenamePrefix'
    | 'multiResourceEndpoint'
    | 'singleFilenamePrefix'
    | 'singleResourceEndpoint'
    | 'supportsCustomTitle'
    | 'supportsMetricFilter'
  > | null,
): ReportingRequestDefinition {
  const metricType =
    definition?.supportsMetricFilter === false ? '' : context.metricType.trim();
  const customTitle = definition?.supportsCustomTitle === false ? '' : context.title.trim();

  if (context.resources.length === 1) {
    const resource = context.resources[0];
    const params = new URLSearchParams({
      resourceType: toReportingResourceType(resource.type),
      resourceId: resource.id,
      format: context.format,
      start: context.start,
      end: context.end,
      title: buildSingleReportTitle(customTitle, resource.name),
    });

    if (metricType) {
      params.append('metricType', metricType);
    }

    return {
      filename: buildReportingFilename(context.format, resource.name, context.now, definition),
      request: {
        url: `${definition?.singleResourceEndpoint ?? '/api/admin/reports/generate'}?${params.toString()}`,
      },
    };
  }

  return {
    filename: buildReportingFilename(context.format, null, context.now, definition),
    request: {
      url: definition?.multiResourceEndpoint ?? '/api/admin/reports/generate-multi',
      init: {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          resources: context.resources.map((resource) => ({
            resourceType: toReportingResourceType(resource.type),
            resourceId: resource.id,
          })),
          format: context.format,
          start: context.start,
          end: context.end,
          title: buildFleetReportTitle(customTitle),
          metricType: metricType || undefined,
        }),
      },
    },
  };
}
