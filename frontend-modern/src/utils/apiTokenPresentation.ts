import { API_SCOPE_LABELS } from '@/constants/apiScopes';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';

type APITokenErrorShape = {
  requiredScope?: string;
  status?: number;
  message?: string;
};

export type APITokenUsagePresentationEntry = {
  count: number;
  items: Array<{ label: string }>;
};

const DOCKER_PODMAN_SOURCE_LABEL = getSourcePlatformLabel('docker');

export const API_TOKEN_DOCKER_PODMAN_RUNTIME_LABEL = `${DOCKER_PODMAN_SOURCE_LABEL} runtime`;
export const API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL = `${DOCKER_PODMAN_SOURCE_LABEL} runtimes`;
export const API_TOKEN_DOCKER_REPORT_PRESET_LABEL = `${DOCKER_PODMAN_SOURCE_LABEL} report`;
export const API_TOKEN_DOCKER_MANAGE_PRESET_LABEL = `${DOCKER_PODMAN_SOURCE_LABEL} manage`;
export const API_TOKEN_NAME_PLACEHOLDER = `e.g. ${DOCKER_PODMAN_SOURCE_LABEL} automation`;
export const API_TOKEN_ACCESS_PANEL_DESCRIPTION = `Generate scoped tokens for ${DOCKER_PODMAN_SOURCE_LABEL}, system agents, and automation pipelines. Tokens are shown once; store them securely and rotate when infrastructure changes.`;
export const API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION = `Permits ${DOCKER_PODMAN_SOURCE_LABEL} agents to stream runtime and container telemetry only.`;
export const API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION = `Extends ${DOCKER_PODMAN_SOURCE_LABEL} reporting with lifecycle actions (restart, stop, etc.).`;

export function getAPITokensLoadErrorMessage(): string {
  return 'Unable to load API tokens.';
}

export function getAPITokenManagementLocationMessage(): string {
  return 'Create or rotate API tokens in Settings → API Access.';
}

export function getAPITokenRevealSettingsNote(): string {
  return 'Copy this token now. You can reopen this dialog from Settings → API Access while this page stays open.';
}

export function getAPITokenGenerateErrorMessage(error?: unknown): string {
  if (error && typeof error === 'object') {
    const typedError = error as APITokenErrorShape;
    if (typedError.status !== 403 || typeof typedError.message !== 'string') {
      return 'Unable to generate the API token.';
    }

    const message = typedError.message.trim();
    if (message.startsWith('Cannot grant scope')) {
      return message;
    }
    if (message === 'missing_scope') {
      const requiredScope = typedError.requiredScope?.trim();
      if (requiredScope) {
        const label = API_SCOPE_LABELS[requiredScope as keyof typeof API_SCOPE_LABELS];
        return label
          ? `This token is missing the required scope: ${label} (${requiredScope}).`
          : `This token is missing the required scope: ${requiredScope}.`;
      }
    }
  }

  return 'Unable to generate the API token.';
}

export function getAPITokenRevokeErrorMessage(): string {
  return 'Unable to revoke the API token.';
}

export function getAPITokenDockerPodmanUsageCountLabel(count: number): string {
  return count === 1
    ? API_TOKEN_DOCKER_PODMAN_RUNTIME_LABEL
    : `${count} ${API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL}`;
}

export function getAPITokenDockerPodmanUsageSummary(entry: APITokenUsagePresentationEntry): string {
  return entry.count === 1
    ? (entry.items[0]?.label ?? API_TOKEN_DOCKER_PODMAN_RUNTIME_LABEL)
    : `${entry.count} ${API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL}`;
}

export function getAPITokenDockerPodmanUsageTitle(entry: APITokenUsagePresentationEntry): string {
  const labels = entry.items.map((item) => item.label).filter(Boolean);
  return labels.length > 0
    ? `${API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL}: ${labels.join(', ')}`
    : API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL;
}
