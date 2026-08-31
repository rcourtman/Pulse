import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';

import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import {
  AI_EXECUTE_SCOPE,
  AGENT_CONFIG_READ_SCOPE,
  AGENT_REPORT_SCOPE,
  AUDIT_READ_SCOPE,
  DOCKER_MANAGE_SCOPE,
  DOCKER_REPORT_SCOPE,
  MONITORING_READ_SCOPE,
  MONITORING_WRITE_SCOPE,
  SETTINGS_READ_SCOPE,
  SETTINGS_WRITE_SCOPE,
} from '@/constants/apiScopes';
import { API_TOKEN_CREATE_ANCHOR, PULSE_MCP_TOKEN_SETUP_PATH } from '@/routing/resourceLinks';
import apiAccessPanelSource from '../APIAccessPanel.tsx?raw';
import apiTokenManagerSource from '../APITokenManager.tsx?raw';
import apiTokenManagerDialogsSource from '../APITokenManagerDialogs.tsx?raw';
import { APIAccessPanel } from '../APIAccessPanel';
import { APITokenManager } from '../APITokenManager';

const listTokensMock = vi.fn();
const createTokenMock = vi.fn();
const updateTokenScopesMock = vi.fn();
const renameTokenMock = vi.fn();
const deleteTokenMock = vi.fn();
const fetchAgentCapabilitiesManifestMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const showTokenRevealMock = vi.fn();
const loggerErrorMock = vi.fn();
const markDockerRuntimesTokenRevokedMock = vi.fn();
const markAgentsTokenRevokedMock = vi.fn();
const scrollIntoViewMock = vi.fn();

let mockResources: Resource[] = [];

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    listTokens: (...args: unknown[]) => listTokensMock(...args),
    createToken: (...args: unknown[]) => createTokenMock(...args),
    updateTokenScopes: (...args: unknown[]) => updateTokenScopesMock(...args),
    renameToken: (...args: unknown[]) => renameTokenMock(...args),
    deleteToken: (...args: unknown[]) => deleteTokenMock(...args),
  },
}));

vi.mock('@/api/agentCapabilities', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/agentCapabilities')>();
  return {
    ...actual,
    fetchAgentCapabilitiesManifest: (...args: unknown[]) =>
      fetchAgentCapabilitiesManifestMock(...args),
  };
});

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/stores/tokenReveal', () => ({
  showTokenReveal: (...args: unknown[]) => showTokenRevealMock(...args),
  useTokenRevealState: () => () => null,
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

vi.mock('@/utils/format', () => ({
  formatRelativeTime: () => 'moments ago',
}));

vi.mock('@/utils/url', () => ({
  getPulseBaseUrl: () => 'https://pulse.example.com',
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markDockerRuntimesTokenRevoked: (...args: unknown[]) =>
      markDockerRuntimesTokenRevokedMock(...args),
    markAgentsTokenRevoked: (...args: unknown[]) => markAgentsTokenRevokedMock(...args),
  }),
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => mockResources,
    byType: (type: string) => mockResources.filter((resource) => resource.type === type),
  }),
}));

const makeToken = (overrides: Partial<APITokenRecord> = {}): APITokenRecord => ({
  id: 'token-1',
  name: 'Runtime token',
  prefix: 'pulse',
  suffix: '1234',
  createdAt: '2026-03-12T10:00:00.000Z',
  lastUsedAt: '2026-03-12T11:00:00.000Z',
  scopes: [DOCKER_REPORT_SCOPE],
  ...overrides,
});

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'Resource One',
  displayName: 'Resource One',
  platformId: 'agent-1',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  tags: [],
  ...overrides,
});

const renderAPIAccessPanel = () =>
  render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <APIAccessPanel onTokensChanged={vi.fn()} refreshing={false} canManage />}
      />
    </Router>
  ));

const getTokenTableRow = (name: string): HTMLTableRowElement => {
  const row = screen
    .getAllByText(name)
    .map((element) => element.closest('tr'))
    .find(
      (candidate): candidate is HTMLTableRowElement => candidate instanceof HTMLTableRowElement,
    );
  if (!row) throw new Error(`Could not find desktop token row for ${name}`);
  return row;
};

