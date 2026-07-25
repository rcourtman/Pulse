/**
 * Branch-coverage tests for commercialBillingModel.ts (0724pm batch).
 *
 * Measured baseline (existing commercialBillingModel.test.ts only):
 *   functions: 2/5 covered
 *     - asUnlimitedLimit                      (line 50)  NEVER called
 *     - buildHostedCommercialPlanModel        (line 120) NEVER called
 *     - buildHostedCommercialUsageModel       (line 153) NEVER called
 *   branches : 3/5 arms covered
 *     - cold: `input.licensedEmail || 'Not available'`  (line 62) false arm
 *     - cold: `input.planTerms ? [...] : []`            (line 70) false arm
 *
 * This file exercises:
 *   - asUnlimitedLimit (private) via buildHostedCommercialUsageModel:
 *       positive limit, zero, negative, absent (status null), and a
 *       deliberately-malformed non-number value.
 *   - buildHostedCommercialPlanModel: status present (email shown) and
 *       status null ('Not configured' fallback), plus full summary shape.
 *   - buildHostedCommercialUsageModel: guests limit shown vs absent;
 *       guest usage below / at / above the limit; zero usage.
 *   - buildSelfHostedBaseDetails cold arms reached via
 *       buildSelfHostedCommercialPlanModel: empty licensedEmail and
 *       absent planTerms.
 *
 * Does NOT modify the source module or any existing test file.
 */
import { describe, expect, it } from 'vitest';

import {
  buildHostedCommercialPlanModel,
  buildHostedCommercialUsageModel,
  buildSelfHostedCommercialPlanModel,
  type HostedCommercialModelInput,
} from '../commercialBillingModel';

const createHostedBaseInput = (): HostedCommercialModelInput => ({
  status: {
    email: 'owner@example.com',
    is_lifetime: false,
    expires_at: '2026-12-31',
    max_guests: 10,
  },
  tierLabel: 'Business',
  licenseStatusLabel: 'Active',
  organizationCount: 3,
  memberCount: 24,
  nodeUsage: 7,
  guestUsage: 5,
  renewsOrExpires: 'Renews 2026-12-31',
});

/* ------------------------------------------------------------------ *
 * asUnlimitedLimit — exercised solely through buildHostedCommercialUsageModel
 *   typeof value === 'number' && value > 0 ? value : undefined
 *
 * Arms:
 *   (A) number && > 0  → returns the value (limit shown)
 *   (B) else           → undefined  (covers 0, negative, absent, non-number)
 * ------------------------------------------------------------------ */

describe('asUnlimitedLimit via buildHostedCommercialUsageModel - positive limit shown', () => {
  it('surfaces max_guests as the guests meter limit when it is a positive number', () => {
    const model = buildHostedCommercialUsageModel(createHostedBaseInput());
    const guests = model.meters[1];
    expect(guests.limit).toBe(10);
    expect(guests.current).toBe(5);
    expect(guests.label).toBe('Guests');
    expect(guests.accentClass).toBe('bg-emerald-600 dark:bg-emerald-500');
  });
});

describe('asUnlimitedLimit via buildHostedCommercialUsageModel - zero limit hidden', () => {
  it('drops the guests limit when max_guests is 0 (returns undefined, no limit shown)', () => {
    // NOTE: src/api/license.ts:19 documents "0 means unlimited" for limit
    // fields, but asUnlimitedLimit treats 0 as "no limit to display"
    // (value > 0 is false). See GLM_REPORT for the suspected source mismatch.
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      status: {
        email: 'owner@example.com',
        is_lifetime: false,
        expires_at: '2026-12-31',
        max_guests: 0,
      },
    });
    expect(model.meters[1].limit).toBeUndefined();
  });
});

describe('asUnlimitedLimit via buildHostedCommercialUsageModel - absent status', () => {
  it('drops the guests limit and still reports node/guest usage when status is null', () => {
    // Exercises both the `status?.` optional-chaining short-circuit and the
    // asUnlimitedLimit(undefined) arm.
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      status: null,
    });
    expect(model.meters[1].limit).toBeUndefined();
    // Monitored Systems meter is unaffected by status.
    expect(model.meters[0]).toEqual({
      label: 'Monitored Systems',
      current: 7,
      accentClass: 'bg-blue-600 dark:bg-blue-500',
    });
    expect(model.meters[1].current).toBe(5);
  });
});

describe('asUnlimitedLimit via buildHostedCommercialUsageModel - malformed non-number', () => {
  it('drops the guests limit when max_guests is a non-number string (typeof guard false arm)', () => {
    // Deliberately-malformed input: the declared type is number, but a string
    // still reaches the guard at runtime; `typeof value === 'number'` is false.
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      status: {
        email: 'owner@example.com',
        is_lifetime: false,
        expires_at: '2026-12-31',
        max_guests: '10' as unknown as number,
      },
    });
    expect(model.meters[1].limit).toBeUndefined();
  });
});

/* ------------------------------------------------------------------ *
 * buildHostedCommercialPlanModel — optional `status` field
 *
 * Arms in `input.status?.email || 'Not configured'` (line 144):
 *   (A) status present + email present → email
 *   (B) status null / email absent    → 'Not configured'
 * ------------------------------------------------------------------ */

