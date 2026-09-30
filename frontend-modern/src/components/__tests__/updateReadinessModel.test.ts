import { describe, expect, it } from 'vitest';
import {
  MAX_SAME_VERSION_HEALTHY_ATTEMPTS,
  resolvePostUpdateReload,
} from '@/components/updateReadinessModel';

describe('resolvePostUpdateReload', () => {
  it('waits while the pre-update process is still answering with the old version', () => {
    // The backend keeps serving for ~2s after reporting 'completed'; a healthy
    // old-version answer must not trigger the reload.
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.1.0-rc.3',
        reportedVersion: '6.1.0-rc.3',
        sameVersionHealthyAttempts: 0,
      }),
    ).toBe('wait');
  });

  it('reloads once the reported version moves off the pre-update version', () => {
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.1.0-rc.3',
        reportedVersion: '6.1.0-rc.4',
        sameVersionHealthyAttempts: 0,
      }),
    ).toBe('reload');
  });

  it('reloads on a rollback to an older version', () => {
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.1.0-rc.4',
        reportedVersion: '6.0.5',
        sameVersionHealthyAttempts: 0,
      }),
    ).toBe('reload');
  });

  it('falls back to reloading when the version never changes', () => {
    // Mock/CI deployments intentionally never exit; bounded fallback applies.
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.1.0-rc.3',
        reportedVersion: '6.1.0-rc.3',
        sameVersionHealthyAttempts: MAX_SAME_VERSION_HEALTHY_ATTEMPTS,
      }),
    ).toBe('reload');
  });

  it('waits on a healthy response without a version while a comparison is possible', () => {
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.1.0-rc.3',
        reportedVersion: '',
        sameVersionHealthyAttempts: 0,
      }),
    ).toBe('wait');
  });

  it('never reloads without a pre-update version while completion is unconfirmed', () => {
    // An old process answering healthy is indistinguishable from the new one.
    for (const completionConfirmed of [undefined, false]) {
      expect(
        resolvePostUpdateReload({
          preUpdateVersion: null,
          reportedVersion: '6.1.0-rc.4',
          sameVersionHealthyAttempts: MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 10,
          completionConfirmed,
          restartObserved: true,
        }),
      ).toBe('wait');
    }
  });

  it('without a pre-update version, reloads after confirmed completion once the restart was seen', () => {
    const base = {
      preUpdateVersion: null,
      reportedVersion: '6.1.0-rc.4',
      completionConfirmed: true,
    };
    // The about-to-exit process still answers right after 'completed'.
    expect(resolvePostUpdateReload({ ...base, sameVersionHealthyAttempts: 0 })).toBe('wait');
    expect(
      resolvePostUpdateReload({ ...base, sameVersionHealthyAttempts: 0, restartObserved: true }),
    ).toBe('reload');
    expect(
      resolvePostUpdateReload({
        ...base,
        sameVersionHealthyAttempts: MAX_SAME_VERSION_HEALTHY_ATTEMPTS,
      }),
    ).toBe('reload');
  });

  it('never falls back to a same-version reload while completion is unconfirmed', () => {
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.4.5-rc.5',
        reportedVersion: '6.4.5-rc.5',
        sameVersionHealthyAttempts: MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 10,
        completionConfirmed: false,
      }),
    ).toBe('wait');
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: '6.4.5-rc.5',
        reportedVersion: '6.4.5',
        sameVersionHealthyAttempts: 0,
        completionConfirmed: false,
      }),
    ).toBe('reload');
  });
});
