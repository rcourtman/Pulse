import { describe, expect, it } from 'vitest';
import type { AICostTargetPresentationInput } from '@/utils/aiCostPresentation';
import { getAICostTargetPresentation } from '@/utils/aiCostPresentation';

// `isOpaqueTargetId` and `readableTargetType` are module-private (non-exported)
// helpers, so they are exercised indirectly through the exported
// `getAICostTargetPresentation` entry point, asserting on their observable
// outputs (mirroring the sibling availabilityProbePresentation.branchcov2 and
// metricThresholds.branchcov2 conventions).
//
// `getAICostTargetPresentation` pre-trims `target_id`
// (`target.target_id?.trim() || ''`) and short-circuits the call site on a falsy
// result (`targetId && ...`), so `isOpaqueTargetId` is never reached with an
// empty/whitespace string through this entry point; its `if (!trimmed) return
// false` early-return arm is therefore not exercisable here.
//
// Likewise `readableTargetType` is only ever called with a non-empty `targetType`
// (`target.target_type?.trim() || 'usage'`), so its `normalized || 'usage'`
// nullish-coalesce right operand is not exercisable here. The 'usage' default
// still flows through when `target_type` is absent and yields
// `humanizeIdentifier('usage')`, which IS covered below.

const input = (
  overrides?: Partial<AICostTargetPresentationInput>,
): AICostTargetPresentationInput => ({
  target_type: undefined,
  target_id: undefined,
  ...overrides,
});

describe('isOpaqueTargetId — branch coverage (via getAICostTargetPresentation)', () => {
  describe('canonical UUID regex arm (returns true)', () => {
    it('classifies a lowercase UUID as opaque, forcing the resource-path plural label', () => {
      const id = '7f5941d9-a503-416d-b84e-5a46c9e1e11f';
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: id }))).toEqual({
        label: 'VMs',
        rawLabel: `vm:${id}`,
      });
    });

    it('classifies an uppercase UUID as opaque (regex is case-insensitive)', () => {
      const id = '7F5941D9-A503-416D-B84E-5A46C9E1E11F';
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: id }))).toEqual({
        label: 'VMs',
        rawLabel: `vm:${id}`,
      });
    });
  });

  describe('20+ hex-char regex arm (returns true)', () => {
    it('classifies a 20-char hex string as opaque (boundary value for {20,})', () => {
      const id = '0123456789abcdef0123'; // exactly 20 hex chars
      expect(id).toHaveLength(20);
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: id }))).toEqual({
        label: 'VMs',
        rawLabel: `vm:${id}`,
      });
    });

    it('classifies a 32-char hex string as opaque (the readable-path detail is dropped)', () => {
      const id = 'ed21e8612df6450fab4ebd4ae2502259';
      expect(
        getAICostTargetPresentation(input({ target_type: 'assistant_session', target_id: id })),
      ).toEqual({
        label: 'Assistant sessions',
        rawLabel: `assistant_session:${id}`,
      });
    });
  });

  describe('20+ hex-char regex arm falls through (returns false)', () => {
    it('does NOT classify a 19-char hex string as opaque (just under the {20,} threshold)', () => {
      const id = '0123456789abcdef012'; // 19 hex chars -> readable detail surfaces
      expect(id).toHaveLength(19);
      expect(
        getAICostTargetPresentation(input({ target_type: 'assistant_session', target_id: id })),
      ).toEqual({
        label: 'Assistant sessions',
        detail: id,
        rawLabel: `assistant_session:${id}`,
      });
    });
  });

  describe('long-alphanumeric fallback arm: length > 32 && allowed charset', () => {
    it('classifies a 33-char allowed-charset string (with non-hex letters) as opaque', () => {
      // 33 chars; contains 'g'/'z' (non-hex) so the UUID and 20+hex regexes miss,
      // but it matches ^[a-z0-9:_-]+$ and length > 32 -> opaque.
      const id = 'g0123456789012345678901234567890z';
      expect(id).toHaveLength(33);
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: id }))).toEqual({
        label: 'VMs',
        rawLabel: `vm:${id}`,
      });
    });

    it('treats ":" and "_" and "-" as allowed charset for the long-alphanumeric arm', () => {
      // 40 chars, no hex-only run (contains 'k','s','p','o','d',...) so the UUID
      // and 20+hex regexes miss; matches ^[a-z0-9:_-]+$ and length > 32 -> opaque.
      const id = 'k8s:pod_namespace_name-1234567890123456789';
      expect(id).toHaveLength(42);
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: id }))).toEqual({
        label: 'VMs',
        rawLabel: `vm:${id}`,
      });
    });

    it('does NOT classify a 32-char allowed-charset string as opaque (length > 32 is false at exactly 32)', () => {
      // 32 chars with a non-hex 'g' so neither UUID nor 20+hex match; length is
      // exactly 32 so the `length > 32` predicate is false -> readable.
      const id = 'g0123456789abcdef0123456789abcde';
      expect(id).toHaveLength(32);
      expect(
        getAICostTargetPresentation(input({ target_type: 'assistant_session', target_id: id })),
      ).toEqual({
        label: 'Assistant sessions',
        detail: id,
        rawLabel: `assistant_session:${id}`,
      });
    });

    it('does NOT classify a >32-char string as opaque when it contains a disallowed char', () => {
      // 34 chars; the '.' breaks ^[a-z0-9:_-]+$, so even though length > 32 the
      // charset predicate is false -> readable (short-circuits to false).
      const id = `${'a'.repeat(33)}.`;
      expect(id).toHaveLength(34);
      expect(
        getAICostTargetPresentation(input({ target_type: 'assistant_session', target_id: id })),
      ).toEqual({
        label: 'Assistant sessions',
        detail: id,
        rawLabel: `assistant_session:${id}`,
      });
    });
  });

  describe('short / readable identifiers (returns false)', () => {
    it('classifies a short numeric id as readable (resource path -> "VM 100")', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: '100' }))).toEqual({
        label: 'VM 100',
        rawLabel: 'vm:100',
      });
    });

    it('classifies a hyphenated human name as readable (resource path -> "Container nginx-web")', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'container', target_id: 'nginx-web' })),
      ).toEqual({
        label: 'Container nginx-web',
        rawLabel: 'container:nginx-web',
      });
    });

    it('surfaces a readable id as `detail` on the non-resource path', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'patrol', target_id: 'nightly' })),
      ).toEqual({
        label: 'Patrol runs',
        detail: 'nightly',
        rawLabel: 'patrol:nightly',
      });
    });
  });
});

