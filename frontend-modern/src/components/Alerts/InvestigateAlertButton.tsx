import { Show, createSignal } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';
import type { Alert } from '@/types/api';
import { formatAlertValue } from '@/utils/alertFormatters';

interface InvestigateAlertButtonProps {
    alert: Alert;
    resourceType?: string;
    vmid?: number;
    size?: 'sm' | 'md';
    variant?: 'icon' | 'text' | 'full';
    class?: string;
}

/**
 * "Ask AI" button for one-click alert investigation.
 * When clicked, opens the AI chat panel with the alert context pre-populated.
 */
export function InvestigateAlertButton(props: InvestigateAlertButtonProps) {
    const [isHovered, setIsHovered] = createSignal(false);

    const handleClick = (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();

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
${props.alert.node ? `**Node:** ${props.alert.node}` : ''}

Please:
1. Identify the root cause
2. Check related metrics
3. Suggest specific remediation steps
4. Execute diagnostic commands if safe`;

        // Determine target type from alert or infer from resource
        let targetType = props.resourceType || 'guest';
        if (props.alert.type.startsWith('node_')) {
            targetType = 'node';
        } else if (props.alert.type.startsWith('docker_')) {
            targetType = 'docker_container';
        } else if (props.alert.type.startsWith('storage_')) {
            targetType = 'storage';
        }

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
          bg-gradient-to-r from-purple-500/10 to-blue-500/10
          hover:from-purple-500/20 hover:to-blue-500/20
          text-purple-600 dark:text-purple-400
          hover:text-purple-700 dark:hover:text-purple-300
          border border-purple-200/50 dark:border-purple-700/50
          hover:border-purple-300 dark:hover:border-purple-600
          ${props.class || ''}`}
                title="Ask AI to investigate this alert"
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
          bg-gradient-to-r from-purple-500/10 to-blue-500/10
          hover:from-purple-500/20 hover:to-blue-500/20
          text-purple-600 dark:text-purple-400
          hover:text-purple-700 dark:hover:text-purple-300
          border border-purple-200/50 dark:border-purple-700/50
          hover:border-purple-300 dark:hover:border-purple-600
          gap-1.5
          ${props.class || ''}`}
                title="Ask AI to investigate this alert"
            >
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
                    />
                </svg>
                <span class="text-xs font-medium">Ask AI</span>
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
        bg-gradient-to-r from-purple-500 to-blue-500
        hover:from-purple-600 hover:to-blue-600
        text-white font-medium
        shadow-sm hover:shadow-md
        gap-2
        ${props.class || ''}`}
            title="Ask AI to investigate this alert"
        >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
                />
            </svg>
            <span>Investigate with AI</span>
            <Show when={isHovered()}>
                <span class="text-xs opacity-80">→</span>
            </Show>
        </button>
    );
}

export default InvestigateAlertButton;
