import type { StatusIndicatorVariant } from '@/utils/status';
import type { RelayConfig, RelayStatus } from '@/api/relay';

export interface RelayConnectionPresentation {
  variant: StatusIndicatorVariant;
  label: string;
  pulse: boolean;
}

export const RELAY_READONLY_NOTICE_CLASS =
  'border border-blue-200 text-xs text-blue-800 dark:border-blue-800 dark:text-blue-200';
export const RELAY_PRIMARY_BUTTON_CLASS =
  'min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50';
export const RELAY_PRIMARY_LINK_CLASS =
  'w-full sm:w-auto min-h-10 text-center sm:min-h-9 px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors';
export const RELAY_SECONDARY_BUTTON_CLASS =
  'min-h-10 sm:min-h-10 px-3 py-2 text-sm font-medium text-base-content bg-surface-hover hover:bg-surface-hover rounded-md disabled:opacity-50';
export const RELAY_INLINE_ACTION_CLASS =
  'text-sm text-indigo-500 hover:underline disabled:opacity-50';
export const RELAY_BETA_TITLE_CLASS = 'text-sm font-medium text-base-content';
export const RELAY_BETA_MESSAGE_CLASS = 'text-xs text-muted mt-1';
export const RELAY_LAST_ERROR_CLASS =
  'mt-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900 rounded px-2 py-1';
export const RELAY_CODE_BLOCK_CLASS =
  'block text-xs font-mono text-base-content bg-surface-alt rounded px-3 py-2 select-all break-all';
export const RELAY_QR_IMAGE_CLASS = 'rounded-md border border-border p-2';
export const RELAY_DIAGNOSTICS_WRAP_CLASS = 'space-y-2';
export const RELAY_DIAGNOSTICS_TITLE_CLASS = 'text-xs font-semibold text-base-content';
export const RELAY_ONBOARDING_TITLE = 'Pair Your Mobile Device';
export const RELAY_ONBOARDING_DESCRIPTION =
  'Pulse Relay lets your phone securely connect to this Pulse instance for remote monitoring.';
export const RELAY_ONBOARDING_UPGRADE_LABEL = 'Get Relay — $49/yr';
export const RELAY_ONBOARDING_TRIAL_LABEL = 'or start a Pro trial';
export const RELAY_ONBOARDING_TRIAL_STARTING_LABEL = 'Starting trial...';
export const RELAY_ONBOARDING_SETUP_LABEL = 'Set Up Relay';
export const RELAY_ONBOARDING_DISCONNECTED_LABEL = 'Relay is currently disconnected.';
export const RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL = 'Start Free Trial & Set Up Mobile';
export const RELAY_ONBOARDING_TRIAL_HINT = '14-DAY PRO TRIAL · NO CREDIT CARD REQUIRED';

export function getRelayDiagnosticClass(severity: 'warning' | 'error'): string {
  return severity === 'error'
    ? 'rounded px-2 py-1 text-xs bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300'
    : 'rounded px-2 py-1 text-xs bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300';
}

export function getRelayConnectionPresentation(
  config?: RelayConfig | null,
  status?: RelayStatus | null,
): RelayConnectionPresentation {
  if (!config?.enabled) {
    return {
      variant: 'muted',
      label: 'Not enabled',
      pulse: false,
    };
  }

  if (status?.connected) {
    return {
      variant: 'success',
      label: 'Connected',
      pulse: true,
    };
  }

  return {
    variant: 'danger',
    label: 'Disconnected',
    pulse: false,
  };
}
