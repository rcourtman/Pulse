import { createContext, useContext, type ParentComponent } from 'solid-js';
import type { Connection } from '@/api/connections';
import {
  buildPowerShellInstallScriptBootstrap,
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  powerShellQuote,
  resolveAgentCommandPlatform,
} from '@/utils/agentInstallCommand';
import {
  TOKEN_PLACEHOLDER,
  getPowerShellInstallProfileEnvFromFlags,
  shellQuoteArg,
  type AgentPlatform,
  type UnifiedAgentRow,
} from './infrastructureOperationsModel';
import {
  useInfrastructureInstallState,
  type InfrastructureInstallStateOptions,
} from './useInfrastructureInstallState';

export type InfrastructureOperationsStateOptions = InfrastructureInstallStateOptions;

// Uninstall commands only need the host identity flags, so callers without a
// full inventory row (e.g. Agent Doctor's removed diagnostics) can hand one in.
export type AgentUninstallIdentity = Pick<
  UnifiedAgentRow,
  'agentActionId' | 'agentId' | 'hostname'
>;

export const useInfrastructureOperationsState = (
  options: InfrastructureOperationsStateOptions = {},
) => {
  const installState = useInfrastructureInstallState(options);

  const selectedCustomCaPath = () => installState.customCaPath().trim();
  const getPowerShellTransportEnv = () => {
    const envAssignments: string[] = [];
    if (installState.insecureMode()) {
      envAssignments.push(`$env:PULSE_INSECURE_SKIP_VERIFY="true"`);
    }
    if (selectedCustomCaPath()) {
      envAssignments.push(`$env:PULSE_CACERT="${powerShellQuote(selectedCustomCaPath())}"`);
    }
    return envAssignments;
  };
  const getPowerShellModeEnv = () => {
    const envAssignments = getPowerShellTransportEnv();
    if (installState.enableCommands()) {
      envAssignments.push(`$env:PULSE_ENABLE_COMMANDS="true"`);
    }
    return envAssignments;
  };
  const resolvedCommandToken = () => {
    if (installState.requiresToken()) {
      return installState.currentToken() || TOKEN_PLACEHOLDER;
    }
    return installState.currentToken();
  };

  const getCanonicalUninstallAgentId = (row?: AgentUninstallIdentity) =>
    row?.agentActionId?.trim() || row?.agentId?.trim() || '';
  const getCanonicalUninstallHostname = (row?: AgentUninstallIdentity) =>
    row?.hostname?.trim() || '';
  const getCanonicalConnectionAgentId = (connection: Connection) => {
    if (connection.type !== 'agent') return '';
    const id = connection.id.trim();
    return id.startsWith('agent:') ? id.slice('agent:'.length).trim() : id;
  };
  const getCanonicalConnectionHostname = (connection: Connection) => {
    const hostname = connection.agentIdentity?.hostname?.trim();
    if (hostname) return hostname;
    const address = connection.address?.trim();
    if (address && !address.includes('://')) return address;
    return connection.name?.trim() || '';
  };
  const getConnectionUpgradePlatform = (connection: Connection): AgentPlatform =>
    resolveAgentCommandPlatform(connection.agentIdentity?.platform);

  const getUninstallCommand = (row?: AgentUninstallIdentity) => {
    const url = installState.selectedAgentUrl();
    const token = resolvedCommandToken();
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    return buildUnixAgentInstallCommand({
      baseUrl: url,
      token,
      insecure: installState.insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraArgs: [
        '--uninstall',
        ...(agentId ? [`--agent-id ${shellQuoteArg(agentId)}`] : []),
        ...(hostname ? [`--hostname ${shellQuoteArg(hostname)}`] : []),
      ],
    });
  };

  const getWindowsUninstallCommand = (row?: AgentUninstallIdentity) => {
    const url = installState.selectedAgentUrl();
    const token = resolvedCommandToken();
    const transportEnv = getPowerShellTransportEnv();
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    const identityEnv: string[] = [];
    if (agentId) {
      identityEnv.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
    }
    if (hostname) {
      identityEnv.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
    }
    const prefixParts = [...transportEnv, ...identityEnv];
    const prefix = prefixParts.length > 0 ? `${prefixParts.join('; ')}; ` : '';
    if (token) {
      return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; $env:PULSE_TOKEN="${powerShellQuote(token)}"; $env:PULSE_UNINSTALL="true"; ${buildPowerShellInstallScriptBootstrap(url)}`;
    }
    return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; $env:PULSE_UNINSTALL="true"; ${buildPowerShellInstallScriptBootstrap(url)}`;
  };

  const getPlatformUninstallCommand = (platform: AgentPlatform, row?: AgentUninstallIdentity) => {
    if (platform === 'windows') {
      return getWindowsUninstallCommand(row);
    }
    return getUninstallCommand(row);
  };

  const getUpgradeCommand = (row: UnifiedAgentRow) => {
    const token = resolvedCommandToken();
    const url = installState.selectedAgentUrl();
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    if (row.upgradePlatform === 'windows') {
      const envAssignments = [
        ...getPowerShellInstallProfileEnvFromFlags(row.installFlags),
        ...getPowerShellModeEnv(),
      ];
      if (agentId) {
        envAssignments.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
      }
      if (hostname) {
        envAssignments.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
      }
      return buildWindowsAgentInstallCommand({
        baseUrl: url,
        token,
        insecure: installState.insecureMode(),
        caCertPath: selectedCustomCaPath(),
        extraEnvAssignments: envAssignments,
      });
    }
    return buildUnixAgentInstallCommand({
      baseUrl: url,
      token,
      insecure: installState.insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraArgs: [
        ...row.installFlags,
        ...(agentId ? [`--agent-id ${shellQuoteArg(agentId)}`] : []),
        ...(hostname ? [`--hostname ${shellQuoteArg(hostname)}`] : []),
      ],
    });
  };

  const getAgentConnectionUpgradeCommand = (
    connection: Connection,
    installFlags: string[] = [],
    platformOverride?: AgentPlatform,
    replaceCredential = false,
  ) => {
    const token = resolvedCommandToken();
    const url = installState.selectedAgentUrl();
    const agentId = getCanonicalConnectionAgentId(connection);
    const hostname = getCanonicalConnectionHostname(connection);
    const commandsEnabled = Boolean(connection.agentIdentity?.commandsEnabled);
    const platform = platformOverride ?? getConnectionUpgradePlatform(connection);
    if (platform === 'windows') {
      const envAssignments = [
        ...getPowerShellInstallProfileEnvFromFlags(installFlags),
        ...getPowerShellModeEnv(),
      ];
      if (commandsEnabled && !installState.enableCommands()) {
        envAssignments.push(`$env:PULSE_ENABLE_COMMANDS="true"`);
      }
      if (agentId) {
        envAssignments.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
      }
      if (hostname) {
        envAssignments.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
      }
      return buildWindowsAgentInstallCommand({
        baseUrl: url,
        token,
        insecure: installState.insecureMode(),
        caCertPath: selectedCustomCaPath(),
        extraEnvAssignments: envAssignments,
      });
    }

    const extraArgs = ['--update', ...installFlags];
    if (commandsEnabled || installState.enableCommands()) {
      extraArgs.push('--enable-commands');
    }
    if (replaceCredential) {
      // Credential replacement pins the existing identity; ordinary updates
      // recover saved identity and credentials through the installer's route.
      if (agentId) extraArgs.push(`--agent-id ${shellQuoteArg(agentId)}`);
      if (hostname) extraArgs.push(`--hostname ${shellQuoteArg(hostname)}`);
    }
    return buildUnixAgentInstallCommand({
      baseUrl: url,
      token: replaceCredential ? token : null,
      insecure: installState.insecureMode(),
      caCertPath: selectedCustomCaPath(),
      extraArgs,
    });
  };

  const getAgentConnectionUpgradeCommandRequiresToken = (
    connection: Connection,
    platformOverride?: AgentPlatform,
    replaceCredential = false,
  ) =>
    installState.requiresToken() &&
    ((platformOverride ?? getConnectionUpgradePlatform(connection)) === 'windows' ||
      replaceCredential ||
      connection.state === 'unauthorized' ||
      connection.fleet?.credentialStatus === 'invalid' ||
      connection.fleet?.credentialHealth?.status === 'expired');

  return {
    ...installState,
    getAgentConnectionUpgradeCommand,
    getAgentConnectionUpgradeCommandRequiresToken,
    getPlatformUninstallCommand,
    getUninstallCommand,
    getUpgradeCommand,
    getWindowsUninstallCommand,
  };
};

export type InfrastructureOperationsState = ReturnType<typeof useInfrastructureOperationsState>;

const InfrastructureOperationsStateContext = createContext<InfrastructureOperationsState>();

export const InfrastructureOperationsStateProvider: ParentComponent<
  InfrastructureOperationsStateOptions
> = (props) => {
  const state = useInfrastructureOperationsState({ embedded: props.embedded });

  return (
    <InfrastructureOperationsStateContext.Provider value={state}>
      {props.children}
    </InfrastructureOperationsStateContext.Provider>
  );
};

export const useInfrastructureOperationsContext = () => {
  const state = useContext(InfrastructureOperationsStateContext);
  if (!state) {
    throw new Error(
      'useInfrastructureOperationsContext must be used inside InfrastructureOperationsStateProvider',
    );
  }
  return state;
};
