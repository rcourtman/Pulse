import { beforeEach, describe, expect, it } from 'vitest';
import {
  dismissMigrationNotice,
  isMigrationNoticeDismissed,
  resolveMigrationNotice,
} from '../migrationNotices';

describe('migration notice helpers', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('returns a workloads notice for kubernetes migrations', () => {
    const notice = resolveMigrationNotice('?migrated=1&from=kubernetes');
    expect(notice).toBeTruthy();
    expect(notice?.id).toBe('kubernetes');
    expect(notice?.target).toBe('workloads');
    expect(notice?.message.toLowerCase()).toContain('deprecated');
  });

  it('returns an infrastructure notice for docker migrations', () => {
    const notice = resolveMigrationNotice('?migrated=1&from=docker');
    expect(notice).toBeTruthy();
    expect(notice?.id).toBe('docker');
    expect(notice?.target).toBe('infrastructure');
  });

  it('calls out services deprecation in migration messaging', () => {
    const notice = resolveMigrationNotice('?migrated=1&from=services');
    expect(notice).toBeTruthy();
    expect(notice?.id).toBe('services');
    expect(notice?.target).toBe('infrastructure');
    expect(notice?.message.toLowerCase()).toContain('deprecated');
  });

  it('returns null when migration parameters are missing or invalid', () => {
    expect(resolveMigrationNotice('?from=kubernetes')).toBeNull();
    expect(resolveMigrationNotice('?migrated=1&from=unknown')).toBeNull();
  });

  it('persists dismissed notices by id', () => {
    expect(isMigrationNoticeDismissed('kubernetes')).toBe(false);
    dismissMigrationNotice('kubernetes');
    expect(isMigrationNoticeDismissed('kubernetes')).toBe(true);
    expect(isMigrationNoticeDismissed('docker')).toBe(false);
  });
});
