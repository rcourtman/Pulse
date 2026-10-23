import { Show, createSignal } from 'solid-js';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import { aiAvailabilityStore } from '@/stores/aiAvailability';
import { notificationStore } from '@/stores/notifications';
import { sessionCanRunPatrol } from '@/stores/sessionSettingsCapabilities';
import { triggerPatrolRun } from '@/api/patrol';
import { t } from '@/i18n';
import type { Alert } from '@/types/api';
import { isPulseSystemAlert } from '@/utils/alertScope';

interface InvestigateAlertButtonProps {
  alert: Alert;
  size?: 'sm' | 'md';
  variant?: 'icon' | 'text';
  class?: string;
  /**
   * The alert's display name (for example "Container Health"). The icon
   * variant names the alert and its resource with it, because a table repeats
   * the button on every row and a shared name tells the rows apart for no one.
   */
  subjectLabel?: string;
  /** When the alert started, as the host shows it; tells repeated alerts apart. */
  subjectTime?: string;
}

/**
 * One-click alert follow-up: runs a manual Patrol check scoped to the alert's
 * resource. Patrol's findings land on the Patrol page. Hidden when AI is not
 * configured, when the session cannot start a Patrol run (the route needs an
 * admin with ai:execute), and for alerts without a resource Patrol can scope
 * to (Pulse system alerts).
 */
export function InvestigateAlertButton(props: InvestigateAlertButtonProps) {
  const resourceId = () => (props.alert.resourceId || '').trim();
  const available = () =>
    aiAvailabilityStore.enabled === true &&
    sessionCanRunPatrol() &&
    !isPulseSystemAlert(props.alert) &&
    resourceId() !== '';

  const iconLabel = () => {
    const resourceName = props.alert.resourceName?.trim();
    if (!resourceName) return t('alerts.investigate.title');
    const alertType = props.subjectLabel?.trim() || props.alert.type;
    const startedAt = props.subjectTime?.trim();
    return startedAt
      ? t('alerts.investigate.iconLabelAt', { alertType, resourceName, startedAt })
      : t('alerts.investigate.iconLabel', { alertType, resourceName });
  };

  // One run per click: a second click while the request is in flight would
  // queue a duplicate Patrol run for the same resource.
  const [starting, setStarting] = createSignal(false);

  const startPatrolInvestigation = async () => {
    if (starting()) return;
    setStarting(true);
    try {
      const result = await triggerPatrolRun({
        resource_ids: [resourceId()],
        alert_identifier: props.alert.id,
        alert_type: props.alert.type,
      });
      if (result.success) {
        notificationStore.success(
          t('alerts.investigate.triggered', {
            resourceName: props.alert.resourceName || resourceId(),
          }),
        );
      } else {
        notificationStore.warning(result.message || t('alerts.investigate.failed'));
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t('alerts.investigate.failed');
      notificationStore.error(message);
    } finally {
      setStarting(false);
    }
  };

  const handleClick = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    void startPatrolInvestigation();
  };

  const sizeClasses = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
  };

  const baseButtonClass = `
    inline-flex items-center justify-center
    rounded-md transition-all duration-200
    focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500
    disabled:opacity-50 disabled:cursor-not-allowed
  `;

  return (
    <Show when={available()}>
      <Show
        when={props.variant === 'icon'}
        fallback={
          <button
            type="button"
            onClick={handleClick}
            disabled={starting()}
            aria-busy={starting()}
            class={`${baseButtonClass} min-h-11 px-2 py-1 sm:min-h-0
              bg-surface
              hover:bg-surface-hover
              text-blue-600 dark:text-blue-400
              hover:text-blue-700 dark:hover:text-blue-300
              border border-border
              gap-1.5
              ${props.class || ''}`}
            title={t('alerts.investigate.title')}
          >
            <ShieldCheckIcon class="h-3.5 w-3.5 shrink-0" />
            <span class="text-xs font-medium">{t('alerts.investigate.label')}</span>
          </button>
        }
      >
        {/* Icon-only variant for tight table rows. */}
        <button
          type="button"
          onClick={handleClick}
          disabled={starting()}
          aria-busy={starting()}
          class={`${baseButtonClass} ${sizeClasses[props.size || 'sm']}
            bg-surface
            hover:bg-surface-hover
            text-blue-600 dark:text-blue-400
            hover:text-blue-700 dark:hover:text-blue-300
            border border-border
            ${props.class || ''}`}
          title={t('alerts.investigate.title')}
          aria-label={iconLabel()}
        >
          <ShieldCheckIcon class={props.size === 'md' ? 'h-4 w-4' : 'h-3.5 w-3.5'} />
        </button>
      </Show>
    </Show>
  );
}

export default InvestigateAlertButton;
