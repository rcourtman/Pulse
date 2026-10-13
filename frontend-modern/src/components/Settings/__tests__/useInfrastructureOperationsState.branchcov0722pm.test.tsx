import { unixBootstrapTestSource } from '@/utils/__tests__/unixBootstrapTestSource';
import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Connection } from '@/api/connections';
import { useInfrastructureOperationsState } from '../useInfrastructureOperationsState';
import type { AgentUninstallIdentity } from '../useInfrastructureOperationsState';
import type { AgentPlatform, UnifiedAgentRow } from '../infrastructureOperationsModel';

// The named sibling tests (InfrastructureOperationsModel.test.tsx,
// infrastructureOperationsModel.branchcov.test.ts) exercise pure exported
// helpers and never mount the hook, so there is no createRoot precedent to copy
// for this module. The createRoot/dispose shape, the hoisted vi.mock factories,
// and the single-static-import (no vi.resetModules) rule are lifted from the
// closest hook-mounting sibling, useSystemSettingsState.branchcov0722pm.test.ts.
// The child hook useInfrastructureInstallState is composed for real (not mocked)
// so the operations-state closures below are exercised as live code; its
// install-state signals are driven through the public setters the composed hook
// re-exposes (setInsecureMode, setCustomCaPath, setCustomAgentUrl,
// setEnableCommands), and requiresToken/currentToken are driven through the
// SecurityAPI.getStatus and NodesAPI.createHostAgentInstallToken mocks.

const mocks = vi.hoisted(() => ({
  securityGetStatus: vi.fn(),
  createHostAgentInstallToken: vi.fn(),
  monitoringGetState: vi.fn(),
  monitoringLookupAgent: vi.fn(),
  securityDeleteToken: vi.fn(),
  notificationSuccess: vi.fn(),
  notificationError: vi.fn(),
  notificationInfo: vi.fn(),
  showTokenReveal: vi.fn(),
  loggerError: vi.fn(),
  loggerWarn: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/settings/infrastructure', search: '' }),
    useNavigate: () => mocks.navigate,
  };
});

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    getStatus: mocks.securityGetStatus,
    deleteToken: mocks.securityDeleteToken,
  },
}));

vi.mock('@/api/nodes', () => ({
  NodesAPI: {
    createHostAgentInstallToken: mocks.createHostAgentInstallToken,
  },
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    getState: mocks.monitoringGetState,
    lookupAgent: mocks.monitoringLookupAgent,
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: mocks.notificationSuccess,
    error: mocks.notificationError,
    info: mocks.notificationInfo,
  },
}));

vi.mock('@/stores/tokenReveal', () => ({
  showTokenReveal: mocks.showTokenReveal,
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: mocks.loggerError,
    warn: mocks.loggerWarn,
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}));

type OpsState = ReturnType<typeof useInfrastructureOperationsState>;

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

const mountHook = (): { dispose: () => void; state: OpsState } => {
  let dispose = () => {};
  let state!: OpsState;
  createRoot((d) => {
    dispose = d;
    state = useInfrastructureOperationsState({});
  });
  return { dispose, state: state! };
};

const HTTPS_URL = 'https://pulse.test';
const HTTP_URL = 'http://pulse.test';

const baseConnection: Connection = {
  id: 'agent:agent-1',
  type: 'agent',
  name: 'conn-1',
  address: '10.0.0.1',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: [],
  scope: {},
  lastSeen: null,
  lastError: null,
  source: 'agent',
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
};

const baseRow: UnifiedAgentRow = {
  rowKey: 'row-1',
  id: 'id-1',
  name: 'node-1',
  capabilities: [],
  status: 'active',
  upgradePlatform: 'windows',
  scope: { label: 'Default', category: 'default' },
  installFlags: [],
  searchText: '',
  surfaces: [],
};

