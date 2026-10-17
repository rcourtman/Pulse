import { describe, expect, it } from 'vitest';
import {
  buildReportSchedulePayload,
  DEFAULT_REPORT_SCHEDULE_FORM,
  normalizeReportSchedule,
  parseReportSchedulesResponse,
  reportScheduleLastRunLabel,
  reportScheduleNeedsRetry,
  reportScheduleOutcomeDetail,
} from '../reportingSchedulesModel';

const schedule = () =>
  normalizeReportSchedule({
    ...buildReportSchedulePayload(DEFAULT_REPORT_SCHEDULE_FORM(), []),
    id: 'weekly',
  });

describe('report delivery outcomes', () => {
  it('distinguishes disk output, accepted mail and legacy completion', () => {
    expect(
      reportScheduleLastRunLabel({
        ...schedule(),
        last_run_status: 'ok',
        last_delivery_status: 'saved',
      }),
    ).toBe('Saved to disk');
    const accepted = {
      ...schedule(),
      last_run_status: 'ok' as const,
      last_delivery_status: 'sent_with_attachment',
    };
    expect(reportScheduleLastRunLabel(accepted)).toBe('Email accepted');
    expect(reportScheduleOutcomeDetail(accepted)).toContain('Inbox receipt is not confirmed');
    expect(reportScheduleLastRunLabel({ ...schedule(), last_run_status: 'ok' })).toBe('Completed');
  });

  it('keeps uncertainty visible and offers deliberate retry', () => {
    const uncertain = {
      ...schedule(),
      run_window_end: '2026-10-17T09:00:00Z',
      last_run_status: 'unconfirmed' as const,
    };
    expect(reportScheduleLastRunLabel(uncertain)).toBe('Delivery unconfirmed');
    expect(reportScheduleOutcomeDetail(uncertain)).toContain('avoid a duplicate');
    expect(reportScheduleNeedsRetry(uncertain)).toBe(true);
    expect(reportScheduleNeedsRetry({ ...schedule(), last_run_status: 'running' })).toBe(false);
    expect(reportScheduleNeedsRetry({ ...schedule(), last_run_status: 'failed' })).toBe(false);
  });

  it('preserves actionable failures and joins current configuration checks', () => {
    const [failed] = parseReportSchedulesResponse({
      schedules: [
        {
          ...schedule(),
          last_run_status: 'failed',
          last_error: 'Fix the email sender in notification settings',
        },
      ],
      delivery_checks: { weekly: { ready: false, message: 'Email is disabled' } },
    });
    expect(reportScheduleOutcomeDetail(failed!)).toBe(
      'Fix the email sender in notification settings',
    );
    expect(failed?.delivery_check).toEqual({ ready: false, message: 'Email is disabled' });
  });
});
