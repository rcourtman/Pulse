/**
 * Branch-coverage tests for alertDetailPresentation.ts (0724pm batch).
 *
 * Measured gap against the module's own existing spec
 * (alertDetailPresentation.test.ts): 4 of 28 branches uncovered,
 * 0 uncovered functions. This file targets exactly those 4 cold arms:
 *
 * - formatPlatformAlertResourceType: `?? type` label fallback (line 87)
 *     reached when a provider's label table has no entry for the given
 *     ResourceType (a realistic cold arm — the backend emitting a type
 *     the provider's presentation table does not cover).
 * - formatPlatformAlertEntityType: the `normalized === 'host'` arm (line 92)
 *     existing tests cover 'vm' / 'datastore' but never 'host'.
 * - formatPlatformAlertStartedAt: the valid-date fall-through to toLocaleString
 *     (line 102 -> 110); existing tests only feed absent / pre-2000 values.
 * - formatPlatformAlertDetailDateTime: the valid-date fall-through to
 *     toLocaleString (line 117 -> 126); existing tests only feed absent /
 *     NaN / pre-2000 values.
 *
 * Does NOT modify the source module or any existing test file.
 */
import { beforeAll, afterAll, describe, expect, it } from 'vitest';

import {
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertEntityType,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';

/* ------------------------------------------------------------------ *
 * formatPlatformAlertResourceType — `?? type` fallback (line 87)
 *
 * RESOURCE_TYPE_LABELS[provider] is a Partial<Record<ResourceType,string>>.
 * A provider alert whose resource type the table does not list falls
 * through to the raw type string. This arm was never exercised by the
 * existing suite, which only passes in-table (provider, type) pairs.
 * ------------------------------------------------------------------ */

describe('formatPlatformAlertResourceType - unmapped resource-type label fallback', () => {
  it('returns the raw type when the docker label table has no entry for it', () => {
    // 'pbs' is a valid ResourceType but is absent from the docker label
    // table (docker only labels containers/services/images/etc.).
    expect(formatPlatformAlertResourceType('pbs', 'docker')).toBe('pbs');
  });
});

/* ------------------------------------------------------------------ *
 * formatPlatformAlertEntityType — `normalized === 'host'` arm (line 92)
 *
 * Existing tests cover 'vm', 'datastore', the generic title-case arm, and
 * the empty-string placeholder, but never the dedicated 'host' canonical
 * spelling.
 * ------------------------------------------------------------------ */

describe('formatPlatformAlertEntityType - host canonicalization', () => {
  it('canonicalizes a lower-case host token to the Host label', () => {
    expect(formatPlatformAlertEntityType('host')).toBe('Host');
  });

  it('canonicalizes host regardless of surrounding whitespace or casing', () => {
    // value.trim().toLowerCase() normalizes before the comparison, so a
    // whitespace-padded / mixed-case host still hits the canonical arm
    // rather than the generic title-case arm.
    expect(formatPlatformAlertEntityType('  HoSt  ')).toBe('Host');
  });
});

/* ------------------------------------------------------------------ *
 * Date formatters — valid-timestamp happy paths
 *
 * Both functions hardcode their Intl options and accept no timeZone /
 * locale overrides, so output is locale- and timezone-dependent. To stay
 * deterministic AND non-tautological we:
 *   1. pin process.env.TZ = 'UTC' so the rendered day is fixed, then
 *      restore it afterwards;
 *   2. assert the exact locale-rendered string by mirroring the same
 *      option set the function hardcodes (matches the runtime default
 *      locale the function itself uses);
 *   3. assert a structural, locale-independent fact that distinguishes the
 *      two formats — startedAt OMITS the year, detailDateTime INCLUDES it.
 *      A swapped-options regression would be caught here.
 *
 * The existing suite only covers the placeholder / fallback arms of these
 * functions (absent value, NaN date, pre-2000 date); the post-2000 valid
 * timestamp fall-through was never reached.
 * ------------------------------------------------------------------ */

describe('platform alert date formatting - present, post-2000 timestamps', () => {
  const previousTz = process.env.TZ;

  beforeAll(() => {
    process.env.TZ = 'UTC';
  });

  afterAll(() => {
    if (previousTz === undefined) delete process.env.TZ;
    else process.env.TZ = previousTz;
  });

  it('formatPlatformAlertStartedAt renders a valid timestamp, omitting the year (table format)', () => {
    const value = '2024-03-15T14:30:00Z';
    const result = formatPlatformAlertStartedAt(value);

    // Happy path, not the placeholder and not a raw passthrough.
    expect(result).not.toBe('-');
    expect(result).not.toBe(value);

    // The compact table format has no year — the structural difference
    // from the detail format and from the raw ISO string.
    expect(result).not.toMatch(/2024/);

    // Under TZ=UTC the day-of-month renders as 15 in every locale.
    expect(result).toMatch(/15/);

    // Mirrors the exact option set the function hardcodes; pinned to the
    // concrete en-US rendering under TZ=UTC (the structural regex checks
    // above carry the locale-independent verification).
    expect(result).toBe('Mar 15, 02:30 PM');
  });

  it('formatPlatformAlertDetailDateTime renders a valid timestamp, including the year (detail format)', () => {
    const value = '2024-03-15T14:30:00Z';
    const result = formatPlatformAlertDetailDateTime(value);

    expect(result).not.toBe('-');
    expect(result).not.toBe(value);

    // The detail format includes the year — distinguishing it from the
    // table format and proving the two option sets were not swapped.
    expect(result).toMatch(/2024/);

    // Under TZ=UTC the day-of-month renders as 15 in every locale.
    expect(result).toMatch(/15/);

    // Concrete en-US rendering under TZ=UTC; structural checks above carry the
    // locale-independent verification (presence of the year + day 15).
    expect(result).toBe('Mar 15, 2024, 02:30 PM');
  });
});