describe('useInfrastructureOperationsState command-building closures', () => {
  beforeEach(() => {
    // requiresToken defaults to true (requiresAuth set), no token present.
    mocks.securityGetStatus.mockResolvedValue({ requiresAuth: true, hasAuthentication: true });
    mocks.createHostAgentInstallToken.mockResolvedValue({
      token: 'tok-1',
      record: {
        id: 'rec-1',
        name: 'Agent',
        prefix: 'pul',
        suffix: 'abcd',
        createdAt: '2026-01-01',
      },
    });
    mocks.monitoringGetState.mockResolvedValue({ connectedInfrastructure: [] });
    mocks.monitoringLookupAgent.mockResolvedValue(null);
    mocks.securityDeleteToken.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  describe('shared Unix bootstrap (via getUninstallCommand)', () => {
    it('uses the complete-download/preflight/privileged private-input path', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const cmd = state.getUninstallCommand();
      expect(cmd).toContain('--uninstall');
      expect(cmd).toContain('set -- sudo');
      expect(cmd).toContain('\"$@\" bash -c');
      expect(cmd).not.toContain('--preflight-only');
      expect(cmd).not.toContain('| bash');
      expect(cmd).not.toContain("--token '");
      dispose();
    });
  });

  describe('saved-credential removal independent of install token', () => {
    it('uses saved credentials without prompting when an installation token is required', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      expect(state.getUninstallCommand()).not.toContain('--token');
      expect(state.getUninstallCommand()).not.toContain('Pulse agent token');
      expect(state.getUninstallCommand()).not.toContain('<api-token>');
      dispose();
    });

    it('keeps the minted token separate when one is present and required', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      await state.handleGenerateToken();

      const cmd = state.getUninstallCommand();
      expect(cmd).not.toContain('--token');
      expect(cmd).not.toContain('Pulse agent token');
      expect(cmd).not.toContain('tok-1');
      expect(cmd).not.toContain('<api-token>');
      expect(mocks.showTokenReveal).toHaveBeenCalledWith({
        token: 'tok-1',
        record: expect.objectContaining({ id: 'rec-1', name: 'Agent' }),
        source: 'agent',
        note: expect.stringContaining('silent “Pulse agent token” prompt'),
      });
      dispose();
    });

    it('reopens the token-only reveal without minting another credential', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      await state.handleGenerateToken();
      mocks.showTokenReveal.mockClear();

      state.showCurrentTokenOnly();

      expect(mocks.createHostAgentInstallToken).toHaveBeenCalledTimes(1);
      expect(mocks.showTokenReveal).toHaveBeenCalledWith({
        token: 'tok-1',
        record: expect.objectContaining({ id: 'rec-1', name: 'Agent' }),
        source: 'agent',
        note: expect.stringContaining('silent “Pulse agent token” prompt'),
      });
      dispose();
    });

    it('omits --token entirely when no token is required and none is present', async () => {
      mocks.securityGetStatus.mockResolvedValue({
        requiresAuth: false,
        hasAuthentication: false,
        apiTokenConfigured: false,
      });
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getUninstallCommand();
      expect(cmd).not.toContain('--token');
      expect(cmd).not.toContain('<api-token>');
      expect(cmd).toContain('--uninstall');
      expect(cmd).not.toContain('--url');
      expect(cmd).toContain('https://pulse.test');
      dispose();
    });
  });

  describe('insecure / custom-CA flag helpers (via getUninstallCommand)', () => {
    it('uses safe curl flags and no insecure/cacert tokens over https with secure mode', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getUninstallCommand();
      expect(cmd).toContain('curl -fsSL ');
      expect(cmd).not.toContain(' --insecure');
      expect(cmd).not.toContain('--cacert');
      dispose();
    });

    it('switches curl to -kfsSL and appends --insecure when insecure mode is on', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setInsecureMode(true);

      const cmd = state.getUninstallCommand();
      expect(cmd).toContain('curl -kfsSL ');
      expect(cmd).toContain(' --insecure');
      dispose();
    });

    it('does not weaken saved server trust because the download URL is HTTP', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTP_URL);

      const cmd = state.getUninstallCommand();
      // The download endpoint is not the saved credential-bearing server.
      expect(cmd).toContain('curl -fsSL ');
      expect(cmd).not.toContain(' --insecure');
      expect(cmd).toContain("'http://pulse.test/install.sh'");
      dispose();
    });

    it('emits --cacert for both curl and installer when a custom CA path is set', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setCustomCaPath('  /etc/ssl/ca.pem  ');

      const cmd = state.getUninstallCommand();
      // selectedCustomCaPath trims surrounding whitespace before quoting.
      expect(cmd).toContain(
        "curl -fsSL --cacert '/etc/ssl/ca.pem' 'https://pulse.test/install.sh'",
      );
      expect(cmd).toContain("--cacert '/etc/ssl/ca.pem'");
      dispose();
    });
  });

  describe('getCanonicalUninstallAgentId / getCanonicalUninstallHostname (via getUninstallCommand)', () => {
    it('prefers agentActionId and emits both identity flags when present', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const row: AgentUninstallIdentity = {
        agentActionId: 'act-9',
        agentId: 'agent-9',
        hostname: 'host-9',
      };

      const cmd = state.getUninstallCommand(row);
      expect(unixBootstrapTestSource(cmd)).toContain("--agent-id 'act-9'");
      expect(unixBootstrapTestSource(cmd)).toContain("--hostname 'host-9'");
      expect(cmd).not.toContain('agent-9');
      dispose();
    });

    it('falls back to agentId when agentActionId is absent', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const row: AgentUninstallIdentity = { agentId: 'agent-7', hostname: 'host-7' };

      const cmd = state.getUninstallCommand(row);
      expect(unixBootstrapTestSource(cmd)).toContain("--agent-id 'agent-7'");
      expect(unixBootstrapTestSource(cmd)).toContain("--hostname 'host-7'");
      dispose();
    });

    it('omits both identity flags when the row carries neither id nor hostname', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getUninstallCommand({});
      expect(cmd).not.toContain('--agent-id');
      expect(cmd).not.toContain('--hostname');
      dispose();
    });
  });

  describe('Windows saved-state uninstall invocation', () => {
    it.each([false, true])(
      'does not read or mint an installation token (minted=%s)',
      async (minted) => {
        const { state, dispose } = mountHook();
        await flushAsync();
        state.setCustomAgentUrl(HTTPS_URL);
        if (minted) await state.handleGenerateToken();
        const issuanceCalls = mocks.createHostAgentInstallToken.mock.calls.length;
        const cmd = state.getWindowsUninstallCommand();
        expect(cmd).toContain("-Url '' -Token '' -TokenFile ''");
        expect(cmd).toContain('-Uninstall $true');
        expect(cmd).toContain("-AgentId '' -Hostname ''");
        expect(cmd).not.toContain('PULSE_TOKEN');
        expect(cmd).not.toContain('tok-1');
        expect(cmd).not.toContain('<api-token>');
        expect(cmd).not.toContain('$env:');
        expect(mocks.createHostAgentInstallToken).toHaveBeenCalledTimes(issuanceCalls);
        dispose();
      },
    );

    it('uses only explicit download trust, leaving saved runtime trust available', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const command = state.getWindowsUninstallCommand({
        agentActionId: ' canonical ',
        agentId: 'alias',
        hostname: " host's-name ",
      });
      expect(command).toContain("-Insecure $false -CACertPath '' -ServerFingerprint ''");
      expect(command).toContain("-AgentId 'canonical' -Hostname 'host''s-name'");
      expect(command).toContain('if ($false -or');
      expect(command).toContain(
        'finally { [System.Net.ServicePointManager]::ServerCertificateValidationCallback = $pulsePrev;',
      );
      state.setCustomCaPath("C:\\trust\\host's-ca.pem");
      expect(state.getWindowsUninstallCommand()).toContain(
        "-CACertPath 'C:\\trust\\host''s-ca.pem'",
      );
      expect(state.getWindowsUninstallCommand()).toContain(
        "LoadCertificate('C:\\trust\\host''s-ca.pem')",
      );
      state.setInsecureMode(true);
      expect(state.getWindowsUninstallCommand()).toContain('-Insecure $true');
      expect(state.getWindowsUninstallCommand()).toContain('if ($true -or');
      dispose();
    });
  });

  describe('getPowerShellModeEnv (via getUpgradeCommand windows arm)', () => {
    it('does not add the commands env when enableCommands is off', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getUpgradeCommand(baseRow);
      expect(cmd).not.toContain('PULSE_ENABLE_COMMANDS');
      dispose();
    });

    it('adds the PULSE_ENABLE_COMMANDS env when enableCommands is on', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setEnableCommands(true);

      const cmd = state.getUpgradeCommand(baseRow);
      expect(cmd).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
      dispose();
    });
  });

  describe('install token and command-option atomicity', () => {
    it('locks copyable commands until a command-enabled replacement token is ready', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      await state.handleGenerateToken();

      let resolveReplacement!: (value: {
        token: string;
        record: {
          id: string;
          name: string;
          prefix: string;
          suffix: string;
          createdAt: string;
        };
      }) => void;
      mocks.createHostAgentInstallToken.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveReplacement = resolve;
          }),
      );

      const replacement = state.setEnableCommands(true);

      expect(state.enableCommands()).toBe(true);
      expect(state.currentToken()).toBeNull();
      expect(state.commandsUnlocked()).toBe(false);
      expect(mocks.createHostAgentInstallToken).toHaveBeenLastCalledWith({
        enableCommands: true,
        name: expect.any(String),
      });

      resolveReplacement({
        token: 'tok-command',
        record: {
          id: 'rec-command',
          name: 'Agent',
          prefix: 'pul',
          suffix: 'exec',
          createdAt: '2026-01-01',
        },
      });
      await replacement;

      expect(state.currentToken()).toBe('tok-command');
      expect(state.commandsUnlocked()).toBe(true);
      expect(mocks.securityDeleteToken).toHaveBeenCalledWith('rec-1');
      dispose();
    });

    it('restores the previous complete command state when replacement minting fails', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      await state.handleGenerateToken();
      mocks.createHostAgentInstallToken.mockRejectedValueOnce(new Error('mint failed'));

      await state.setEnableCommands(true);

      expect(state.enableCommands()).toBe(false);
      expect(state.currentToken()).toBe('tok-1');
      expect(state.commandsUnlocked()).toBe(true);
      expect(mocks.securityDeleteToken).not.toHaveBeenCalled();
      dispose();
    });
  });

  describe('getCanonicalConnectionAgentId (via getAgentConnectionUpgradeCommand windows arm)', () => {
    it('resolves to empty for a non-agent connection (no PULSE_AGENT_ID env)', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        type: 'pve',
        id: 'pve:node-1',
        agentIdentity: undefined,
      };

      const cmd = state.getAgentConnectionUpgradeCommand(connection, [], 'windows');
      expect(cmd).not.toContain('PULSE_AGENT_ID');
      dispose();
    });

    it('strips the "agent:" prefix from the connection id', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        type: 'agent',
        id: 'agent:agent-42',
        agentIdentity: { hostname: 'host-42' },
      };

      const cmd = state.getAgentConnectionUpgradeCommand(connection, [], 'windows');
      expect(cmd).toContain('$env:PULSE_AGENT_ID="agent-42"');
      dispose();
    });

    it('keeps the id unchanged when it has no "agent:" prefix', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        type: 'agent',
        id: 'agent-99',
        agentIdentity: { hostname: 'host-99' },
      };

      const cmd = state.getAgentConnectionUpgradeCommand(connection, [], 'windows');
      expect(cmd).toContain('$env:PULSE_AGENT_ID="agent-99"');
      dispose();
    });
  });

  describe('getCanonicalConnectionHostname (via getAgentConnectionUpgradeCommand windows arm)', () => {
    const platform = 'windows' as AgentPlatform;

    it('uses the reported agent hostname when present', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        agentIdentity: { hostname: 'node-1.internal' },
      };

      expect(state.getAgentConnectionUpgradeCommand(connection, [], platform)).toContain(
        '$env:PULSE_HOSTNAME="node-1.internal"',
      );
      dispose();
    });

    it('falls back to a scheme-less address when no hostname is reported', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        address: '10.0.0.9',
        agentIdentity: undefined,
      };

      expect(state.getAgentConnectionUpgradeCommand(connection, [], platform)).toContain(
        '$env:PULSE_HOSTNAME="10.0.0.9"',
      );
      dispose();
    });

    it('skips an address that carries a scheme and falls back to the connection name', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        name: 'NamedBox',
        address: 'https://10.0.0.5',
        agentIdentity: undefined,
      };

      expect(state.getAgentConnectionUpgradeCommand(connection, [], platform)).toContain(
        '$env:PULSE_HOSTNAME="NamedBox"',
      );
      dispose();
    });

    it('falls back to the connection name when hostname and address are both empty', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const connection: Connection = {
        ...baseConnection,
        name: 'NameOnly',
        address: '',
        agentIdentity: undefined,
      };

      expect(state.getAgentConnectionUpgradeCommand(connection, [], platform)).toContain(
        '$env:PULSE_HOSTNAME="NameOnly"',
      );
      dispose();
    });
  });

  describe('Linux authentication repair commands', () => {
    it('uses a fresh credential through a temporary token file and preserves update flags', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      await state.handleGenerateToken();

      const cmd = state.getAgentConnectionUpgradeCommand(
        baseConnection,
        ['--enable-proxmox', '--proxmox-type', 'pve'],
        'linux',
        true,
      );

      expect(cmd).toContain('bootstrap_dir=$(mktemp -d /tmp/pulse-bootstrap.XXXXXX)');
      expect(cmd).not.toContain('tok-1');
      expect(cmd).toContain('--token-file "$token_file"');
      expect(cmd).toContain('--update');
      expect(cmd).toContain('--enable-proxmox');
      expect(cmd).toContain('--proxmox-type');
      expect(cmd).toContain('pve');
      expect(cmd).not.toContain("--token 'tok-1'");
      dispose();
    });

    it('keeps an ordinary Linux update tokenless', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getAgentConnectionUpgradeCommand(baseConnection, [], 'linux');

      expect(cmd).toContain('--update');
      expect(cmd).toContain('set -- sudo');
      expect(cmd).toContain('\"$@\" bash -c');
      expect(cmd).not.toContain('--token-file');
      expect(cmd).not.toContain('token_dir=');
      dispose();
    });

    it('requires a fresh token for explicit or ledger-derived credential repair', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();

      expect(
        state.getAgentConnectionUpgradeCommandRequiresToken(baseConnection, 'linux', true),
      ).toBe(true);
      expect(
        state.getAgentConnectionUpgradeCommandRequiresToken(
          {
            ...baseConnection,
            fleet: { credentialStatus: 'invalid' } as Connection['fleet'],
          },
          'linux',
        ),
      ).toBe(true);
      expect(state.getAgentConnectionUpgradeCommandRequiresToken(baseConnection, 'linux')).toBe(
        false,
      );
      dispose();
    });
  });
});
