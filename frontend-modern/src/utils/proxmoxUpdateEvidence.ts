import type { Node } from '@/types/api';
import type { HelpContentId } from '@/content/help';
import { formatRelativeTime } from '@/utils/format';

export interface ProxmoxUpdateEvidencePresentation {
  value: string;
  title: string;
  tone: 'success' | 'warning' | 'default';
  current: boolean;
  helpContentId?: HelpContentId;
}

type UpdateEvidenceNode = Pick<
  Node,
  'pendingUpdates' | 'pendingUpdatesCheckedAt' | 'pendingUpdatesStatus' | 'pendingUpdatesReason'
>;

const reasonLabel = (reason: Node['pendingUpdatesReason']): string => {
  switch (reason) {
    case 'permission_denied':
      return 'Update check access denied';
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
  const accessHelp =
    node.pendingUpdatesReason === 'permission_denied'
      ? { helpContentId: 'updates.proxmox.access' }
      : {};
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
        ...accessHelp,
      };
    }
    case 'unavailable':
      return {
        value: ['Unavailable', reason].filter(Boolean).join(' · '),
        title: reason || 'Update evidence is unavailable',
        // Proxmox requires Sys.Modify for the package-update list, which is
        // broader than monitoring needs; docs/TROUBLESHOOTING.md calls keeping
        // the narrower role a valid choice. That is a setting, not a fault, so
        // it reads neutral. A source outage or failed check stays a warning.
        tone: node.pendingUpdatesReason === 'permission_denied' ? 'default' : 'warning',
        current: false,
        ...accessHelp,
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
