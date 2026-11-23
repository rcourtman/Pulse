import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { AgentCapabilitiesManifest } from '@/api/agentCapabilities';
import {
  API_TOKEN_CREATE_ANCHOR,
  API_TOKEN_PRESET_QUERY_PARAM,
  EXTERNAL_AGENT_SETUP_ANCHOR,
  PATROL_CONTROL_PATH,
  PULSE_MCP_SETUP_ANCHOR,
  PULSE_MCP_TOKEN_SETUP_PATH,
  SETTINGS_API_ACCESS_PATH,
} from '@/routing/resourceLinks';
import { AgentIntegrationsPanel } from '../AgentIntegrationsPanel';

const mocks = vi.hoisted(() => ({
  fetchAgentCapabilitiesManifest: vi.fn(),
}));
const scrollIntoViewMock = vi.fn();

vi.mock('@/api/agentCapabilities', async () => {
  const actual =
    await vi.importActual<typeof import('@/api/agentCapabilities')>('@/api/agentCapabilities');
  return {
    ...actual,
    fetchAgentCapabilitiesManifest: (...args: unknown[]) =>
      mocks.fetchAgentCapabilitiesManifest(...args),
  };
});

const manifest: AgentCapabilitiesManifest = {
  version: 'v1',
  surfaceContract: {
    core: {
      id: 'pulse_intelligence_core',
      label: 'Pulse Intelligence Core',
      description:
        'Canonical context, governed actions, safety gates, approval state, action audit, and verification.',
    },
    proactiveEngine: {
      id: 'pulse_patrol',
      label: 'Pulse Patrol',
      description: 'The proactive detection and investigation engine.',
    },
    operatorSurfaces: [
      {
        id: 'pulse_assistant',
        label: 'Pulse Assistant',
        description: 'The first-party in-app surface.',
        native: true,
        externalAdapter: false,
        affordances: {
          tools: true,
        },
      },
      {
        id: 'pulse_mcp',
        label: 'Pulse MCP',
        description: 'The external-agent adapter.',
        native: false,
        externalAdapter: true,
        affordances: {
          tools: true,
          resources: true,
          prompts: true,
          capabilityMetadata: true,
        },
      },
    ],
  },
  surfaceToolContracts: [
    {
      surfaceId: 'pulse_mcp',
      surfaceLabel: 'Pulse MCP',
      toolSource: 'capability_manifest',
      toolNames: ['get_resource_context'],
      capabilityNames: ['get_resource_context'],
      affordances: {
        tools: true,
        resources: true,
        prompts: true,
        capabilityMetadata: true,
      },
    },
  ],
  mcpAdapter: {
    serverName: 'pulse-test',
    command: 'pulse-test-mcp',
    baseUrlFlag: '--pulse-url',
    defaultBaseUrl: 'https://pulse.test',
    tokenEnv: 'PULSE_TEST_TOKEN',
    configFamilies: [
      {
        id: 'opencode',
        label: 'Test OpenCode',
        shape: 'opencode_mcp',
        clientLabels: ['OpenCode'],
      },
      {
        id: 'claude-style',
        label: 'Test MCP servers',
        shape: 'mcp_servers',
        clientLabels: ['Claude Desktop', 'Claude Code'],
      },
      {
        id: 'custom-mcp',
        label: 'custom clients',
        shape: 'custom',
        clientLabels: ['custom clients'],
      },
    ],
  },
  requiredScopes: [
    'monitoring:read',
    'monitoring:write',
    'settings:read',
    'settings:write',
    'ai:execute',
  ],
  categories: [
    {
      id: 'context',
      label: 'Context (read-only)',
      description: 'Discovery and read-only situated reads. Agents start here.',
    },
  ],
  workflowPrompts: [
    {
      name: 'pulse_operations_loop',
      label: 'Ask Patrol to handle an issue',
      presentationKind: 'workflow',
      description:
        'Have Patrol investigate active findings, follow the configured Patrol mode, take approved actions, verify the outcome, and record what happened.',
    },
    {
      name: 'pulse_investigate_resource',
      label: 'Investigate resource',
      presentationKind: 'resource',
      description: 'Investigate one resource.',
      arguments: [
        {
          name: 'resourceId',
          description: 'Canonical resource id.',
          required: true,
        },
      ],
    },
  ],
  capabilities: [
    {
      name: 'get_fleet_context',
      description: 'Return a thin per-resource triage rollup.',
      category: 'context',
      method: 'GET',
      path: '/api/agent/fleet-context',
      scope: 'monitoring:read',
      actionMode: 'read',
      approvalPolicy: 'scope_only',
      errorCodes: ['resource_not_found'],
    },
    {
      name: 'subscribe_events',
      description: 'Subscribe to Pulse Intelligence events.',
      category: 'events',
      method: 'GET',
      path: '/api/agent/events',
      scope: 'monitoring:read',
      actionMode: 'read',
      approvalPolicy: 'scope_only',
    },
  ],
};

