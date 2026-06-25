import { Show, createEffect, createSignal, onCleanup } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import { aiChatStore } from '@/stores/aiChat';
import { notificationStore } from '@/stores/notifications';
import { triggerPatrolRun } from '@/api/patrol';
import { t } from '@/i18n';
import type { Alert } from '@/types/api';
import { useUpgradeNavigation } from '@/components/shared/useUpgradeNavigation';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { buildAlertAssistantHandoff } from './alertAssistantHandoffModel';

interface InvestigateAlertButtonProps {
  alert: Alert;
  resourceType?: string;
  vmid?: number;
  size?: 'sm' | 'md';
  variant?: 'icon' | 'text' | 'full';
  class?: string;
  licenseLocked?: boolean;
  /**
   * When true (and the alert has a resource), the text/full variants render as
   * a split investigation button: the primary action runs a manual targeted
   * Patrol check scoped to this alert's resource, while a small menu keeps
   * Pulse Assistant available for context-only explanation. The icon variant is
   * single-purpose regardless.
   */
  patrolOption?: boolean;
}

/**
 * Investigation button for one-click alert follow-up.
 * Plain Assistant opens remain context-only. On alert cards that opt into
 * Patrol routing, resource-backed alerts make Patrol the primary doer and keep
 * Assistant as the secondary explanation path.
 * Hidden entirely when AI is not configured.
 */
