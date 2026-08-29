import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { InfrastructureAgentDoctorPage } from '../InfrastructureAgentDoctorPage';
import type { InfrastructureAgentDoctorTarget } from '../infrastructureAgentUpdateCommandsModel';

const issueCredential = vi.hoisted(() => vi.fn());

vi.mock('@/api/agentDiagnostics', async () => {
  const actual =
    await vi.importActual<typeof import('@/api/agentDiagnostics')>('@/api/agentDiagnostics');
  return {
    ...actual,
    AgentDiagnosticsAPI: {
      ...actual.AgentDiagnosticsAPI,
      issueActionRunnerCredential: issueCredential,
    },
  };
});

vi.mock('../useInfrastructureOperationsState', () => ({
  useInfrastructureOperationsContext: () => ({
    getAgentConnectionUpgradeCommandRequiresToken: () => false,
    commandsUnlocked: () => true,
    selectedAgentUrl: () => 'https://pulse.example.test',
    insecureMode: () => false,
    customCaPath: () => '/etc/pulse/ca.pem',
  }),
}));

const targetFixture = (
  overrides: Partial<InfrastructureAgentDoctorTarget> = {},
): InfrastructureAgentDoctorTarget => ({
  key: 'agent:host-1',
  connectionId: 'agent:host-1',
  displayName: 'host-1',
  contextLabel: 'Machine',
  currentVersion: '6.3.0',
  installFlags: [],
  status: 'healthy',
  reasons: [],
  evidence: [],
  needsUpdate: false,
  needsCredentialRepair: false,
  commandPlatform: 'linux',
  privilegeLabel:
    'least privilege (pulse-agent) · commands monitoring-only · monitoring credential · typed helper configured',
  safeCollector: true,
  actionRunnerPosture: ['Credential not issued', 'Runner not connected'],
  actionRunnerCredentialEligible: true,
  actionRunnerCredentialAction: 'issue',
  actionRunnerAgentId: 'host-1',
  actionRunnerHostname: 'host-1.local',
  source: 'diagnostics',
  ...overrides,
});

describe('InfrastructureAgentDoctorPage action-runner enrollment', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    issueCredential.mockResolvedValue({
      token: 'one-time-runner-secret',
      tokenId: 'token-1',
      organizationId: 'org-1',
      agentId: 'host-1',
      hostname: 'HOST-1.LOCAL.',
      runtimeRole: 'action-runner',
      actionCapability: 'typed_actions.v1',
    });
  });

  it('reveals the host-bound secret once and keeps it out of both commands', async () => {
    render(() => <InfrastructureAgentDoctorPage targets={[targetFixture()]} />);

    expect(screen.getByText('Safe collector confirmed')).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole('button', { name: 'Issue one-time action-runner credential' }),
    );

    await waitFor(() => expect(screen.getByText('one-time-runner-secret')).toBeInTheDocument());
    expect(issueCredential).toHaveBeenCalledWith({
      agentId: 'host-1',
      hostname: 'host-1.local',
      name: 'host-1 action runner',
    });
    const promptCommand = screen
      .getByRole('button', { name: 'Copy private token-file command' })
      .parentElement?.querySelector('code')?.textContent;
    const installCommand = screen
      .getByRole('button', { name: 'Copy action-runner installer command' })
      .parentElement?.querySelector('code')?.textContent;
    expect(promptCommand).toContain('read -rsp');
    expect(promptCommand).toContain('Action runner token: ');
    expect(promptCommand).toContain('chmod 0600');
    expect(installCommand).toContain('--preflight-only');
    expect(installCommand).toContain('--update');
    expect(installCommand).toContain('--non-interactive');
    expect(installCommand).toContain('--action-token-file');
    expect(installCommand).toContain("--cacert '/etc/pulse/ca.pem'");
    expect(promptCommand).not.toContain('one-time-runner-secret');
    expect(installCommand).not.toContain('one-time-runner-secret');

    fireEvent.click(screen.getByRole('button', { name: 'Clear credential from this page' }));
    expect(screen.queryByText('one-time-runner-secret')).not.toBeInTheDocument();
  });

  it('does not offer issuance when the reported posture is ineligible', () => {
    render(() => (
      <InfrastructureAgentDoctorPage
        targets={[
          targetFixture({
            safeCollector: false,
            safeProfileGuidance: 'Safe collector profile is not confirmed.',
            actionRunnerCredentialEligible: false,
            actionRunnerCredentialBlockReason: 'Confirm the safe collector profile first.',
          }),
        ]}
      />
    ));

    expect(
      screen.queryByRole('button', { name: 'Issue one-time action-runner credential' }),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Safe collector profile is not confirmed.')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Copy safe-profile inspection command' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Copy safe-profile apply command' }),
    ).toBeInTheDocument();
    const commands = screen.getAllByText(/--preflight-only/).map((node) => node.textContent ?? '');
    expect(commands).toEqual(
      expect.arrayContaining([
        expect.stringContaining('--safe-profile-inspect'),
        expect.stringContaining('--safe-profile-apply'),
      ]),
    );
    expect(commands.every((command) => !command.includes('--update'))).toBe(true);
  });

  it('offers explicit rotation for an issued credential when the runner is disconnected', () => {
    render(() => (
      <InfrastructureAgentDoctorPage
        targets={[
          targetFixture({
            actionRunnerPosture: ['Credential issued (active)', 'Runner not connected'],
            actionRunnerCredentialAction: 'rotate',
          }),
        ]}
      />
    ));

    expect(
      screen.getByRole('button', { name: 'Rotate action-runner credential' }),
    ).toBeInTheDocument();
  });
});
