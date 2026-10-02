import type { SelectedResource } from '@/components/Settings/ResourcePicker';
import type { ReportingFormat } from '@/components/Settings/reportingCatalogModel';

export type ReportScheduleCadenceType = 'monthly' | 'weekly';
export type ReportScheduleRunStatus = '' | 'ok' | 'failed';
export type ReportScheduleDeliveryMethod = 'email' | 'disk';
// 'resources' is the multi-resource performance report; 'patrol_digest' emails
// the last seven days of Patrol work and has no scope, format, or disk output.
export type ReportScheduleKind = 'resources' | 'patrol_digest';
// Digest schedules carry the server-assigned 'email' format: the email body is
// the report, so there is no PDF or CSV artifact.
export type ReportScheduleFormat = ReportingFormat | 'email';

export interface ReportScheduleResource {
  resourceType: string;
  resourceId: string;
  name?: string;
}

export interface ReportSchedule {
  id: string;
  name: string;
  kind?: ReportScheduleKind;
  enabled: boolean;
  cadence: {
    type: ReportScheduleCadenceType;
    day_of_month?: number;
    weekday?: string;
    time: string;
    timezone: string;
  };
  scope: {
    resources?: ReportScheduleResource[];
    tags?: string[];
  };
  format: ReportScheduleFormat;
  delivery: {
    method: ReportScheduleDeliveryMethod;
    to?: string[];
    attach: boolean;
    save_to_disk: boolean;
  };
  retention_count?: number;
  last_run_at?: string;
  last_run_status?: ReportScheduleRunStatus;
  last_error?: string;
  next_run_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ReportScheduleFormState {
  id: string;
  name: string;
  kind: ReportScheduleKind;
  enabled: boolean;
  cadenceType: ReportScheduleCadenceType;
  dayOfMonth: number;
  weekday: string;
  time: string;
  timezone: string;
  format: ReportingFormat;
  deliveryMethod: ReportScheduleDeliveryMethod;
  recipients: string;
  attach: boolean;
  saveToDisk: boolean;
  tagFilter: string;
  retentionCount: number;
}

export interface ReportSchedulesResponse {
  schedules?: ReportSchedule[];
}

const WEEKDAY_LABELS: Record<string, string> = {
  monday: 'Monday',
  tuesday: 'Tuesday',
  wednesday: 'Wednesday',
  thursday: 'Thursday',
  friday: 'Friday',
  saturday: 'Saturday',
  sunday: 'Sunday',
};

export const DEFAULT_REPORT_SCHEDULE_FORM = (): ReportScheduleFormState => ({
  id: '',
  name: '',
  kind: 'resources',
  enabled: true,
  cadenceType: 'monthly',
  dayOfMonth: 1,
  weekday: 'monday',
  time: '09:00',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  format: 'pdf',
  deliveryMethod: 'email',
  recipients: '',
  attach: true,
  saveToDisk: true,
  tagFilter: '',
  retentionCount: 12,
});

export function parseReportSchedulesResponse(value: unknown): ReportSchedule[] {
  if (!value || typeof value !== 'object') return [];
  const schedules = (value as ReportSchedulesResponse).schedules;
  return Array.isArray(schedules) ? schedules.map(normalizeReportSchedule) : [];
}

export function normalizeReportScheduleKind(kind: unknown): ReportScheduleKind {
  return kind === 'patrol_digest' ? 'patrol_digest' : 'resources';
}

export function isPatrolDigestSchedule(schedule: Pick<ReportSchedule, 'kind'>): boolean {
  return normalizeReportScheduleKind(schedule.kind) === 'patrol_digest';
}

export function normalizeReportSchedule(schedule: ReportSchedule): ReportSchedule {
  return {
    ...schedule,
    kind: normalizeReportScheduleKind(schedule.kind),
    enabled: schedule.enabled !== false,
    cadence: {
      type: schedule.cadence?.type === 'weekly' ? 'weekly' : 'monthly',
      day_of_month: schedule.cadence?.day_of_month ?? 1,
      weekday: schedule.cadence?.weekday || 'monday',
      time: schedule.cadence?.time || '09:00',
      timezone: schedule.cadence?.timezone || 'UTC',
    },
    scope: {
      resources: Array.isArray(schedule.scope?.resources) ? schedule.scope.resources : [],
      tags: Array.isArray(schedule.scope?.tags) ? schedule.scope.tags : [],
    },
    format: schedule.format === 'csv' || schedule.format === 'email' ? schedule.format : 'pdf',
    delivery: {
      method: schedule.delivery?.method === 'disk' ? 'disk' : 'email',
      to: Array.isArray(schedule.delivery?.to) ? schedule.delivery.to : [],
      attach: schedule.delivery?.attach !== false,
      save_to_disk: schedule.delivery?.save_to_disk !== false,
    },
    retention_count: schedule.retention_count ?? 12,
  };
}

export function scheduleToForm(schedule: ReportSchedule): ReportScheduleFormState {
  const normalized = normalizeReportSchedule(schedule);
  return {
    id: normalized.id,
    name: normalized.name,
    kind: normalizeReportScheduleKind(normalized.kind),
    enabled: normalized.enabled,
    cadenceType: normalized.cadence.type,
    dayOfMonth: normalized.cadence.day_of_month ?? 1,
    weekday: normalized.cadence.weekday || 'monday',
    time: normalized.cadence.time,
    timezone: normalized.cadence.timezone,
    format: normalized.format === 'csv' ? 'csv' : 'pdf',
    deliveryMethod: normalized.delivery.method,
    recipients: (normalized.delivery.to ?? []).join(', '),
    attach: normalized.delivery.attach,
    saveToDisk: normalized.delivery.save_to_disk,
    tagFilter: (normalized.scope.tags ?? []).join(', '),
    retentionCount: normalized.retention_count ?? 12,
  };
}

export function scheduleToSelectedResources(schedule: ReportSchedule): SelectedResource[] {
  return (schedule.scope.resources ?? []).map((resource) => ({
    id: resource.resourceId,
    type: resource.resourceType as SelectedResource['type'],
    name: resource.name || resource.resourceId,
  }));
}

// applyReportScheduleKind switches the form between kinds. The Patrol weekly
// summary is fleet-wide and email-only, so it pins the fields the server would
// reject or ignore (cadence, delivery, attachments, disk copy, tag scope) rather
// than leaving stale performance-report values behind hidden controls. Going
// back to a performance report restores the delivery defaults it cleared.
export function applyReportScheduleKind(
  form: ReportScheduleFormState,
  kind: ReportScheduleKind,
): ReportScheduleFormState {
  if (form.kind === kind) return form;
  if (kind === 'patrol_digest') {
    return {
      ...form,
      kind,
      cadenceType: 'weekly',
      deliveryMethod: 'email',
      attach: false,
      saveToDisk: false,
      tagFilter: '',
    };
  }
  const defaults = DEFAULT_REPORT_SCHEDULE_FORM();
  return {
    ...form,
    kind,
    attach: defaults.attach,
    saveToDisk: defaults.saveToDisk,
  };
}

export function reportScheduleKindLabel(kind: unknown): string {
  return normalizeReportScheduleKind(kind) === 'patrol_digest'
    ? 'Patrol weekly summary'
    : 'Performance report';
}

export function buildReportSchedulePayload(
  form: ReportScheduleFormState,
  resources: SelectedResource[],
): Omit<ReportSchedule, 'created_at' | 'updated_at'> {
  if (form.kind === 'patrol_digest') {
    return {
      id: form.id,
      name: form.name.trim(),
      kind: 'patrol_digest',
      enabled: form.enabled,
      cadence: {
        type: 'weekly',
        weekday: form.weekday,
        time: form.time,
        timezone: form.timezone.trim() || 'UTC',
      },
      scope: { resources: [], tags: [] },
      format: 'email',
      delivery: {
        method: 'email',
        to: parseCommaList(form.recipients),
        attach: false,
        save_to_disk: false,
      },
      retention_count: form.retentionCount,
    };
  }
  return {
    id: form.id,
    name: form.name.trim(),
    kind: 'resources',
    enabled: form.enabled,
    cadence: {
      type: form.cadenceType,
      day_of_month: form.cadenceType === 'monthly' ? form.dayOfMonth : undefined,
      weekday: form.cadenceType === 'weekly' ? form.weekday : undefined,
      time: form.time,
      timezone: form.timezone.trim() || 'UTC',
    },
    scope: {
      resources: resources.map((resource) => ({
        resourceType: resource.type,
        resourceId: resource.id,
        name: resource.name,
      })),
      tags: parseCommaList(form.tagFilter),
    },
    format: form.format,
    delivery: {
      method: form.deliveryMethod,
      to: parseCommaList(form.recipients),
      attach: form.attach,
      save_to_disk: form.saveToDisk,
    },
    retention_count: form.retentionCount,
  };
}

export function parseCommaList(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(',')
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false;
      const key = item.toLowerCase();
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
}

export function reportScheduleCadenceLabel(schedule: ReportSchedule): string {
  const normalized = normalizeReportSchedule(schedule);
  if (normalized.cadence.type === 'weekly') {
    return `${WEEKDAY_LABELS[normalized.cadence.weekday || 'monday'] ?? normalized.cadence.weekday} at ${normalized.cadence.time}`;
  }
  return `Monthly on day ${normalized.cadence.day_of_month ?? 1} at ${normalized.cadence.time}`;
}

export function reportScheduleScopeLabel(schedule: ReportSchedule): string {
  if (isPatrolDigestSchedule(schedule)) return 'Patrol activity, last 7 days';
  const resources = schedule.scope.resources?.length ?? 0;
  const tags = schedule.scope.tags?.length ?? 0;
  const parts = [];
  if (resources > 0) parts.push(`${resources} resource${resources === 1 ? '' : 's'}`);
  if (tags > 0) parts.push(`${tags} tag${tags === 1 ? '' : 's'}`);
  return parts.length ? parts.join(', ') : 'No scope';
}

export function reportScheduleDeliveryLabel(schedule: ReportSchedule): string {
  const normalized = normalizeReportSchedule(schedule);
  if (normalized.delivery.method === 'disk') return 'Save to disk';
  if ((normalized.delivery.to ?? []).length > 0)
    return `${normalized.delivery.to!.length} email recipient${normalized.delivery.to!.length === 1 ? '' : 's'}`;
  return 'Email config recipients';
}

export function reportScheduleLastRunLabel(schedule: ReportSchedule): string {
  if (!schedule.last_run_status) return 'Not run yet';
  if (schedule.last_run_status === 'ok') return 'Last run OK';
  return schedule.last_error ? `Failed: ${schedule.last_error}` : 'Last run failed';
}

export function formatReportScheduleTime(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
