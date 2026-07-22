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

  it('reloads on the first healthy response when no pre-update version is known', () => {
    expect(
      resolvePostUpdateReload({
        preUpdateVersion: null,
        reportedVersion: '6.1.0-rc.4',
        sameVersionHealthyAttempts: 0,
      }),
    ).toBe('reload');
  });
});