describe('buildHostedCommercialPlanModel - status present surfaces licensed email', () => {
  it('builds the full summary and shows the licensed email in details', () => {
    const model = buildHostedCommercialPlanModel(createHostedBaseInput());
    expect(model.summary).toEqual([
      { label: 'Plan Tier', value: 'Business' },
      { label: 'License Status', value: 'Active' },
      { label: 'Organizations', value: 3 },
      { label: 'Members (Current Org)', value: 24 },
    ]);
    expect(model.details).toEqual([
      { label: 'Licensed Email', value: 'owner@example.com' },
      { label: 'Renews / Expires', value: 'Renews 2026-12-31' },
    ]);
  });
});

describe('buildHostedCommercialPlanModel - status null falls back to Not configured', () => {
  it('shows "Not configured" for the licensed email when status is absent', () => {
    const model = buildHostedCommercialPlanModel({
      ...createHostedBaseInput(),
      status: null,
    });
    expect(model.details[0]).toEqual({
      label: 'Licensed Email',
      value: 'Not configured',
    });
    // Summary still reflects the non-optional scalar inputs.
    expect(model.summary[2]).toEqual({ label: 'Organizations', value: 3 });
  });
});

describe('buildHostedCommercialPlanModel - status present but email empty', () => {
  it('falls back to "Not configured" when status exists but email is empty string', () => {
    const model = buildHostedCommercialPlanModel({
      ...createHostedBaseInput(),
      status: {
        email: '',
        is_lifetime: true,
        expires_at: null,
        max_guests: undefined,
      },
    });
    expect(model.details[0]).toEqual({
      label: 'Licensed Email',
      value: 'Not configured',
    });
  });
});

/* ------------------------------------------------------------------ *
 * buildHostedCommercialUsageModel — guest usage relative to the limit
 *
 * The model is a pure passthrough for `current`; it does not branch on
 * whether usage is at / below / above the limit. These cases assert the
 * observable current+limit pair is faithfully recorded in each regime.
 * ------------------------------------------------------------------ */

describe('buildHostedCommercialUsageModel - guest usage below the limit', () => {
  it('records current 5 against limit 10', () => {
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      guestUsage: 5,
    });
    expect(model.meters[1]).toMatchObject({ current: 5, limit: 10 });
  });
});

describe('buildHostedCommercialUsageModel - guest usage at the limit', () => {
  it('records current 10 against limit 10', () => {
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      guestUsage: 10,
    });
    expect(model.meters[1]).toMatchObject({ current: 10, limit: 10 });
  });
});

describe('buildHostedCommercialUsageModel - guest usage above the limit', () => {
  it('records current 15 against limit 10 (model does not clamp)', () => {
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      guestUsage: 15,
    });
    expect(model.meters[1]).toMatchObject({ current: 15, limit: 10 });
  });
});

describe('buildHostedCommercialUsageModel - zero guest usage', () => {
  it('records current 0 and still surfaces the configured limit', () => {
    const model = buildHostedCommercialUsageModel({
      ...createHostedBaseInput(),
      guestUsage: 0,
    });
    expect(model.meters[1]).toMatchObject({ current: 0, limit: 10 });
  });
});

/* ------------------------------------------------------------------ *
 * buildSelfHostedBaseDetails — cold arms reached via
 * buildSelfHostedCommercialPlanModel (no retailPlanDefinition so the
 * base-details path is exercised).
 *
 * Cold arm 1: `input.licensedEmail || 'Not available'` (line 62)
 * Cold arm 2: `input.planTerms ? [...] : []`         (line 70)
 * ------------------------------------------------------------------ */

describe('buildSelfHostedBaseDetails - empty licensed email falls back to Not available', () => {
  it('shows "Not available" when licensedEmail is undefined', () => {
    const model = buildSelfHostedCommercialPlanModel({
      statusLabel: 'Active',
      tierLabel: 'Pro',
      expires: '12/31/2026',
      daysRemaining: 30,
      retailPlanDefinition: null,
    });
    const licensedEmailRow = model.details.find((d) => d.label === 'Licensed Email');
    expect(licensedEmailRow).toEqual({ label: 'Licensed Email', value: 'Not available' });
  });
});

describe('buildSelfHostedBaseDetails - absent plan terms omits the Plan Terms row', () => {
  it('produces only 4 detail rows (no Plan Terms) when planTerms is undefined', () => {
    const model = buildSelfHostedCommercialPlanModel({
      licensedEmail: 'owner@example.com',
      statusLabel: 'Active',
      tierLabel: 'Pro',
      expires: '12/31/2026',
      daysRemaining: 30,
      retailPlanDefinition: null,
    });
    expect(model.details.map((d) => d.label)).toEqual([
      'Tier',
      'Licensed Email',
      'Expires',
      'Days Remaining',
    ]);
    expect(model.details.some((d) => d.label === 'Plan Terms')).toBe(false);
  });
});

describe('buildSelfHostedBaseDetails - both cold arms together', () => {
  it('shows "Not available" and omits Plan Terms when both are absent', () => {
    const model = buildSelfHostedCommercialPlanModel({
      statusLabel: 'Active',
      tierLabel: 'Pro',
      expires: '12/31/2026',
      daysRemaining: 0,
      retailPlanDefinition: null,
    });
    expect(model.details).toEqual([
      { label: 'Tier', value: 'Pro' },
      { label: 'Licensed Email', value: 'Not available' },
      { label: 'Expires', value: '12/31/2026' },
      { label: 'Days Remaining', value: 0 },
    ]);
  });
});
