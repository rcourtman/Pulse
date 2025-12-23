import { Component, createEffect, createSignal, For, Show } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { logger } from '@/utils/logger';
import type { FailurePrediction, InfrastructureChange, RemediationRecord, RemediationStats, AnomalyReport } from '@/types/aiIntelligence';

const DEFAULT_UPGRADE_URL = 'https://pulserelay.pro';

interface InsightRow {
    id: string;
    type: 'prediction' | 'impact' | 'memory' | 'anomaly';
    typeBadge: string;
    typeBadgeClass: string;
    title: string;
    subtitle: string;
    timestamp: string;
    confidence?: number;
    locked: boolean;
    badgeClass: string;
}

/**
 * AIOverviewTable displays AI Insights, Pulse AI Impact, and Operational Memory
 * in a unified, scannable table format
 */
export const AIOverviewTable: Component<{ showWhenEmpty?: boolean }> = (props) => {
    const [predictions, setPredictions] = createSignal<FailurePrediction[]>([]);
    const [remediations, setRemediations] = createSignal<RemediationRecord[]>([]);
    const [remediationStats, setRemediationStats] = createSignal<RemediationStats | null>(null);
    const [changes, setChanges] = createSignal<InfrastructureChange[]>([]);
    const [anomalies, setAnomalies] = createSignal<AnomalyReport[]>([]);
    const [loading, setLoading] = createSignal(false);
    const [error, setError] = createSignal('');

    // Locked states
    const [insightsLocked, setInsightsLocked] = createSignal(false);
    const [impactLocked, setImpactLocked] = createSignal(false);
    const [memoryLocked, setMemoryLocked] = createSignal(false);
    const [upgradeUrl, setUpgradeUrl] = createSignal(DEFAULT_UPGRADE_URL);

    // Locked counts (for locked state display)
    const [insightsLockedCount, setInsightsLockedCount] = createSignal(0);
    const [memoryLockedCount, setMemoryLockedCount] = createSignal(0);

    // Section expand/collapse state - default limits prevent overwhelming list
    const [showAllActions, setShowAllActions] = createSignal(false);
    const [showAllChanges, setShowAllChanges] = createSignal(false);

    // Limits for each section (when not expanded)
    const ACTION_LIMIT = 5;
    const CHANGE_LIMIT = 5;

    const showWhenEmpty = () => Boolean(props.showWhenEmpty);


    const loadData = async () => {
        setLoading(true);
        setError('');
        try {
            // Use allSettled so one failing endpoint doesn't break everything
            const results = await Promise.allSettled([
                AIAPI.getPredictions(),
                AIAPI.getCorrelations(),
                AIAPI.getRemediations({ hours: 168, limit: 6 }),
                AIAPI.getRecentChanges(24),
                AIAPI.getAnomalies(),
            ]);

            // Extract results with fallbacks for failed requests
            const predResp = results[0].status === 'fulfilled' ? results[0].value : { predictions: [], license_required: false, count: 0, upgrade_url: '' };
            const corrResp = results[1].status === 'fulfilled' ? results[1].value : { correlations: [], license_required: false, count: 0, upgrade_url: '' };
            const remResp = results[2].status === 'fulfilled' ? results[2].value : { remediations: [], license_required: false, stats: null, upgrade_url: '' };
            const changesResp = results[3].status === 'fulfilled' ? results[3].value : { changes: [], license_required: false, count: 0, upgrade_url: '' };
            const anomalyResp = results[4].status === 'fulfilled' ? results[4].value : { anomalies: [], count: 0, severity_counts: { critical: 0, high: 0, medium: 0, low: 0 } };


            // Handle anomalies (FREE - no license required)
            setAnomalies(anomalyResp.anomalies || []);


            // Handle insights lock
            const insightsLockedState = Boolean(predResp.license_required || corrResp.license_required);
            setInsightsLocked(insightsLockedState);
            if (insightsLockedState) {
                const predCount = predResp.count || 0;
                const corrCount = corrResp.count || 0;
                setInsightsLockedCount(predCount + corrCount);
                setPredictions([]);
            } else {
                setInsightsLockedCount(0);
                setPredictions(predResp.predictions || []);
            }

            // Handle impact lock
            setImpactLocked(Boolean(remResp.license_required));
            setRemediationStats(remResp.stats || null);
            setRemediations(remResp.remediations || []);

            // Handle memory lock
            const memoryLockedState = Boolean(changesResp.license_required);
            setMemoryLocked(memoryLockedState);
            if (memoryLockedState) {
                setMemoryLockedCount(changesResp.count || 0);
                setChanges([]);
            } else {
                setMemoryLockedCount(0);
                setChanges(changesResp.changes || []);
            }

            // Set upgrade URL from any response
            setUpgradeUrl(
                predResp.upgrade_url || corrResp.upgrade_url || remResp.upgrade_url || changesResp.upgrade_url || DEFAULT_UPGRADE_URL
            );
        } catch (e) {
            logger.error('Failed to load AI overview data:', e);
            setError('Failed to load AI overview data.');
        } finally {
            setLoading(false);
        }
    };

    createEffect(() => {
        void loadData();
    });

    // Format relative time
    const formatRelativeTime = (ts: string) => {
        const date = new Date(ts);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        if (diffMins < 1) return 'just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 7) return `${diffDays}d ago`;
        return date.toLocaleDateString();
    };

    // Format days until
    const formatDaysUntil = (days: number) => {
        if (days < 0) return 'Overdue';
        if (days < 1) return 'Today';
        if (days < 2) return 'Tomorrow';
        return `In ${Math.round(days)} days`;
    };

    // Build unified rows
    const unifiedRows = (): InsightRow[] => {
        const rows: InsightRow[] = [];

        // Anomalies (FREE - show at top since they're real-time)
        for (const anomaly of anomalies()) {
            const severityClasses: Record<string, string> = {
                critical: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
                high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
                medium: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
                low: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
            };
            const badgeColors: Record<string, string> = {
                critical: 'text-red-600 dark:text-red-400',
                high: 'text-orange-600 dark:text-orange-400',
                medium: 'text-amber-600 dark:text-amber-400',
                low: 'text-blue-600 dark:text-blue-400',
            };

            // Format the deviation ratio
            const ratio = anomaly.baseline_mean > 0
                ? (anomaly.current_value / anomaly.baseline_mean).toFixed(1)
                : 'N/A';

            rows.push({
                id: `anomaly-${anomaly.resource_id}-${anomaly.metric}`,
                type: 'anomaly',
                typeBadge: `${anomaly.severity.charAt(0).toUpperCase() + anomaly.severity.slice(1)} Anomaly`,
                typeBadgeClass: severityClasses[anomaly.severity] || severityClasses.low,
                title: `${anomaly.resource_name || anomaly.resource_id}: ${anomaly.metric.toUpperCase()} at ${ratio}x baseline`,
                subtitle: anomaly.description || `Current: ${anomaly.current_value.toFixed(1)}%, Baseline: ${anomaly.baseline_mean.toFixed(1)}%`,
                timestamp: 'Now',
                locked: false,
                badgeClass: badgeColors[anomaly.severity] || badgeColors.low,
            });
        }

        // Predictions
        for (const pred of predictions()) {
            const colorClass = pred.is_overdue || pred.days_until < 0
                ? 'text-red-600 dark:text-red-400'
                : pred.days_until < 3
                    ? 'text-amber-600 dark:text-amber-400'
                    : pred.days_until < 7
                        ? 'text-yellow-600 dark:text-yellow-400'
                        : 'text-blue-600 dark:text-blue-400';

            rows.push({
                id: `pred-${pred.resource_id}-${pred.event_type}`,
                type: 'prediction',
                typeBadge: 'Prediction',
                typeBadgeClass: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
                title: formatEventName(pred.event_type),
                subtitle: pred.basis,
                timestamp: formatDaysUntil(pred.days_until),
                confidence: pred.confidence,
                locked: false,
                badgeClass: colorClass,
            });
        }

        // CORRELATIONS HIDDEN - they need more work to be genuinely useful
        // Current state: "A and B alert together" (bidirectional) is not actionable
        // Shows that correlated things are correlated - no root cause identification
        // TODO: Re-enable when we can identify root cause chains (A causes B, not just A+B)
        // 
        // const MIN_CORRELATION_CONFIDENCE = 0.70;
        // const highConfidenceCorrelations = correlations().filter(c => c.confidence >= MIN_CORRELATION_CONFIDENCE);
        // const sortedCorrelations = [...highConfidenceCorrelations].sort((a, b) => b.confidence - a.confidence);
        // const visibleCorrelations = showAllDependencies()
        //     ? sortedCorrelations
        //     : sortedCorrelations.slice(0, DEPENDENCY_LIMIT);
        // 
        // for (const corr of visibleCorrelations) {
        //     rows.push({
        //         id: `corr-${corr.source_id}-${corr.target_id}`,
        //         type: 'prediction',
        //         typeBadge: 'Dependency',
        //         typeBadgeClass: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
        //         title: `${corr.source_name || corr.source_id} → ${corr.target_name || corr.target_id}`,
        //         subtitle: corr.description || `${corr.event_pattern} (${corr.occurrences} observations)`,
        //         timestamp: `Delay: ${corr.avg_delay}`,
        //         confidence: corr.confidence,
        //         locked: false,
        //         badgeClass: 'text-indigo-600 dark:text-indigo-400',
        //     });
        // }




        // Remediations - ONLY show actual ACTIONS (restarts, resizes, cleanups, fixes)
        // Skip diagnostic commands (df, grep, cat, tail, ps) - they don't provide lasting value
        const isActionableCommand = (action: string): boolean => {
            const cmd = action.trim().replace(/^\[[^\]]+\]\s*/, ''); // Strip [host] prefix
            const actionPatterns = [
                'docker restart', 'docker start', 'docker stop', 'docker rm',
                'docker compose up', 'docker compose down', 'docker compose restart',
                'systemctl restart', 'systemctl start', 'systemctl stop', 'systemctl enable', 'systemctl disable',
                'service restart', 'service start', 'service stop',
                'pct resize', 'pct start', 'pct stop', 'pct shutdown', 'pct reboot',
                'qm resize', 'qm start', 'qm stop', 'qm shutdown', 'qm reboot',
                'rm -', 'rm /',
                'chmod', 'chown',
                'mkdir',
                'mv ', 'cp ',
                'apt install', 'apt upgrade', 'apt remove',
                'yum install', 'dnf install',
                'pip install', 'npm install',
                'kill ', 'pkill ', 'killall ',
                'reboot', 'shutdown',
            ];
            return actionPatterns.some(pattern => cmd.includes(pattern));
        };

        const actionableRemediations = remediations().filter(rem => isActionableCommand(rem.action));
        const visibleRemediations = showAllActions()
            ? actionableRemediations
            : actionableRemediations.slice(0, ACTION_LIMIT);

        for (const rem of visibleRemediations) {
            // Get the title - skip if it's just the generic fallback
            const title = rem.summary || summarizeValue(rem.problem, rem.action);
            if (title === 'Ran diagnostic' || title === 'Executed command') {
                continue; // Skip generic fallbacks - not actionable
            }

            const outcomeBadgeClass: Record<string, string> = {
                resolved: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
                partial: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
                failed: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
            };

            rows.push({
                id: `rem-${rem.finding_id}-${rem.timestamp}`,
                type: 'impact',
                typeBadge: rem.outcome === 'resolved' ? 'Fixed' : rem.outcome === 'failed' ? 'Failed' : 'Action',
                typeBadgeClass: outcomeBadgeClass[rem.outcome] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300',
                title: title,
                subtitle: rem.resource_name || '',
                timestamp: formatRelativeTime(rem.timestamp),
                locked: false,
                badgeClass: 'text-emerald-600 dark:text-emerald-400',
            });
        }



        // Changes - only show meaningful changes, not noise
        // Skip: created (just patrol discovering resources)
        // Skip: backed_up (backups have their own section, no need to duplicate here)
        // Skip: status changes from "unknown" (just startup, not real state changes)
        const meaningfulChanges = changes().filter(c => {
            if (c.change_type === 'created') return false;
            if (c.change_type === 'backed_up') return false;
            // Filter out startup noise: "unknown → running" or "unknown → stopped"
            if (c.change_type === 'status' && c.description?.includes('unknown →')) return false;
            return true;
        });
        const visibleChanges = showAllChanges()
            ? meaningfulChanges
            : meaningfulChanges.slice(0, CHANGE_LIMIT);


        for (const change of visibleChanges) {
            const changeTypeBadgeClass: Record<string, string> = {
                deleted: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
                config: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
                status: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
                migrated: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
                restarted: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
                backed_up: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
            };

            rows.push({
                id: `change-${change.resource_id}-${change.detected_at}`,
                type: 'memory',
                typeBadge: formatChangeType(change.change_type),
                typeBadgeClass: changeTypeBadgeClass[change.change_type] || 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
                title: change.resource_name || change.resource_id,
                subtitle: change.description || 'Change detected by AI Patrol.',
                timestamp: formatRelativeTime(change.detected_at),
                locked: false,
                badgeClass: 'text-teal-600 dark:text-teal-400',
            });
        }


        return rows;
    };

    const formatEventName = (eventType: string) => {
        const names: Record<string, string> = {
            high_memory: 'High Memory',
            high_cpu: 'High CPU',
            disk_full: 'Disk Full',
            oom: 'Out of Memory',
            restart: 'Restart',
            unresponsive: 'Unresponsive',
            backup_failed: 'Backup Failure',
        };
        if (names[eventType]) return names[eventType];
        return eventType
            .split(/[_-]/)
            .map(part => {
                if (part === 'cpu') return 'CPU';
                if (part === 'vm') return 'VM';
                if (part === 'pbs') return 'PBS';
                if (part === 'raid') return 'RAID';
                return part.charAt(0).toUpperCase() + part.slice(1);
            })
            .join(' ');
    };

    const formatChangeType = (changeType: string) => {
        const labels: Record<string, string> = {
            created: 'Created',
            deleted: 'Deleted',
            config: 'Config',
            status: 'Status',
            migrated: 'Migrated',
            restarted: 'Restarted',
            backed_up: 'Backup',
        };
        if (labels[changeType]) return labels[changeType];
        return changeType.split(/[_-]/).map(p => p.charAt(0).toUpperCase() + p.slice(1)).join(' ');
    };

    // Summarize a shell command action into a human-readable description
    // Prefixed with underscore - not currently used but kept for future expansion
    const _summarizeAction = (action: string): { title: string; subtitle: string } => {
        const cmd = action.trim();

        // Common command patterns and their summaries
        if (cmd.includes('df -h') || cmd.includes('du -s')) {
            const pathMatch = cmd.match(/\/([\w/.-]+)/);
            const path = pathMatch ? pathMatch[0] : 'storage';
            return {
                title: 'Checked disk usage',
                subtitle: `Analyzed space on ${path}`
            };
        }
        if (cmd.includes('docker ps') || cmd.includes('docker status')) {
            const containerMatch = cmd.match(/name[=:]?\s*(\w+)/i);
            const container = containerMatch ? containerMatch[1] : 'containers';
            return {
                title: 'Checked container status',
                subtitle: `Verified ${container} is running`
            };
        }
        if (cmd.includes('docker restart') || cmd.includes('docker start')) {
            const containerMatch = cmd.match(/(?:restart|start)\s+(\w+)/i);
            const container = containerMatch ? containerMatch[1] : 'container';
            return {
                title: 'Restarted container',
                subtitle: `Restored ${container} service`
            };
        }
        if (cmd.includes('systemctl restart') || (cmd.includes('service') && cmd.includes('restart'))) {
            const serviceMatch = cmd.match(/(?:restart|start)\s+(\S+)/i);
            const service = serviceMatch ? serviceMatch[1] : 'service';
            return {
                title: 'Restarted service',
                subtitle: `Restored ${service}`
            };
        }
        if (cmd.includes('grep') && cmd.includes('config')) {
            return {
                title: 'Checked configuration',
                subtitle: 'Verified config settings'
            };
        }
        if (cmd.includes('grep')) {
            return {
                title: 'Searched logs/config',
                subtitle: 'Retrieved diagnostic information'
            };
        }
        if (cmd.includes('tail') || cmd.includes('cat') || cmd.includes('head')) {
            return {
                title: 'Inspected logs/files',
                subtitle: 'Retrieved diagnostic information'
            };
        }
        if (cmd.includes('ps aux') || cmd.includes('top') || cmd.includes('htop')) {
            return {
                title: 'Checked processes',
                subtitle: 'Analyzed running processes'
            };
        }
        if (cmd.includes('free') || cmd.includes('memory')) {
            return {
                title: 'Checked memory usage',
                subtitle: 'Analyzed memory allocation'
            };
        }
        if (cmd.includes('ping') || cmd.includes('curl') || cmd.includes('wget')) {
            return {
                title: 'Tested connectivity',
                subtitle: 'Verified network connection'
            };
        }
        if (cmd.startsWith('[host]') || cmd.startsWith('[')) {
            // Agent command - extract the actual command
            const innerCmd = cmd.replace(/^\[[\w\s]+\]\s*/, '');
            const inner = _summarizeAction(innerCmd);
            return {
                title: inner.title,
                subtitle: `${inner.subtitle} (via agent)`
            };
        }

        // Fallback: truncate and show the command
        const truncated = cmd.length > 50 ? cmd.substring(0, 47) + '...' : cmd;
        return {
            title: 'Executed command',
            subtitle: truncated
        };
    };

    // Create a meaningful summary from the ACTION (command) only
    // The problem field is just chat context and not useful here
    const summarizeValue = (_problem: string, action: string): string => {
        const cmd = action.trim();

        // Extract target/context from the command (container names, paths, services)
        const extractTarget = (): string | null => {
            // Container name from docker commands
            const containerMatch = cmd.match(/name[=:]?\s*(\w+)/i) || cmd.match(/filter.*?(\w+)/);
            if (containerMatch) return containerMatch[1];

            // Service name from systemctl
            const serviceMatch = cmd.match(/systemctl\s+\w+\s+(\S+)/);
            if (serviceMatch) return serviceMatch[1];

            // Path for disk commands - get the meaningful part
            const pathMatch = cmd.match(/\/([\w]+)(?:\/[\w_-]+)*(?:\s|$)/);
            if (pathMatch) {
                const fullPath = cmd.match(/\/[\w\/_-]+/)?.[0] || '';
                // Extract meaningful name from path
                if (fullPath.includes('frigate')) return 'Frigate';
                if (fullPath.includes('plex')) return 'Plex';
                if (fullPath.includes('docker')) return 'Docker';
                if (fullPath.includes('recordings')) return 'recordings';
                if (fullPath.includes('media')) return 'media';
                if (fullPath.includes('config')) return 'config';
                return fullPath.split('/').filter(Boolean).pop() || null;
            }

            // Config file reference
            if (cmd.includes('config.yml') || cmd.includes('config.yaml')) return 'config';

            return null;
        };

        const target = extractTarget();
        const targetStr = target ? ` (${target})` : '';

        // Generate meaningful descriptions based on command type
        if (cmd.includes('docker restart') || cmd.includes('docker start')) {
            return `Restarted container${targetStr}`;
        }
        if (cmd.includes('docker stop')) {
            return `Stopped container${targetStr}`;
        }
        if (cmd.includes('docker ps') || cmd.includes('docker status')) {
            return `Verified container status${targetStr}`;
        }
        if (cmd.includes('docker logs')) {
            return `Retrieved container logs${targetStr}`;
        }
        if (cmd.includes('systemctl restart') || cmd.includes('service') && cmd.includes('restart')) {
            return `Restarted service${targetStr}`;
        }
        if (cmd.includes('systemctl status')) {
            return `Checked service status${targetStr}`;
        }
        if ((cmd.includes('df') || cmd.includes('du')) && target) {
            return `Analyzed storage usage${targetStr}`;
        }
        if (cmd.includes('df') || cmd.includes('du')) {
            return 'Analyzed disk space';
        }
        if (cmd.includes('grep') && (cmd.includes('config') || cmd.includes('.yml') || cmd.includes('.yaml'))) {
            return `Inspected configuration${targetStr}`;
        }
        if (cmd.includes('grep') || cmd.includes('tail -f') || cmd.includes('journalctl')) {
            return `Reviewed logs${targetStr}`;
        }
        if (cmd.includes('tail') || cmd.includes('cat') || cmd.includes('head')) {
            return `Retrieved file contents${targetStr}`;
        }
        if (cmd.includes('ping') || cmd.includes('curl') || cmd.includes('wget')) {
            return 'Tested network connectivity';
        }
        if (cmd.includes('free') || cmd.includes('/proc/meminfo')) {
            return 'Checked memory usage';
        }
        if (cmd.includes('ps aux') || cmd.includes('top') || cmd.includes('htop')) {
            return 'Analyzed running processes';
        }
        if (cmd.includes('chmod') || cmd.includes('chown')) {
            return `Fixed permissions${targetStr}`;
        }
        if (cmd.includes('rm ') || cmd.includes('rm -')) {
            return `Cleaned up files${targetStr}`;
        }
        if (cmd.includes('mkdir')) {
            return `Created directory${targetStr}`;
        }
        if (cmd.startsWith('[host]') || cmd.startsWith('[')) {
            // Agent command - recurse with inner command
            const innerCmd = cmd.replace(/^\[[\w\s]+\]\s*/, '');
            return summarizeValue('', innerCmd);
        }

        return 'Ran diagnostic';
    };

    const totalItems = () => unifiedRows().length;
    const anyLocked = () => insightsLocked() || impactLocked() || memoryLocked();

    const shouldShow = () => showWhenEmpty() || loading() || totalItems() > 0 || anyLocked();

    // Stats for quick summary
    const statsDisplay = () => {
        const stats = remediationStats();
        if (!stats) return null;
        return stats;
    };

    // Calculate hidden items per section
    const hiddenDependencies = () => {
        // Correlations hidden from display - always return 0
        return 0;
    };


    const hiddenActions = () => {
        const actionable = remediations().filter(rem => {
            const cmd = rem.action.trim().replace(/^\[[^\]]+\]\s*/, '');
            const patterns = ['docker restart', 'docker start', 'systemctl restart', 'pct resize', 'qm resize', 'rm -', 'chmod', 'chown', 'reboot'];
            return patterns.some(p => cmd.includes(p));
        });
        return showAllActions() ? 0 : Math.max(0, actionable.length - ACTION_LIMIT);
    };
    const hiddenChanges = () => {
        const meaningful = changes().filter(c => c.change_type !== 'created');
        return showAllChanges() ? 0 : Math.max(0, meaningful.length - CHANGE_LIMIT);
    };
    const anyHidden = () => hiddenDependencies() > 0 || hiddenActions() > 0 || hiddenChanges() > 0;

    return (
        <Show when={shouldShow()}>
            <div class="bg-white dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden shadow-sm">
                {/* Header */}
                <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-purple-50 via-emerald-50 to-teal-50 dark:from-purple-900/20 dark:via-emerald-900/20 dark:to-teal-900/20">
                    <div class="flex items-center justify-between flex-wrap gap-2">
                        <div class="flex items-center gap-3">
                            <div class="flex items-center gap-1.5">
                                <svg class="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                                </svg>
                                <span class="font-semibold text-gray-900 dark:text-gray-100">AI Intelligence Summary</span>
                            </div>
                            <Show when={totalItems() > 0}>
                                <span class="px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full">
                                    {totalItems()} items
                                </span>
                            </Show>
                            <Show when={anyLocked()}>
                                <span class="px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded-full">
                                    Some Locked
                                </span>
                            </Show>
                        </div>

                        {/* Quick Stats */}
                        <Show when={statsDisplay()}>
                            <div class="flex items-center gap-3 text-xs">
                                <div class="flex items-center gap-1">
                                    <span class="text-green-600 dark:text-green-400">✓</span>
                                    <span class="text-gray-600 dark:text-gray-400">{statsDisplay()!.resolved} resolved</span>
                                </div>
                                <div class="flex items-center gap-1">
                                    <span class="text-blue-600 dark:text-blue-400">⚡</span>
                                    <span class="text-gray-600 dark:text-gray-400">{statsDisplay()!.automatic} auto-fix</span>
                                </div>
                            </div>
                        </Show>
                    </div>
                </div>

                {/* Loading */}
                <Show when={loading()}>
                    <div class="px-4 py-6 text-center">
                        <div class="inline-flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                            <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                            Loading AI intelligence...
                        </div>
                    </div>
                </Show>

                {/* Error */}
                <Show when={error() && !loading()}>
                    <div class="px-4 py-4">
                        <div class="text-sm text-red-600 dark:text-red-400">{error()}</div>
                    </div>
                </Show>

                {/* Locked notice (if all are locked) */}
                <Show when={insightsLocked() && impactLocked() && memoryLocked() && !loading()}>
                    <div class="px-4 py-4">
                        <div class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-800 dark:text-amber-200">
                            <div class="flex items-center justify-between gap-3">
                                <div>
                                    <p class="font-medium">Pulse Pro required</p>
                                    <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                                        Full AI intelligence features including predictions, impact tracking, and operational memory require Pulse Pro.
                                    </p>
                                </div>
                                <a
                                    class="text-xs font-medium text-amber-800 dark:text-amber-200 underline whitespace-nowrap"
                                    href={upgradeUrl()}
                                    target="_blank"
                                    rel="noreferrer"
                                >
                                    Upgrade
                                </a>
                            </div>
                        </div>
                    </div>
                </Show>

                {/* Table */}
                <Show when={!loading() && (totalItems() > 0 || !anyLocked())}>
                    <Show
                        when={totalItems() > 0}
                        fallback={
                            <div class="px-4 py-8 text-center">
                                <div class="inline-flex flex-col items-center gap-2">
                                    <svg class="w-10 h-10 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                                    </svg>
                                    <p class="text-sm text-gray-500 dark:text-gray-400">
                                        No AI insights yet. The AI will learn patterns and surface intelligence over time.
                                    </p>
                                </div>
                            </div>
                        }
                    >
                        <div class="overflow-x-auto">
                            <table class="w-full text-sm">
                                <thead>
                                    <tr class="bg-gray-50 dark:bg-gray-800/80 text-gray-600 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                                        <th class="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider w-24">Type</th>
                                        <th class="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider">Details</th>
                                        <th class="px-4 py-2 text-right text-xs font-medium uppercase tracking-wider w-24">When</th>
                                    </tr>
                                </thead>
                                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                                    <For each={unifiedRows()}>
                                        {(row) => (
                                            <tr class="hover:bg-gray-50 dark:hover:bg-gray-800/40 transition-colors">
                                                <td class="px-4 py-2.5">
                                                    <span class={`inline-flex px-2 py-0.5 text-[10px] font-semibold rounded-full whitespace-nowrap ${row.typeBadgeClass}`}>
                                                        {row.typeBadge}
                                                    </span>
                                                </td>
                                                <td class="px-4 py-2.5">
                                                    <div class="flex flex-col gap-0.5">
                                                        <span class={`font-medium text-gray-800 dark:text-gray-200 ${row.badgeClass}`}>
                                                            {row.title}
                                                        </span>
                                                        <span class="text-xs text-gray-500 dark:text-gray-400 line-clamp-1">
                                                            {row.subtitle}
                                                        </span>
                                                    </div>
                                                    <Show when={row.confidence !== undefined}>
                                                        <span class="text-[10px] text-gray-400 dark:text-gray-500">
                                                            {Math.round(row.confidence! * 100)}% confidence
                                                        </span>
                                                    </Show>
                                                </td>
                                                <td class="px-4 py-2.5 text-right">
                                                    <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
                                                        {row.timestamp}
                                                    </span>
                                                </td>
                                            </tr>
                                        )}
                                    </For>
                                </tbody>
                            </table>
                        </div>
                    </Show>
                </Show>

                {/* Show more buttons (if any hidden) */}
                <Show when={!loading() && anyHidden()}>
                    <div class="px-4 py-2 border-t border-gray-100 dark:border-gray-700/50 bg-gray-50/50 dark:bg-gray-800/30">
                        <div class="flex flex-wrap items-center gap-3 text-xs">
                            <Show when={hiddenActions() > 0}>
                                <button
                                    class="text-emerald-600 dark:text-emerald-400 hover:underline"
                                    onClick={() => setShowAllActions(true)}
                                >
                                    + {hiddenActions()} more actions
                                </button>
                            </Show>
                            <Show when={hiddenChanges() > 0}>
                                <button
                                    class="text-teal-600 dark:text-teal-400 hover:underline"
                                    onClick={() => setShowAllChanges(true)}
                                >
                                    + {hiddenChanges()} more changes
                                </button>
                            </Show>
                        </div>
                    </div>
                </Show>

                {/* Section locked notices (partial lock) */}

                <Show when={!loading() && anyLocked() && !(insightsLocked() && impactLocked() && memoryLocked())}>
                    <div class="px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-amber-50/50 dark:bg-amber-900/10">
                        <div class="flex flex-wrap items-center gap-4 text-xs">
                            <Show when={insightsLocked()}>
                                <div class="flex items-center gap-1 text-amber-700 dark:text-amber-300">
                                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15V11m0 0V7m0 4h4m-4 0H8" />
                                    </svg>
                                    <span>{insightsLockedCount()} AI Insights locked</span>
                                </div>
                            </Show>
                            <Show when={impactLocked()}>
                                <div class="flex items-center gap-1 text-amber-700 dark:text-amber-300">
                                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                                    </svg>
                                    <span>Impact details locked</span>
                                </div>
                            </Show>
                            <Show when={memoryLocked()}>
                                <div class="flex items-center gap-1 text-amber-700 dark:text-amber-300">
                                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 4H7a2 2 0 01-2-2V7a2 2 0 012-2h5l5 5v9a2 2 0 01-2 2z" />
                                    </svg>
                                    <span>{memoryLockedCount()} changes locked</span>
                                </div>
                            </Show>
                            <a
                                class="ml-auto text-amber-800 dark:text-amber-200 font-medium underline"
                                href={upgradeUrl()}
                                target="_blank"
                                rel="noreferrer"
                            >
                                Upgrade to Pulse Pro
                            </a>
                        </div>
                    </div>
                </Show>
            </div>
        </Show>
    );
};
