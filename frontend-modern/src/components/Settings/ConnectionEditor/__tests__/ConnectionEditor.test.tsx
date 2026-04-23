import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { ConnectionsAPI, type ProbeResponse } from '@/api/connections';
import { ConnectionEditor } from '../ConnectionEditor';

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

    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));

    await waitFor(() => expect(mockedProbe).toHaveBeenCalledWith('pve.lab'));
    await waitFor(() =>
      expect(onboardingMetricsTrackerMock.recordProbeResult).toHaveBeenCalledWith('detected'),
    );
    expect(onboardingMetricsTrackerMock.recordPathSelected).toHaveBeenCalledWith('api');

    fireEvent.click((await screen.findByText('https://pve.lab:8006')).closest('button')!);

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:pve'));
    expect(onboardingMetricsTrackerMock.recordCredentialsOpened).toHaveBeenCalledWith('pve');
    const lastCall = renderSlot.mock.calls.at(-1)![0];
    expect(lastCall.type).toBe('pve');
    expect(lastCall.candidate?.host).toBe('https://pve.lab:8006');
    expect(lastCall.mode).toBe('add');
  });

  it('renders the detect landing and can return to the source picker', () => {
    const onBackToCatalog = vi.fn();

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onBackToCatalog={onBackToCatalog}
        onClose={() => {}}
      />
    ));

    expect(screen.getByText('Address probe')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Back to source types/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Install Pulse Agent/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Back to source types/i }));
    expect(onBackToCatalog).toHaveBeenCalledTimes(1);
  });

  it('offers the source picker as a no-match escape hatch when provided', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 203 });
    const onBackToCatalog = vi.fn();

    render(() => (
      <ConnectionEditor
        renderCredentialSlot={() => <div />}
        onBackToCatalog={onBackToCatalog}
        onClose={() => {}}
      />
    ));

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: '192.168.1.50' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));

    await waitFor(() =>
      expect(onboardingMetricsTrackerMock.recordProbeResult).toHaveBeenCalledWith('no-match'),
    );

    fireEvent.click(screen.getByRole('button', { name: /Choose a source type instead/i }));
    expect(onBackToCatalog).toHaveBeenCalledTimes(1);
  });

  it('offers the agent path contextually when a probe returns no match', async () => {
    mockedProbe.mockResolvedValueOnce({ candidates: [], probedMs: 180 });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'baremetal.lan' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));
    await waitFor(() => expect(mockedProbe).toHaveBeenCalled());

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

  it('reopens the detect form from a selected credential slot', async () => {
    mockedProbe.mockResolvedValueOnce({
      candidates: [
        { type: 'truenas', host: 'https://truenas.lab', port: 443, hints: { product: 'TrueNAS SCALE' } },
      ],
      probedMs: 142,
    });

    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => <ConnectionEditor renderCredentialSlot={renderSlot} onClose={() => {}} />);

    const input = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    fireEvent.input(input, { target: { value: 'truenas.lab' } });
    fireEvent.click(screen.getByRole('button', { name: /probe address/i }));

    fireEvent.click((await screen.findByText('https://truenas.lab')).closest('button')!);
    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));

    fireEvent.click(screen.getByRole('button', { name: /Back to detect/i }));

    const resetInput = screen.getByPlaceholderText(/vcenter\.lab/) as HTMLInputElement;
    expect(resetInput.value).toBe('');
    expect(screen.queryByTestId('slot')).toBeNull();
    expect(screen.getByText('Address probe')).toBeInTheDocument();
  });

  it('uses an injected tracker for direct type routes without creating another one', async () => {
    const externalTracker = {
      recordOpened: vi.fn(),
      recordPathSelected: vi.fn(),
      recordProbeResult: vi.fn(),
      recordCatalogSelected: vi.fn(),
      recordCredentialsOpened: vi.fn(),
    };
    const renderSlot = vi.fn((props) => <div data-testid="slot">slot:{props.type}</div>);

    render(() => (
      <ConnectionEditor
        initialType="truenas"
        trackInitialCatalogSelection={true}
        onboardingMetricsTracker={externalTracker}
        renderCredentialSlot={renderSlot}
        onClose={() => {}}
      />
    ));

    await waitFor(() => expect(screen.getByTestId('slot').textContent).toBe('slot:truenas'));
    expect(createInfrastructureOnboardingMetricsTrackerMock).not.toHaveBeenCalled();
    expect(externalTracker.recordOpened).not.toHaveBeenCalled();
    expect(externalTracker.recordPathSelected).toHaveBeenCalledWith('api');
    expect(externalTracker.recordCatalogSelected).toHaveBeenCalledWith('truenas');
    expect(externalTracker.recordCredentialsOpened).toHaveBeenCalledWith('truenas');
  });

  it('skips probe setup when an initialType is supplied in edit mode', () => {
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
});
