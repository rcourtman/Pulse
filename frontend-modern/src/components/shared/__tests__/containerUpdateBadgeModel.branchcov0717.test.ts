import { describe, expect, it, vi } from 'vitest';
import {
  hasContainerUpdate,
  hasContainerUpdateError,
  hasContainerUpdateCurrent,
  getContainerUpdateErrorTooltip,
  getContainerUpdateCurrentTooltip,
  getContainerUpdateBadgeTooltip,
  getUpdateIconTooltip,
  getUpdateButtonClass,
  getUpdateButtonLabel,
  getUpdateButtonTooltip,
  type UpdateState,
} from '@/components/shared/containerUpdateBadgeModel';
import type { DockerContainerUpdateStatus } from '@/types/api';

// Branch-coverage suite for every exported helper in
// containerUpdateBadgeModel.ts. Each `it()` pins a concrete return value
// (exact string / boolean / class string) so it locks the precise branch
// taken — no truthiness-only checks, no source-string echoes, no snapshots.
//
// `UPDATE_BUTTON_BASE_CLASS` is a module-private (non-exported) const, so its
// literal value is inlined here as `BASE_CLASS` to assert the full produced
// class string verbatim. This is an observable runtime value, not an
// implementation-detail import.
const BASE_CLASS =
  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-all';

// A digest long enough that slice(0, 12) and slice(0, 19) both truncate.
// Pre-computed truncations (verified, not recomputed via slice in the test):
//   slice(0, 12) -> 'sha256:01234'
//   slice(0, 19) -> 'sha256:0123456789ab'
const LONG_DIGEST = 'sha256:0123456789abcdefghijklmnopqrstuvwxyz';

function makeStatus(
  overrides: Partial<DockerContainerUpdateStatus> = {},
): DockerContainerUpdateStatus {
  return {
    updateAvailable: false,
    lastChecked: 0,
    ...overrides,
  };
}

