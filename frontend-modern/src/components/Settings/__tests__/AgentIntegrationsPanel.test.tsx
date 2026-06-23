import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { AgentCapabilitiesManifest } from '@/api/agentCapabilities';
import {
  EXTERNAL_AGENT_SETUP_ANCHOR,
  PATROL_CONTROL_PATH,
  PULSE_MCP_SETUP_ANCHOR,
  PULSE_MCP_TOKEN_SETUP_PATH,
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
          interactiveQuestions: true,
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

  it('keeps external agent setup brief and governed by Patrol mode', async () => {
    renderPanel();

    expect(
      (await screen.findByRole('heading', { name: 'External agents' })).closest(
        `#${EXTERNAL_AGENT_SETUP_ANCHOR}`,
      ),
    ).not.toBeNull();
    expect(
      await screen.findByText('Connect external tools to read Pulse context', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Patrol mode and scoped tokens control', { exact: false }),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('Patrol remains the operator');
    expect(document.body.textContent).not.toContain('Connected tools do not get separate powers');
    expect(screen.getByRole('link', { name: 'Choose Patrol mode' })).toHaveAttribute(
      'href',
      PATROL_CONTROL_PATH,
    );
    expect(screen.getByRole('button', { name: 'Show connector setup' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.getAllByRole('link', { name: 'Choose Patrol mode' })).toHaveLength(1);
    expect(
      screen.queryByRole('heading', { name: 'Connector setup' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Create token' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show connector setup' }));

    expect(screen.getByRole('button', { name: 'Hide connector setup' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByRole('heading', { name: 'Connector setup' })).toBeInTheDocument();
    expect(screen.getByText('Step 1')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: 'Choose Patrol mode' })).toHaveLength(1);
    expect(screen.getByRole('link', { name: 'Choose Patrol mode' })).toHaveAttribute(
      'href',
      PATROL_CONTROL_PATH,
    );
    expect(
      screen.getByText(
        'Required before agents can request Patrol work',
        {
          exact: false,
        },
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Step 2')).toBeInTheDocument();
    expect(screen.getByText('Create a scoped token')).toBeInTheDocument();
    expect(screen.getByText('Patrol external agent')).toBeInTheDocument();
    expect(
      screen.getByText('Create a token with the', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.getByText('Step 3')).toBeInTheDocument();
    expect(screen.getByText('Connect the agent')).toBeInTheDocument();
    expect(document.body.textContent).toContain('Install the connector');
    expect(document.body.textContent).toContain('paste the client config');
    expect(document.body.textContent).not.toContain('The installer and client snippets are below');
    expect(document.body.textContent).not.toContain('Install the Pulse MCP bridge');
    expect(document.body.textContent).not.toContain('on the machine that runs the external tool');

    const createTokenLink = screen.getByRole('link', { name: 'Create token' });
    expect(createTokenLink).toHaveAttribute('href', PULSE_MCP_TOKEN_SETUP_PATH);
    const installerCommands = screen.getByText('Installer commands').closest('details');
    expect(installerCommands).not.toBeNull();
    expect(installerCommands).not.toHaveAttribute('open');
    expect(document.body.textContent).not.toContain(
      'https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh',
    );
    expect(document.body.textContent).not.toContain(
      'https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1',
    );
    installerCommands!.open = true;
    fireEvent(installerCommands!, new Event('toggle'));
    expect(document.body.textContent).toContain(
      'https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh',
    );
    expect(document.body.textContent).toContain(
      'https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.ps1',
    );
    const clientConfigSnippets = screen.getByText('Client config').closest('details');
    expect(clientConfigSnippets).not.toBeNull();
    expect(clientConfigSnippets).not.toHaveAttribute('open');
    expect(document.body.textContent).not.toContain('pulse-test-mcp');
    expect(document.body.textContent).not.toContain('--pulse-url');
    expect(document.body.textContent).not.toContain('PULSE_TEST_TOKEN');
    clientConfigSnippets!.open = true;
    fireEvent(clientConfigSnippets!, new Event('toggle'));
    expect(document.body.textContent).toContain('pulse-test-mcp');
    expect(document.body.textContent).toContain('--pulse-url');
    expect(document.body.textContent).toContain('PULSE_TEST_TOKEN');
    expect(screen.getByText('Test OpenCode', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Test MCP servers', { exact: false })).toBeInTheDocument();
    expect(screen.getAllByText('"pulse-test":', { exact: false }).length).toBeGreaterThan(0);
    const advancedDetails = screen.getByText('Developer details').closest('details');
    expect(advancedDetails).not.toBeNull();
    expect(advancedDetails).not.toHaveAttribute('open');
    expect(
      screen.queryByText('Only open this when you are building or debugging a client', {
        exact: false,
      }),
    ).not.toBeInTheDocument();
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
      screen.getByText('Built-in Pulse views and connected clients all sit behind', {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Connected agents do not get separate powers', { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getAllByText('External agents').length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByText('Pulse MCP')).not.toBeInTheDocument();
    expect(screen.getByText('External adapter')).toBeInTheDocument();
    expect(screen.getAllByText('Actions').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Interactive questions')).toBeInTheDocument();
    expect(screen.getByText('Capability metadata')).toBeInTheDocument();
    expect(screen.getByText('Live manifest details')).toBeInTheDocument();
    expect(screen.getByText('Manifest scopes:', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('ai:execute')).toBeInTheDocument();
    expect(screen.getByText('Agent starting points')).toBeInTheDocument();
    expect(screen.getByText('(2 from manifest)')).toBeInTheDocument();
    expect(screen.getByText('Ask Patrol to handle an issue')).toBeInTheDocument();
    expect(screen.getAllByText('Wire name:').length).toBeGreaterThan(0);
    expect(screen.getByText('pulse_operations_loop')).toBeInTheDocument();
    expect(screen.getByText('Patrol')).toBeInTheDocument();
    expect(screen.getByText('Investigate resource')).toBeInTheDocument();
    expect(screen.getByText('resourceId')).toBeInTheDocument();
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
    expect(screen.getByText('branch on these codes', { exact: false })).toBeInTheDocument();
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
    expect(screen.getByRole('heading', { name: 'Connector setup' })).toBeInTheDocument();
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
    expect(screen.getByRole('heading', { name: 'Connector setup' })).toBeInTheDocument();
  });
});