describe('readableTargetType — branch coverage (via getAICostTargetPresentation.label)', () => {
  describe('TARGET_TYPE_LABELS map hits', () => {
    it('maps "assistant_session" -> "Assistant sessions"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'assistant_session' }))).toEqual({
        label: 'Assistant sessions',
        rawLabel: 'assistant_session',
      });
    });

    it('maps "chat" -> "Assistant sessions" (alias)', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'chat' }))).toEqual({
        label: 'Assistant sessions',
        rawLabel: 'chat',
      });
    });

    it('maps "discovery" -> "Discovery runs"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'discovery' }))).toEqual({
        label: 'Discovery runs',
        rawLabel: 'discovery',
      });
    });

    it('maps "discovery_run" -> "Discovery runs"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'discovery_run' }))).toEqual({
        label: 'Discovery runs',
        rawLabel: 'discovery_run',
      });
    });

    it('maps "patrol_session" -> "Patrol runs"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'patrol_session' }))).toEqual({
        label: 'Patrol runs',
        rawLabel: 'patrol_session',
      });
    });

    it('maps "resource" -> "Resource checks"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'resource' }))).toEqual({
        label: 'Resource checks',
        rawLabel: 'resource',
      });
    });

    it('maps "resource_context" -> "Resource checks"', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'resource_context' }))).toEqual({
        label: 'Resource checks',
        rawLabel: 'resource_context',
      });
    });
  });

  describe('trim + lowercase normalization before the map lookup', () => {
    it('normalizes whitespace and casing so "  PATROL_RUN  " still maps to "Patrol runs"', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: '  PATROL_RUN  ' })),
      ).toEqual({
        label: 'Patrol runs',
        rawLabel: 'PATROL_RUN',
      });
    });
  });

  describe('map miss -> humanizeIdentifier fallback', () => {
    it('humanizes an unmapped snake_case type ("backup_job" -> "Backup Job")', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'backup_job' }))).toEqual({
        label: 'Backup Job',
        rawLabel: 'backup_job',
      });
    });

    it('humanizes an unmapped mixed snake/kebab type', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'vm-snapshot_task' })),
      ).toEqual({
        label: 'Vm Snapshot Task',
        rawLabel: 'vm-snapshot_task',
      });
    });
  });

  describe('default "usage" type (target_type absent or blank)', () => {
    it('falls back to "Usage" when target_type is undefined', () => {
      expect(getAICostTargetPresentation(input({}))).toEqual({
        label: 'Usage',
        rawLabel: 'usage',
      });
    });

    it('falls back to "Usage" when target_type is null', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: null })),
      ).toEqual({
        label: 'Usage',
        rawLabel: 'usage',
      });
    });

    it('falls back to "Usage" when target_type is whitespace-only', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: '   ' })),
      ).toEqual({
        label: 'Usage',
        rawLabel: 'usage',
      });
    });
  });
});

