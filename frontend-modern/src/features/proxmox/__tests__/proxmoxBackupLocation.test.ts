import { describe, expect, it } from 'vitest';
import { backupLocationScope, resolveBackupLocationFilter } from '../proxmoxBackupLocation';

const location = (instance: string | undefined, repository: string | undefined) =>
  backupLocationScope('pbs', instance, repository);

describe('backup location identity', () => {
  it.each([
    [
      ['pbs', 'main'],
      ['pbs', 'Main'],
    ],
    [
      ['pbs', 'main'],
      ['PBS', 'main'],
    ],
    [
      ['a:b', 'c'],
      ['a', 'b:c'],
    ],
    [
      ['same', 'same'],
      ['same', ''],
    ],
    [
      ['same', ''],
      ['', 'same'],
    ],
    [
      ['a / b', 'c'],
      ['a', 'b / c'],
    ],
    [
      ['a"b', 'c'],
      ['a', 'b"c'],
    ],
    [
      ['a', 'c'],
      [' a ', 'c'],
    ],
  ])('preserves case, field boundaries and positions: %j vs %j', (left, right) => {
    const a = location(...(left as [string, string]));
    const b = location(...(right as [string, string]));
    expect(a.locationKey).not.toBe(b.locationKey);
    expect(location(...(left as [string, string])).locationKey).toBe(a.locationKey);
    expect(JSON.parse(a.locationKey!)).toEqual(['pbs', ...left]);
    const url = new URLSearchParams({ location: a.locationKey! });
    expect(new URLSearchParams(url.toString()).get('location')).toBe(a.locationKey);
  });

  it('keeps source kinds and missing facts distinct, with readable role-preserving labels', () => {
    expect(backupLocationScope('pbs', 'same', 'same').locationKey).not.toBe(
      backupLocationScope('archive', 'same', 'same').locationKey,
    );
    expect(location('same', 'same').locationLabel).toBe('PBS · same / same');
    expect(location('', 'same').locationLabel).toBe('PBS · Unknown connection / same');
    expect(location('same', '').locationLabel).toBe('PBS · same / Unknown location');
    expect(location(undefined, undefined)).toEqual({});
    expect(location(' ', '')).toEqual({});
  });

  it('resolves new exact values and only unambiguous legacy values, not caller-invented locations', () => {
    const lower = location('pbs', 'main');
    const upper = location('pbs', 'Main');
    const exact = { key: upper.locationKey, unresolved: false };
    expect(resolveBackupLocationFilter(upper.locationKey!, [lower, upper])).toEqual(exact);
    expect(resolveBackupLocationFilter(upper.legacyLocationKey!, [upper, upper])).toEqual(exact);
    expect(resolveBackupLocationFilter(upper.legacyLocationKey!, [lower, upper])).toEqual({
      key: undefined,
      unresolved: true,
    });
    expect(resolveBackupLocationFilter('unknown', [lower])).toEqual({
      key: undefined,
      unresolved: true,
    });
    expect(resolveBackupLocationFilter(upper.locationKey!, [lower])).toEqual({
      key: undefined,
      unresolved: true,
    });
    expect(resolveBackupLocationFilter('', [lower])).toEqual({ key: undefined, unresolved: false });
  });

  it('withdraws a previously unique legacy interpretation when new inventory exposes a collision', () => {
    const one = location('a:b', 'c');
    const two = location('a', 'b:c');
    expect(one.legacyLocationKey).toBe(two.legacyLocationKey);
    expect(resolveBackupLocationFilter(one.legacyLocationKey!, [one]).key).toBe(one.locationKey);
    expect(resolveBackupLocationFilter(one.legacyLocationKey!, [one, two]).unresolved).toBe(true);
    expect(resolveBackupLocationFilter(one.locationKey!, [one, two]).key).toBe(one.locationKey);
  });
});
