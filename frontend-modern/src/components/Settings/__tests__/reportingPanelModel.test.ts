import { describe, expect, it } from 'vitest';
import type { SelectedResource } from '../ResourcePicker';
import {
  buildReportingRequest,
  getReportingRangeStart,
} from '../reportingPanelModel';
import type { ReportingPerformanceReportDefinition } from '../reportingCatalogModel';

const performanceDefinition: ReportingPerformanceReportDefinition = {
  id: 'performance_reports',
  title: 'Performance Reports',
  description: 'Historical performance reporting',
  singleResourceEndpoint: '/api/admin/reports/generate',
  multiResourceEndpoint: '/api/admin/reports/generate-multi',
  singleFilenamePrefix: 'report',
  multiFilenamePrefix: 'fleet-report',
  formats: [
    { value: 'pdf', label: 'PDF Report' },
    { value: 'csv', label: 'CSV Data' },
  ],
  defaultFormat: 'pdf',
  ranges: [
    {
      key: '24h',
      label: 'Last 24 Hours',
      description: 'Daily review',
      windowHours: 24,
    },
    {
      key: '7d',
      label: 'Last 7 Days',
      description: 'Weekly review',
      windowHours: 168,
    },
    {
      key: '30d',
      label: 'Last 30 Days',
      description: 'Monthly review',
      windowHours: 720,
    },
  ],
  defaultRange: '24h',
  multiResourceMax: 50,
  supportsMetricFilter: true,
  supportsCustomTitle: true,
};

describe('reporting panel model', () => {
  it('builds a single-resource reporting request and filename', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    const resources: SelectedResource[] = [
      {
        id: 'agent-1',
        type: 'agent',
        name: 'node-a',
      },
    ];

    const request = buildReportingRequest({
      end: now.toISOString(),
      format: 'pdf',
      metricType: 'cpu',
      now,
      resources,
      start: '2026-03-19T12:34:56.000Z',
      title: '',
    }, performanceDefinition);

    expect(request.filename).toBe('report-node-a-2026-03-20.pdf');
    expect(request.request.init).toBeUndefined();
    expect(request.request.url).toContain('/api/admin/reports/generate?');
    expect(request.request.url).toContain('resourceType=agent');
    expect(request.request.url).toContain('resourceId=agent-1');
    expect(request.request.url).toContain('metricType=cpu');
    expect(request.request.url).toContain('title=Pulse+Report+-+node-a');
  });

  it('builds a fleet reporting request body and filename', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    const resources: SelectedResource[] = [
      {
        id: 'agent-1',
        type: 'agent',
        name: 'node-a',
      },
      {
        id: 'vm-1',
        type: 'vm',
        name: 'vm-a',
      },
    ];

    const request = buildReportingRequest({
      end: now.toISOString(),
      format: 'csv',
      metricType: '',
      now,
      resources,
      start: '2026-03-19T12:34:56.000Z',
      title: '',
    }, performanceDefinition);

    expect(request.filename).toBe('fleet-report-2026-03-20.csv');
    expect(request.request.url).toBe('/api/admin/reports/generate-multi');
    expect(request.request.init).toMatchObject({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    expect(request.request.init?.body).toBe(
      JSON.stringify({
        resources: [
          { resourceType: 'agent', resourceId: 'agent-1' },
          { resourceType: 'vm', resourceId: 'vm-1' },
        ],
        format: 'csv',
        start: '2026-03-19T12:34:56.000Z',
        end: now.toISOString(),
        title: 'Pulse Fleet Report',
        metricType: undefined,
      }),
    );
  });

  it('omits unsupported metric filters and custom titles from the request contract', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    const resources: SelectedResource[] = [
      {
        id: 'vm-1',
        type: 'vm',
        name: 'vm-a',
      },
    ];

    const request = buildReportingRequest(
      {
        end: now.toISOString(),
        format: 'pdf',
        metricType: 'cpu',
        now,
        resources,
        start: '2026-03-19T12:34:56.000Z',
        title: 'Custom fleet title',
      },
      {
        ...performanceDefinition,
        supportsMetricFilter: false,
        supportsCustomTitle: false,
      },
    );

    expect(request.request.url).toContain('title=Pulse+Report+-+vm-a');
    expect(request.request.url).not.toContain('metricType=');
    expect(request.request.url).not.toContain('Custom+fleet+title');
  });

  it('derives canonical range starts from the selected preset', () => {
    const now = new Date('2026-03-20T12:00:00.000Z');

    expect(getReportingRangeStart('24h', now, performanceDefinition).toISOString()).toBe(
      '2026-03-19T12:00:00.000Z',
    );
    expect(getReportingRangeStart('7d', now, performanceDefinition).toISOString()).toBe(
      '2026-03-13T12:00:00.000Z',
    );
    expect(getReportingRangeStart('30d', now, performanceDefinition).toISOString()).toBe(
      '2026-02-18T12:00:00.000Z',
    );
  });
});