const findTokenTableRow = async (name: string): Promise<HTMLTableRowElement> => {
  await screen.findAllByText(name);
  return getTokenTableRow(name);
};

describe('APITokenManager security surface', () => {
  // The API Access tab is the canonical security surface for
  // operator-controlled machine access. Pulse Intelligence owns
  // external-agent setup; API Access only mints and manages the
  // scoped token that setup links to when credentials are needed.
  it('keeps API Access separate from Pulse Intelligence external-agent setup', () => {
    expect(apiAccessPanelSource).toContain('<APITokenManager');
    expect(apiAccessPanelSource).toContain('api-access-token-section');
    expect(apiAccessPanelSource).not.toContain('AgentIntegrationsPanel');
    expect(apiAccessPanelSource).not.toContain('isExternalAgentSetupHash');
    expect(apiAccessPanelSource).not.toContain('api-access-external-agent-section');
    expect(apiTokenManagerSource).toContain("import('./APITokenManagerDialogs')");
    expect(apiTokenManagerSource).not.toContain('<Dialog');
    expect(apiTokenManagerDialogsSource).toContain('ariaLabel="Edit API token scopes"');
    expect(apiTokenManagerDialogsSource).toContain('ariaLabel="Revoke API token"');
    expect(apiTokenManagerSource).toContain(
      'flex flex-col items-stretch gap-4 sm:flex-row sm:items-start sm:justify-between',
    );
    expect(apiTokenManagerSource).toContain('min-h-11 w-full items-center justify-center');
    expect(apiTokenManagerSource).toContain('sm:min-h-10 sm:w-auto');
  });
});

