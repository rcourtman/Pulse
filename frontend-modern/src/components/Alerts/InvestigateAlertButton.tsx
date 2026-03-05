import { Show, createSignal } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';
import type { Alert } from '@/types/api';
import { formatAlertValue } from '@/utils/alertFormatters';
import { getUpgradeActionUrlOrFallback } from '@/stores/license';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';

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
  const isLocked = () => props.licenseLocked === true;
  const canonicalTargetType = (raw?: string): string | undefined => {
    const normalized = (raw || '').trim().toLowerCase();
    switch (normalized) {
      case 'vm':
      case 'qemu':
        return 'vm';
      case 'system-container':
      case 'oci-container':
      case 'lxc':
      case 'ct':
      case 'container':
      case 'system_container':
        return 'system-container';
      case 'app-container':
      case 'docker-container':
      case 'docker_container':
      case 'docker-service':
      case 'docker_service':
      case 'swarm_service':
        return 'app-container';
      case 'agent':
      case 'node':
      case 'storage':
      case 'disk':
      case 'docker-host':
      case 'pbs':
      case 'pmg':
      case 'k8s-node':
      case 'k8s-cluster':
      case 'k8s-deployment':
      case 'k8s-service':
        return normalized;
      case 'host':
        return 'agent';
      case 'k8s':
      case 'kubernetes':
      case 'kubernetes-cluster':
      case 'kubernetes-node':
      case 'kubernetes-deployment':
      case 'kubernetes-service':
        return 'k8s-cluster';
      case '':
        return undefined;
      default:
        return undefined;
    }
  };

  const inferTargetTypeFromResourceID = (resourceID?: string): string | undefined => {
    const normalized = (resourceID || '').trim().toLowerCase();
    if (!normalized) {
      return undefined;
    }

    if (
      normalized.startsWith('vm-') ||
      normalized.startsWith('qemu-') ||
      normalized.includes('/qemu/')
    ) {
      return 'vm';
    }
    if (
      normalized.startsWith('ct-') ||
      normalized.startsWith('lxc-') ||
      normalized.includes('/lxc/')
    ) {
      return 'system-container';
    }
    if (
      normalized.startsWith('docker:') ||
      normalized.startsWith('app-container:') ||
      normalized.includes('/container:')
    ) {
      return 'app-container';
    }
    if (normalized.startsWith('node:')) {
      return 'node';
    }
    if (normalized.startsWith('storage:')) {
      return 'storage';
    }
    if (normalized.startsWith('disk:')) {
      return 'disk';
    }
    if (normalized.startsWith('pbs:')) {
      return 'pbs';
    }
    if (normalized.startsWith('pmg:')) {
      return 'pmg';
    }
    if (
      normalized.startsWith('k8s-') ||
      normalized.startsWith('pod:') ||
      normalized.includes(':pod:')
    ) {
      return 'k8s-cluster';
    }

    return undefined;
  };

  const resolveTargetType = (): string => {
    const metadataType =
      typeof props.alert.metadata?.resourceType === 'string'
        ? (props.alert.metadata.resourceType as string)
        : undefined;

    const fromExplicitType = canonicalTargetType(props.resourceType);
    if (fromExplicitType) {
      return fromExplicitType;
    }

    const fromMetadataType = canonicalTargetType(metadataType);
    if (fromMetadataType) {
      return fromMetadataType;
    }

    const fromResourceID = inferTargetTypeFromResourceID(props.alert.resourceId);
    if (fromResourceID) {
      return fromResourceID;
    }

    return 'agent';
  };

  const resolveTargetTypeForAlert = (): string => {
    const baseType = resolveTargetType();
    if (props.alert.type.startsWith('node_')) {
      return 'node';
    }
    if (props.alert.type.startsWith('docker_')) {
      return 'app-container';
    }
    if (props.alert.type.startsWith('storage_')) {
      return 'storage';
    }

    return baseType;
  };

  // Don't render if AI is not enabled
  if (aiChatStore.enabled !== true) {
    return null;
  }

  const handleClick = (e: MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    if (isLocked()) {
      trackUpgradeClicked('investigate_alert_button', 'ai_alerts');
      window.open(getUpgradeActionUrlOrFallback('ai_alerts'), '_blank');
      return;
    }

    // Calculate how long the alert has been active
    const startTime = new Date(props.alert.startTime);
    const now = new Date();
    const durationMs = now.getTime() - startTime.getTime();
    const durationMins = Math.floor(durationMs / 60000);
    const durationStr =
      durationMins < 60
        ? `${durationMins} min${durationMins !== 1 ? 's' : ''}`
        : `${Math.floor(durationMins / 60)}h ${durationMins % 60}m`;

    // Format a focused prompt for investigation
    const prompt = `Investigate this ${props.alert.level.toUpperCase()} alert:

**Resource:** ${props.alert.resourceName}
**Alert Type:** ${props.alert.type}
**Current Value:** ${formatAlertValue(props.alert.value, props.alert.type)}
**Threshold:** ${formatAlertValue(props.alert.threshold, props.alert.type)}
**Duration:** ${durationStr}
${props.alert.node ? `**Node:** ${props.alert.nodeDisplayName || props.alert.node}` : ''}

Please:
1. Identify the root cause
2. Check related metrics
3. Suggest specific remediation steps
4. Execute diagnostic commands if safe`;

    const targetType = resolveTargetTypeForAlert();

    // Open AI chat with this context and prompt
    aiChatStore.openWithPrompt(prompt, {
      targetType,
      targetId: props.alert.resourceId,
      context: {
        alertId: props.alert.id,
        alertType: props.alert.type,
        alertLevel: props.alert.level,
        alertMessage: props.alert.message,
        guestName: props.alert.resourceName,
        node: props.alert.node,
        vmid: props.vmid,
      },
    });
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
          bg-purple-100 dark:bg-purple-950
          hover:bg-purple-900
          text-purple-600 dark:text-purple-400
          hover:text-purple-700 dark:hover:text-purple-300
          border border-purple-200 dark:border-purple-700
          hover:border-purple-300 dark:hover:border-purple-600
          ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-purple-100 dark:hover:bg-purple-950' : ''}
          ${props.class || ''}`}
        title={
          isLocked()
            ? 'Pro required to investigate alerts with Pulse Assistant'
            : 'Ask Pulse Assistant to investigate this alert'
        }
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
          bg-purple-100 dark:bg-purple-950
          hover:bg-purple-900
          text-purple-600 dark:text-purple-400
          hover:text-purple-700 dark:hover:text-purple-300
          border border-purple-200 dark:border-purple-700
          hover:border-purple-300 dark:hover:border-purple-600
          gap-1.5
          ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-purple-100 dark:hover:bg-purple-950' : ''}
          ${props.class || ''}`}
        title={
          isLocked()
            ? 'Pro required to investigate alerts with Pulse Assistant'
            : 'Ask Pulse Assistant to investigate this alert'
        }
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
        <span class="text-xs font-medium">Ask Pulse Assistant</span>
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
        bg-purple-500
        hover:bg-purple-600
        text-white font-medium
        shadow-sm hover:shadow-sm
        gap-2
        ${isLocked() ? 'opacity-60 cursor-not-allowed hover:bg-purple-500' : ''}
        ${props.class || ''}`}
      title={
        isLocked()
          ? 'Pro required to investigate alerts with Pulse Assistant'
          : 'Ask Pulse Assistant to investigate this alert'
      }
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
      <span>Investigate with Pulse Assistant</span>
      <Show when={isHovered()}>
        <span class="text-xs opacity-80">→</span>
      </Show>
    </button>
  );
}

export default InvestigateAlertButton;