describe('getAICostTargetPresentation — branch coverage', () => {
  describe('RESOURCE_TYPE_PREFIXES resource path (early return, no `detail` field)', () => {
    it('returns "VM <id>" for vm + readable id (ternary true arm)', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'vm', target_id: '100' }))).toEqual({
        label: 'VM 100',
        rawLabel: 'vm:100',
      });
    });

    it('returns the plural "VMs" for vm + opaque id (ternary false arm via isOpaque)', () => {
      expect(
        getAICostTargetPresentation(
          input({ target_type: 'vm', target_id: '7f5941d9-a503-416d-b84e-5a46c9e1e11f' }),
        ),
      ).toEqual({
        label: 'VMs',
        rawLabel: 'vm:7f5941d9-a503-416d-b84e-5a46c9e1e11f',
      });
    });

    it('returns the plural "VMs" for vm + no id (ternary false arm via falsy targetId)', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'vm' }))).toEqual({
        label: 'VMs',
        rawLabel: 'vm',
      });
    });

    it('maps "container" to prefix "Container" with a readable id', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'container', target_id: 'web-1' })),
      ).toEqual({
        label: 'Container web-1',
        rawLabel: 'container:web-1',
      });
    });

    it('maps "lxc" to prefix "Container" (lxc alias) with the plural form when id is opaque', () => {
      expect(
        getAICostTargetPresentation(
          input({ target_type: 'lxc', target_id: '0123456789abcdef0123' }),
        ),
      ).toEqual({
        label: 'Containers',
        rawLabel: 'lxc:0123456789abcdef0123',
      });
    });

    it('lowercases target_type before the resource-prefix lookup ("VM" -> vm)', () => {
      expect(getAICostTargetPresentation(input({ target_type: 'VM', target_id: '7' }))).toEqual({
        label: 'VM 7',
        rawLabel: 'VM:7',
      });
    });

    it('never emits a `detail` field on the resource path', () => {
      const result = getAICostTargetPresentation(
        input({ target_type: 'vm', target_id: 'web-1' }),
      );
      expect(result).not.toHaveProperty('detail');
    });
  });

  describe('readable (non-resource) path — `detail` ternary', () => {
    it('emits detail = id for a readable id (ternary true arm)', () => {
      expect(
        getAICostTargetPresentation(
          input({ target_type: 'assistant_session', target_id: 'design-review' }),
        ),
      ).toEqual({
        label: 'Assistant sessions',
        detail: 'design-review',
        rawLabel: 'assistant_session:design-review',
      });
    });

    it('sets detail = undefined for an opaque id (ternary false arm via isOpaque)', () => {
      // NOTE: the readable path always returns a `detail` *key* (set to
      // `undefined` here), unlike the resource path which omits the key entirely.
      // This shape inconsistency is a suspected source quirk (see GLM_REPORT.md).
      const result = getAICostTargetPresentation(
        input({ target_type: 'assistant_session', target_id: '0123456789abcdef0123' }),
      );
      expect(result.label).toBe('Assistant sessions');
      expect(result.detail).toBeUndefined();
      expect(result.rawLabel).toBe('assistant_session:0123456789abcdef0123');
    });

    it('sets detail = undefined when there is no id (ternary false arm via falsy targetId)', () => {
      const result = getAICostTargetPresentation(input({ target_type: 'patrol_run' }));
      expect(result.label).toBe('Patrol runs');
      expect(result.detail).toBeUndefined();
      expect(result.rawLabel).toBe('patrol_run');
    });
  });

  describe('target_id coercion and rawLabel shaping', () => {
    it('coerces a null target_id to "" (no id, rawLabel has no colon)', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'patrol_run', target_id: null })),
      ).toEqual({
        label: 'Patrol runs',
        rawLabel: 'patrol_run',
      });
    });

    it('trims a padded target_id before shaping rawLabel and detail', () => {
      expect(
        getAICostTargetPresentation(
          input({ target_type: 'patrol_run', target_id: '  nightly  ' }),
        ),
      ).toEqual({
        label: 'Patrol runs',
        detail: 'nightly',
        rawLabel: 'patrol_run:nightly',
      });
    });

    it('treats a whitespace-only target_id as absent (rawLabel has no colon)', () => {
      expect(
        getAICostTargetPresentation(input({ target_type: 'patrol_run', target_id: '   ' })),
      ).toEqual({
        label: 'Patrol runs',
        rawLabel: 'patrol_run',
      });
    });
  });
});
