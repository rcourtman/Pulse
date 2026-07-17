import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { collectIdentityConflictHosts } from '../dockerIdentityConflict';

// Minimal docker-host Resource factory. Only the fields the function under test
// reads are populated; the partial fixture is cast to Resource (the same pattern
// used by the sibling dockerIdentityConflict.test.tsx).
const host = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'docker-host-1',
    type: 'docker-host',
    name: 'docker-host-1',
    ...overrides,
  }) as Resource;

describe('dockerIdentityConflict.branchcov', () => {
  describe('collectIdentityConflictHosts — input shape branches', () => {
    it('returns an empty array when the input list is empty', () => {
      expect(collectIdentityConflictHosts([])).toEqual([]);
    });

    it('returns an empty array when no host has docker.identityConflict (docker absent arm)', () => {
      // `host.docker?.identityConflict` short-circuits at `docker` undefined ->
      // `!conflict` is true -> continue.
      const hosts = [
        host({ id: 'h1', name: 'h1' }),
        host({ id: 'h2', name: 'h2' }),
      ];
      expect(collectIdentityConflictHosts(hosts)).toEqual([]);
    });

    it('returns an empty array when docker is present but identityConflict is undefined', () => {
      // `docker` truthy, `docker.identityConflict` undefined -> !conflict continue.
      expect(
        collectIdentityConflictHosts([
          host({ id: 'h1', name: 'h1', docker: { runtime: 'docker' } }),
        ]),
      ).toEqual([]);
    });
  });

  describe('collectIdentityConflictHosts — falsy conflict token (continue arm)', () => {
    it('skips a host whose identityConflict is explicitly null', () => {
      // null is falsy, so `!conflict` is true and the host is excluded entirely.
      expect(
        collectIdentityConflictHosts([
          host({
            id: 'h1',
            name: 'h1',
            docker: {
              identityConflict: null as unknown as { hostnames: string[] },
            },
          }),
        ]),
      ).toEqual([]);
    });
  });

  describe('collectIdentityConflictHosts — hostname normalization branches', () => {
    it('handles identityConflict being an empty object via the hostnames ?? [] fallback', () => {
      // Truthy `{}` passes the !conflict guard; `conflict.hostnames` is
      // undefined -> ?? produces [] -> entry is emitted with an empty list.
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: { identityConflict: {} },
        }),
      ]);
      expect(result).toEqual([{ name: 'h1', hostnames: [] }]);
    });

    it('trims each hostname and preserves non-empty entries verbatim', () => {
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: {
            identityConflict: { hostnames: ['  clone-a  ', 'clone-b'] },
          },
        }),
      ]);
      expect(result).toEqual([
        { name: 'h1', hostnames: ['clone-a', 'clone-b'] },
      ]);
    });

    it('drops hostnames that trim to empty (length === 0 filter arm)', () => {
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: {
            identityConflict: { hostnames: ['clone-a', '   ', '', '\t'] },
          },
        }),
      ]);
      expect(result).toEqual([{ name: 'h1', hostnames: ['clone-a'] }]);
    });

    it('returns an empty hostnames list when every hostname is whitespace-only', () => {
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: {
            identityConflict: { hostnames: ['   ', '', '  \t '] },
          },
        }),
      ]);
      expect(result).toEqual([{ name: 'h1', hostnames: [] }]);
    });

    it('falls back to [] when hostnames is undefined', () => {
      // `conflict.hostnames ?? []` right operand when the key is absent.
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: { identityConflict: { firstSeen: '2026-07-16T00:00:00Z' } },
        }),
      ]);
      expect(result).toEqual([{ name: 'h1', hostnames: [] }]);
    });

    it('falls back to [] when hostnames is null (deliberately invalid token)', () => {
      // `hostnames` is typed optional string[]; coerce null through `as unknown`
      // to exercise the ?? arm against a non-undefined falsy value.
      const result = collectIdentityConflictHosts([
        host({
          id: 'h1',
          name: 'h1',
          docker: {
            identityConflict: {
              hostnames: null as unknown as string[],
            },
          },
        }),
      ]);
      expect(result).toEqual([{ name: 'h1', hostnames: [] }]);
    });
  });

  describe('collectIdentityConflictHosts — name resolution branches', () => {
    it('uses the trimmed resource name when it is non-empty', () => {
      expect(
        collectIdentityConflictHosts([
          host({
            id: 'fallback-id',
            name: '  production-host  ',
            docker: { identityConflict: { hostnames: ['a'] } },
          }),
        ]),
      ).toEqual([{ name: 'production-host', hostnames: ['a'] }]);
    });

    it('falls back to host.id when name is whitespace-only', () => {
      // `host.name?.trim()` is '' (falsy) -> `|| host.id` arm.
      expect(
        collectIdentityConflictHosts([
          host({
            id: 'docker-host-42',
            name: '   ',
            docker: { identityConflict: { hostnames: ['a'] } },
          }),
        ]),
      ).toEqual([{ name: 'docker-host-42', hostnames: ['a'] }]);
    });

    it('falls back to host.id when name is undefined', () => {
      // `host.name?.trim()` is undefined (falsy) -> `|| host.id` arm.
      expect(
        collectIdentityConflictHosts([
          host({
            id: 'docker-host-7',
            name: undefined as unknown as string,
            docker: { identityConflict: { hostnames: ['a'] } },
          }),
        ]),
      ).toEqual([{ name: 'docker-host-7', hostnames: ['a'] }]);
    });

    it('falls back to the literal "host" when both name and id are blank', () => {
      // Last `|| 'host'` fallback arm; name '' -> falsy, id '' -> falsy.
      expect(
        collectIdentityConflictHosts([
          host({
            id: '',
            name: '',
            docker: { identityConflict: { hostnames: ['a'] } },
          }),
        ]),
      ).toEqual([{ name: 'host', hostnames: ['a'] }]);
    });

    it('falls back to the literal "host" when name is whitespace and id is empty', () => {
      // Combines the trim-to-empty arm for name with the falsy id arm.
      expect(
        collectIdentityConflictHosts([
          host({
            id: '',
            name: '   ',
            docker: { identityConflict: { hostnames: ['a'] } },
          }),
        ]),
      ).toEqual([{ name: 'host', hostnames: ['a'] }]);
    });
  });

  describe('collectIdentityConflictHosts — multi-host collection', () => {
    it('collects every conflicting host when 2+ hosts carry identityConflict tokens', () => {
      // The function does not cross-compare hosts; it emits one entry per host
      // that carries its own identityConflict. Both hosts here are flagged.
      const hosts = [
        host({
          id: 'h1',
          name: 'host-one',
          docker: {
            identityConflict: { hostnames: ['clone-a', 'clone-b'] },
          },
        }),
        host({
          id: 'h2',
          name: 'host-two',
          docker: {
            identityConflict: { hostnames: ['clone-c'] },
          },
        }),
      ];
      expect(collectIdentityConflictHosts(hosts)).toEqual([
        { name: 'host-one', hostnames: ['clone-a', 'clone-b'] },
        { name: 'host-two', hostnames: ['clone-c'] },
      ]);
    });

    it('excludes non-conflicting hosts from a mixed list', () => {
      // First and third hosts conflict; the middle host is healthy and must be
      // excluded while preserving the order of the conflictors.
      const hosts = [
        host({
          id: 'h1',
          name: 'conflictor-one',
          docker: {
            identityConflict: { hostnames: ['dup-a', 'dup-b'] },
          },
        }),
        host({ id: 'h2', name: 'healthy-middle' }),
        host({
          id: 'h3',
          name: 'conflictor-two',
          docker: {
            identityConflict: { hostnames: ['dup-c'] },
          },
        }),
      ];
      expect(collectIdentityConflictHosts(hosts)).toEqual([
        { name: 'conflictor-one', hostnames: ['dup-a', 'dup-b'] },
        { name: 'conflictor-two', hostnames: ['dup-c'] },
      ]);
    });

    it('preserves input order when emitting multiple conflicting hosts', () => {
      // The function iterates hosts in order and pushes in order, with no
      // deduplication or sorting applied.
      const hosts = [
        host({
          id: 'zeta',
          name: 'zeta',
          docker: { identityConflict: { hostnames: ['z-1'] } },
        }),
        host({
          id: 'alpha',
          name: 'alpha',
          docker: { identityConflict: { hostnames: ['a-1'] } },
        }),
      ];
      expect(collectIdentityConflictHosts(hosts).map((h) => h.name)).toEqual([
        'zeta',
        'alpha',
      ]);
    });
  });
});