describe('APITokenManager', () => {
  beforeEach(() => {
    listTokensMock.mockReset();
    createTokenMock.mockReset();
    updateTokenScopesMock.mockReset();
    renameTokenMock.mockReset();
    deleteTokenMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    showTokenRevealMock.mockReset();
    loggerErrorMock.mockReset();
    markDockerRuntimesTokenRevokedMock.mockReset();
    markAgentsTokenRevokedMock.mockReset();
    fetchAgentCapabilitiesManifestMock.mockReset();
    scrollIntoViewMock.mockReset();
    Element.prototype.scrollIntoView = scrollIntoViewMock;

    mockResources = [];
    listTokensMock.mockResolvedValue([]);
    fetchAgentCapabilitiesManifestMock.mockResolvedValue({
      version: 'v1',
      surfaceContract: {
        core: { id: 'pulse_intelligence_core', label: 'Pulse Intelligence Core', description: '' },
        proactiveEngine: { id: 'pulse_patrol', label: 'Pulse Patrol', description: '' },
        operatorSurfaces: [],
      },
      mcpAdapter: {
        serverName: 'pulse',
        command: 'pulse-mcp',
        baseUrlFlag: '--base-url',
        defaultBaseUrl: 'http://localhost:7655',
        tokenEnv: 'PULSE_API_TOKEN',
        configFamilies: [],
      },
      requiredScopes: [
        MONITORING_READ_SCOPE,
        MONITORING_WRITE_SCOPE,
        SETTINGS_READ_SCOPE,
        SETTINGS_WRITE_SCOPE,
        AI_EXECUTE_SCOPE,
      ],
      categories: [],
      capabilities: [],
    });
    createTokenMock.mockResolvedValue({
      token: 'pulse_secret_value',
      record: makeToken({
        id: 'token-created',
        name: 'Container automation',
        scopes: [DOCKER_MANAGE_SCOPE, DOCKER_REPORT_SCOPE],
      }),
    });
    deleteTokenMock.mockResolvedValue(undefined);
    updateTokenScopesMock.mockImplementation(
      async (id: string, scopes: string[]): Promise<APITokenRecord> => makeToken({ id, scopes }),
    );
    renameTokenMock.mockImplementation(async (id: string, name: string) => makeToken({ id, name }));
  });

  afterEach(() => {
    cleanup();
    window.history.pushState({}, '', '/');
  });

  it('keeps API Access focused on token management', async () => {
    window.history.pushState({}, '', '/settings/security/api');

    renderAPIAccessPanel();

    const tokenHeading = await screen.findByRole('heading', { name: 'API tokens' });
    const tokenSection = tokenHeading.closest('[data-testid="api-access-token-section"]');

    expect(tokenSection).not.toBeNull();
    expect(screen.queryByRole('heading', { name: 'External agents' })).not.toBeInTheDocument();
    expect(document.querySelector('[data-testid="api-access-external-agent-section"]')).toBeNull();
  });

  it('creates scoped tokens from the canonical preset path', async () => {
    expect(apiTokenManagerSource).toContain('@/components/shared/SelectablePillButton');
    expect(apiTokenManagerSource.match(/<SelectablePillButton/g) ?? []).toHaveLength(3);
    expect(apiTokenManagerSource).not.toContain(
      'inline-flex min-h-10 sm:min-h-10 items-center rounded-full border px-3 py-2 text-sm font-semibold transition',
    );
    expect(apiTokenManagerSource).not.toContain(
      'min-h-10 sm:min-h-10 rounded-full border px-3 py-2 text-sm font-semibold transition',
    );
    expect(apiTokenManagerSource).not.toContain('border-blue-500 bg-blue-600 text-white shadow-sm');
    expect(apiTokenManagerSource).not.toContain('hover:border-blue-400 hover:text-blue-600');

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'Container automation' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Docker / Podman manage' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Container automation', [
        DOCKER_MANAGE_SCOPE,
        DOCKER_REPORT_SCOPE,
      ]);
    });

    expect(showTokenRevealMock).toHaveBeenCalledWith(
      expect.objectContaining({
        token: 'pulse_secret_value',
        source: 'security',
        record: expect.objectContaining({
          id: 'token-created',
          name: 'Container automation',
          scopes: [DOCKER_MANAGE_SCOPE, DOCKER_REPORT_SCOPE],
        }),
        note: 'Copy this token now. You can reopen this dialog from Settings → API Access while this page stays open.',
      }),
    );
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'New API token generated. Copy it below while it is still visible.',
    );

    await waitFor(() => {
      expect(screen.getAllByText('Container automation')).toHaveLength(3);
      expect(screen.getByText(/Token generated:/)).toBeInTheDocument();
      expect(
        screen.getAllByText('Docker / Podman lifecycle management').length,
      ).toBeGreaterThanOrEqual(2);
      expect(screen.getAllByText('Docker / Podman reporting').length).toBeGreaterThanOrEqual(2);
    });
  });

  it('creates manually cycled agent tokens with bound collector scopes', async () => {
    createTokenMock.mockResolvedValue(
      makeToken({
        name: 'Replacement agent token',
        scopes: [AGENT_REPORT_SCOPE, AGENT_CONFIG_READ_SCOPE],
      }),
    );

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'Replacement agent token' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Agent' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Replacement agent token', [
        AGENT_CONFIG_READ_SCOPE,
        AGENT_REPORT_SCOPE,
      ]);
    });
  });

  it('offers agent lifecycle management when editing token scopes', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'agent-token-edit',
        name: 'Agent token',
        scopes: [AGENT_REPORT_SCOPE],
      }),
    ]);

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('Agent token');
    fireEvent.click(within(row).getByRole('button', { name: 'Edit scopes' }));

    const dialog = await screen.findByRole('dialog', { name: 'Edit API token scopes' });
    expect(
      within(dialog).getByRole('checkbox', { name: /Agent lifecycle management/i }),
    ).not.toBeChecked();
  });

  it('requires an explicit scope selection before generating a token', async () => {
    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByRole('button', { name: 'Full access' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled();
    expect(
      screen.getByText(
        'Choose a scoped preset for least privilege, or deliberately choose Full access.',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Kiosk / Monitoring' }));

    expect(screen.getByRole('button', { name: 'Generate' })).not.toBeDisabled();
    expect(screen.getByRole('button', { name: 'Kiosk / Monitoring' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: 'Full access' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('creates Patrol external agent tokens from the live manifest scopes', async () => {
    createTokenMock.mockResolvedValueOnce({
      token: 'pulse_agent_secret',
      record: makeToken({
        id: 'token-agent',
        name: 'OpenCode Pulse',
        scopes: [
          AI_EXECUTE_SCOPE,
          MONITORING_READ_SCOPE,
          MONITORING_WRITE_SCOPE,
          SETTINGS_READ_SCOPE,
          SETTINGS_WRITE_SCOPE,
        ],
      }),
    });

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(fetchAgentCapabilitiesManifestMock).toHaveBeenCalledTimes(1);
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    const preset = await screen.findByRole('button', { name: 'Patrol external agent' });
    expect(preset).toHaveAttribute(
      'title',
      'Scopes for connected agents that read Pulse context and request Patrol work.',
    );

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'OpenCode Pulse' },
    });
    fireEvent.click(preset);
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('OpenCode Pulse', [
        AI_EXECUTE_SCOPE,
        MONITORING_READ_SCOPE,
        MONITORING_WRITE_SCOPE,
        SETTINGS_READ_SCOPE,
        SETTINGS_WRITE_SCOPE,
      ]);
    });

    expect(showTokenRevealMock).toHaveBeenCalledWith(
      expect.objectContaining({
        token: 'pulse_agent_secret',
        source: 'security',
        record: expect.objectContaining({
          id: 'token-agent',
          name: 'OpenCode Pulse',
        }),
      }),
    );
  });

  it('preselects the Patrol external agent preset from the MCP setup route', async () => {
    window.history.pushState({}, '', PULSE_MCP_TOKEN_SETUP_PATH);
    createTokenMock.mockResolvedValueOnce({
      token: 'pulse_mcp_secret',
      record: makeToken({
        id: 'token-mcp',
        name: 'Patrol external agent',
        scopes: [
          AI_EXECUTE_SCOPE,
          MONITORING_READ_SCOPE,
          MONITORING_WRITE_SCOPE,
          SETTINGS_READ_SCOPE,
          SETTINGS_WRITE_SCOPE,
        ],
      }),
    });

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(fetchAgentCapabilitiesManifestMock).toHaveBeenCalledTimes(1);
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });
    await screen.findByDisplayValue('Patrol external agent');
    await waitFor(() => {
      expect(scrollIntoViewMock).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' });
    });
    expect(document.getElementById(API_TOKEN_CREATE_ANCHOR)?.className).toContain('ring-2');

    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Patrol external agent', [
        AI_EXECUTE_SCOPE,
        MONITORING_READ_SCOPE,
        MONITORING_WRITE_SCOPE,
        SETTINGS_READ_SCOPE,
        SETTINGS_WRITE_SCOPE,
      ]);
    });
  });

  it('surfaces the dedicated audit scope in presets and grouped custom scopes', async () => {
    createTokenMock.mockResolvedValueOnce({
      token: 'pulse_audit_secret',
      record: makeToken({
        id: 'token-audit',
        name: 'Audit export',
        scopes: [AUDIT_READ_SCOPE],
      }),
    });

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByRole('button', { name: 'Audit read' })).toBeInTheDocument();

    fireEvent.click(screen.getByText('Custom scopes'));
    expect(screen.getByText('AI')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Pulse Assistant chat' })).toHaveAttribute(
      'title',
      'Use interactive Pulse Assistant sessions, models, and read knowledge.',
    );
    expect(screen.getByRole('button', { name: 'Pulse Intelligence actions' })).toHaveAttribute(
      'title',
      'Use governed Patrol actions for plans, approvals, policy-allowed fixes, verification, history, and knowledge changes.',
    );
    expect(screen.getByText('Security')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Audit logs (read)' })).toBeInTheDocument();

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'Audit export' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Audit read' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Audit export', [AUDIT_READ_SCOPE]);
    });
  });

  it('maps token usage from unified resources and fans revocation out to affected runtimes and agents', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-runtime',
        name: 'Runtime token',
        scopes: [DOCKER_REPORT_SCOPE, AGENT_REPORT_SCOPE],
      }),
      makeToken({
        id: 'token-unused',
        name: 'Unused token',
        suffix: '9999',
        scopes: [DOCKER_REPORT_SCOPE],
      }),
    ]);

    mockResources = [
      makeResource({
        id: 'docker-resource',
        type: 'docker-host',
        name: 'Docker Edge',
        displayName: 'Docker Edge',
        platformType: 'docker',
        sourceType: 'agent',
        platformData: {
          docker: {
            hostSourceId: 'docker-runtime-1',
            tokenId: 'token-runtime',
          },
        } as Record<string, unknown>,
      }),
      makeResource({
        id: 'agent-resource',
        type: 'agent',
        name: 'Edge Agent',
        displayName: 'Edge Agent',
        platformData: {
          agent: {
            agentId: 'agent-007',
            tokenId: 'token-runtime',
          },
        } as Record<string, unknown>,
      }),
    ];

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('Runtime token');
    expect(within(row).getByText('Docker Edge • Edge Agent')).toBeInTheDocument();
    expect(within(row).queryByText(/container runtime/i)).not.toBeInTheDocument();
    expect(within(row).getByText('Agent reporting')).toBeInTheDocument();

    fireEvent.click(within(row).getByRole('button', { name: 'Revoke' }));

    // Confirm modal opens — click "Revoke token" to actually trigger the delete.
    const confirmBtn = await screen.findByRole('button', { name: 'Revoke token' });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(deleteTokenMock).toHaveBeenCalledWith('token-runtime');
    });

    expect(markDockerRuntimesTokenRevokedMock).toHaveBeenCalledWith('token-runtime', [
      'docker-runtime-1',
    ]);
    expect(markAgentsTokenRevokedMock).toHaveBeenCalledWith('token-runtime', ['agent-007']);
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      expect.stringContaining('Token "Runtime token" was previously used by'),
    );
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      expect.stringContaining('Docker / Podman runtime: Docker Edge'),
    );

    await waitFor(() => {
      expect(screen.queryByText('Runtime token')).not.toBeInTheDocument();
      expect(screen.getAllByText('Unused token')).toHaveLength(2);
    });
  });

  it('presents the complete token inventory as manageable compact cards below 1600px', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-compact',
        name: 'Phone-managed token',
        scopes: [DOCKER_REPORT_SCOPE, AGENT_REPORT_SCOPE],
      }),
    ]);

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const compactList = await screen.findByTestId('api-token-compact-list');
    expect(compactList).toHaveClass('min-[1600px]:hidden');
    const card = compactList.querySelector('[data-api-token-card="token-compact"]');
    expect(card).not.toBeNull();
    const compactCard = within(card as HTMLElement);
    expect(compactCard.getByRole('heading', { name: 'Phone-managed token' })).toBeInTheDocument();
    expect(compactCard.getByText('Scopes')).toBeInTheDocument();
    expect(compactCard.getByText('Docker / Podman reporting')).toBeInTheDocument();
    expect(compactCard.getByText('Agent reporting')).toBeInTheDocument();
    expect(compactCard.getByText('Usage')).toBeInTheDocument();
    expect(compactCard.getByText('Created')).toBeInTheDocument();
    expect(compactCard.getByText('Last used')).toBeInTheDocument();
    expect(compactCard.getByRole('button', { name: 'Edit scopes' })).toBeInTheDocument();
    expect(compactCard.getByRole('button', { name: 'Revoke' })).toBeInTheDocument();
    expect(apiTokenManagerSource).toContain('class="hidden min-[1600px]:block"');
    expect(apiTokenManagerSource).toContain('desktopMinWidth="960px"');
  });

  it('edits a token scope set in place without rotating or revoking it', async () => {
    const onTokensChanged = vi.fn();
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-edit',
        name: 'Editable token',
        scopes: [DOCKER_REPORT_SCOPE],
      }),
    ]);
    updateTokenScopesMock.mockResolvedValue(
      makeToken({
        id: 'token-edit',
        name: 'Editable token',
        scopes: [MONITORING_READ_SCOPE],
      }),
    );

    render(() => <APITokenManager onTokensChanged={onTokensChanged} canManage />);

    const row = await findTokenTableRow('Editable token');
    fireEvent.click(within(row).getByRole('button', { name: 'Edit scopes' }));

    const dialog = await screen.findByRole('dialog', { name: 'Edit API token scopes' });
    expect(within(dialog).getByText(/take effect on its next request/i)).toBeInTheDocument();
    const dockerReport = within(dialog).getByRole('checkbox', {
      name: /Docker \/ Podman reporting/i,
    });
    const monitoringRead = within(dialog).getByRole('checkbox', {
      name: /Monitoring & alerts \(read\)/i,
    });
    expect(dockerReport).toBeChecked();
    expect(monitoringRead).not.toBeChecked();

    fireEvent.click(dockerReport);
    fireEvent.click(monitoringRead);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save scopes' }));

    await waitFor(() => {
      expect(updateTokenScopesMock).toHaveBeenCalledWith('token-edit', [MONITORING_READ_SCOPE]);
    });
    expect(deleteTokenMock).not.toHaveBeenCalled();
    expect(createTokenMock).not.toHaveBeenCalled();
    expect(onTokensChanged).toHaveBeenCalledTimes(1);
    expect(notificationSuccessMock).toHaveBeenCalledWith('Scopes updated for Editable token.');

    await waitFor(() => {
      expect(
        screen.queryByRole('dialog', { name: 'Edit API token scopes' }),
      ).not.toBeInTheDocument();
    });
    const updatedRow = getTokenTableRow('Editable token');
    expect(within(updatedRow).getByText('Monitoring & alerts (read)')).toBeInTheDocument();
    expect(within(updatedRow).queryByText('Docker / Podman reporting')).not.toBeInTheDocument();
  });

  it('returns focus to token action triggers after dialogs close', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-focus-return',
        name: 'Keyboard token',
        scopes: [DOCKER_REPORT_SCOPE],
      }),
    ]);

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('Keyboard token');
    const trigger = within(row).getByRole('button', { name: 'Edit scopes' });
    fireEvent.click(trigger);

    const dialog = await screen.findByRole('dialog', { name: 'Edit API token scopes' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(trigger).toHaveFocus());

    fireEvent.click(trigger);
    await screen.findByRole('dialog', { name: 'Edit API token scopes' });
    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => expect(trigger).toHaveFocus());

    const revokeTrigger = within(row).getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeTrigger);

    const revokeDialog = await screen.findByRole('dialog', { name: 'Revoke API token' });
    fireEvent.click(within(revokeDialog).getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(revokeTrigger).toHaveFocus());
  });

  it('requires at least one changed scope and keeps failed edits open', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-edit-failure',
        name: 'Protected token',
        scopes: [DOCKER_REPORT_SCOPE],
      }),
    ]);
    updateTokenScopesMock.mockRejectedValueOnce(new Error('Cannot grant scope "monitoring:read"'));

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('Protected token');
    fireEvent.click(within(row).getByRole('button', { name: 'Edit scopes' }));

    const dialog = await screen.findByRole('dialog', { name: 'Edit API token scopes' });
    const save = within(dialog).getByRole('button', { name: 'Save scopes' });
    expect(save).toBeDisabled();

    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Docker \/ Podman reporting/i }));
    expect(
      within(dialog).getByText('Select at least one scope before saving.'),
    ).toBeInTheDocument();
    expect(save).toBeDisabled();

    fireEvent.click(
      within(dialog).getByRole('checkbox', { name: /Monitoring & alerts \(read\)/i }),
    );
    expect(save).not.toBeDisabled();
    fireEvent.click(save);

    await waitFor(() => {
      expect(updateTokenScopesMock).toHaveBeenCalledWith('token-edit-failure', [
        MONITORING_READ_SCOPE,
      ]);
    });
    expect(notificationErrorMock).toHaveBeenCalledWith('Unable to update API token scopes.');
    expect(screen.getByRole('dialog', { name: 'Edit API token scopes' })).toBeInTheDocument();
  });

  it('keeps governed infrastructure token usage labels on local operator identity', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-runtime',
        name: 'Runtime token',
        scopes: [AGENT_REPORT_SCOPE],
      }),
    ]);

    mockResources = [
      makeResource({
        id: 'pbs-resource',
        type: 'pbs',
        name: 'redacted-pbs',
        displayName: 'PBS Main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        platformData: {
          pbs: {
            hostname: 'pbs.local',
            instanceId: 'pbs-main',
          },
          agent: {
            agentId: 'pbs-agent-1',
            tokenId: 'token-runtime',
          },
        } as Record<string, unknown>,
      }),
    ];

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('Runtime token');
    expect(within(row).getByText('PBS Main')).toBeInTheDocument();
    expect(
      within(row).queryByText('backup server resource; status online; sources pbs'),
    ).not.toBeInTheDocument();
  });

  it('renames an existing token without revoking it', async () => {
    listTokensMock.mockResolvedValue([
      makeToken({
        id: 'token-pbs',
        name: 'proxmox-agent-pbs-1785306083',
        scopes: [AGENT_REPORT_SCOPE],
      }),
    ]);

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    const row = await findTokenTableRow('proxmox-agent-pbs-1785306083');
    const renameButton = within(row).getByRole('button', { name: 'Rename' });
    fireEvent.click(renameButton);

    const dialog = await screen.findByRole('dialog', { name: 'Rename API token' });
    const input = within(dialog).getByRole('textbox', { name: 'Token name' });
    fireEvent.input(input, { target: { value: 'PBS 01 telemetry' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(renameTokenMock).toHaveBeenCalledWith('token-pbs', 'PBS 01 telemetry');
      expect(screen.getAllByText('PBS 01 telemetry').length).toBeGreaterThan(0);
      expect(document.activeElement).toHaveTextContent('Rename');
    });
    expect(deleteTokenMock).not.toHaveBeenCalled();
    expect(notificationSuccessMock).toHaveBeenCalledWith('Token renamed');
  });

  it('surfaces scope denial when token generation is blocked by caller scope', async () => {
    createTokenMock.mockRejectedValueOnce(
      Object.assign(
        new Error('Cannot grant scope "monitoring:read": your token does not have this scope'),
        {
          status: 403,
        },
      ),
    );

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'Blocked token' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Kiosk / Monitoring' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Blocked token', ['monitoring:read']);
    });

    expect(notificationErrorMock).toHaveBeenCalledWith(
      'Cannot grant scope "monitoring:read": your token does not have this scope',
    );
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    expect(showTokenRevealMock).not.toHaveBeenCalled();
  });

  it('surfaces required scope when middleware rejects token generation', async () => {
    createTokenMock.mockRejectedValueOnce(
      Object.assign(new Error('missing_scope'), {
        status: 403,
        requiredScope: 'settings:write',
      }),
    );

    render(() => <APITokenManager onTokensChanged={vi.fn()} canManage />);

    await waitFor(() => {
      expect(listTokensMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.input(screen.getByPlaceholderText('e.g. Docker / Podman automation'), {
      target: { value: 'Needs settings scope' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Docker / Podman report' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }));

    await waitFor(() => {
      expect(createTokenMock).toHaveBeenCalledWith('Needs settings scope', [DOCKER_REPORT_SCOPE]);
    });

    expect(notificationErrorMock).toHaveBeenCalledWith(
      'This token is missing the required scope: Settings (write) (settings:write).',
    );
    expect(notificationSuccessMock).not.toHaveBeenCalled();
    expect(showTokenRevealMock).not.toHaveBeenCalled();
  });
});
