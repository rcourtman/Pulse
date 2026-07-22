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

  describe('withPrivilegeEscalation (via getUninstallCommand)', () => {
    it('wraps a "| bash -s --" command in the root/sudo escalation block verbatim', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getUninstallCommand();

      expect(cmd).toContain('| { if [ "$(id -u)" -eq 0 ]; then bash -s -- --uninstall --url');
      expect(cmd).toContain(
        '; elif command -v sudo >/dev/null 2>&1; then sudo bash -s -- --uninstall --url',
      );
      expect(cmd).toContain(
        '; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }',
      );
      // The original pipe form is consumed by the replacement.
      expect(cmd).not.toMatch(/^\S+ \| bash -s --/);
      dispose();
    });
  });

  describe('resolvedCommandToken (via getUninstallCommand)', () => {
    it('substitutes the api-token placeholder when a token is required but absent', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      expect(state.getUninstallCommand()).toContain("--token '<api-token>'");
      dispose();
    });

    it('uses the minted token verbatim when one is present and required', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      await state.handleGenerateToken();

      const cmd = state.getUninstallCommand();
      expect(cmd).toContain("--token 'tok-1'");
      expect(cmd).not.toContain('<api-token>');
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
      expect(cmd).toContain("--uninstall --url 'https://pulse.test'");
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

    it('appends --insecure for an http:// url even in secure mode (url-driven arm)', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTP_URL);

      const cmd = state.getUninstallCommand();
      // insecureMode is off, so curl stays -fsSL, but the http url forces --insecure.
      expect(cmd).toContain('curl -fsSL ');
      expect(cmd).toContain(' --insecure');
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
      expect(cmd).toContain("--agent-id 'act-9'");
      expect(cmd).toContain("--hostname 'host-9'");
      expect(cmd).not.toContain('agent-9');
      dispose();
    });

    it('falls back to agentId when agentActionId is absent', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      const row: AgentUninstallIdentity = { agentId: 'agent-7', hostname: 'host-7' };

      const cmd = state.getUninstallCommand(row);
      expect(cmd).toContain("--agent-id 'agent-7'");
      expect(cmd).toContain("--hostname 'host-7'");
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

  describe('getPowerShellTransportEnv (via getWindowsUninstallCommand)', () => {
    it('produces no transport prefix when insecure mode is off and no CA is set', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      const cmd = state.getWindowsUninstallCommand();
      expect(cmd.startsWith('$env:PULSE_URL=')).toBe(true);
      dispose();
    });

    it('emits only the insecure-skip-verify env when insecure mode is on', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setInsecureMode(true);

      const cmd = state.getWindowsUninstallCommand();
      expect(cmd).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"; $env:PULSE_URL=');
      // The bootstrap always references $env:PULSE_CACERT in a guard, so assert
      // the assignment form (`="`) is absent — only transportEnv produces that.
      expect(cmd).not.toContain('$env:PULSE_CACERT="');
      dispose();
    });

    it('emits only the CA cert env when a custom CA path is set', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setCustomCaPath('/etc/ssl/ca.pem');

      const cmd = state.getWindowsUninstallCommand();
      expect(cmd).toContain('$env:PULSE_CACERT="/etc/ssl/ca.pem"; $env:PULSE_URL=');
      dispose();
    });

    it('emits both env assignments when insecure mode and a CA path are set', async () => {
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);
      state.setInsecureMode(true);
      state.setCustomCaPath('/etc/ssl/ca.pem');

      const cmd = state.getWindowsUninstallCommand();
      expect(cmd).toContain(
        '$env:PULSE_INSECURE_SKIP_VERIFY="true"; $env:PULSE_CACERT="/etc/ssl/ca.pem"; ',
      );
      dispose();
    });

    it('omits PULSE_TOKEN when no token resolves, and includes it after one is minted', async () => {
      // resolvedCommandToken only yields a falsy value (no PULSE_TOKEN line)
      // when no token is required and none has been minted; the placeholder arm
      // is covered in the resolvedCommandToken group above.
      mocks.securityGetStatus.mockResolvedValue({
        requiresAuth: false,
        hasAuthentication: false,
        apiTokenConfigured: false,
      });
      const { state, dispose } = mountHook();
      await flushAsync();
      state.setCustomAgentUrl(HTTPS_URL);

      expect(state.getWindowsUninstallCommand()).not.toContain('PULSE_TOKEN');
      await state.handleGenerateToken();
      expect(state.getWindowsUninstallCommand()).toContain('$env:PULSE_TOKEN="tok-1"');
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
});
