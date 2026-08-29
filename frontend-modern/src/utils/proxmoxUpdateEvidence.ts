import type { Node } from '@/types/api';
import { formatRelativeTime } from '@/utils/format';

export interface ProxmoxUpdateEvidencePresentation {
  value: string;
  title: string;
  tone: 'success' | 'warning' | 'default';
  current: boolean;
}

type UpdateEvidenceNode = Pick<
  Node,
  'pendingUpdates' | 'pendingUpdatesCheckedAt' | 'pendingUpdatesStatus' | 'pendingUpdatesReason'
>;

const reasonLabel = (reason: Node['pendingUpdatesReason']): string => {
  switch (reason) {
    case 'permission_denied':
      return 'Sys.Audit permission required';
    case 'source_unavailable':
      return 'Proxmox source unavailable';
    case 'check_failed':
      return 'Latest check failed';
    case 'node_offline':
      return 'Node offline';
    default:
      return '';
  }
};

const countLabel = (count: number): string =>
  count === 0 ? 'No pending updates' : `${count} pending`;

export const getProxmoxUpdateEvidencePresentation = (
  node: UpdateEvidenceNode,
  formatCheckedAt: (value: string) => string = (value) => formatRelativeTime(value),
): ProxmoxUpdateEvidencePresentation | null => {
  const count = Math.max(0, Math.trunc(node.pendingUpdates ?? 0));
  const checkedAt = node.pendingUpdatesCheckedAt
    ? formatCheckedAt(node.pendingUpdatesCheckedAt)
    : '';
  const reason = reasonLabel(node.pendingUpdatesReason);
  const status =
    node.pendingUpdatesStatus ||
    (node.pendingUpdatesCheckedAt || (typeof node.pendingUpdates === 'number' && count > 0)
      ? 'checked'
      : '');

  switch (status) {
    case 'checked': {
      const value = [countLabel(count), checkedAt ? `checked ${checkedAt}` : '']
        .filter(Boolean)
        .join(' · ');
      return {
        value,
        title: checkedAt ? `Last successful update check ${checkedAt}` : 'Update check completed',
        tone: count > 0 ? 'warning' : 'success',
        current: true,
      };
    }
    case 'stale': {
      const value = [countLabel(count), 'stale', checkedAt ? `checked ${checkedAt}` : '']
        .filter(Boolean)
        .join(' · ');
      return {
        value,
        title: [
          reason || 'Latest update check failed',
          checkedAt ? `last success ${checkedAt}` : '',
        ]
          .filter(Boolean)
          .join(' · '),
        tone: 'warning',
        current: false,
      };
    }
    case 'unavailable':
      return {
        value: ['Unavailable', reason].filter(Boolean).join(' · '),
        title: reason || 'Update evidence is unavailable',
        tone: 'warning',
        current: false,
      };
    case 'not_checked':
      return {
        value: ['Not checked', reason].filter(Boolean).join(' · '),
        title: reason || 'Update evidence has not been checked',
        tone: 'default',
        current: false,
      };
    default:
      return null;
  }
};

export const hasCurrentProxmoxUpdateEvidence = (node: UpdateEvidenceNode): boolean => {
  if (node.pendingUpdatesStatus) return node.pendingUpdatesStatus === 'checked';
  return Boolean(node.pendingUpdatesCheckedAt) || (node.pendingUpdates ?? 0) > 0;
};
