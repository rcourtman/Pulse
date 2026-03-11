import { describe, expect, it } from 'vitest';
import {
  getAgentLedgerErrorState,
  getAgentLedgerLoadingState,
  getUnifiedAgentAllowReconnectErrorMessage,
  getUnifiedAgentAllowReconnectSuccessMessage,
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
  getUnifiedAgentCommandsUnavailableMessage,
  getUnifiedAgentConfigUpdateErrorMessage,
  getUnifiedAgentConfigUpdateSuccessMessage,
  getInventorySubjectLabel,
  getRemovedUnifiedAgentItemLabel,
  getUnifiedAgentStopMonitoringErrorMessage,
  getUnifiedAgentStopMonitoringSuccessMessage,
  getUnifiedAgentStopMonitoringUnavailableMessage,
  getUnifiedAgentUninstallCommandCopiedMessage,
  getUnifiedAgentUpgradeCommandCopiedMessage,
  getUnifiedAgentLastSeenLabel,
} from '@/utils/unifiedAgentInventoryPresentation';

describe('unifiedAgentInventoryPresentation', () => {
  it('returns canonical inventory subject labels', () => {
    expect(getInventorySubjectLabel('tower', 'fallback')).toBe('tower');
    expect(getInventorySubjectLabel('', 'fallback')).toBe('fallback');
    expect(getInventorySubjectLabel()).toBe('this host');
  });

  it('returns canonical removed-item labels from capability mix', () => {
    expect(getRemovedUnifiedAgentItemLabel({ capabilities: ['kubernetes'] })).toBe(
      'Kubernetes cluster',
    );
    expect(getRemovedUnifiedAgentItemLabel({ capabilities: ['docker'] })).toBe('Docker runtime');
    expect(getRemovedUnifiedAgentItemLabel({ capabilities: ['proxmox'] })).toBe('Proxmox node');
    expect(getRemovedUnifiedAgentItemLabel({ capabilities: ['agent'] })).toBe('Host agent');
  });

  it('returns canonical last-seen labels', () => {
    expect(
      getUnifiedAgentLastSeenLabel({ status: 'removed' }, 'Monitoring stopped'),
    ).toBe('Monitoring stopped');
    expect(getUnifiedAgentLastSeenLabel({ status: 'active' }, 'Monitoring stopped')).toBe('—');
  });

  it('returns canonical agent ledger loading and error copy', () => {
    expect(getAgentLedgerLoadingState()).toEqual({
      text: 'Loading agent ledger...',
    });
    expect(getAgentLedgerErrorState()).toEqual({
      title: 'Failed to load agent ledger.',
      retryingLabel: 'Retrying…',
      retryLabel: 'Retry',
    });
  });

  it('returns canonical unified-agent operational action copy', () => {
    expect(getUnifiedAgentStopMonitoringUnavailableMessage()).toBe(
      'No host identifiers are available to stop monitoring.',
    );
    expect(getUnifiedAgentStopMonitoringSuccessMessage('tower')).toBe(
      'Monitoring stopped for tower. Pulse will ignore future reports until reconnect is allowed.',
    );
    expect(getUnifiedAgentStopMonitoringErrorMessage('tower')).toBe(
      'Failed to stop monitoring tower.',
    );
    expect(getUnifiedAgentAllowReconnectSuccessMessage('tower')).toBe(
      'Reconnect allowed for tower. Pulse will accept reports from it again.',
    );
    expect(getUnifiedAgentAllowReconnectErrorMessage('tower')).toBe(
      'Failed to allow reconnect for tower.',
    );
    expect(getUnifiedAgentCommandsUnavailableMessage()).toBe(
      'Agent ID unavailable for command configuration',
    );
    expect(getUnifiedAgentConfigUpdateSuccessMessage(true)).toBe(
      'Pulse command execution enabled. Syncing with agent...',
    );
    expect(getUnifiedAgentConfigUpdateSuccessMessage(false)).toBe(
      'Pulse command execution disabled. Syncing with agent...',
    );
    expect(getUnifiedAgentConfigUpdateErrorMessage()).toBe(
      'Failed to update agent configuration',
    );
    expect(getUnifiedAgentClipboardCopySuccessMessage()).toBe('Copied to clipboard');
    expect(getUnifiedAgentClipboardCopyErrorMessage()).toBe('Failed to copy');
    expect(getUnifiedAgentUninstallCommandCopiedMessage()).toBe('Uninstall command copied');
    expect(getUnifiedAgentUpgradeCommandCopiedMessage()).toBe('Upgrade command copied');
  });
});
