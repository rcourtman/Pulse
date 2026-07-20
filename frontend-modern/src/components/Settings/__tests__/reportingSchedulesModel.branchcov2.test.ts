/**
 * Branch-coverage tests for the exported helpers in reportingSchedulesModel.
 * Each block targets a specific function and drives both arms of every
 * conditional / optional-chain / nullish-coalescing branch that is reachable
 * from the public surface.
 */
import { describe, expect, it } from 'vitest';
import type { SelectedResource } from '@/components/Settings/ResourcePicker';
import {
  DEFAULT_REPORT_SCHEDULE_FORM,
  buildReportSchedulePayload,
  formatReportScheduleTime,
  normalizeReportSchedule,
  parseCommaList,
  parseReportSchedulesResponse,
  reportScheduleCadenceLabel,
  reportScheduleDeliveryLabel,
  reportScheduleLastRunLabel,
  reportScheduleScopeLabel,
  scheduleToForm,
  scheduleToSelectedResources,
  type ReportSchedule,
  type ReportScheduleFormState,
} from '../reportingSchedulesModel';

// ---- Fixtures ---------------------------------------------------------------

const makeSchedule = (overrides: Partial<ReportSchedule> = {}): ReportSchedule => ({
  id: 'sched-1',
  name: 'Nightly ops digest',
  enabled: true,
  cadence: {
    type: 'monthly',
    day_of_month: 15,
    weekday: 'monday',
    time: '09:00',
    timezone: 'UTC',
  },
  scope: {
    resources: [],
    tags: [],
  },
  format: 'pdf',
  delivery: {
    method: 'email',
    to: [],
    attach: true,
    save_to_disk: true,
  },
  retention_count: 12,
  ...overrides,
});

const makeForm = (overrides: Partial<ReportScheduleFormState> = {}): ReportScheduleFormState => ({
  ...DEFAULT_REPORT_SCHEDULE_FORM(),
  ...overrides,
});

const makeResource = (overrides: Partial<SelectedResource> = {}): SelectedResource => ({
  id: 'res-1',
  type: 'vm',
  name: 'web-01',
  ...overrides,
});

// ---- parseReportSchedulesResponse ------------------------------------------

describe('parseReportSchedulesResponse', () => {
  it('returns an empty array for a null input (!value arm)', () => {
    expect(parseReportSchedulesResponse(null)).toStrictEqual([]);
  });

  it('returns an empty array for an undefined input (!value arm)', () => {
    expect(parseReportSchedulesResponse(undefined)).toStrictEqual([]);
  });

  it('returns an empty array for a non-object primitive (typeof !== "object" arm)', () => {
    expect(parseReportSchedulesResponse('not-an-object')).toStrictEqual([]);
    expect(parseReportSchedulesResponse(42)).toStrictEqual([]);
  });

  it('returns an empty array when the value is an object without a schedules field', () => {
    expect(parseReportSchedulesResponse({ foo: 'bar' })).toStrictEqual([]);
  });

  it('returns an empty array when schedules is present but not an array', () => {
    expect(parseReportSchedulesResponse({ schedules: 'nope' })).toStrictEqual([]);
    expect(parseReportSchedulesResponse({ schedules: null })).toStrictEqual([]);
  });

  it('maps each entry through normalizeReportSchedule (Array.isArray truthy arm)', () => {
    const result = parseReportSchedulesResponse({
      schedules: [
        makeSchedule({ id: 'a', enabled: false }),
        makeSchedule({ id: 'b', format: 'csv' }),
      ],
    });
    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe('a');
    expect(result[0]?.enabled).toBe(false);
    expect(result[1]?.format).toBe('csv');
  });

  it('normalizes a malformed schedule carried inside the response', () => {
    const result = parseReportSchedulesResponse({
      schedules: [
        {
          id: 'c',
          name: 'sparse',
          enabled: true,
          scope: {},
          format: 'pdf',
          delivery: { method: 'email', attach: true, save_to_disk: true },
        } as unknown as ReportSchedule,
      ],
    });
    expect(result[0]?.cadence).toStrictEqual({
      type: 'monthly',
      day_of_month: 1,
      weekday: 'monday',
      time: '09:00',
      timezone: 'UTC',
    });
  });
});

