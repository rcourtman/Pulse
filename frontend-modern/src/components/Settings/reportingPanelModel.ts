import type { ReportingRangeOption } from '@/utils/reportingPresentation';
import { toReportingResourceType } from '@/utils/reportingResourceTypes';
import type { SelectedResource } from '@/components/Settings/ResourcePicker';

export type ReportingRangeValue = ReportingRangeOption['value'];
export type ReportingFormat = 'pdf' | 'csv';

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

export function getReportingRangeStart(range: ReportingRangeValue, now: Date): Date {
  const start = new Date(now);
  if (range === '24h') start.setHours(start.getHours() - 24);
  else if (range === '7d') start.setDate(start.getDate() - 7);
  else if (range === '30d') start.setDate(start.getDate() - 30);
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
): string {
  const date = now.toISOString().split('T')[0];
  if (resourceName) {
    return `report-${resourceName}-${date}.${format}`;
  }
  return `fleet-report-${date}.${format}`;
}

export function buildReportingRequest(context: ReportingRequestContext): ReportingRequestDefinition {
  if (context.resources.length === 1) {
    const resource = context.resources[0];
    const params = new URLSearchParams({
      resourceType: toReportingResourceType(resource.type),
      resourceId: resource.id,
      format: context.format,
      start: context.start,
      end: context.end,
      title: buildSingleReportTitle(context.title, resource.name),
    });

    if (context.metricType) {
      params.append('metricType', context.metricType);
    }

    return {
      filename: buildReportingFilename(context.format, resource.name, context.now),
      request: {
        url: `/api/admin/reports/generate?${params.toString()}`,
      },
    };
  }

  return {
    filename: buildReportingFilename(context.format, null, context.now),
    request: {
      url: '/api/admin/reports/generate-multi',
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
          title: buildFleetReportTitle(context.title),
          metricType: context.metricType || undefined,
        }),
      },
    },
  };
}
