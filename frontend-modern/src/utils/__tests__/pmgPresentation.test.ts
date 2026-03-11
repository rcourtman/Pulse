import { describe, expect, it } from 'vitest';
import {
  getPMGDetailsDrawerPresentation,
  getPMGDisconnectedState,
  getPMGSearchEmptyState,
  PMG_DETAILS_FAILURE_STATE_TITLE,
  PMG_DETAILS_LOADING_STATE_DESCRIPTION,
  PMG_DETAILS_LOADING_STATE_TITLE,
  PMG_DETAILS_EMPTY_STATE_DESCRIPTION,
  PMG_DETAILS_EMPTY_STATE_TITLE,
  PMG_DISCONNECTED_STATE_TITLE,
  PMG_EMPTY_STATE_DESCRIPTION,
  PMG_EMPTY_STATE_TITLE,
  PMG_LOADING_STATE_DESCRIPTION,
  PMG_LOADING_STATE_TITLE,
  PMG_SEARCH_PLACEHOLDER,
} from '../pmgPresentation';

describe('pmgPresentation', () => {
  it('exports canonical PMG details drawer presentation', () => {
    expect(getPMGDetailsDrawerPresentation()).toEqual({
      defaultResourceName: 'Mail Gateway',
      unknownHostLabel: 'Unknown host',
      updatedPrefix: 'Updated',
      nodesSectionTitle: 'Nodes',
      relayDomainsSectionTitle: 'Relay Domains',
      domainStatsSectionTitle: 'Domain Stats',
      spamDistributionSectionTitle: 'Spam Distribution',
      asOfPrefix: 'As of',
      domainSearchPlaceholder: 'Search domains...',
      nodeColumnLabel: 'Node',
      roleColumnLabel: 'Role',
      statusColumnLabel: 'Status',
      queueColumnLabel: 'Queue',
      domainColumnLabel: 'Domain',
      commentColumnLabel: 'Comment',
      mailColumnLabel: 'Mail',
      spamColumnLabel: 'Spam',
      virusColumnLabel: 'Virus',
      bytesColumnLabel: 'Bytes',
    });
  });

  it('exports canonical PMG empty-state copy', () => {
    expect(PMG_EMPTY_STATE_TITLE).toBe('No Mail Gateways configured');
    expect(PMG_EMPTY_STATE_DESCRIPTION).toContain('Add a Proxmox Mail Gateway');
    expect(PMG_LOADING_STATE_TITLE).toBe('Loading mail gateway data...');
    expect(PMG_LOADING_STATE_DESCRIPTION).toBe('Connecting to the monitoring service.');
    expect(PMG_DISCONNECTED_STATE_TITLE).toBe('Connection lost');
    expect(PMG_SEARCH_PLACEHOLDER).toBe('Search gateways...');
    expect(PMG_DETAILS_EMPTY_STATE_TITLE).toBe('No PMG details for this resource yet');
    expect(PMG_DETAILS_EMPTY_STATE_DESCRIPTION).toContain("Pulse hasn't ingested PMG analytics");
    expect(PMG_DETAILS_LOADING_STATE_TITLE).toBe('Loading mail gateway details...');
    expect(PMG_DETAILS_LOADING_STATE_DESCRIPTION).toBe('Fetching PMG resource details.');
    expect(PMG_DETAILS_FAILURE_STATE_TITLE).toBe('Failed to load PMG details');
  });

  it('exports canonical PMG disconnected and search state helpers', () => {
    expect(getPMGDisconnectedState(true)).toEqual({
      title: 'Connection lost',
      description: 'Attempting to reconnect…',
      actionLabel: undefined,
    });
    expect(getPMGDisconnectedState(false)).toEqual({
      title: 'Connection lost',
      description: 'Unable to connect to the backend server',
      actionLabel: 'Reconnect now',
    });
    expect(getPMGSearchEmptyState('edge')).toEqual({
      description: 'No gateways match "edge"',
      actionLabel: 'Clear search',
    });
  });
});
