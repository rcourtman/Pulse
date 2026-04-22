import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@solidjs/testing-library';
import { ConnectionEditor } from '../ConnectionEditor';
import { ConnectionsAPI, type ProbeResponse } from '@/api/connections';

const connectionsApiMock = vi.hoisted(() => ({
  list: vi.fn(),
  probe: vi.fn(),
  setEnabled: vi.fn(),
  remove: vi.fn(),
}));
const onboardingMetricsTrackerMock = vi.hoisted(() => ({
  recordOpened: vi.fn(),
  recordPathSelected: vi.fn(),
  recordProbeResult: vi.fn(),
  recordCatalogSelected: vi.fn(),
  recordCredentialsOpened: vi.fn(),
}));
const createInfrastructureOnboardingMetricsTrackerMock = vi.hoisted(() =>
  vi.fn(() => onboardingMetricsTrackerMock),
);

vi.mock('@/api/connections', () => ({
  ConnectionsAPI: connectionsApiMock,
}));

vi.mock('@/utils/infrastructureOnboardingMetrics', () => ({
  createInfrastructureOnboardingMetricsTracker: createInfrastructureOnboardingMetricsTrackerMock,
}));

const mockedProbe = vi.mocked(ConnectionsAPI.probe);

function expectNodeBefore(a: Node, b: Node) {
  expect(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

describe('ConnectionEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('runs a probe and dispatches the detected type into the credential slot', async () => {
    const response: ProbeResponse = {
      candidates: [
        { type: 'pve', host: 'https://pve.lab:8006', port: 8006, hints: { product: 'Proxmox VE' } },
      ],
      probedMs: 418,
    };
    mockedProbe.mockResolvedValueOnce(response);

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);
    expect(createInfrastructureOnboardingMetricsTrackerMock).toHaveBeenCalledTimes(1);
    expect(onboardingMetricsTrackerMock.recordOpened).toHaveBeenCalledTimes(1);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'pve.lab' } });

    const probeButton = screen.getByRole('button', { name: /probe address/i });
    fireEvent.click(probeButton);

    await waitFor(() => expect(mockedProbe).toHaveBeenCalledWith('pve.lab'));
    await waitFor(() =>
      expect(onboardingMetricsTrackerMock.recordProbeResult).toHaveBeenCalledWith('detected'),
    );
    expect(onboardingMetricsTrackerMock.recordPathSelected).toHaveBeenCalledWith('api');

    const candidateHost = await screen.findByText('https://pve.lab:8006');
    const candidateButton = candidateHost.closest('button');
    expect(candidateButton).not.toBeNull();
    fireEvent.click(candidateButton!);

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
    expect(onboardingMetricsTrackerMock.recordCredentialsOpened).toHaveBeenCalledWith('pve');
    expect(renderSlot).toHaveBeenCalled();
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('pve');
    expect(lastCall.candidate?.host).toBe('https://pve.lab:8006');
    expect(lastCall.mode).toBe('add');
  });

  it('lets the user pick a product tile when probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });

    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());
    await waitFor(() =>
      expect(onboardingMetricsTrackerMock.recordProbeResult).toHaveBeenCalledWith('no-match'),
    );

    await screen.findByText(/no supported api-backed platform detected/i);

    // The catalog grid is always visible below the probe, so the user picks a
    // tile directly — no intermediate "enter credentials manually" toggle.
    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));
    expect(onboardingMetricsTrackerMock.recordCatalogSelected).toHaveBeenCalledWith('truenas');
    expect(onboardingMetricsTrackerMock.recordCredentialsOpened).toHaveBeenCalledWith('truenas');
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('truenas');
    expect(lastCall.candidate).toBeNull();
  });

  it('renders a platform-first catalog with the host-install path beneath it', () => {
    render(() => <ConnectionEditor renderCredentialSlot={() => <div />} onClose={() => {}} />);

    const platformHeading = screen.getAllByText('Connect a supported platform')[0];
    const agentButton = screen.getByRole('button', { name: /Install Pulse Agent/i });
    const probeButton = screen.getByRole('button', { name: /probe address/i });
    const vmwareButton = screen.getByRole('button', { name: /VMware vCenter/i });
    const trueNASButton = screen.getByRole('button', { name: /TrueNAS SCALE/i });
    const proxmoxButton = screen.getByRole('button', { name: /^Proxmox\b/i });

    // Catalog landing — lead with management-platform onboarding, then keep
    // host install as a secondary path below it.
    expect(platformHeading).toBeInTheDocument();
    expect(agentButton).toBeInTheDocument();
    expect(probeButton).toBeInTheDocument();
    expect(vmwareButton).toBeInTheDocument();
    expect(trueNASButton).toBeInTheDocument();
    expect(proxmoxButton).toBeInTheDocument();
    expectNodeBefore(platformHeading, probeButton);
    expectNodeBefore(vmwareButton, trueNASButton);
    expectNodeBefore(trueNASButton, proxmoxButton);
    expectNodeBefore(proxmoxButton, agentButton);
    expect(screen.queryByRole('button', { name: /^Proxmox VE/i })).toBeNull();
    expect(screen.queryByText('Recommended')).toBeNull();
    expect(screen.getAllByText('Available now').length).toBeGreaterThan(0);
    expect(screen.getByText('What happens next')).toBeInTheDocument();
  });

  it('groups Proxmox products under one family step before entering credentials', async () => {
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    fireEvent.click(screen.getByRole('button', { name: /^Proxmox\b/i }));

    expect(screen.getByText('Choose a Proxmox product')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Proxmox VE/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Proxmox Backup Server/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Proxmox Mail Gateway/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Proxmox VE/i }));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
    expect(renderSlot.mock.calls.at(-1)![0].type).toBe('pve');
  });

  it('keeps a single available Proxmox product as a direct tile', () => {
    render(() => (
      <ConnectionEditor
        manualTypeOptions={['pve']}
        renderCredentialSlot={() => <div />}
        onClose={() => {}}
      />
    ));

    expect(screen.getByRole('button', { name: /^Proxmox VE/i })).toBeInTheDocument();
    expect(screen.queryByText('VE, Backup Server, Mail Gateway')).toBeNull();
  });

  it('offers the agent path contextually when a probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 180 });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'baremetal.lan' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

    // The no-match box names bare-metal Linux / Unraid / FreeBSD and offers
    // the agent as a first-class alternative, so a user who probed the wrong
    // thing isn't left in a Platform-API-only dead end.
    const agentButton = await screen.findByRole('button', {
      name: /install pulse agent instead/i,
    });
    fireEvent.click(agentButton);

    expect(screen.getByTestId('slot').textContent).toBe('slot:agent');
    expect(onboardingMetricsTrackerMock.recordPathSelected).toHaveBeenCalledWith('agent');
    expect(onboardingMetricsTrackerMock.recordCredentialsOpened).toHaveBeenCalledWith('agent');
    const call = renderSlot.mock.calls.at(-1)![0];
    expect(call.type).toBe('agent');
    expect(call.candidate).toBeNull();
  });

  it('skips the probe step when an initialType is supplied (edit mode)', () => {
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => (
      <ConnectionEditor
        mode="edit"
        initialType="vmware"
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    expect(createInfrastructureOnboardingMetricsTrackerMock).not.toHaveBeenCalled();
    expect(screen.getByTestId('slot').textContent).toBe('slot:vmware');
    const call = renderSlot.mock.calls.at(0)![0];
    expect(call.mode).toBe('edit');
    expect(call.type).toBe('vmware');
    expect(call.candidate).toBeNull();
  });

  it('can return from a provider family to the top-level platform catalog', () => {
    render(() => <ConnectionEditor renderCredentialSlot={() => <div />} onClose={() => {}} />);

    fireEvent.click(screen.getByRole('button', { name: /^Proxmox\b/i }));
    expect(screen.getByText('Choose a Proxmox product')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /back to platforms/i }));

    expect(screen.getByRole('button', { name: /^Proxmox\b/i })).toBeInTheDocument();
    expect(screen.queryByText('Choose a Proxmox product')).toBeNull();
  });

  it('resets probe state when returning to the catalog from a credential slot', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));

    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());
    await screen.findByText(/no supported api-backed platform detected/i);

    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));
    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));

    fireEvent.click(screen.getByRole('button', { name: /back to catalog/i }));

    const resetInput = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    expect(resetInput.value).toBe('');
    expect(screen.queryByText(/no supported api-backed platform detected/i)).toBeNull();
    expect(screen.queryByTestId('slot')).toBeNull();
  });
});