// ---- normalizeReportSchedule -----------------------------------------------

describe('normalizeReportSchedule', () => {
  it('passes through fully-populated truthy values unchanged on the happy path', () => {
    const schedule = makeSchedule({
      enabled: true,
      cadence: {
        type: 'weekly',
        day_of_month: 20,
        weekday: 'friday',
        time: '17:30',
        timezone: 'Europe/Paris',
      },
      scope: {
        resources: [{ resourceType: 'vm', resourceId: 'v1', name: 'web' }],
        tags: ['tier1'],
      },
      format: 'csv',
      delivery: { method: 'disk', to: ['ops@x'], attach: true, save_to_disk: true },
      retention_count: 7,
    });
    expect(normalizeReportSchedule(schedule)).toMatchObject({
      enabled: true,
      cadence: {
        type: 'weekly',
        day_of_month: 20,
        weekday: 'friday',
        time: '17:30',
        timezone: 'Europe/Paris',
      },
      format: 'csv',
      delivery: { method: 'disk', to: ['ops@x'], attach: true, save_to_disk: true },
      retention_count: 7,
    });
  });

  it('coerces enabled to false only when explicitly false, and to true otherwise', () => {
    expect(normalizeReportSchedule(makeSchedule({ enabled: false })).enabled).toBe(false);
    // The `!== false` arm: any non-false value becomes true.
    expect(
      normalizeReportSchedule(makeSchedule({ enabled: 'true' as unknown as boolean })).enabled,
    ).toBe(true);
  });

  it('keeps day_of_month of 0 (nullish coalescing does not default on falsy numbers)', () => {
    expect(
      normalizeReportSchedule(
        makeSchedule({
          cadence: { type: 'monthly', day_of_month: 0, time: '09:00', timezone: 'UTC' },
        }),
      ).cadence.day_of_month,
    ).toBe(0);
  });

  it('applies every cadence default when cadence is entirely missing', () => {
    const schedule = {
      id: 'x',
      name: 'x',
      enabled: true,
      scope: {},
      format: 'pdf',
      delivery: { method: 'email', attach: true, save_to_disk: true },
    } as unknown as ReportSchedule;
    expect(normalizeReportSchedule(schedule).cadence).toStrictEqual({
      type: 'monthly',
      day_of_month: 1,
      weekday: 'monday',
      time: '09:00',
      timezone: 'UTC',
    });
  });

  it('classifies an unknown cadence type as monthly', () => {
    const schedule = makeSchedule({
      cadence: {
        type: 'daily' as unknown as 'monthly',
        day_of_month: 3,
        time: '08:00',
        timezone: 'UTC',
      },
    });
    expect(normalizeReportSchedule(schedule).cadence.type).toBe('monthly');
  });

  it('defaults weekday/time/timezone via || when they are empty strings', () => {
    const schedule = makeSchedule({
      cadence: {
        type: 'monthly',
        day_of_month: 1,
        weekday: '',
        time: '',
        timezone: '',
        day_of_month_ignored: true,
      } as unknown as ReportSchedule['cadence'],
    });
    expect(normalizeReportSchedule(schedule).cadence).toMatchObject({
      weekday: 'monday',
      time: '09:00',
      timezone: 'UTC',
    });
  });

  it('keeps scope arrays when they are arrays, and replaces them when not', () => {
    const kept = normalizeReportSchedule(
      makeSchedule({
        scope: { resources: [{ resourceType: 'vm', resourceId: 'v1' }], tags: ['a'] },
      }),
    );
    expect(kept.scope.resources).toHaveLength(1);
    expect(kept.scope.tags).toEqual(['a']);

    const replaced = normalizeReportSchedule(
      makeSchedule({
        scope: { resources: null as unknown as [], tags: 'nope' as unknown as string[] },
      }),
    );
    expect(replaced.scope.resources).toStrictEqual([]);
    expect(replaced.scope.tags).toStrictEqual([]);
  });

  it('classifies format as csv only for csv, pdf otherwise', () => {
    expect(normalizeReportSchedule(makeSchedule({ format: 'csv' })).format).toBe('csv');
    expect(
      normalizeReportSchedule(makeSchedule({ format: 'xlsx' as unknown as 'pdf' })).format,
    ).toBe('pdf');
  });

  it('classifies delivery method as disk only for disk, email otherwise', () => {
    expect(
      normalizeReportSchedule(
        makeSchedule({ delivery: { method: 'disk', attach: true, save_to_disk: true } }),
      ).delivery.method,
    ).toBe('disk');
    expect(
      normalizeReportSchedule(
        makeSchedule({
          delivery: {
            method: 'carrier-pigeon' as unknown as 'email',
            attach: true,
            save_to_disk: true,
          },
        }),
      ).delivery.method,
    ).toBe('email');
  });

  it('defaults delivery.to to [] when missing and keeps it when an array', () => {
    expect(
      normalizeReportSchedule(
        makeSchedule({
          delivery: {
            method: 'email',
            to: undefined as unknown as string[],
            attach: true,
            save_to_disk: true,
          },
        }),
      ).delivery.to,
    ).toStrictEqual([]);
    expect(
      normalizeReportSchedule(
        makeSchedule({
          delivery: { method: 'email', to: ['a@b'], attach: true, save_to_disk: true },
        }),
      ).delivery.to,
    ).toEqual(['a@b']);
  });

  it('coerces attach and save_to_disk to false only when explicitly false', () => {
    const allFalse = normalizeReportSchedule(
      makeSchedule({ delivery: { method: 'email', attach: false, save_to_disk: false } }),
    );
    expect(allFalse.delivery.attach).toBe(false);
    expect(allFalse.delivery.save_to_disk).toBe(false);

    const defaults = normalizeReportSchedule(
      makeSchedule({
        delivery: {
          method: 'email',
          attach: undefined as unknown as boolean,
          save_to_disk: undefined as unknown as boolean,
        },
      }),
    );
    expect(defaults.delivery.attach).toBe(true);
    expect(defaults.delivery.save_to_disk).toBe(true);
  });

  it('defaults retention_count to 12 when missing', () => {
    const schedule = { ...makeSchedule(), retention_count: undefined } as unknown as ReportSchedule;
    delete (schedule as Partial<ReportSchedule>).retention_count;
    expect(normalizeReportSchedule(schedule).retention_count).toBe(12);
  });
});

