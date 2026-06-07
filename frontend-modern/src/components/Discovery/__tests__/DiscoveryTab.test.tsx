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

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getSettings: vi.fn(async () => ({ discovery_enabled: true })),
  },
}));

import { AIAPI } from '@/api/ai';
import * as discoveryApi from '@/api/discovery';
import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';
import { copyToClipboard } from '@/utils/clipboard';
import { getDiscoveryProvenanceTitle } from '@/utils/discoveryPresentation';

const aiSettingsWithDiscovery = (discovery_enabled: boolean) =>
  ({ discovery_enabled }) as Awaited<ReturnType<typeof AIAPI.getSettings>>;

// A configured analysis provider — the most-fundamental discovery prerequisite.
// Command/connectivity guidance only surfaces once a provider exists, so tests
// that exercise those later states must establish this first.
const discoveryInfoWithProvider = () => ({
  ai_provider: {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    is_local: false,
    label: 'Cloud (Anthropic)',
  },
  commands: [],
  command_categories: [],
});

describe('DiscoveryTab', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.mocked(AIAPI.getSettings).mockResolvedValue(aiSettingsWithDiscovery(true));
  });

  it('keeps run action visible while discovery lookup is still loading', async () => {
    render(() => (
      <DiscoveryTab resourceType="agent" agentId="agent-1" resourceId="agent-1" hostname="pve1" />
    ));

    expect(await screen.findByRole('button', { name: 'Run Discovery Now' })).toBeInTheDocument();
  });

  it('does not fetch discovery data when AI discovery is disabled', async () => {
    vi.mocked(AIAPI.getSettings).mockResolvedValue(aiSettingsWithDiscovery(false));

    render(() => (
      <DiscoveryTab resourceType="agent" agentId="agent-1" resourceId="agent-1" hostname="pve1" />
    ));

    expect(await screen.findByText('AI Discovery Disabled')).toBeInTheDocument();
    expect(discoveryApi.getDiscovery).not.toHaveBeenCalled();
    expect(discoveryApi.getDiscoveryInfo).not.toHaveBeenCalled();
    expect(discoveryApi.getConnectedAgents).not.toHaveBeenCalled();
  });

  it('treats placeholder-only discovery records as unidentified instead of valid results', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue({
      id: 'system-container:pve4:152',
      resource_type: 'system-container',
      resource_id: '152',
      target_id: 'pve4',
      hostname: 'smtp-relay-32',
      service_type: 'unknown',
      service_name: 'Unknown Container',
      service_version: 'unknown',
      category: 'unknown',
      cli_access: 'pct exec 152 -- /bin/bash',
      facts: [
        {
          category: 'service',
          key: 'status',
          value: 'online',
          source: 'metadata',
          confidence: 1,
          discovered_at: '2026-05-19T00:00:00Z',
        },
        {
          category: 'config',
          key: 'config_availability',
          value: 'missing_node_config',
          source: 'all_commands',
          confidence: 1,
          discovered_at: '2026-05-19T00:00:00Z',
        },
      ],
      config_paths: [],
      data_paths: [],
      log_paths: [],
      ports: [],
      user_notes: '',
      user_secrets: {},
      confidence: 0,
      ai_reasoning: '',
      discovered_at: '2026-05-19T00:00:00Z',
      updated_at: '2026-05-19T00:00:00Z',
      scan_duration: 0,
      suggested_url_diagnostic: 'no host or IP candidate available',
    });

    render(() => (
      <DiscoveryTab
        resourceType="system-container"
        agentId="pve4"
        resourceId="152"
        hostname="smtp-relay-32"
      />
    ));

    expect(await screen.findByText('Unknown Service')).toBeInTheDocument();
    expect(screen.queryByText('Unknown Container')).toBeNull();
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

    const runButton = await screen.findByRole('button', { name: 'Run Discovery' });
    await waitFor(() => expect(runButton).not.toBeDisabled());
    fireEvent.click(runButton);

    await waitFor(() => {
      expect(discoveryApi.triggerDiscovery).toHaveBeenCalledWith('agent', 'agent-1', 'agent-1', {
        force: true,
        hostname: 'pve1',
      });
    });
  });

  it('surfaces the AI-provider prerequisite before command guidance', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);
    // Default getDiscoveryInfo mock returns null → no provider configured.

    render(() => (
      <DiscoveryTab
        resourceType="agent"
        agentId="agent-1"
        resourceId="agent-1"
        hostname="pve1"
        commandsEnabled={false}
      />
    ));

    expect(await screen.findByText('AI provider not configured')).toBeInTheDocument();
    expect(screen.queryByText('Commands not enabled')).not.toBeInTheDocument();
  });

  it('surfaces the AI-provider prerequisite tab-wide for non-agent resources', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);
    // Default getDiscoveryInfo mock returns null → no provider configured. The
    // provider gate previously only rendered inside the agent-only trio, so a
    // VM/container tab gave no hint Discovery could not run. It is now tab-wide.

    render(() => (
      <DiscoveryTab
        resourceType="system-container"
        agentId="pve4"
        resourceId="152"
        hostname="smtp-relay-32"
      />
    ));

    expect(await screen.findByText('AI provider not configured')).toBeInTheDocument();
  });

  it('uses canonical settings copy for disabled command guidance', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);
    vi.mocked(discoveryApi.getDiscoveryInfo).mockResolvedValue(discoveryInfoWithProvider());

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
    vi.mocked(discoveryApi.getDiscoveryInfo).mockResolvedValue(discoveryInfoWithProvider());

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
    expect(screen.getAllByLabelText(getDiscoveryProvenanceTitle()).length).toBeGreaterThan(1);
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
