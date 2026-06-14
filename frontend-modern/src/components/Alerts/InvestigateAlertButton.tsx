import { Show, createSignal } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';
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
}

/**
 * "Ask AI" button for one-click alert investigation.
 * When clicked, opens the AI chat panel with the alert context pre-populated.
 * Hidden entirely when AI is not configured.
 */
export function InvestigateAlertButton(props: InvestigateAlertButtonProps) {
  const [isHovered, setIsHovered] = createSignal(false);
  const openUpgradeDestination = useUpgradeNavigation();
  const isLocked = () => props.licenseLocked === true;
  const canShowUpgradePrompt = () => !presentationPolicyHidesUpgradePrompts();
  const lockedTitle = () =>
    canShowUpgradePrompt()
      ? t('alerts.assistant.locked.proRequired')
      : t('alerts.assistant.locked.unavailable');
  const buttonTitle = () => (isLocked() ? lockedTitle() : t('alerts.assistant.unlockedTitle'));
  // Don't render if AI is not enabled
  if (aiChatStore.enabled !== true) {
    return null;
  }

  const handleClick = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    if (isLocked()) {
      if (!canShowUpgradePrompt()) {
        return;
      }
      openUpgradeDestination(getUpgradeActionDestination('ai_alerts'));
      return;
    }

    const handoff = buildAlertAssistantHandoff({
      alert: props.alert,
      resourceType: props.resourceType,
      vmid: props.vmid,
    });

    aiChatStore.open(handoff.context);
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

  // Icon-only variant (smallest footprint)
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
          ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-surface' : ''}
          ${props.class || ''}`}
        title={buttonTitle()}
        aria-disabled={isLocked()}
      >
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
          />
        </svg>
        <span class="text-xs font-medium">{t('alerts.assistant.button.text')}</span>
      </button>
    );
  }

  // Full variant (with expanded label)
  return (
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
        ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-blue-600' : ''}
        ${props.class || ''}`}
      title={buttonTitle()}
      aria-disabled={isLocked()}
    >
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
        />
      </svg>
      <span>{t('alerts.assistant.button.full')}</span>
      <Show when={isHovered()}>
        <span class="text-xs opacity-80">→</span>
      </Show>
    </button>
  );
}

export default InvestigateAlertButton;
