import { createContext, useContext, type ParentComponent } from 'solid-js';
import {
  buildPowerShellInstallScriptBootstrap,
  powerShellQuote,
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
import { useInfrastructureReportingState } from './useInfrastructureReportingState';

export type InfrastructureOperationsStateOptions = InfrastructureInstallStateOptions;

export const useInfrastructureOperationsState = (
  options: InfrastructureOperationsStateOptions = {},
) => {
  const installState = useInfrastructureInstallState(options);
  const reportingState = useInfrastructureReportingState();

  const withPrivilegeEscalation = (command: string) => {
    if (!command.includes('| bash -s --')) return command;
    return command.replace(/\|\s*bash -s --([\s\S]*)$/, (_match, args: string) => {
      return `| { if [ "$(id -u)" -eq 0 ]; then bash -s --${args}; elif command -v sudo >/dev/null 2>&1; then sudo bash -s --${args}; else echo "Root privileges required. Run as root (su -) and retry." >&2; exit 1; fi; }`;
    });
  };

  const selectedCustomCaPath = () => installState.customCaPath().trim();
  const urlRequiresInstallerInsecure = (url: string) =>
    installState.insecureMode() || url.trim().toLowerCase().startsWith('http://');
  const getInsecureFlag = (url: string) => (urlRequiresInstallerInsecure(url) ? ' --insecure' : '');
  const getCurlFlags = () => (installState.insecureMode() ? '-kfsSL' : '-fsSL');
  const getShellCustomCaCurlFlag = () => {
    const caPath = selectedCustomCaPath();
    return caPath ? ` --cacert ${shellQuoteArg(caPath)}` : '';
  };
  const getShellCustomCaInstallerFlag = () => {
    const caPath = selectedCustomCaPath();
    return caPath ? ` --cacert ${shellQuoteArg(caPath)}` : '';
  };
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

  const getCanonicalUninstallAgentId = (row?: UnifiedAgentRow) =>
    row?.agentActionId?.trim() || row?.agentId?.trim() || '';
  const getCanonicalUninstallHostname = (row?: UnifiedAgentRow) => row?.hostname?.trim() || '';

  const getUninstallCommand = (row?: UnifiedAgentRow) => {
    const url = installState.selectedAgentUrl();
    const token = resolvedCommandToken();
    const insecure = getInsecureFlag(url);
    const agentId = getCanonicalUninstallAgentId(row);
    const hostname = getCanonicalUninstallHostname(row);
    const baseArgs = token
      ? `--uninstall --url ${shellQuoteArg(url)} --token ${shellQuoteArg(token)}${insecure}${getShellCustomCaInstallerFlag()}`
      : `--uninstall --url ${shellQuoteArg(url)}${insecure}${getShellCustomCaInstallerFlag()}`;
    const identityArgs = `${agentId ? ` --agent-id ${shellQuoteArg(agentId)}` : ''}${hostname ? ` --hostname ${shellQuoteArg(hostname)}` : ''}`;
    return withPrivilegeEscalation(
      `curl ${getCurlFlags()}${getShellCustomCaCurlFlag()} ${shellQuoteArg(`${url}/install.sh`)} | bash -s -- ${baseArgs}${identityArgs}`,
    );
  };

  const getWindowsUninstallCommand = (row?: UnifiedAgentRow) => {
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

  const getPlatformUninstallCommand = (platform: AgentPlatform, row?: UnifiedAgentRow) => {
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
      const prefix = envAssignments.length > 0 ? `${envAssignments.join('; ')}; ` : '';
      const tokenEnv = token ? `$env:PULSE_TOKEN="${powerShellQuote(token)}"; ` : '';
      return `${prefix}$env:PULSE_URL="${powerShellQuote(url)}"; ${tokenEnv}${buildPowerShellInstallScriptBootstrap(url)}`;
    }
    let command = `curl ${getCurlFlags()}${getShellCustomCaCurlFlag()} ${shellQuoteArg(`${url}/install.sh`)} | bash -s -- --url ${shellQuoteArg(url)}`;
    if (token) {
      command += ` --token ${shellQuoteArg(token)}`;
    }
    if (row.installFlags.length > 0) {
      command += ` ${row.installFlags.join(' ')}`;
    }
    if (urlRequiresInstallerInsecure(url)) {
      command += getInsecureFlag(url);
    }
    command += getShellCustomCaInstallerFlag();
    if (agentId) {
      command += ` --agent-id ${shellQuoteArg(agentId)}`;
    }
    if (hostname) {
      command += ` --hostname ${shellQuoteArg(hostname)}`;
    }
    return withPrivilegeEscalation(command);
  };

  return {
    ...installState,
    ...reportingState,
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
