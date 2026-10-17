import { describe, expect, it } from 'vitest';
import {
  applyReportScheduleKind,
  buildReportSchedulePayload,
  DEFAULT_REPORT_SCHEDULE_FORM,
  parseReportSchedulesResponse,
  reportScheduleScopeLabel,
  scheduleToForm,
} from '../reportingSchedulesModel';

describe('client backup health schedules', () => {
  it('supports an organization-wide backup report without a performance selection cap', () => {
    const form = applyReportScheduleKind(DEFAULT_REPORT_SCHEDULE_FORM(), 'backup_health');
    const payload = buildReportSchedulePayload({ ...form, name: 'Client backups' }, []);
    expect(payload.kind).toBe('backup_health');
    expect(payload.format).toBe('pdf');
    expect(payload.scope).toEqual({ resources: [], tags: [] });
    expect(reportScheduleScopeLabel(payload)).toBe('All current VMs and containers');
    const resources = Array.from({ length: 61 }, (_, i) => ({
      id: `vm-${i}`,
      type: 'vm' as const,
      name: `VM ${i}`,
    }));
    expect(buildReportSchedulePayload(form, resources).scope.resources).toHaveLength(61);
  });

  it('retains client tags and selected format when switching from performance reporting', () => {
    const form = applyReportScheduleKind(
      { ...DEFAULT_REPORT_SCHEDULE_FORM(), format: 'csv', tagFilter: 'client-a' },
      'backup_health',
    );
    const payload = buildReportSchedulePayload(form, [
      { id: 'vm-client-a', type: 'vm', name: 'Client A' },
    ]);
    expect(payload.scope.tags).toEqual(['client-a']);
    expect(payload.scope.resources?.[0].resourceId).toBe('vm-client-a');
    expect(payload.format).toBe('csv');
    const restored = applyReportScheduleKind(
      applyReportScheduleKind(form, 'patrol_digest'),
      'backup_health',
    );
    expect(restored.attach).toBe(true);
    expect(restored.saveToDisk).toBe(true);
  });

  it('keeps missing backup data separate from successful delivery after reload', () => {
    const schedule = {
      ...buildReportSchedulePayload(
        { ...DEFAULT_REPORT_SCHEDULE_FORM(), kind: 'backup_health' },
        [],
      ),
      last_run_status: 'ok',
      last_delivery_status: 'sent_with_attachment',
      last_report_data_state: 'partial',
      last_report_data_message: 'Backup data is incomplete.',
    };
    const parsed = parseReportSchedulesResponse({ schedules: [schedule] });
    expect(parsed[0].last_report_data_state).toBe('partial');
    expect(parsed[0].last_run_status).toBe('ok');
    expect(scheduleToForm(parsed[0]).kind).toBe('backup_health');
    const outgoing = buildReportSchedulePayload(scheduleToForm(parsed[0]), []);
    expect(outgoing).not.toHaveProperty('last_report_data_state');
  });
});
