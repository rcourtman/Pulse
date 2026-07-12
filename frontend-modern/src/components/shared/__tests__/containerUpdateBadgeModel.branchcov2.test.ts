import { describe, expect, it, vi } from 'vitest';
import type { DockerContainerUpdateStatus } from '@/types/api';
import {
  getContainerUpdateBadgeTooltip,
  getUpdateIconTooltip,
  getUpdateButtonTooltip,
  getUpdateButtonLabel,
  getUpdateButtonClass,
  getContainerUpdateErrorTooltip,
  getContainerUpdateCurrentTooltip,
} from '@/components/shared/containerUpdateBadgeModel';
import type { UpdateState } from '@/components/shared/containerUpdateBadgeModel';

// Mirrors the module-private UPDATE_BUTTON_BASE_CLASS so the class assertions
// are exact-string equality rather than substring/truthiness checks.
const UPDATE_BUTTON_BASE_CLASS =
  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-all';

// A digest comfortably longer than the longest preview length used (19) so the
// truncation arms are genuinely exercised. 'sha256:' is 7 chars.
const LONG_DIGEST = 'sha256:' + 'a'.repeat(40);
const DIGEST_FIRST_19 = 'sha256:' + 'a'.repeat(12); // 7 + 12 = 19
const DIGEST_FIRST_12 = 'sha256:' + 'a'.repeat(5); // 7 + 5 = 12

function makeUpdateStatus(
  overrides: Partial<DockerContainerUpdateStatus> = {},
): DockerContainerUpdateStatus {
  return {
    updateAvailable: true,
    lastChecked: 1_000,
    ...overrides,
  };
}

