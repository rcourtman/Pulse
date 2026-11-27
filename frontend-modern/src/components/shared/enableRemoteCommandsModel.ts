import {
  buildUnixAgentInstallCommand,
  buildWindowsAgentInstallCommand,
  powerShellQuote,
  resolveAgentCommandPlatform,
  type AgentCommandPlatform,
} from '@/utils/agentInstallCommand';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  stripAgentPrefix,
  type EnableRemoteCommandsInstallerStep,
  type EnableRemoteCommandsTarget,
  type HostCommandAccessContext,
} from './hostCommandAccessContext';

export {
  extractConnectionCommandAccessContext,
  extractHostCommandAccessContext,
  type EnableRemoteCommandsActionKind,
  type EnableRemoteCommandsInstallerStep,
  type EnableRemoteCommandsTarget,
  type HostCommandAccessContext,
} from './hostCommandAccessContext';

const shellQuoteArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`;

export function isLeastPrivilegeAgentTarget(
  target: Pick<HostCommandAccessContext, 'collectionMode'> | null | undefined,
): boolean {
  return (asTrimmedString(target?.collectionMode) ?? '').toLowerCase() === 'typed-helper-summary';
}

export function resolveCommandAccessInstallerStep(
  target: Pick<HostCommandAccessContext, 'hostProfile'> | null | undefined,
): EnableRemoteCommandsInstallerStep {
  const profile = (asTrimmedString(target?.hostProfile) ?? '').toLowerCase();
  if (profile === 'unraid' || profile === 'docker' || profile === 'kubernetes') {
    return profile;
  }
  return 'agent';
}

export function resolveTargetCommandPlatform(
  target: Pick<HostCommandAccessContext, 'platform'> | null | undefined,
): AgentCommandPlatform {
  return resolveAgentCommandPlatform(target?.platform);
}

export function buildEnableCommandsTokenName(hostName: string): string {
  const trimmed = asTrimmedString(hostName) || 'host';
  const raw = `Remote commands (${trimmed})`;
  return raw.length > 64 ? `${raw.slice(0, 61)}...` : raw;
}

export function buildEnableRemoteCommandsScript(options: {
  baseUrl: string;
  target: Pick<HostCommandAccessContext, 'agentId' | 'hostname' | 'platform'>;
  token?: string | null;
  insecure?: boolean;
  caCertPath?: string | null;
}): string {
  const platform = resolveTargetCommandPlatform(options.target);
  const agentId = stripAgentPrefix(options.target.agentId);
  const hostname = asTrimmedString(options.target.hostname) ?? '';
  const insecure = options.insecure === true;
  const caCertPath = asTrimmedString(options.caCertPath) || undefined;

  if (platform === 'windows') {
    const extraEnvAssignments = ['$env:PULSE_ENABLE_COMMANDS="true"'];
    if (agentId) {
      extraEnvAssignments.push(`$env:PULSE_AGENT_ID="${powerShellQuote(agentId)}"`);
    }
    if (hostname) {
      extraEnvAssignments.push(`$env:PULSE_HOSTNAME="${powerShellQuote(hostname)}"`);
    }
    return buildWindowsAgentInstallCommand({
      baseUrl: options.baseUrl,
      token: options.token,
      insecure,
      caCertPath,
      extraEnvAssignments,
    });
  }

  const extraArgs = ['--update', '--enable-commands'];
  if (agentId) {
    extraArgs.push(`--agent-id ${shellQuoteArg(agentId)}`);
  }
  if (hostname) {
    extraArgs.push(`--hostname ${shellQuoteArg(hostname)}`);
  }
  return buildUnixAgentInstallCommand({
    baseUrl: options.baseUrl,
    token: options.token,
    insecure,
    caCertPath,
    extraArgs,
  });
}

export function getEnableCommandsIntroCopy(target: EnableRemoteCommandsTarget): string {
  const host = asTrimmedString(target.hostName) || 'this host';
  const container = asTrimmedString(target.containerName);
  if (target.actionKind === 'update' && container) {
    return `${container} has a newer image, but the Pulse agent on ${host} is monitoring-only. Command access must be granted on ${host}.`;
  }
  if (target.actionKind && container) {
    return `The Pulse agent on ${host} is monitoring-only, so Pulse cannot ${target.actionKind} ${container} yet. Command access must be granted on ${host}.`;
  }
  return `The Pulse agent on ${host} is monitoring-only. Command access must be granted on ${host}.`;
}

export function getEnableCommandsTerminalInstruction(target: HostCommandAccessContext): string {
  const host = asTrimmedString(target.hostName) || 'this host';
  const platform = resolveTargetCommandPlatform(target);
  if (platform === 'windows') {
    return `Run the enable command in an elevated PowerShell window on ${host}`;
  }
  if ((asTrimmedString(target.hostProfile) ?? '').toLowerCase() === 'unraid') {
    return `Run the enable command in the Unraid terminal or SSH on ${host}`;
  }
  return `Run the enable command in a terminal or SSH session on ${host}`;
}

export function getEnableCommandsContinueLabel(target: EnableRemoteCommandsTarget): string {
  const container = asTrimmedString(target.containerName);
  if (target.actionKind && container) {
    return `Continue to ${target.actionKind} ${container}`;
  }
  return 'Continue';
}
