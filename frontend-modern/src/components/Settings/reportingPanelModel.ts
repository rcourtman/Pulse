import { toReportingResourceType } from '@/utils/reportingResourceTypes';
import { formatReportingFilenameDate } from '@/utils/reportingPresentation';
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
  definition: Pick<ReportingPerformanceReportDefinition, 'defaultRange' | 'ranges'>,
): Date {
  const start = new Date(now);
  const resolvedRange =
    definition.ranges.find((candidate) => candidate.key === range) ??
    definition.ranges.find((candidate) => candidate.key === definition.defaultRange);
  if (!resolvedRange) {
    throw new Error('Invalid reporting range definition');
  }
  start.setHours(start.getHours() - resolvedRange.windowHours);
  return start;
}

export function buildReportingFilename(
  format: ReportingFormat,
  resourceName: string | null,
  now: Date,
  definition: Pick<
    ReportingPerformanceReportDefinition,
    'filenameDateStyle' | 'multiFilenamePrefix' | 'singleFilenamePrefix'
  >,
): string {
  const date = formatReportingFilenameDate(now, definition.filenameDateStyle);
  if (resourceName) {
    return `${definition.singleFilenamePrefix}-${resourceName}-${date}.${format}`;
  }
  return `${definition.multiFilenamePrefix}-${date}.${format}`;
}

export function buildReportingRequest(
  context: ReportingRequestContext,
  definition: Pick<
    ReportingPerformanceReportDefinition,
    | 'multiFilenamePrefix'
    | 'multiResourceEndpoint'
    | 'filenameDateStyle'
    | 'singleFilenamePrefix'
    | 'singleResourceEndpoint'
    | 'supportsCustomTitle'
    | 'supportsMetricFilter'
  >,
): ReportingRequestDefinition {
  const metricType = definition.supportsMetricFilter === false ? '' : context.metricType.trim();
  const customTitle = definition.supportsCustomTitle === false ? '' : context.title.trim();

  if (context.resources.length === 1) {
    const resource = context.resources[0];
    const params = new URLSearchParams({
      resourceType: toReportingResourceType(resource.type),
      resourceId: resource.id,
      format: context.format,
      start: context.start,
      end: context.end,
    });

    if (metricType) {
      params.append('metricType', metricType);
    }
    if (customTitle) {
      params.append('title', customTitle);
    }

    return {
      filename: buildReportingFilename(context.format, resource.name, context.now, definition),
      request: {
        url: `${definition.singleResourceEndpoint}?${params.toString()}`,
      },
    };
  }

  return {
    filename: buildReportingFilename(context.format, null, context.now, definition),
    request: {
      url: definition.multiResourceEndpoint,
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
          title: customTitle || undefined,
          metricType: metricType || undefined,
        }),
      },
    },
  };
}