describe('containerUpdateBadgeModel.branchcov2', () => {
  describe('getContainerUpdateBadgeTooltip', () => {
    it('falls back to "unknown" for both digests when updateStatus is undefined', () => {
      expect(getContainerUpdateBadgeTooltip(undefined)).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('falls back to "unknown" when both digests are absent on a defined status', () => {
      expect(getContainerUpdateBadgeTooltip(makeUpdateStatus())).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('truncates both digests to 19 characters', () => {
      const status = makeUpdateStatus({
        currentDigest: LONG_DIGEST,
        latestDigest: LONG_DIGEST,
      });
      expect(getContainerUpdateBadgeTooltip(status)).toBe(
        `Image update available\nCurrent: ${DIGEST_FIRST_19}...\nLatest: ${DIGEST_FIRST_19}...`,
      );
    });

    it('mixes a present currentDigest with an absent latestDigest', () => {
      const status = makeUpdateStatus({
        currentDigest: LONG_DIGEST,
        latestDigest: undefined,
      });
      expect(getContainerUpdateBadgeTooltip(status)).toBe(
        `Image update available\nCurrent: ${DIGEST_FIRST_19}...\nLatest: unknown...`,
      );
    });

    it('treats empty-string digests as unknown (|| fallback)', () => {
      const status = makeUpdateStatus({ currentDigest: '', latestDigest: '' });
      expect(getContainerUpdateBadgeTooltip(status)).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });
  });

  describe('getUpdateIconTooltip', () => {
    it('returns the short tooltip via the early return when updateStatus is undefined', () => {
      expect(getUpdateIconTooltip(undefined)).toBe('Image update available');
    });

    it('falls back to "unknown" for both digests on a defined status with no digests', () => {
      expect(getUpdateIconTooltip(makeUpdateStatus())).toBe(
        'Update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('truncates both digests to 12 characters (icon uses a shorter preview than the badge)', () => {
      const status = makeUpdateStatus({
        currentDigest: LONG_DIGEST,
        latestDigest: LONG_DIGEST,
      });
      expect(getUpdateIconTooltip(status)).toBe(
        `Update available\nCurrent: ${DIGEST_FIRST_12}...\nLatest: ${DIGEST_FIRST_12}...`,
      );
    });

    it('handles empty-string digests via the || fallback', () => {
      const status = makeUpdateStatus({ currentDigest: '', latestDigest: '' });
      expect(getUpdateIconTooltip(status)).toBe(
        'Update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });
  });

  describe('getUpdateButtonTooltip', () => {
    it('returns the confirm prompt for the "confirming" state', () => {
      expect(
        getUpdateButtonTooltip({ state: 'confirming', now: 1000 }),
      ).toBe('Click again to confirm update');
    });

    it('returns the success message for the "success" state', () => {
      expect(getUpdateButtonTooltip({ state: 'success', now: 1000 })).toBe(
        '✓ Update completed successfully!',
      );
    });

    it('defaults step to "Processing..." and elapsed to 0 when storeState is missing (updating)', () => {
      expect(
        getUpdateButtonTooltip({ state: 'updating', now: 100_000 }),
      ).toBe('Processing... (0s)');
    });

    it('uses storeState.message and computes elapsed seconds when elapsed <= 60', () => {
      const result = getUpdateButtonTooltip({
        state: 'updating',
        now: 100_000,
        storeState: { startedAt: 95_000, message: 'Pulling layers' },
      });
      expect(result).toBe('Pulling layers (5s)');
    });

    it('falls back to "Processing..." when storeState exists but message is absent/empty', () => {
      expect(
        getUpdateButtonTooltip({
          state: 'updating',
          now: 100_000,
          storeState: { startedAt: 100_000 },
        }),
      ).toBe('Processing... (0s)');

      expect(
        getUpdateButtonTooltip({
          state: 'updating',
          now: 100_000,
          storeState: { startedAt: 95_000, message: '' },
        }),
      ).toBe('Processing... (5s)');
    });

    it('formats as minutes+seconds when elapsed > 60', () => {
      const result = getUpdateButtonTooltip({
        state: 'updating',
        now: 1_000_000,
        storeState: { startedAt: 875_000, message: 'Extracting' },
      });
      // elapsed = round(125000/1000) = 125 -> 2m 5s
      expect(result).toBe('Extracting (2m 5s)');
    });

    it('keeps the seconds format exactly at the 60-second boundary (not > 60)', () => {
      const result = getUpdateButtonTooltip({
        state: 'updating',
        now: 1_000_000,
        storeState: { startedAt: 940_000, message: 'Working' },
      });
      // elapsed = 60 -> not > 60 -> seconds form
      expect(result).toBe('Working (60s)');
    });

    it('falls back to Date.now() for `now` when omitted (?? right arm) — observed via updating elapsed', () => {
      const spy = vi.spyOn(Date, 'now').mockReturnValue(200_000);
      try {
        const result = getUpdateButtonTooltip({
          state: 'updating',
          storeState: { startedAt: 150_000, message: 'Working' },
        });
        // elapsed = round((200000 - 150000)/1000) = 50
        expect(result).toBe('Working (50s)');
      } finally {
        spy.mockRestore();
      }
    });

    it('uses storeState.message for the error tooltip when present', () => {
      expect(
        getUpdateButtonTooltip({
          state: 'error',
          now: 1000,
          storeState: { startedAt: 0, message: 'image not found' },
        }),
      ).toBe('✗ Update failed: image not found');
    });

    it('falls back to errorMessage when storeState.message is missing (error)', () => {
      expect(
        getUpdateButtonTooltip({
          state: 'error',
          now: 1000,
          errorMessage: 'network timeout',
        }),
      ).toBe('✗ Update failed: network timeout');
    });

    it('falls back to "Unknown error" when neither storeState.message nor errorMessage is set', () => {
      expect(getUpdateButtonTooltip({ state: 'error', now: 1000 })).toBe(
        '✗ Update failed: Unknown error',
      );
    });

    it('returns "Update container" for the default state when updateStatus is absent', () => {
      expect(getUpdateButtonTooltip({ state: 'idle', now: 1000 })).toBe(
        'Update container',
      );
    });

    it('truncates digests to 12 chars for the default state when updateStatus is present', () => {
      const result = getUpdateButtonTooltip({
        state: 'idle',
        now: 1000,
        updateStatus: makeUpdateStatus({
          currentDigest: LONG_DIGEST,
          latestDigest: LONG_DIGEST,
        }),
      });
      expect(result).toBe(
        `Click to update\nCurrent: ${DIGEST_FIRST_12}...\nLatest: ${DIGEST_FIRST_12}...`,
      );
    });
  });

  describe('getUpdateButtonLabel', () => {
    it('short-circuits to "Update" when settingsLoaded is false, regardless of state', () => {
      const states: UpdateState[] = [
        'confirming',
        'updating',
        'success',
        'error',
        'idle',
      ];
      for (const state of states) {
        expect(getUpdateButtonLabel(state, false)).toBe('Update');
      }
    });

    it('returns "Confirm?" for the confirming state', () => {
      expect(getUpdateButtonLabel('confirming', true)).toBe('Confirm?');
    });

    it('returns "Updating..." for the updating state', () => {
      expect(getUpdateButtonLabel('updating', true)).toBe('Updating...');
    });

    it('returns "Queued!" for the success state', () => {
      expect(getUpdateButtonLabel('success', true)).toBe('Queued!');
    });

    it('returns "Failed" for the error state', () => {
      expect(getUpdateButtonLabel('error', true)).toBe('Failed');
    });

    it('returns "Update" for the default (idle) state', () => {
      expect(getUpdateButtonLabel('idle', true)).toBe('Update');
    });
  });

  describe('getUpdateButtonClass', () => {
    it('returns the amber confirming classes with pointer hover', () => {
      expect(getUpdateButtonClass('confirming')).toBe(
        `${UPDATE_BUTTON_BASE_CLASS} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 cursor-pointer hover:bg-amber-200 dark:hover:bg-amber-900`,
      );
    });

    it('returns the blue updating classes with cursor-wait', () => {
      expect(getUpdateButtonClass('updating')).toBe(
        `${UPDATE_BUTTON_BASE_CLASS} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-wait`,
      );
    });

    it('returns the green success classes with no special cursor', () => {
      expect(getUpdateButtonClass('success')).toBe(
        `${UPDATE_BUTTON_BASE_CLASS} bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300`,
      );
    });

    it('returns the red error classes with cursor-help', () => {
      expect(getUpdateButtonClass('error')).toBe(
        `${UPDATE_BUTTON_BASE_CLASS} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 cursor-help`,
      );
    });

    it('returns the blue idle/default classes with pointer hover', () => {
      expect(getUpdateButtonClass('idle')).toBe(
        `${UPDATE_BUTTON_BASE_CLASS} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900`,
      );
    });
  });

  // getDigestPreview is module-private (not exported), so its branches are
  // exercised transitively through the public callers above and below. The
  // empty-string / undefined / truncation arms are all reached via the badge,
  // icon, current, and button-default tooltip tests.
  describe('getDigestPreview (via public callers)', () => {
    it('returns "unknown" for an undefined digest (badge tooltip, undefined status)', () => {
      expect(getContainerUpdateBadgeTooltip(undefined)).toContain(
        'Current: unknown...',
      );
    });

    it('returns the full digest when it is shorter than the preview length', () => {
      const shortDigest = 'sha256:abc'; // 10 chars < 19
      const status = makeUpdateStatus({
        currentDigest: shortDigest,
        latestDigest: shortDigest,
      });
      expect(getContainerUpdateBadgeTooltip(status)).toBe(
        'Image update available\nCurrent: sha256:abc...\nLatest: sha256:abc...',
      );
    });

    it('truncates at exactly the requested length (12 for icon, 19 for badge)', () => {
      const status = makeUpdateStatus({
        currentDigest: LONG_DIGEST,
        latestDigest: LONG_DIGEST,
      });
      expect(getContainerUpdateBadgeTooltip(status)).toContain(
        `Current: ${DIGEST_FIRST_19}...`,
      );
      expect(getUpdateIconTooltip(status)).toContain(
        `Current: ${DIGEST_FIRST_12}...`,
      );
    });
  });

  describe('getContainerUpdateErrorTooltip', () => {
    it('falls back to "Unknown error" when updateStatus is undefined', () => {
      expect(getContainerUpdateErrorTooltip(undefined)).toBe(
        'Update check failed: Unknown error',
      );
    });

    it('falls back to "Unknown error" when error is absent on a defined status', () => {
      expect(getContainerUpdateErrorTooltip(makeUpdateStatus())).toBe(
        'Update check failed: Unknown error',
      );
    });

    it('embeds the concrete error message when present', () => {
      const status = makeUpdateStatus({ error: 'registry unreachable' });
      expect(getContainerUpdateErrorTooltip(status)).toBe(
        'Update check failed: registry unreachable',
      );
    });

    it('falls back to "Unknown error" for an empty-string error (|| falsy arm)', () => {
      const status = makeUpdateStatus({ error: '' });
      expect(getContainerUpdateErrorTooltip(status)).toBe(
        'Update check failed: Unknown error',
      );
    });
  });

  describe('getContainerUpdateCurrentTooltip', () => {
    it('returns the plain "Image is current" via the early return when updateStatus is undefined', () => {
      expect(getContainerUpdateCurrentTooltip(undefined)).toBe('Image is current');
    });

    it('takes the early return when currentDigest is absent', () => {
      expect(getContainerUpdateCurrentTooltip(makeUpdateStatus())).toBe(
        'Image is current',
      );
    });

    it('takes the early return for an empty-string currentDigest (falsy guard)', () => {
      const status = makeUpdateStatus({ currentDigest: '' });
      expect(getContainerUpdateCurrentTooltip(status)).toBe('Image is current');
    });

    it('appends a 12-char digest preview when currentDigest is present', () => {
      const status = makeUpdateStatus({ currentDigest: LONG_DIGEST });
      expect(getContainerUpdateCurrentTooltip(status)).toBe(
        `Image is current\nDigest: ${DIGEST_FIRST_12}...`,
      );
    });

    it('shows the full digest (no truncation effect) when it is shorter than 12', () => {
      const status = makeUpdateStatus({ currentDigest: 'sha256:xy' });
      expect(getContainerUpdateCurrentTooltip(status)).toBe(
        'Image is current\nDigest: sha256:xy...',
      );
    });
  });
});
