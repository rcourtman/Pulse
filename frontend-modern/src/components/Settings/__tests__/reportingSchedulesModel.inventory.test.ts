import { describe, expect, it } from 'vitest';
import {
  applyReportScheduleKind,
  buildReportSchedulePayload,
  DEFAULT_REPORT_SCHEDULE_FORM,
  normalizeReportSchedule,
  reportScheduleKindLabel,
  reportScheduleScopeLabel,
  scheduleToForm,
} from '../reportingSchedulesModel';

describe('scheduled VM inventory CSV', () => {
  it('removes stale performance scope and PDF settings, preserving cadence and delivery', () => {
    const form = applyReportScheduleKind(
      {
        ...DEFAULT_REPORT_SCHEDULE_FORM(),
        name: 'Inventory',
        format: 'pdf',
        tagFilter: 'client',
        cadenceType: 'weekly',
        deliveryMethod: 'disk',
      },
      'vm_inventory',
    );
    const payload = buildReportSchedulePayload(form, [
      { id: 'old-vm', type: 'vm', name: 'Old VM' },
    ]);
    expect(payload).toMatchObject({
      kind: 'vm_inventory',
      format: 'csv',
      cadence: { type: 'weekly' },
      scope: { resources: [], tags: [] },
      delivery: { method: 'disk', save_to_disk: true },
    });
    expect(reportScheduleScopeLabel(payload)).toBe('All VMs in this organization');
    expect(reportScheduleKindLabel(payload.kind)).toBe('VM inventory CSV');
    expect(scheduleToForm(normalizeReportSchedule(payload))).toMatchObject({
      kind: 'vm_inventory',
      format: 'csv',
    });
  });

  it('restores a performance report after switching from inventory or Patrol', () => {
    const inventory = applyReportScheduleKind(DEFAULT_REPORT_SCHEDULE_FORM(), 'vm_inventory');
    expect(applyReportScheduleKind(inventory, 'resources')).toMatchObject({
      kind: 'resources',
      format: 'pdf',
    });
    const digest = applyReportScheduleKind(inventory, 'patrol_digest');
    expect(buildReportSchedulePayload(digest, [])).toMatchObject({
      kind: 'patrol_digest',
      format: 'email',
      delivery: { attach: false },
    });
    expect(applyReportScheduleKind(digest, 'vm_inventory')).toMatchObject({
      kind: 'vm_inventory',
      format: 'csv',
      attach: true,
      saveToDisk: true,
    });
  });
});
