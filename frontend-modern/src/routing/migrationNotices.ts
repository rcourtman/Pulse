import type { LegacyRouteSource } from './navigation';
import { readLegacyMigrationSource } from './navigation';
import {
  LEGACY_ROUTE_MIGRATION_METADATA,
  type MigrationNoticeTarget,
} from './legacyRouteMetadata';

export interface MigrationNotice {
  id: LegacyRouteSource;
  target: MigrationNoticeTarget;
  title: string;
  message: string;
  learnMoreHref?: string;
}

const DISMISSED_NOTICE_PREFIX = 'pulse.migrationNotice.dismissed.';

const NOTICE_BY_SOURCE: Record<LegacyRouteSource, MigrationNotice> = Object.fromEntries(
  Object.values(LEGACY_ROUTE_MIGRATION_METADATA).map((notice) => [
    notice.id,
    {
      id: notice.id,
      target: notice.target,
      title: notice.title,
      message: notice.message,
      learnMoreHref: '/migration-guide',
    },
  ]),
) as Record<LegacyRouteSource, MigrationNotice>;

function getDismissedStorageKey(id: LegacyRouteSource): string {
  return `${DISMISSED_NOTICE_PREFIX}${id}`;
}

export function resolveMigrationNotice(search: string): MigrationNotice | null {
  const source = readLegacyMigrationSource(search);
  if (!source) return null;
  return NOTICE_BY_SOURCE[source] ?? null;
}

export function isMigrationNoticeDismissed(id: LegacyRouteSource): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(getDismissedStorageKey(id)) === '1';
  } catch {
    return false;
  }
}

export function dismissMigrationNotice(id: LegacyRouteSource): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(getDismissedStorageKey(id), '1');
  } catch {
    // Ignore write failures (private mode, quota exceeded, etc.)
  }
}