const renderPanel = () =>
  render(() => (
    <Router>
      <Route path="/" component={() => <AgentIntegrationsPanel />} />
    </Router>
  ));

describe('AgentIntegrationsPanel', () => {
  beforeEach(() => {
    mocks.fetchAgentCapabilitiesManifest.mockReset();
    mocks.fetchAgentCapabilitiesManifest.mockResolvedValue(manifest);
    scrollIntoViewMock.mockReset();
    Element.prototype.scrollIntoView = scrollIntoViewMock;
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('leads immediately with un-collapsed 3-step pulse CLI onboarding, client switcher, and zero-token manual config', async () => {
    renderPanel();

    expect(
      (await screen.findByRole('heading', { name: 'External agents' })).closest(
        `#${EXTERNAL_AGENT_SETUP_ANCHOR}`,
      ),
    ).not.toBeNull();

    // No outer toggle hiding the 3-step setup, and no curl | bash or irm | iex snippets.
    expect(screen.queryByRole('button', { name: 'Show connector setup' })).not.toBeInTheDocument();
    expect(document.body.textContent).not.toContain('curl -fsSL');
    expect(document.body.textContent).not.toContain('| bash');
    expect(document.body.textContent).not.toContain('| iex');
    expect(document.body.textContent).not.toContain('<your-api-token>');
    expect(document.body.textContent).not.toContain('Patrol external agent');

    // Step 1: Install pulse CLI (Homebrew default + verified installers).
    expect(screen.getByText('Step 1')).toBeInTheDocument();
    expect(screen.getByText('brew install rcourtman/tap/pulse')).toBeInTheDocument();

    const unixButton = screen.getByRole('button', { name: 'macOS / Linux (verified)' });
    fireEvent.click(unixButton);
    expect(unixButton).toHaveAttribute('aria-pressed', 'true');
    expect(document.body.textContent).toContain('gh release verify-asset');
    expect(document.body.textContent).toContain('install-cli.sh');

    const windowsButton = screen.getByRole('button', { name: 'Windows (verified)' });
    fireEvent.click(windowsButton);
    expect(windowsButton).toHaveAttribute('aria-pressed', 'true');
    expect(document.body.textContent).toContain('install-cli.ps1');

    // Step 2: Sign in read-only with keychain storage & read-only preset link.
    expect(screen.getByText('Step 2')).toBeInTheDocument();
    expect(screen.getByText(`pulse auth login ${window.location.origin}`)).toBeInTheDocument();
    expect(screen.getByText('External agent (read-only)')).toBeInTheDocument();
    const createTokenLink = screen.getByRole('link', { name: 'Create token' });
    expect(createTokenLink).toHaveAttribute('href', PULSE_MCP_TOKEN_SETUP_PATH);

    // Step 3: Connect agent with quick switcher for Claude Code, Codex, OpenCode, Claude Desktop.
    expect(screen.getByText('Step 3')).toBeInTheDocument();
    const claudeCodeBtn = screen.getByRole('button', { name: 'Claude Code' });
    expect(claudeCodeBtn).toHaveAttribute('aria-pressed', 'true');
    expect(document.body.textContent).toContain('pulse connect claude-code');
    expect(document.body.textContent).toContain('pulse doctor');
    expect(document.body.textContent).toContain('~/.claude.json');

    fireEvent.click(screen.getByRole('button', { name: 'Codex' }));
    expect(screen.getByRole('button', { name: 'Codex' })).toHaveAttribute('aria-pressed', 'true');
    expect(document.body.textContent).toContain('pulse connect codex');
    expect(document.body.textContent).toContain('~/.codex/config.toml');

    fireEvent.click(screen.getByRole('button', { name: 'OpenCode' }));
    expect(document.body.textContent).toContain('pulse connect opencode');
    expect(document.body.textContent).toContain('~/.config/opencode/opencode.json');

    fireEvent.click(screen.getByRole('button', { name: 'Claude Desktop' }));
    expect(document.body.textContent).toContain('pulse connect claude-desktop');
    expect(document.body.textContent).toContain('claude_desktop_config.json');

    // Secondary section: Want agents to propose fixes? (demoted Patrol mode + propose token).
    expect(
      screen.getByRole('heading', { name: 'Want agents to propose fixes?' }),
    ).toBeInTheDocument();
    expect(screen.getByText('External agent (propose fixes)')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Create propose token' })).toHaveAttribute(
      'href',
      `${SETTINGS_API_ACCESS_PATH}?${API_TOKEN_PRESET_QUERY_PARAM}=external-agent-propose#${API_TOKEN_CREATE_ANCHOR}`,
    );
    expect(screen.getByRole('link', { name: 'Choose Patrol mode' })).toHaveAttribute(
      'href',
      PATROL_CONTROL_PATH,
    );

    // Manual config disclosure (closed by default; zero plaintext tokens, includes Codex TOML).
    const manualConfig = screen.getByText('Manual config').closest('details');
    expect(manualConfig).not.toBeNull();
    expect(manualConfig).not.toHaveAttribute('open');
    expect(document.body.textContent).not.toContain('[mcp_servers.pulse-test]');

    manualConfig!.open = true;
    fireEvent(manualConfig!, new Event('toggle'));

    expect(document.body.textContent).toContain('"command": "pulse"');
    expect(document.body.textContent).toContain('"mcp"');
    expect(document.body.textContent).toContain('[mcp_servers.pulse-test]');
    expect(document.body.textContent).toContain('command = "pulse"');
    expect(document.body.textContent).toContain('args = ["mcp"]');
    expect(document.body.textContent).not.toContain('PULSE_API_TOKEN');
    expect(document.body.textContent).not.toContain('PULSE_TEST_TOKEN');
    expect(document.body.textContent).not.toContain('<your-api-token>');

    // Developer details disclosure (closed by default, preserves manifest inspection).
    const advancedDetails = screen.getByText('Developer details').closest('details');
    expect(advancedDetails).not.toBeNull();
    expect(advancedDetails).not.toHaveAttribute('open');
    expect(screen.queryByText('Patrol access model')).not.toBeInTheDocument();
    expect(screen.queryByText('Live manifest details')).not.toBeInTheDocument();

    advancedDetails!.open = true;
    fireEvent(advancedDetails!, new Event('toggle'));

    expect(
      screen.getByText('Only open this when you are building or debugging a client', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.getByText('Patrol access model')).toBeInTheDocument();
    expect(
      screen.getByText('Connected agents do not get separate powers', { exact: false }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Pulse MCP')).not.toBeInTheDocument();
    expect(screen.getByText('External adapter')).toBeInTheDocument();
    expect(screen.queryByText('Interactive questions')).not.toBeInTheDocument();
    expect(screen.getByText('Capability metadata')).toBeInTheDocument();
    expect(screen.getByText('Live manifest details')).toBeInTheDocument();
    expect(screen.getByText('Manifest scopes:', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('ai:execute')).toBeInTheDocument();
    expect(screen.getByText('Agent starting points')).toBeInTheDocument();
    expect(screen.getByText('(2 from manifest)')).toBeInTheDocument();
    expect(screen.getByText('Ask Patrol to handle an issue')).toBeInTheDocument();
    expect(screen.getByText('pulse_operations_loop')).toBeInTheDocument();
    expect(screen.getByText('resourceId').closest('p')).toHaveTextContent(
      'Arguments: resourceId required',
    );
    const mcpToolPosture = screen.getByTestId('agent-mcp-tool-posture');
    expect(mcpToolPosture.closest('details')).toBe(advancedDetails);
    expect(mcpToolPosture).toHaveTextContent('External agents expose 1 capability');
    expect(mcpToolPosture).toHaveTextContent('through Patrol mode');
    expect(mcpToolPosture.getAttribute('title') || '').toContain('Capability manifest');
    expect(screen.getByText('Failure codes')).toBeInTheDocument();
    expect(screen.getByText('(1 from the live manifest)')).toBeInTheDocument();
    expect(screen.getAllByText('resource_not_found').length).toBeGreaterThanOrEqual(2);
  });

  it('scrolls direct setup links to External agents instead of the API token top', async () => {
    window.history.pushState({}, '', `/#${EXTERNAL_AGENT_SETUP_ANCHOR}`);
    renderPanel();

    const heading = await screen.findByRole('heading', { name: 'External agents' });
    const panel = heading.closest(`#${EXTERNAL_AGENT_SETUP_ANCHOR}`);

    await waitFor(() => {
      expect(scrollIntoViewMock).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' });
    });
    expect(panel?.className).toContain('ring-2');
    expect(screen.getByText('Step 1')).toBeInTheDocument();
  });

  it('normalizes legacy MCP setup links to external agent setup', async () => {
    window.history.pushState({}, '', `/#${PULSE_MCP_SETUP_ANCHOR}`);
    renderPanel();

    const heading = await screen.findByRole('heading', { name: 'External agents' });
    expect(heading.closest(`#${EXTERNAL_AGENT_SETUP_ANCHOR}`)).not.toBeNull();

    await waitFor(() => {
      expect(window.location.hash).toBe(`#${EXTERNAL_AGENT_SETUP_ANCHOR}`);
    });
    expect(scrollIntoViewMock).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' });
    expect(screen.getByText('Step 1')).toBeInTheDocument();
  });
});
