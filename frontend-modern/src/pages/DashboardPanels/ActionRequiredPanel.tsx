import { For, Show, createSignal, createEffect, onCleanup, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { aiIntelligenceStore, type UnifiedFinding } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { AlertsAPI } from '@/api/alerts';
import {
  investigationOutcomeLabels,
  investigationOutcomeColors,
  type InvestigationOutcome,
} from '@/api/patrol';
import { hasFeature } from '@/stores/license';
import { formatRelativeTime } from '@/utils/format';
import { ALERTS_OVERVIEW_PATH, AI_PATROL_PATH } from '@/routing/resourceLinks';
import type { Alert } from '@/types/api';
import type { ApprovalRequest } from '@/api/ai';
import CheckIcon from 'lucide-solid/icons/check';
import XIcon from 'lucide-solid/icons/x';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

// ─── Shared severity badge ─────────────────────────────────────────
const severityBadgeClass = (level: 'critical' | 'warning' | string): string => {
  const base =
    'inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase';
  if (level === 'critical')
    return `${base} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300`;
  if (level === 'warning')
    return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`;
  return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300`;
};

// ─── Props ──────────────────────────────────────────────────────────
interface ActionRequiredPanelProps {
  pendingApprovals: ApprovalRequest[];
  unackedCriticalAlerts: Alert[];
  findingsNeedingAttention: UnifiedFinding[];
}

// ─── Pending Approvals sub-section ──────────────────────────────────
function PendingApprovalRows(props: { approvals: ApprovalRequest[] }) {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [tick, setTick] = createSignal(Date.now());

  createEffect(() => {
    if (props.approvals.length > 0) {
      const interval = setInterval(() => setTick(Date.now()), 1000);
      onCleanup(() => clearInterval(interval));
    }
  });

  const timeRemaining = (expiresAt: string) => {
    void tick();
    const diff = new Date(expiresAt).getTime() - Date.now();
    if (diff <= 0) return 'expired';
    const mins = Math.floor(diff / 60000);
    const secs = Math.floor((diff % 60000) / 1000);
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
  };

  const riskBadgeColor = (level: string) => {
    switch (level) {
      case 'high':
        return 'bg-red-200 text-red-800 dark:bg-red-900 dark:text-red-300';
      case 'medium':
        return 'bg-amber-200 text-amber-800 dark:bg-amber-900 dark:text-amber-300';
      default:
        return 'bg-green-200 text-green-800 dark:bg-green-900 dark:text-green-300';
    }
  };

  const handleApprove = async (approval: ApprovalRequest) => {
    setActionLoading(approval.id);
    try {
      const result = await aiIntelligenceStore.approveInvestigationFix(approval.id);
      if (result?.success) {
        notificationStore.success('Fix executed successfully');
      } else {
        notificationStore.error(result?.error || 'Fix execution failed');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeny = async (approval: ApprovalRequest) => {
    setActionLoading(approval.id);
    try {
      const success = await aiIntelligenceStore.denyInvestigationFix(approval.id);
      if (success) {
        notificationStore.success('Fix denied');
      } else {
        notificationStore.error('Failed to deny fix');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to deny fix');
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div class="space-y-1.5">
      <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">Pending Approvals</p>
      <ul class="space-y-1" role="list">
        <For each={props.approvals}>
          {(approval) => (
            <li class="flex items-center gap-2 py-1.5 px-2 -mx-2 rounded hover:bg-surface-hover transition-colors">
              <span
                class={`shrink-0 px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(approval.riskLevel)}`}
              >
                {approval.riskLevel}
              </span>
              <p class="min-w-0 text-xs text-base-content truncate flex-1" title={approval.context}>
                {approval.context || approval.command}
              </p>
              <span class="shrink-0 text-[10px] font-mono text-amber-600 dark:text-amber-400">
                {timeRemaining(approval.expiresAt)}
              </span>
              <div class="shrink-0 flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => handleApprove(approval)}
                  disabled={actionLoading() === approval.id}
                  class="flex items-center gap-1 px-2 py-1 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white text-[10px] font-medium rounded transition-colors"
                >
                  <Show
                    when={actionLoading() !== approval.id}
                    fallback={
                      <span class="h-3 w-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    }
                  >
                    <CheckIcon class="w-3 h-3" />
                  </Show>
                  Approve
                </button>
                <button
                  type="button"
                  onClick={() => handleDeny(approval)}
                  disabled={actionLoading() === approval.id}
                  class="flex items-center gap-1 px-2 py-1 bg-surface-alt hover:bg-surface-hover disabled:opacity-50 text-base-content text-[10px] font-medium rounded transition-colors"
                >
                  <XIcon class="w-3 h-3" />
                  Deny
                </button>
              </div>
            </li>
          )}
        </For>
      </ul>
    </div>
  );
}

// ─── Unacked Critical Alerts sub-section ────────────────────────────
function UnackedAlertRows(props: { alerts: Alert[] }) {
  const [ackLoading, setAckLoading] = createSignal<string | null>(null);
  const MAX_SHOWN = 5;

  const displayed = createMemo(() => props.alerts.slice(0, MAX_SHOWN));

  const handleAck = async (alert: Alert) => {
    setAckLoading(alert.id);
    try {
      await AlertsAPI.acknowledge(alert.id);
      notificationStore.success('Alert acknowledged');
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to acknowledge alert');
    } finally {
      setAckLoading(null);
    }
  };

  // Alert.type is the metric/threshold type (e.g. 'cpu', 'powered-off'), not resource type.
  // Link to alerts overview since we can't reliably determine resource page from alert data.
  const alertLink = () => ALERTS_OVERVIEW_PATH;

  return (
    <div class="space-y-1.5">
      <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">
        Unacknowledged Critical Alerts
      </p>
      <ul class="space-y-1" role="list">
        <For each={displayed()}>
          {(alert) => (
            <li class="flex items-center gap-2 py-1.5 px-2 -mx-2 rounded hover:bg-surface-hover transition-colors">
              <StatusDot variant="danger" size="sm" pulse />
              <a
                href={alertLink()}
                class="min-w-0 text-xs font-medium text-base-content truncate hover:underline"
              >
                {alert.resourceName}
              </a>
              <p class="min-w-0 text-xs text-muted truncate flex-1">{alert.message}</p>
              <span class="shrink-0 text-[10px] font-mono text-slate-400">
                {formatRelativeTime(alert.startTime, { compact: true })}
              </span>
              <button
                type="button"
                onClick={() => handleAck(alert)}
                disabled={ackLoading() === alert.id}
                class="shrink-0 px-2 py-1 text-[10px] font-medium text-base-content bg-surface-alt hover:bg-surface-hover disabled:opacity-50 rounded transition-colors"
              >
                {ackLoading() === alert.id ? 'Acking...' : 'Ack'}
              </button>
            </li>
          )}
        </For>
      </ul>
      <Show when={props.alerts.length > MAX_SHOWN}>
        <a
          href={ALERTS_OVERVIEW_PATH}
          class="text-[11px] text-blue-600 hover:underline dark:text-blue-400"
        >
          +{props.alerts.length - MAX_SHOWN} more
        </a>
      </Show>
    </div>
  );
}

// ─── Findings Needing Attention sub-section ─────────────────────────
function FindingsAttentionRows(props: { findings: UnifiedFinding[] }) {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);

  const handleReinvestigate = async (finding: UnifiedFinding) => {
    setActionLoading(finding.id);
    try {
      const { reinvestigateFinding } = await import('@/api/patrol');
      await reinvestigateFinding(finding.id);
      await aiIntelligenceStore.loadFindings();
      notificationStore.success('Re-investigation triggered');
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to reinvestigate');
    } finally {
      setActionLoading(null);
    }
  };

  const handleSnooze = async (finding: UnifiedFinding) => {
    setActionLoading(finding.id);
    try {
      const success = await aiIntelligenceStore.snoozeFinding(finding.id, 24);
      if (success) {
        notificationStore.success('Finding snoozed for 24h');
      } else {
        notificationStore.error('Failed to snooze finding');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to snooze finding');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDismiss = async (finding: UnifiedFinding) => {
    setActionLoading(finding.id);
    try {
      const success = await aiIntelligenceStore.dismissFinding(finding.id, 'not_an_issue');
      if (success) {
        notificationStore.success('Finding dismissed');
      } else {
        notificationStore.error('Failed to dismiss finding');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to dismiss finding');
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div class="space-y-1.5">
      <p class="text-[11px] font-semibold uppercase tracking-wide text-muted">
        Findings Needing Attention
      </p>
      <ul class="space-y-1" role="list">
        <For each={props.findings}>
          {(finding) => (
            <li class="flex items-center gap-2 py-1.5 px-2 -mx-2 rounded hover:bg-surface-hover transition-colors">
              <span class={severityBadgeClass(finding.severity)}>
                {finding.severity === 'critical'
                  ? 'CRIT'
                  : finding.severity === 'warning'
                    ? 'WARN'
                    : finding.severity.toUpperCase()}
              </span>
              <p
                class="min-w-0 text-xs font-medium text-base-content truncate flex-1"
                title={finding.title}
              >
                {finding.title}
              </p>
              <Show when={finding.investigationOutcome}>
                <span
                  class={`shrink-0 inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${
                    investigationOutcomeColors[
                      finding.investigationOutcome as InvestigationOutcome
                    ] || ''
                  }`}
                >
                  {investigationOutcomeLabels[
                    finding.investigationOutcome as InvestigationOutcome
                  ] || finding.investigationOutcome}
                </span>
              </Show>
              <div class="shrink-0 flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => handleReinvestigate(finding)}
                  disabled={actionLoading() === finding.id}
                  class="flex items-center gap-1 px-2 py-1 text-[10px] font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900 rounded transition-colors disabled:opacity-50"
                  title="Re-investigate this finding"
                >
                  <RefreshCwIcon
                    class={`w-3 h-3 ${actionLoading() === finding.id ? 'animate-spin' : ''}`}
                  />
                  Retry
                </button>
                <button
                  type="button"
                  onClick={() => handleSnooze(finding)}
                  disabled={actionLoading() === finding.id}
                  class="px-2 py-1 text-[10px] font-medium text-muted hover:bg-surface-hover rounded transition-colors disabled:opacity-50"
                  title="Snooze for 24 hours"
                >
                  Snooze
                </button>
                <button
                  type="button"
                  onClick={() => handleDismiss(finding)}
                  disabled={actionLoading() === finding.id}
                  class="px-2 py-1 text-[10px] font-medium text-muted hover:bg-surface-hover rounded transition-colors disabled:opacity-50"
                  title="Dismiss this finding"
                >
                  Dismiss
                </button>
              </div>
            </li>
          )}
        </For>
      </ul>
      <Show when={props.findings.length > 5}>
        <a
          href={AI_PATROL_PATH}
          class="text-[11px] text-blue-600 hover:underline dark:text-blue-400"
        >
          View all findings
        </a>
      </Show>
    </div>
  );
}

// ─── Main Panel ─────────────────────────────────────────────────────
export function ActionRequiredPanel(props: ActionRequiredPanelProps) {
  const hasPatrol = () => hasFeature('ai_patrol');
  const hasApprovals = () => hasPatrol() && props.pendingApprovals.length > 0;
  const hasAlerts = () => props.unackedCriticalAlerts.length > 0;
  const hasFindings = () => hasPatrol() && props.findingsNeedingAttention.length > 0;
  const hasAny = () => hasApprovals() || hasAlerts() || hasFindings();

  return (
    <Show when={hasAny()}>
      <Card padding="md" tone="default" class="border-amber-200 dark:border-amber-800">
        <div class="flex items-center gap-2 mb-3">
          <AlertTriangleIcon class="w-4 h-4 text-amber-500 dark:text-amber-400" />
          <h2 class="text-sm font-semibold text-base-content">Action Required</h2>
        </div>

        <div class="space-y-4">
          <Show when={hasApprovals()}>
            <PendingApprovalRows approvals={props.pendingApprovals} />
          </Show>

          <Show when={hasAlerts()}>
            <UnackedAlertRows alerts={props.unackedCriticalAlerts} />
          </Show>

          <Show when={hasFindings()}>
            <FindingsAttentionRows findings={props.findingsNeedingAttention} />
          </Show>
        </div>
      </Card>
    </Show>
  );
}

export default ActionRequiredPanel;
