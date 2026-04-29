import { describe, expect, it } from 'vitest';
import { API_SCOPE_LABELS, DOCKER_MANAGE_SCOPE, DOCKER_REPORT_SCOPE } from '@/constants/apiScopes';
import {
  API_TOKEN_ACCESS_PANEL_DESCRIPTION,
  API_TOKEN_DOCKER_MANAGE_PRESET_LABEL,
  API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION,
  API_TOKEN_DOCKER_PODMAN_RUNTIME_LABEL,
  API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL,
  API_TOKEN_DOCKER_REPORT_PRESET_LABEL,
  API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION,
  API_TOKEN_NAME_PLACEHOLDER,
  getAPITokenDockerPodmanUsageCountLabel,
  getAPITokenDockerPodmanUsageSummary,
  getAPITokenDockerPodmanUsageTitle,
  getAPITokenGenerateErrorMessage,
  getAPITokenManagementLocationMessage,
  getAPITokenRevealSettingsNote,
  getAPITokensLoadErrorMessage,
  getAPITokenRevokeErrorMessage,
} from '@/utils/apiTokenPresentation';

describe('apiTokenPresentation', () => {
  it('returns canonical API token error copy', () => {
    expect(getAPITokensLoadErrorMessage()).toBe('Unable to load API tokens.');
    expect(getAPITokenGenerateErrorMessage()).toBe('Unable to generate the API token.');
    expect(getAPITokenRevokeErrorMessage()).toBe('Unable to revoke the API token.');
  });

  it('returns canonical API token settings location copy', () => {
    expect(getAPITokenManagementLocationMessage()).toBe(
      'Create or rotate API tokens in Settings → API Access.',
    );
    expect(getAPITokenRevealSettingsNote()).toBe(
      'Copy this token now. You can reopen this dialog from Settings → API Access while this page stays open.',
    );
  });

  it('keeps Docker and Podman token copy on the shared source-platform label', () => {
    expect(API_TOKEN_DOCKER_PODMAN_RUNTIME_LABEL).toBe('Docker / Podman runtime');
    expect(API_TOKEN_DOCKER_PODMAN_RUNTIMES_LABEL).toBe('Docker / Podman runtimes');
    expect(API_TOKEN_DOCKER_REPORT_PRESET_LABEL).toBe('Docker / Podman report');
    expect(API_TOKEN_DOCKER_MANAGE_PRESET_LABEL).toBe('Docker / Podman manage');
    expect(API_TOKEN_NAME_PLACEHOLDER).toBe('e.g. Docker / Podman automation');
    expect(API_TOKEN_ACCESS_PANEL_DESCRIPTION).toBe(
      'Generate scoped tokens for Docker / Podman, system agents, and automation pipelines. Tokens are shown once; store them securely and rotate when infrastructure changes.',
    );
    expect(API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION).toBe(
      'Permits Docker / Podman agents to stream runtime and container telemetry only.',
    );
    expect(API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION).toBe(
      'Extends Docker / Podman reporting with lifecycle actions (restart, stop, etc.).',
    );
    expect(API_SCOPE_LABELS[DOCKER_REPORT_SCOPE]).toBe('Docker / Podman reporting');
    expect(API_SCOPE_LABELS[DOCKER_MANAGE_SCOPE]).toBe('Docker / Podman lifecycle management');

    const singleUsage = { count: 1, items: [{ label: 'Docker Edge' }] };
    const multiUsage = {
      count: 2,
      items: [{ label: 'Docker Edge' }, { label: 'Podman Lab' }],
    };

    expect(getAPITokenDockerPodmanUsageCountLabel(1)).toBe('Docker / Podman runtime');
    expect(getAPITokenDockerPodmanUsageCountLabel(2)).toBe('2 Docker / Podman runtimes');
    expect(getAPITokenDockerPodmanUsageSummary(singleUsage)).toBe('Docker Edge');
    expect(getAPITokenDockerPodmanUsageSummary({ count: 1, items: [] })).toBe(
      'Docker / Podman runtime',
    );
    expect(getAPITokenDockerPodmanUsageSummary(multiUsage)).toBe('2 Docker / Podman runtimes');
    expect(getAPITokenDockerPodmanUsageTitle(multiUsage)).toBe(
      'Docker / Podman runtimes: Docker Edge, Podman Lab',
    );
  });

  it('surfaces token scope denial copy for generate failures', () => {
    const error = Object.assign(
      new Error('Cannot grant scope "monitoring:read": your token does not have this scope'),
      { status: 403 },
    );

    expect(getAPITokenGenerateErrorMessage(error)).toBe(
      'Cannot grant scope "monitoring:read": your token does not have this scope',
    );
  });

  it('surfaces required scope when middleware returns missing_scope', () => {
    const error = Object.assign(new Error('missing_scope'), {
      status: 403,
      requiredScope: 'settings:write',
    });

    expect(getAPITokenGenerateErrorMessage(error)).toBe(
      'This token is missing the required scope: Settings (write) (settings:write).',
    );
  });
});