// ---- scheduleToForm ---------------------------------------------------------
//
// scheduleToForm consumes the *normalized* schedule, whose fields are always
// populated by normalizeReportSchedule. The internal `?? 1` / `|| 'monday'` /
// `?? []` defensives therefore never fire from the public surface; here we
// cover the reachable mapping logic for both cadence arms.

describe('scheduleToForm', () => {
  it('maps a populated monthly schedule into the form shape', () => {
    const schedule = makeSchedule({
      cadence: { type: 'monthly', day_of_month: 27, time: '23:00', timezone: 'UTC' },
      delivery: { method: 'email', to: ['a@b.com', 'c@d.com'], attach: true, save_to_disk: true },
      scope: { resources: [], tags: ['tier1', 'tier2'] },
      retention_count: 5,
    });
    expect(scheduleToForm(schedule)).toStrictEqual({
      id: 'sched-1',
      name: 'Nightly ops digest',
      enabled: true,
      cadenceType: 'monthly',
      dayOfMonth: 27,
      weekday: 'monday',
      time: '23:00',
      timezone: 'UTC',
      format: 'pdf',
      deliveryMethod: 'email',
      recipients: 'a@b.com, c@d.com',
      attach: true,
      saveToDisk: true,
      tagFilter: 'tier1, tier2',
      retentionCount: 5,
    });
  });

  it('maps a weekly schedule and surfaces the weekday field', () => {
    const schedule = makeSchedule({
      cadence: { type: 'weekly', weekday: 'friday', time: '17:30', timezone: 'Europe/Paris' },
    });
    const form = scheduleToForm(schedule);
    expect(form.cadenceType).toBe('weekly');
    expect(form.weekday).toBe('friday');
    expect(form.dayOfMonth).toBe(1); // monthly default carried through normalization
    expect(form.timezone).toBe('Europe/Paris');
  });

  it('joins recipients and tags into empty strings when both lists are empty', () => {
    const form = scheduleToForm(makeSchedule());
    expect(form.recipients).toBe('');
    expect(form.tagFilter).toBe('');
  });
});

