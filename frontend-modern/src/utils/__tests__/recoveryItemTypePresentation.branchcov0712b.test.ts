import { describe, expect, it, vi } from 'vitest';

import { getRecoveryItemTypePresentation } from '@/utils/recoveryItemTypePresentation';

// vitest hoists vi.mock above the imports. We wrap getResourceTypePresentation
// so that a single sentinel key returns null — reaching the otherwise-dead
// defensive `else` branch in the default case of getRecoveryItemTypePresentation
// — while delegating every other call to the real implementation. The factory
// cannot reference outer consts (it is hoisted), so the sentinel literal is
// duplicated below in FORCE_NULL_KEY.
vi.mock('@/utils/resourceTypePresentation', async () => {
  const actual = await vi.importActual<typeof import('@/utils/resourceTypePresentation')>(
    '@/utils/resourceTypePresentation',
  );
  return {
    ...actual,
    getResourceTypePresentation: (
      ...args: Parameters<typeof actual.getResourceTypePresentation>
    ): ReturnType<typeof actual.getResourceTypePresentation> =>
      args[0] === '__force_null__' ? null : actual.getResourceTypePresentation(...args),
  };
});

// Must match the literal baked into the hoisted mock factory above.
const FORCE_NULL_KEY = '__force_null__';

// Badge-class building blocks mirrored from the source module so the assertions
// describe the documented badge composition rather than echoing the unit under
// test. These are the primitives the module concatenates.
const BADGE_BASE_CLASSES =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';
const TABLE_BADGE_BASE_CLASSES =
  'inline-flex items-center px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap';
const DEFAULT_BADGE_TONE_CLASSES = 'bg-surface-alt text-base-content';
const DEFAULT_BADGE_CLASSES = `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`;
const DEFAULT_TABLE_BADGE_CLASSES = `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`;

// Workload tones mirrored from workloadTypePresentation's PRESENTATION_MAP; the
// vm / system-container / app-container arms thread these through verbatim.
const VM_TONE = 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
const SYSTEM_CONTAINER_TONE = 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
const APP_CONTAINER_TONE = 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300';

describe('getRecoveryItemTypePresentation — branch coverage (branchcov2)', () => {
  it('composes the full vm presentation (workload-map badge + table badge composition)', () => {
    // Existing tests only assert key/label for this arm; this newly exercises
    // the badgeClasses / tableBadgeClasses concatenation lines for case 'vm'.
    expect(getRecoveryItemTypePresentation('vm')).toStrictEqual({
      key: 'vm',
      label: 'VM',
      badgeClasses: `${BADGE_BASE_CLASSES} ${VM_TONE}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${VM_TONE}`,
    });
  });

  it('composes the full system-container presentation from the workload map', () => {
    // case 'system-container' badge composition; the workload label is 'LXC'.
    expect(getRecoveryItemTypePresentation('system-container')).toStrictEqual({
      key: 'system-container',
      label: 'LXC',
      badgeClasses: `${BADGE_BASE_CLASSES} ${SYSTEM_CONTAINER_TONE}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${SYSTEM_CONTAINER_TONE}`,
    });
  });

  it('applies the App Container label override while keeping the app-container tone', () => {
    // case 'app-container' is the only arm that forwards label/title overrides
    // into getWorkloadTypePresentation; the override wins for `label` while the
    // className still comes from the app-container workload entry.
    expect(getRecoveryItemTypePresentation('app-container')).toStrictEqual({
      key: 'app-container',
      label: 'App Container',
      badgeClasses: `${BADGE_BASE_CLASSES} ${APP_CONTAINER_TONE}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${APP_CONTAINER_TONE}`,
    });
  });

  it('composes the full cluster presentation from the k8s-cluster resource entry', () => {
    // case 'cluster' badge composition; getResourceTypePresentation('k8s-cluster')
    // yields the default tone, so the || DEFAULT_BADGE_TONE_CLASSES arm is NOT
    // taken (presentation?.badgeClasses is truthy).
    expect(getRecoveryItemTypePresentation('cluster')).toStrictEqual({
      key: 'cluster',
      label: 'K8s Cluster',
      badgeClasses: `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
    });
  });

  it('re-title-cases the key when the resource label equals the key (default, label === key arm)', () => {
    // 'custom-thing' is unknown to both recovery and resource maps, so
    // getResourceTypePresentation returns { label: 'custom-thing' } — equal to the
    // key — driving the ternary onto its titleCaseDelimitedLabel arm and using
    // presentation.badgeClasses for the badge strings.
    expect(getRecoveryItemTypePresentation('custom-thing')).toStrictEqual({
      key: 'custom-thing',
      label: 'Custom Thing',
      badgeClasses: `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
    });
  });

  it('title-cases a hyphenated multi-token key whose first token is short (documents inert preserveShortAllCaps)', () => {
    // NOTE: source passes preserveShortAllCaps:true to titleCaseDelimitedLabel,
    // but normalizeRecoveryItemTypeQueryValue lowercases the key first, so the
    // 'nfs' token is already 'nfs' by the time title-casing runs and the option
    // cannot rescue it. The real, concrete output is therefore 'Nfs Share' —
    // asserting it pins this (suspected) inert-option behaviour in place.
    expect(getRecoveryItemTypePresentation('nfs-share')).toStrictEqual({
      key: 'nfs-share',
      label: 'Nfs Share',
      badgeClasses: `${BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
      tableBadgeClasses: `${TABLE_BADGE_BASE_CLASSES} ${DEFAULT_BADGE_TONE_CLASSES}`,
    });
  });

  it('falls back to default badges when the resource presentation is null (default, else arm)', () => {
    // The sentinel key is force-mocked so getResourceTypePresentation returns
    // null, reaching the defensive else-branch that yields DEFAULT_*_BADGE_CLASSES.
    // '__force_null__' normalizes to itself and title-cases to 'Force Null'.
    expect(getRecoveryItemTypePresentation(FORCE_NULL_KEY)).toStrictEqual({
      key: FORCE_NULL_KEY,
      label: 'Force Null',
      badgeClasses: DEFAULT_BADGE_CLASSES,
      tableBadgeClasses: DEFAULT_TABLE_BADGE_CLASSES,
    });
  });
});
