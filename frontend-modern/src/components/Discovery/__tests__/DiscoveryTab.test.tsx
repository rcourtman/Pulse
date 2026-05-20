import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ResourceDiscovery } from '@/types/discovery';

vi.mock('@/api/discovery', () => {
  const never = new Promise<null>(() => undefined);
  return {
    getDiscovery: vi.fn(() => never),
    getDiscoveryInfo: vi.fn(async () => null),
    triggerDiscovery: vi.fn(),
    updateDiscoveryNotes: vi.fn(),
    formatDiscoveryAge: vi.fn(() => 'just now'),
    getCategoryDisplayName: vi.fn((category: string) => category),
    getConfidenceLevel: vi.fn(() => ({ label: 'Low confidence', color: 'text-slate-500' })),
    getConnectedAgents: vi.fn(async () => ({ count: 0, agents: [] })),
  };
});

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn(async () => true),
}));

import * as discoveryApi from '@/api/discovery';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { copyToClipboard } from '@/utils/clipboard';

describe('DiscoveryTab', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('keeps run action visible while discovery lookup is still loading', async () => {
    render(() => (
      <DiscoveryTab resourceType="agent" agentId="agent-1" resourceId="agent-1" hostname="pve1" />
    ));

    expect(await screen.findByRole('button', { name: 'Run Discovery Now' })).toBeInTheDocument();
  });

  it('shows a drawer run action and triggers discovery for the current resource', async () => {
    const discovered: ResourceDiscovery = {
      id: 'discovery-1',
      resource_type: 'agent',
      resource_id: 'agent-1',
      target_id: 'agent-1',
      agent_id: 'agent-1',
      hostname: 'pve1',
      service_type: 'database',
      service_name: 'postgresql',
      service_version: '16.1',
      category: 'database',
      cli_access: 'psql',
      facts: [],
      config_paths: [],
      data_paths: [],
      log_paths: [],
      ports: [],
      user_notes: '',
      user_secrets: {},
      confidence: 0.72,
      ai_reasoning: '',
      discovered_at: '2026-04-15T00:00:00Z',
      updated_at: '2026-04-15T00:00:00Z',
      scan_duration: 12,
    };
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(discovered);
    vi.mocked(discoveryApi.triggerDiscovery).mockResolvedValue(discovered);

    render(() => (
      <DiscoveryTab
        resourceType="agent"
        agentId="agent-1"
        resourceId="agent-1"
        hostname="pve1"
        showManualRunAction
      />
    ));

    fireEvent.click(await screen.findByRole('button', { name: 'Run Discovery' }));

    await waitFor(() => {
      expect(discoveryApi.triggerDiscovery).toHaveBeenCalledWith('agent', 'agent-1', 'agent-1', {
        force: true,
        hostname: 'pve1',
      });
    });
  });

  it('uses canonical settings copy for disabled command guidance', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);

    render(() => (
      <DiscoveryTab
        resourceType="agent"
        agentId="agent-1"
        resourceId="agent-1"
        hostname="pve1"
        commandsEnabled={false}
      />
    ));

    expect(await screen.findByText('Commands not enabled')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Settings → Infrastructure' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
    expect(screen.queryByText('Settings → Unified Agents')).not.toBeInTheDocument();
  });

  it('uses canonical API Access copy for missing command connection guidance', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);

    render(() => (
      <DiscoveryTab
        resourceType="agent"
        agentId="agent-1"
        resourceId="agent-1"
        hostname="pve1"
        commandsEnabled={true}
      />
    ));

    expect(await screen.findByText('Agent not connected for commands')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Settings → API Access' })).toHaveAttribute(
      'href',
      '/settings/security/api',
    );
    expect(screen.queryByText('Settings → API Tokens')).not.toBeInTheDocument();
  });

  it('renders task-first analysis copy for provider and reasoning context', async () => {
    vi.mocked(discoveryApi.getDiscoveryInfo).mockResolvedValue({
      ai_provider: {
        provider: 'anthropic',
        model: 'claude-haiku-4-5',
        is_local: false,
        label: 'Cloud (Anthropic)',
      },
      commands: [],
      command_categories: [],
    });
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue({
      id: 'discovery-1',
      resource_type: 'agent',
      resource_id: 'agent-1',
      target_id: 'agent-1',
      agent_id: 'agent-1',
      hostname: 'pve1',
      service_type: 'database',
      service_name: 'postgresql',
      service_version: '16.1',
      category: 'database',
      cli_access: 'psql',
      facts: [],
      config_paths: [],
      data_paths: [],
      log_paths: [],
      ports: [],
      user_notes: '',
      user_secrets: {},
      confidence: 0.72,
      ai_reasoning: 'Mapped open ports and running processes to a PostgreSQL service.',
      discovered_at: '2026-04-15T00:00:00Z',
      updated_at: '2026-04-15T00:00:00Z',
      scan_duration: 12,
      suggested_url: 'http://192.0.2.10:5432',
      suggested_url_source_code: 'web_port_inference',
      suggested_url_source_detail: 'detected 5432/tcp',
    });

    render(() => (
      <DiscoveryTab resourceType="agent" agentId="agent-1" resourceId="agent-1" hostname="pve1" />
    ));

    expect(await screen.findByText('Analysis: Cloud (Anthropic)')).toBeInTheDocument();
    expect(await screen.findByText('Observed by Discovery')).toBeInTheDocument();
    expect(await screen.findByText('Available to Pulse Assistant')).toBeInTheDocument();
    expect(await screen.findByText('Analysis Reasoning')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open suggested URL' })).toHaveAttribute(
      'href',
      'http://192.0.2.10:5432',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Copy suggested URL' }));
    await waitFor(() => {
      expect(copyToClipboard).toHaveBeenCalledWith('http://192.0.2.10:5432');
    });
    expect(
      await screen.findByText('Mapped open ports and running processes to a PostgreSQL service.'),
    ).toBeInTheDocument();
  });
});