// ---- scheduleToSelectedResources -------------------------------------------

describe('scheduleToSelectedResources', () => {
  it('returns an empty array when scope.resources is undefined (?? [] arm)', () => {
    const schedule = makeSchedule({ scope: {} });
    expect(scheduleToSelectedResources(schedule)).toStrictEqual([]);
  });

  it('maps each resource, using the explicit name when present', () => {
    const schedule = makeSchedule({
      scope: {
        resources: [
          { resourceType: 'vm', resourceId: 'v1', name: 'web-01' },
          { resourceType: 'agent', resourceId: 'a1', name: 'host-01' },
        ],
      },
    });
    expect(scheduleToSelectedResources(schedule)).toStrictEqual([
      { id: 'v1', type: 'vm', name: 'web-01' },
      { id: 'a1', type: 'agent', name: 'host-01' },
    ]);
  });

  it('falls back to resourceId when name is missing (|| arm)', () => {
    const schedule = makeSchedule({
      scope: { resources: [{ resourceType: 'vm', resourceId: 'v9' }] },
    });
    expect(scheduleToSelectedResources(schedule)).toStrictEqual([
      { id: 'v9', type: 'vm', name: 'v9' },
    ]);
  });

  it('falls back to resourceId when name is an empty string', () => {
    const schedule = makeSchedule({
      scope: { resources: [{ resourceType: 'vm', resourceId: 'v9', name: '' }] },
    });
    expect(scheduleToSelectedResources(schedule)[0]?.name).toBe('v9');
  });
});

// ---- buildReportSchedulePayload --------------------------------------------

describe('buildReportSchedulePayload', () => {
  it('emits day_of_month and omits weekday for a monthly form', () => {
    const payload = buildReportSchedulePayload(
      makeForm({ cadenceType: 'monthly', dayOfMonth: 12 }),
      [makeResource()],
    );
    expect(payload.cadence).toMatchObject({
      type: 'monthly',
      day_of_month: 12,
      weekday: undefined,
    });
  });

  it('emits weekday and omits day_of_month for a weekly form', () => {
    const payload = buildReportSchedulePayload(
      makeForm({ cadenceType: 'weekly', weekday: 'wednesday' }),
      [],
    );
    expect(payload.cadence).toMatchObject({
      type: 'weekly',
      weekday: 'wednesday',
      day_of_month: undefined,
    });
  });

  it('falls back to UTC when the timezone trims to empty', () => {
    const payload = buildReportSchedulePayload(makeForm({ timezone: '   ' }), []);
    expect(payload.cadence.timezone).toBe('UTC');
  });

  it('trims the schedule name and maps resources into the scope', () => {
    const payload = buildReportSchedulePayload(makeForm({ name: '  trimmed  ' }), [
      makeResource({ id: 'r1', type: 'agent', name: 'host' }),
    ]);
    expect(payload.name).toBe('trimmed');
    expect(payload.scope.resources).toStrictEqual([
      { resourceType: 'agent', resourceId: 'r1', name: 'host' },
    ]);
  });

  it('parses recipients and tag filters via parseCommaList', () => {
    const payload = buildReportSchedulePayload(
      makeForm({ recipients: 'a@b.com, c@d.com, a@b.com', tagFilter: 'tier1,tier2' }),
      [],
    );
    expect(payload.delivery.to).toEqual(['a@b.com', 'c@d.com']);
    expect(payload.scope.tags).toEqual(['tier1', 'tier2']);
  });

  it('forwards format, delivery flags, and retention_count verbatim', () => {
    const payload = buildReportSchedulePayload(
      makeForm({
        format: 'csv',
        deliveryMethod: 'disk',
        attach: false,
        saveToDisk: false,
        retentionCount: 3,
      }),
      [],
    );
    expect(payload.format).toBe('csv');
    expect(payload.delivery).toStrictEqual({
      method: 'disk',
      to: [],
      attach: false,
      save_to_disk: false,
    });
    expect(payload.retention_count).toBe(3);
  });
});

