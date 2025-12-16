import { Component, Show, createMemo } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';
import type { VM, Container } from '@/types/api';
import { getBackupInfo } from '@/utils/format';
import { DEGRADED_HEALTH_STATUSES, OFFLINE_HEALTH_STATUSES } from '@/utils/status';
import { useAlertsActivation } from '@/stores/alertsActivation';

interface ProblemGuest {
    guest: VM | Container;
    issues: string[];
}

interface InvestigateProblemsButtonProps {
    /** The filtered guests currently showing as "problems" */
    problemGuests: (VM | Container)[];
    /** Whether the problems filter is active */
    isProblemsMode: boolean;
}

/**
 * "Investigate Problems with AI" button.
 * Appears when the Problems filter is active and finds results.
 * Opens AI chat with rich context about all problem guests.
 */
export const InvestigateProblemsButton: Component<InvestigateProblemsButtonProps> = (props) => {
    // Analyze each guest to determine what issues they have
    const analyzedProblems = createMemo((): ProblemGuest[] => {
        return props.problemGuests.map(guest => {
            const issues: string[] = [];

            // Check for degraded status
            const status = (guest.status || '').toLowerCase();
            const isDegraded = DEGRADED_HEALTH_STATUSES.has(status) ||
                (status !== 'running' && !OFFLINE_HEALTH_STATUSES.has(status) && status !== 'stopped');
            if (isDegraded) {
                issues.push(`Status: ${status || 'unknown'}`);
            }

            // Check for backup issues
            if (!guest.template) {
                const alertsActivation = useAlertsActivation();
                const backupInfo = getBackupInfo(guest.lastBackup, alertsActivation.getBackupThresholds());
                if (backupInfo.status === 'critical') {
                    issues.push('Backup: Critical (very overdue)');
                } else if (backupInfo.status === 'stale') {
                    issues.push('Backup: Stale');
                } else if (backupInfo.status === 'never') {
                    issues.push('Backup: Never backed up');
                }
            }

            // Check for high CPU
            if (guest.cpu > 0.9) {
                issues.push(`CPU: ${(guest.cpu * 100).toFixed(0)}%`);
            }

            // Check for high memory
            if (guest.memory && guest.memory.usage && guest.memory.usage > 90) {
                issues.push(`Memory: ${guest.memory.usage.toFixed(0)}%`);
            }

            return { guest, issues };
        }).filter(p => p.issues.length > 0);
    });

    const handleClick = (e: MouseEvent) => {
        e.stopPropagation();
        e.preventDefault();

        const problems = analyzedProblems();
        if (problems.length === 0) return;

        // Build a rich prompt with all problem details
        const problemSummary = problems.map(({ guest, issues }) => {
            const type = guest.type === 'qemu' ? 'VM' : 'LXC';
            return `- **${guest.name}** (${type} ${guest.vmid} on ${guest.node}): ${issues.join(', ')}`;
        }).join('\n');

        // Categorize issues for better AI understanding
        const issueCategories = {
            backup: problems.filter(p => p.issues.some(i => i.startsWith('Backup:'))).length,
            cpu: problems.filter(p => p.issues.some(i => i.startsWith('CPU:'))).length,
            memory: problems.filter(p => p.issues.some(i => i.startsWith('Memory:'))).length,
            status: problems.filter(p => p.issues.some(i => i.startsWith('Status:'))).length,
        };

        const categoryBreakdown = Object.entries(issueCategories)
            .filter(([, count]) => count > 0)
            .map(([type, count]) => `${count} ${type}`)
            .join(', ');

        const prompt = `I have ${problems.length} guest${problems.length !== 1 ? 's' : ''} that need attention (${categoryBreakdown}):

${problemSummary}

Please help me:
1. **Prioritize** - Which issues should I address first?
2. **Investigate** - For the most critical issues, check their current status
3. **Remediate** - Suggest specific steps to resolve each issue
4. **Prevent** - Recommend any configuration changes to prevent these issues

Start with the most critical problems first.`;

        // Open AI chat with this rich context
        aiChatStore.openWithPrompt(prompt, {
            context: {
                problemCount: problems.length,
                issueCategories,
                guests: problems.map(p => ({
                    id: p.guest.id,
                    name: p.guest.name,
                    vmid: p.guest.vmid,
                    type: p.guest.type,
                    node: p.guest.node,
                    issues: p.issues,
                })),
            },
        });
    };

    // Only show if problems mode is active AND there are results
    const shouldShow = createMemo(() =>
        props.isProblemsMode && props.problemGuests.length > 0
    );

    return (
        <Show when={shouldShow()}>
            <button
                type="button"
                onClick={handleClick}
                class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg
               bg-gradient-to-r from-purple-500 to-blue-500 
               hover:from-purple-600 hover:to-blue-600
               text-white shadow-md shadow-purple-500/25 
               hover:shadow-lg hover:shadow-purple-500/30
               transition-all duration-150 active:scale-95
               ring-1 ring-purple-400/50"
                title={`Ask AI to help investigate and resolve ${props.problemGuests.length} problem${props.problemGuests.length !== 1 ? 's' : ''}`}
            >
                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
                    />
                </svg>
                <span>Investigate {props.problemGuests.length} with AI</span>
                <svg class="w-3 h-3 opacity-70" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                </svg>
            </button>
        </Show>
    );
};

export default InvestigateProblemsButton;