export function InvestigateAlertButton(props: InvestigateAlertButtonProps) {
  const [isHovered, setIsHovered] = createSignal(false);
  const [menuOpen, setMenuOpen] = createSignal(false);
  let containerRef: HTMLDivElement | undefined;
  const openUpgradeDestination = useUpgradeNavigation();
  const isLocked = () => props.licenseLocked === true;
  const canShowUpgradePrompt = () => !presentationPolicyHidesUpgradePrompts();
  const lockedTitle = () =>
    canShowUpgradePrompt()
      ? t('alerts.assistant.locked.proRequired')
      : t('alerts.assistant.locked.unavailable');
  // Don't render if AI is not enabled
  if (aiChatStore.enabled !== true) {
    return null;
  }

  const showPatrolOption = () =>
    props.patrolOption === true &&
    !isLocked() &&
    !!(props.alert.resourceId || '').trim();
  const usesPatrolPrimary = () => showPatrolOption() && props.variant !== 'icon';
  const buttonTitle = () =>
    isLocked()
      ? lockedTitle()
      : usesPatrolPrimary()
        ? t('alerts.assistant.patrol.title')
        : t('alerts.assistant.unlockedTitle');

  const closeMenu = () => setMenuOpen(false);

  const handleMenuClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      closeMenu();
    }
  };
  const handleMenuEscape = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      closeMenu();
    }
  };
  createEffect(() => {
    if (!menuOpen()) return;
    document.addEventListener('mousedown', handleMenuClickOutside);
    document.addEventListener('keydown', handleMenuEscape);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleMenuClickOutside);
      document.removeEventListener('keydown', handleMenuEscape);
    });
  });

  const openAssistant = () => {
    const handoff = buildAlertAssistantHandoff({
      alert: props.alert,
      resourceType: props.resourceType,
      vmid: props.vmid,
    });

    aiChatStore.open(handoff.context);
  };

  const startPatrolInvestigation = async () => {
    const resourceId = (props.alert.resourceId || '').trim();
    if (!resourceId) {
      notificationStore.warning(t('alerts.assistant.patrol.noResource'));
      return;
    }
    try {
      const result = await triggerPatrolRun({
        resource_ids: [resourceId],
        alert_identifier: props.alert.id,
        alert_type: props.alert.type,
        context: `Manual targeted check from alert: ${props.alert.type}`,
      });
      if (result.success) {
        notificationStore.success(
          t('alerts.assistant.patrol.triggered', {
            resourceName: props.alert.resourceName || resourceId,
          }),
        );
      } else {
        notificationStore.warning(result.message || t('alerts.assistant.patrol.failed'));
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t('alerts.assistant.patrol.failed');
      notificationStore.error(message);
    }
  };

  const handleClick = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    closeMenu();
    if (isLocked()) {
      if (!canShowUpgradePrompt()) {
        return;
      }
      openUpgradeDestination(getUpgradeActionDestination('ai_alerts'));
      return;
    }

    if (usesPatrolPrimary()) {
      void startPatrolInvestigation();
      return;
    }

    openAssistant();
  };

  const handleToggleMenu = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    setMenuOpen((value) => !value);
  };

  const handleOpenAssistant = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    closeMenu();
    openAssistant();
  };

  const sizeClasses = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
  };

  const baseButtonClass = `
    inline-flex items-center justify-center
    rounded-md transition-all duration-200
    focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500
    disabled:opacity-50 disabled:cursor-not-allowed
  `;

  const SecondaryActionMenu = (splitClass: string, iconClass: string) => (
    <Show when={showPatrolOption()}>
      <button
        type="button"
        onClick={handleToggleMenu}
        aria-haspopup="menu"
        aria-expanded={menuOpen()}
        aria-label={t('alerts.assistant.patrol.chevron')}
        title={t('alerts.assistant.patrol.chevron')}
        class={`${baseButtonClass} ${splitClass}`}
      >
        <ChevronDownIcon class={iconClass} />
      </button>
      <Show when={menuOpen()}>
        <div
          role="menu"
          class="absolute right-0 top-[calc(100%+0.25rem)] z-50 w-60 max-w-[calc(100vw-2rem)] rounded-md border border-border bg-surface shadow-lg"
        >
          <button
            type="button"
            role="menuitem"
            onClick={handleOpenAssistant}
            class="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-base-content hover:bg-surface-hover"
          >
            <SparklesIcon class="h-4 w-4 flex-shrink-0 text-blue-600 dark:text-blue-400" />
            <span class="flex flex-col">
              <span class="font-medium">{t('alerts.assistant.explain.menuLabel')}</span>
              <span class="text-[10px] text-muted">{t('alerts.assistant.explain.menuHint')}</span>
            </span>
          </button>
        </div>
      </Show>
    </Show>
  );

  // Icon-only variant (smallest footprint). Stays single-purpose: opens
  // Assistant directly. No Patrol split in tight table rows.
  if (props.variant === 'icon') {
    return (
      <button
        type="button"
        onClick={handleClick}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        class={`${baseButtonClass} ${sizeClasses[props.size || 'sm']}
          bg-surface
          hover:bg-surface-hover
          text-blue-600 dark:text-blue-400
          hover:text-blue-700 dark:hover:text-blue-300
          border border-border
          ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-surface' : ''}
          ${props.class || ''}`}
        title={buttonTitle()}
        aria-disabled={isLocked()}
      >
        <svg
          class={`${props.size === 'sm' ? 'w-3.5 h-3.5' : 'w-4 h-4'}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
          />
        </svg>
      </button>
    );
  }

  // Text variant (shows "Ask AI" on hover)
  if (props.variant === 'text') {
    return (
      <div ref={containerRef} class="relative inline-flex items-stretch">
        <button
          type="button"
          onClick={handleClick}
          onMouseEnter={() => setIsHovered(true)}
          onMouseLeave={() => setIsHovered(false)}
          class={`${baseButtonClass} px-2 py-1
            bg-surface
            hover:bg-surface-hover
            text-blue-600 dark:text-blue-400
            hover:text-blue-700 dark:hover:text-blue-300
            border border-border
            gap-1.5
            ${showPatrolOption() ? 'rounded-r-none' : ''}
            ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-surface' : ''}
            ${props.class || ''}`}
          title={buttonTitle()}
          aria-disabled={isLocked()}
        >
          <Show
            when={usesPatrolPrimary()}
            fallback={
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
                />
              </svg>
            }
          >
            <ShieldCheckIcon class="h-3.5 w-3.5 flex-shrink-0" />
          </Show>
          <span class="text-xs font-medium">
            {usesPatrolPrimary()
              ? t('alerts.assistant.patrol.menuLabel')
              : t('alerts.assistant.button.text')}
          </span>
        </button>
        {SecondaryActionMenu(
          `px-1.5 py-1 rounded-l-none border border-border border-l-0 bg-surface hover:bg-surface-hover text-blue-600 dark:text-blue-400 ${
            isLocked() ? 'opacity-60 cursor-not-allowed' : ''
          }`,
          'w-3 h-3',
        )}
      </div>
    );
  }

  // Full variant (with expanded label)
  return (
    <div ref={containerRef} class="relative inline-flex items-stretch">
      <button
        type="button"
        onClick={handleClick}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
        class={`${baseButtonClass} px-3 py-1.5
          bg-blue-600
          hover:bg-blue-700
          text-white font-medium
          shadow-sm hover:shadow-sm
          gap-2
          ${showPatrolOption() ? 'rounded-r-none' : ''}
          ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-blue-600' : ''}
          ${props.class || ''}`}
        title={buttonTitle()}
        aria-disabled={isLocked()}
      >
        <Show
          when={usesPatrolPrimary()}
          fallback={
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
              />
            </svg>
          }
        >
          <ShieldCheckIcon class="h-4 w-4 flex-shrink-0" />
        </Show>
        <span>
          {usesPatrolPrimary()
            ? t('alerts.assistant.patrol.menuLabel')
            : t('alerts.assistant.button.full')}
        </span>
        <Show when={isHovered() && !showPatrolOption()}>
          <span class="text-xs opacity-80">→</span>
        </Show>
      </button>
      {SecondaryActionMenu(
        `px-2 py-1.5 rounded-l-none bg-blue-600 hover:bg-blue-700 text-white border-l border-blue-500/60 ${
          isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-blue-600' : ''
        }`,
        'w-3.5 h-3.5',
      )}
    </div>
  );
}

export default InvestigateAlertButton;