// ---- parseCommaList ---------------------------------------------------------

describe('parseCommaList', () => {
  it('returns an empty array for an empty string', () => {
    expect(parseCommaList('')).toStrictEqual([]);
  });

  it('returns a single trimmed item', () => {
    expect(parseCommaList('  alpha  ')).toStrictEqual(['alpha']);
  });

  it('splits, trims, and preserves order for multiple items', () => {
    expect(parseCommaList('a, b ,c')).toStrictEqual(['a', 'b', 'c']);
  });

  it('drops empty entries produced by trailing/leading/double commas', () => {
    expect(parseCommaList(',a,,b,')).toStrictEqual(['a', 'b']);
  });

  it('dedupes case-insensitively while keeping the first-seen casing', () => {
    expect(parseCommaList('Alpha, ALPHA, alpha, Beta, beta')).toStrictEqual(['Alpha', 'Beta']);
  });
});

// ---- reportScheduleCadenceLabel --------------------------------------------

describe('reportScheduleCadenceLabel', () => {
  it('labels a monthly schedule with its day-of-month and time', () => {
    const schedule = makeSchedule({
      cadence: { type: 'monthly', day_of_month: 15, time: '09:00', timezone: 'UTC' },
    });
    expect(reportScheduleCadenceLabel(schedule)).toBe('Monthly on day 15 at 09:00');
  });

  it('preserves a day_of_month of 0 in the monthly label', () => {
    const schedule = makeSchedule({
      cadence: { type: 'monthly', day_of_month: 0, time: '09:00', timezone: 'UTC' },
    });
    expect(reportScheduleCadenceLabel(schedule)).toBe('Monthly on day 0 at 09:00');
  });

  it('labels a weekly schedule with the prettified weekday and time', () => {
    const schedule = makeSchedule({
      cadence: { type: 'weekly', weekday: 'friday', time: '17:30', timezone: 'UTC' },
    });
    expect(reportScheduleCadenceLabel(schedule)).toBe('Friday at 17:30');
  });

  it('falls back to the raw weekday string when it is not in WEEKDAY_LABELS', () => {
    const schedule = makeSchedule({
      cadence: { type: 'weekly', weekday: 'funday', time: '08:00', timezone: 'UTC' },
    });
    expect(reportScheduleCadenceLabel(schedule)).toBe('funday at 08:00');
  });
});

// ---- reportScheduleScopeLabel ----------------------------------------------

describe('reportScheduleScopeLabel', () => {
  it('returns "No scope" when both resources and tags are absent (parts.length falsy arm)', () => {
    expect(reportScheduleScopeLabel(makeSchedule({ scope: {} }))).toBe('No scope');
  });

  it('renders the singular form for exactly one resource', () => {
    expect(
      reportScheduleScopeLabel(
        makeSchedule({ scope: { resources: [{ resourceType: 'vm', resourceId: 'v1' }] } }),
      ),
    ).toBe('1 resource');
  });

  it('renders the plural form for multiple resources', () => {
    expect(
      reportScheduleScopeLabel(
        makeSchedule({
          scope: {
            resources: [
              { resourceType: 'vm', resourceId: 'v1' },
              { resourceType: 'vm', resourceId: 'v2' },
            ],
          },
        }),
      ),
    ).toBe('2 resources');
  });

  it('renders the singular vs plural form for tags', () => {
    expect(reportScheduleScopeLabel(makeSchedule({ scope: { tags: ['tier1'] } }))).toBe('1 tag');
    expect(reportScheduleScopeLabel(makeSchedule({ scope: { tags: ['tier1', 'tier2'] } }))).toBe(
      '2 tags',
    );
  });

  it('joins resources and tags with a comma', () => {
    expect(
      reportScheduleScopeLabel(
        makeSchedule({
          scope: {
            resources: [{ resourceType: 'vm', resourceId: 'v1' }],
            tags: ['tier1', 'tier2'],
          },
        }),
      ),
    ).toBe('1 resource, 2 tags');
  });

  it('treats undefined resources as zero (?.length ?? 0 arm)', () => {
    expect(reportScheduleScopeLabel(makeSchedule({ scope: { tags: ['t1'] } }))).toBe('1 tag');
  });
});

