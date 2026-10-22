import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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

    const facts = within(screen.getByTestId('agent-doctor-facts'));
    expect(facts.getByText('Safe collector')).toBeInTheDocument();
    expect(facts.getByText('Confirmed')).toBeInTheDocument();
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

  afterEach(() => {
    vi.useRealTimers();
  });

  it('ages a target last-seen label on the shared clock without new targets', async () => {
    const start = Date.parse('2026-10-16T22:00:00Z');
    vi.useFakeTimers({ now: start });
    render(() => (
      <InfrastructureAgentDoctorPage
        targets={[targetFixture({ status: 'warning', lastSeen: start - 60_000 })]}
      />
    ));

    expect(screen.getByText('1m ago')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(2 * 3_600_000);
    expect(screen.queryByText('1m ago')).not.toBeInTheDocument();
    expect(screen.getByText('2h ago')).toBeInTheDocument();
  });

  it('keeps an attention row and its host-local repair guidance available through disclosure', () => {
    render(() => (
      <InfrastructureAgentDoctorPage
        targets={[
          targetFixture({
            status: 'warning',
            needsUpdate: true,
            expectedVersion: '6.4.0',
            lastSeen: Date.now(),
          }),
        ]}
      />
    ));

    expect(screen.getAllByText('Needs attention')).toHaveLength(2);
    expect(screen.getByText('just now')).toBeInTheDocument();
    expect(screen.getByText(/commands are host-local/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse details for host-1' }));
    expect(screen.getByRole('button', { name: 'Expand details for host-1' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for host-1' }));
    expect(screen.getByRole('button', { name: 'Collapse details for host-1' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });
  it('opens one target at a time under a header that names it and closes it', () => {
    render(() => (
      <InfrastructureAgentDoctorPage
        targets={[
          targetFixture({
            status: 'warning',
            reasons: [
              {
                code: 'agent_outdated',
                severity: 'warning',
                message: 'Agent is older than the server.',
                evidence: ['Reported 6.3.0'],
              },
            ],
            evidence: ['Hostname: host-1.local', 'Machine identity: abc123'],
          }),
          targetFixture({
            key: 'agent:host-2',
            connectionId: 'agent:host-2',
            displayName: 'host-2',
            actionRunnerAgentId: 'host-2',
            actionRunnerHostname: 'host-2.local',
          }),
        ]}
      />
    ));

    // Several targets start collapsed.
    expect(screen.queryByTestId('agent-doctor-detail')).toBeNull();

    const first = screen.getByRole('button', { name: 'Expand details for host-1' });
    fireEvent.click(first);
    const controlled = first.getAttribute('aria-controls')!;
    expect(controlled).toBeTruthy();
    const panel = screen.getByTestId('agent-doctor-detail');
    expect(document.getElementById(controlled)).toContainElement(panel);
    const detail = within(panel);
    expect(detail.getByRole('heading', { name: 'host-1' })).toBeInTheDocument();
    expect(detail.getByLabelText('Needs attention')).toBeInTheDocument();
    expect(detail.getByText('1 issue to look at')).toBeInTheDocument();
    expect(detail.getByText('Agent is older than the server.')).toBeInTheDocument();
    // The connection ID and identity evidence wait behind Details.
    expect(detail.getByText('Details')).toBeInTheDocument();
    expect(detail.queryByText('agent:host-1')).toBeNull();

    // Opening a second target closes the first.
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for host-2' }));
    expect(screen.getAllByTestId('agent-doctor-detail')).toHaveLength(1);
    expect(
      within(screen.getByTestId('agent-doctor-detail')).getByRole('heading', { name: 'host-2' }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Collapse host-2 details' }));
    expect(screen.queryByTestId('agent-doctor-detail')).toBeNull();
  });
});
