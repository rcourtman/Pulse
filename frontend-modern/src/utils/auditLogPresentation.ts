import type { Component } from 'solid-js';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';

export type AuditVerificationStatus = 'verified' | 'failed' | 'unavailable' | 'error';

export interface AuditBadgePresentation {
  label: string;
  className: string;
}

export interface AuditEventStatusPresentation {
  icon: Component<{ class?: string }>;
  className: string;
}

export interface AuditLogEmptyStatePresentation {
  title: string;
  description: string;
}

export const AUDIT_TOOLBAR_BUTTON_CLASS =
  'flex min-h-10 sm:min-h-10 items-center gap-2 px-3 py-2 text-sm font-medium bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50';
export const AUDIT_REFRESH_BUTTON_CLASS = `${AUDIT_TOOLBAR_BUTTON_CLASS} text-base-content`;
export const AUDIT_VERIFY_ALL_BUTTON_CLASS =
  'flex min-h-10 sm:min-h-10 items-center gap-2 px-3 py-2 text-sm font-medium text-blue-700 dark:text-blue-200 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-700 rounded-md hover:bg-blue-100 dark:hover:bg-blue-800 disabled:opacity-50';
export const AUDIT_VERIFY_ROW_BUTTON_CLASS =
  'inline-flex min-h-10 sm:min-h-10 items-center rounded-md border border-blue-200 dark:border-blue-700 px-3 py-2 text-sm font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900 disabled:opacity-50';

export function getAuditEventTypeBadgeClass(event?: string | null): string {
  switch ((event ?? '').trim()) {
    case 'login':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200';
    case 'config_change':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';
    case 'startup':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
    case 'logout':
    case 'oidc_token_refresh':
    default:
      return 'bg-surface-alt text-base-content';
  }
}

export function getAuditEventStatusPresentation(
  success: boolean,
): AuditEventStatusPresentation {
  return success
    ? {
        icon: CheckCircle,
        className: 'w-4 h-4 text-emerald-400',
      }
    : {
        icon: XCircle,
        className: 'w-4 h-4 text-rose-400',
      };
}

export function getAuditVerificationBadgePresentation(
  state?: { status: AuditVerificationStatus } | null,
): AuditBadgePresentation {
  if (!state) {
    return { label: 'Not checked', className: 'bg-surface-alt text-base-content' };
  }

  switch (state.status) {
    case 'verified':
      return {
        label: 'Verified',
        className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
      };
    case 'failed':
      return {
        label: 'Failed',
        className: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
      };
    case 'error':
      return {
        label: 'Error',
        className: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
      };
    default:
      return { label: 'Unavailable', className: 'bg-surface-alt text-base-content' };
  }
}

export function getAuditLogLoadingState() {
  return {
    text: 'Loading audit events…',
  } as const;
}

export function getAuditLogEmptyState(activeFilterCount: number): AuditLogEmptyStatePresentation {
  return activeFilterCount > 0
    ? {
        title: 'No audit events found',
        description: 'No events match your current filters. Try adjusting or clearing them.',
      }
    : {
        title: 'No audit events found',
        description: 'Audit logging is active, but no events have been recorded yet.',
      };
}