// ---- reportScheduleDeliveryLabel -------------------------------------------

describe('reportScheduleDeliveryLabel', () => {
  it('returns "Save to disk" for the disk method', () => {
    expect(
      reportScheduleDeliveryLabel(
        makeSchedule({ delivery: { method: 'disk', attach: true, save_to_disk: true } }),
      ),
    ).toBe('Save to disk');
  });

  it('renders the singular recipient form for one email recipient', () => {
    expect(
      reportScheduleDeliveryLabel(
        makeSchedule({
          delivery: { method: 'email', to: ['ops@x'], attach: true, save_to_disk: true },
        }),
      ),
    ).toBe('1 email recipient');
  });

  it('renders the plural recipient form for multiple email recipients', () => {
    expect(
      reportScheduleDeliveryLabel(
        makeSchedule({
          delivery: { method: 'email', to: ['a@x', 'b@x'], attach: true, save_to_disk: true },
        }),
      ),
    ).toBe('2 email recipients');
  });

  it('falls back to the generic email copy when there are no recipients', () => {
    expect(
      reportScheduleDeliveryLabel(
        makeSchedule({ delivery: { method: 'email', to: [], attach: true, save_to_disk: true } }),
      ),
    ).toBe('Email config recipients');
  });
});

// ---- reportScheduleLastRunLabel --------------------------------------------

describe('reportScheduleLastRunLabel', () => {
  it('returns "Not run yet" when last_run_status is empty (!status arm)', () => {
    expect(reportScheduleLastRunLabel(makeSchedule({ last_run_status: '' }))).toBe('Not run yet');
  });

  it('returns "Not run yet" when last_run_status is undefined', () => {
    const schedule = makeSchedule();
    delete schedule.last_run_status;
    expect(reportScheduleLastRunLabel(schedule)).toBe('Not run yet');
  });

  it('returns "Last run OK" for the ok status', () => {
    expect(reportScheduleLastRunLabel(makeSchedule({ last_run_status: 'ok' }))).toBe('Last run OK');
  });

  it('surfaces last_error for a failed run', () => {
    expect(
      reportScheduleLastRunLabel(
        makeSchedule({ last_run_status: 'failed', last_error: 'timeout' }),
      ),
    ).toBe('Failed: timeout');
  });

  it('falls back to the generic failed copy when last_error is missing', () => {
    const schedule = makeSchedule({ last_run_status: 'failed' });
    delete schedule.last_error;
    expect(reportScheduleLastRunLabel(schedule)).toBe('Last run failed');
  });
});

// ---- formatReportScheduleTime ----------------------------------------------

describe('formatReportScheduleTime', () => {
  it('returns an empty string for a falsy value', () => {
    expect(formatReportScheduleTime(undefined)).toBe('');
    expect(formatReportScheduleTime('')).toBe('');
  });

  it('returns an empty string for an unparseable date (NaN arm)', () => {
    expect(formatReportScheduleTime('not-a-date')).toBe('');
  });

  it('formats a valid timestamp into a localized, non-empty string', () => {
    const formatted = formatReportScheduleTime('2026-07-15T10:30:00Z');
    // The exact locale formatting is environment-dependent, but a real
    // localized date string always contains digits and is longer than the
    // empty fallback returned for invalid input.
    expect(formatted).toMatch(/\d/);
    expect(formatted.length).toBeGreaterThan(4);
  });

  it('distinguishes two different dates in the formatted output', () => {
    const jan = formatReportScheduleTime('2026-01-05T10:30:00Z');
    const jul = formatReportScheduleTime('2026-07-15T10:30:00Z');
    expect(jan).not.toBe(jul);
    expect(jan).not.toBe('');
    expect(jul).not.toBe('');
  });
});