describe('containerUpdateBadgeModel.branchcov', () => {
  describe('hasContainerUpdate', () => {
    it('returns true when updateStatus.updateAvailable === true', () => {
      expect(hasContainerUpdate(makeStatus({ updateAvailable: true }))).toBe(true);
    });

    it('returns false when updateStatus.updateAvailable === false', () => {
      expect(hasContainerUpdate(makeStatus({ updateAvailable: false }))).toBe(false);
    });

    it('returns false when updateStatus is undefined (optional-chain arm)', () => {
      expect(hasContainerUpdate(undefined)).toBe(false);
    });

    it('returns false when updateAvailable is absent (loose-equality undefined arm)', () => {
      // `updateStatus?.updateAvailable === true` uses strict equality, so a
      // missing field (undefined) must NOT be treated as an update.
      const partial = { lastChecked: 1 } as DockerContainerUpdateStatus;
      expect(hasContainerUpdate(partial)).toBe(false);
    });
  });

  describe('hasContainerUpdateError', () => {
    it('returns true when updateStatus.error is a non-empty string', () => {
      expect(hasContainerUpdateError(makeStatus({ error: 'boom' }))).toBe(true);
    });

    it('returns false when error is absent (Boolean(undefined) arm)', () => {
      expect(hasContainerUpdateError(makeStatus({}))).toBe(false);
    });

    it('returns false when updateStatus is undefined (optional-chain arm)', () => {
      expect(hasContainerUpdateError(undefined)).toBe(false);
    });

    it('returns false when error is an empty string (Boolean("") arm)', () => {
      // Boolean('') === false — pins the falsy-string edge of Boolean().
      expect(hasContainerUpdateError(makeStatus({ error: '' }))).toBe(false);
    });
  });

  describe('hasContainerUpdateCurrent', () => {
    it('returns true when updateAvailable === false and there is no error', () => {
      expect(hasContainerUpdateCurrent(makeStatus({ updateAvailable: false }))).toBe(true);
    });

    it('returns false when updateAvailable === true (left operand of && is false)', () => {
      expect(hasContainerUpdateCurrent(makeStatus({ updateAvailable: true }))).toBe(false);
    });

    it('returns false when an error is present alongside updateAvailable === false (right operand of && is false)', () => {
      expect(
        hasContainerUpdateCurrent(makeStatus({ updateAvailable: false, error: 'x' })),
      ).toBe(false);
    });

    it('returns false when updateStatus is undefined (optional-chain / left-operand arm)', () => {
      expect(hasContainerUpdateCurrent(undefined)).toBe(false);
    });

    it('returns false when updateAvailable is absent (strict === false fails)', () => {
      const partial = { lastChecked: 1 } as DockerContainerUpdateStatus;
      expect(hasContainerUpdateCurrent(partial)).toBe(false);
    });
  });

  describe('getContainerUpdateErrorTooltip', () => {
    it('uses the provided error string (|| left arm)', () => {
      expect(getContainerUpdateErrorTooltip(makeStatus({ error: 'registry 500' }))).toBe(
        'Update check failed: registry 500',
      );
    });

    it('falls back to "Unknown error" when error is undefined (|| right arm)', () => {
      expect(getContainerUpdateErrorTooltip(makeStatus({}))).toBe(
        'Update check failed: Unknown error',
      );
    });

    it('falls back to "Unknown error" when updateStatus itself is undefined', () => {
      expect(getContainerUpdateErrorTooltip(undefined)).toBe(
        'Update check failed: Unknown error',
      );
    });

    it('falls back to "Unknown error" when error is an empty string (|| right arm, falsy string)', () => {
      // `'' || 'Unknown error'` evaluates to the right operand.
      expect(getContainerUpdateErrorTooltip(makeStatus({ error: '' }))).toBe(
        'Update check failed: Unknown error',
      );
    });
  });

  describe('getContainerUpdateCurrentTooltip', () => {
    it('returns the bare "Image is current" message when currentDigest is absent (early-return arm)', () => {
      expect(getContainerUpdateCurrentTooltip(makeStatus({}))).toBe('Image is current');
    });

    it('returns the bare "Image is current" message when updateStatus is undefined', () => {
      expect(getContainerUpdateCurrentTooltip(undefined)).toBe('Image is current');
    });

    it('returns the bare "Image is current" message when currentDigest is an empty string', () => {
      // `!updateStatus?.currentDigest` is true for '' — pins the falsy-string edge.
      expect(getContainerUpdateCurrentTooltip(makeStatus({ currentDigest: '' }))).toBe(
        'Image is current',
      );
    });

    it('appends a 12-char digest preview when currentDigest is present', () => {
      expect(
        getContainerUpdateCurrentTooltip(makeStatus({ currentDigest: LONG_DIGEST })),
      ).toBe('Image is current\nDigest: sha256:01234...');
    });

    it('uses the full digest when it is shorter than 12 chars (slice no-op arm)', () => {
      expect(
        getContainerUpdateCurrentTooltip(makeStatus({ currentDigest: 'abc' })),
      ).toBe('Image is current\nDigest: abc...');
    });
  });

  describe('getContainerUpdateBadgeTooltip', () => {
    it('renders "unknown" for both digests when updateStatus is undefined', () => {
      // getDigestPreview(undefined, 19) -> 'unknown'
      expect(getContainerUpdateBadgeTooltip(undefined)).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('renders "unknown" for both digests when neither digest is set', () => {
      expect(getContainerUpdateBadgeTooltip(makeStatus({}))).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('truncates both digests to 19 chars when both are present', () => {
      expect(
        getContainerUpdateBadgeTooltip(
          makeStatus({ currentDigest: LONG_DIGEST, latestDigest: LONG_DIGEST }),
        ),
      ).toBe(
        'Image update available\nCurrent: sha256:0123456789ab...\nLatest: sha256:0123456789ab...',
      );
    });

    it('coerces an empty-string digest to "unknown" (|| right arm of getDigestPreview)', () => {
      // Empty string is falsy: `(''.slice(0,19)) || 'unknown'` -> 'unknown'.
      expect(
        getContainerUpdateBadgeTooltip(
          makeStatus({ currentDigest: '', latestDigest: '' }),
        ),
      ).toBe(
        'Image update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });

    it('mixes a present currentDigest with an absent latestDigest', () => {
      expect(
        getContainerUpdateBadgeTooltip(
          makeStatus({ currentDigest: LONG_DIGEST }),
        ),
      ).toBe(
        'Image update available\nCurrent: sha256:0123456789ab...\nLatest: unknown...',
      );
    });
  });

  describe('getUpdateIconTooltip', () => {
    it('returns the short message when updateStatus is undefined (early-return arm)', () => {
      expect(getUpdateIconTooltip(undefined)).toBe('Image update available');
    });

    it('returns the full multi-line message with 12-char previews when updateStatus is present', () => {
      expect(
        getUpdateIconTooltip(
          makeStatus({ currentDigest: LONG_DIGEST, latestDigest: LONG_DIGEST }),
        ),
      ).toBe(
        'Update available\nCurrent: sha256:01234...\nLatest: sha256:01234...',
      );
    });

    it('returns "unknown" previews when updateStatus is present but digests are absent', () => {
      // Exercises the non-undefined updateStatus arm with both digests missing.
      expect(getUpdateIconTooltip(makeStatus({}))).toBe(
        'Update available\nCurrent: unknown...\nLatest: unknown...',
      );
    });
  });

  describe('getUpdateButtonClass', () => {
    it('returns the blue cursor-wait "updating" classes', () => {
      expect(getUpdateButtonClass('updating')).toBe(
        [
          BASE_CLASS,
          'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-wait',
        ].join(' '),
      );
    });

    it('returns the green "success" classes', () => {
      expect(getUpdateButtonClass('success')).toBe(
        [BASE_CLASS, 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'].join(' '),
      );
    });

    it('returns the red cursor-help "error" classes', () => {
      expect(getUpdateButtonClass('error')).toBe(
        [
          BASE_CLASS,
          'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 cursor-help',
        ].join(' '),
      );
    });

    it('returns the blue cursor-pointer "idle" classes for the default arm', () => {
      // The literal 'idle' value falls through to `default`.
      expect(getUpdateButtonClass('idle')).toBe(
        [
          BASE_CLASS,
          'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900',
        ].join(' '),
      );
    });

    it('routes any unknown state value to the default arm', () => {
      // Cast a bogus value to prove the default branch, not just 'idle'.
      const bogus = 'not-a-state' as unknown as UpdateState;
      expect(getUpdateButtonClass(bogus)).toBe(
        [
          BASE_CLASS,
          'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900',
        ].join(' '),
      );
    });
  });

  describe('getUpdateButtonLabel', () => {
    describe('settingsLoaded = false (early-return arm: always "Update")', () => {
      it.each<UpdateState>(['idle', 'updating', 'success', 'error'])(
        'returns "Update" for state %s when settings are not loaded',
        (state) => {
          expect(getUpdateButtonLabel(state, false)).toBe('Update');
        },
      );
    });

    describe('settingsLoaded = true (switch arms)', () => {
      it('returns "Update" for the idle/default arm', () => {
        expect(getUpdateButtonLabel('idle', true)).toBe('Update');
      });

      it('returns "Updating..." for the updating arm', () => {
        expect(getUpdateButtonLabel('updating', true)).toBe('Updating...');
      });

      it('returns "Queued!" for the success arm', () => {
        expect(getUpdateButtonLabel('success', true)).toBe('Queued!');
      });

      it('returns "Failed" for the error arm', () => {
        expect(getUpdateButtonLabel('error', true)).toBe('Failed');
      });

      it('routes an unknown state value to the default "Update" arm', () => {
        const bogus = 'not-a-state' as unknown as UpdateState;
        expect(getUpdateButtonLabel(bogus, true)).toBe('Update');
      });
    });
  });

  describe('getUpdateButtonTooltip', () => {
    describe('success state', () => {
      it('returns the static success message', () => {
        expect(getUpdateButtonTooltip({ state: 'success' })).toBe(
          '✓ Update completed successfully!',
        );
      });
    });

    describe('updating state', () => {
      it('reports 0s elapsed with the default step when storeState is absent (elapsed=0, <=60 arm)', () => {
        expect(getUpdateButtonTooltip({ state: 'updating', now: 100000 })).toBe(
          'Processing... (0s)',
        );
      });

      it('computes elapsed seconds from storeState.startedAt and uses storeState.message (<=60 arm)', () => {
        // now=100000, startedAt=95000 -> elapsed = round(5000/1000) = 5
        expect(
          getUpdateButtonTooltip({
            state: 'updating',
            now: 100000,
            storeState: { startedAt: 95000, message: 'Pulling layers' },
          }),
        ).toBe('Pulling layers (5s)');
      });

      it('falls back to "Processing..." when storeState is present but has no message', () => {
        // step = storeState?.message || 'Processing...' -> right operand.
        expect(
          getUpdateButtonTooltip({
            state: 'updating',
            now: 100000,
            storeState: { startedAt: 100000 },
          }),
        ).toBe('Processing... (0s)');
      });

      it('formats as "Xm Ys" once elapsed exceeds 60 seconds (>60 arm)', () => {
        // now=1000000, startedAt=0 -> elapsed = 1000 -> floor(1000/60)=16, 1000%60=40
        expect(
          getUpdateButtonTooltip({
            state: 'updating',
            now: 1000000,
            storeState: { startedAt: 0, message: 'Extracting' },
          }),
        ).toBe('Extracting (16m 40s)');
      });

      it('pins the exact 60s boundary as still the seconds format (<=60 arm, not >60)', () => {
        // elapsed == 60 is NOT > 60, so the "(60s)" arm must be taken.
        expect(
          getUpdateButtonTooltip({
            state: 'updating',
            now: 60000,
            storeState: { startedAt: 0, message: 'Extracting' },
          }),
        ).toBe('Extracting (60s)');
      });

      it('uses Date.now() for `now` when options.now is omitted (?? right arm)', () => {
        // Drive the `options.now ?? Date.now()` right arm deterministically by
        // stubbing Date.now(); startedAt is anchored to the same stubbed value
        // so elapsed is pinned to 7 seconds.
        const spy = vi.spyOn(Date, 'now').mockReturnValue(7000);
        try {
          expect(
            getUpdateButtonTooltip({
              state: 'updating',
              storeState: { startedAt: 0, message: 'Restarting' },
            }),
          ).toBe('Restarting (7s)');
        } finally {
          spy.mockRestore();
        }
      });
    });

    describe('error state', () => {
      it('prefers storeState.message when present', () => {
        expect(
          getUpdateButtonTooltip({
            state: 'error',
            storeState: { startedAt: 0, message: 'image not found' },
            errorMessage: 'fallback',
          }),
        ).toBe('✗ Update failed: image not found');
      });

      it('falls back to errorMessage when storeState is absent', () => {
        expect(
          getUpdateButtonTooltip({ state: 'error', errorMessage: 'timeout' }),
        ).toBe('✗ Update failed: timeout');
      });

      it('falls back to errorMessage when storeState is present but has no message', () => {
        // storeState?.message is undefined -> next operand (errorMessage).
        expect(
          getUpdateButtonTooltip({
            state: 'error',
            storeState: { startedAt: 0 },
            errorMessage: 'denied',
          }),
        ).toBe('✗ Update failed: denied');
      });

      it('falls back to "Unknown error" when neither storeState.message nor errorMessage is set', () => {
        expect(getUpdateButtonTooltip({ state: 'error' })).toBe(
          '✗ Update failed: Unknown error',
        );
      });
    });

    describe('idle/default state', () => {
      it('returns "Update container" when updateStatus is absent (early-return arm)', () => {
        expect(getUpdateButtonTooltip({ state: 'idle' })).toBe('Update container');
      });

      it('returns the digest-preview message when updateStatus is present', () => {
        expect(
          getUpdateButtonTooltip({
            state: 'idle',
            updateStatus: makeStatus({
              currentDigest: LONG_DIGEST,
              latestDigest: LONG_DIGEST,
            }),
          }),
        ).toBe(
          'Click to review and update\nCurrent: sha256:01234...\nLatest: sha256:01234...',
        );
      });

      it('returns "unknown" previews when updateStatus is present but digests are absent', () => {
        expect(
          getUpdateButtonTooltip({ state: 'idle', updateStatus: makeStatus({}) }),
        ).toBe('Click to review and update\nCurrent: unknown...\nLatest: unknown...');
      });

      it('routes an unknown state value (with no updateStatus) to the default early-return arm', () => {
        const bogus = 'not-a-state' as unknown as UpdateState;
        expect(getUpdateButtonTooltip({ state: bogus })).toBe('Update container');
      });
    });
  });
});
