import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SetupCompletionPanel } from '../SetupCompletionPanel';
import type { WizardState } from '../SetupWizard';

const copyToClipboardMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const startProTrialMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();
const navigateMock = vi.fn();
const createObjectURLMock = vi.fn(() => 'blob:mock-url');
const revokeObjectURLMock = vi.fn();
const trackAgentFirstConnectedMock = vi.fn();
const trackPaywallViewedMock = vi.fn();
const trackUpgradeClickedMock = vi.fn();

class MockBlob {
  readonly parts: string[];
  readonly type: string;

  constructor(parts: unknown[], options?: { type?: string }) {
    this.parts = parts.map((part) => String(part));
    this.type = options?.type || '';
  }

  async text() {
    return this.parts.join('');
  }
}

const getLastDownloadedBlob = (): MockBlob => {
  const lastCall = createObjectURLMock.mock.calls.at(-1) as [MockBlob] | undefined;
  expect(lastCall).toBeDefined();
  return lastCall![0];
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: (...args: unknown[]) => copyToClipboardMock(...args),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/utils/url', () => ({
  getPulseBaseUrl: () => 'https://pulse.example.com',
}));

vi.mock('@/stores/licenseCommercial', () => ({
  isCommercialTrialActive: () => false,
  loadCommercialPosture: (...args: unknown[]) => loadLicenseStatusMock(...args),
  commercialPosture: () => ({ relay: false }),
  getUpgradeActionUrlOrFallback: () => 'https://pulse.example.com/upgrade',
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackAgentFirstConnected: (...args: unknown[]) => trackAgentFirstConnectedMock(...args),
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

const baseState: WizardState = {
  username: 'admin',
  password: 'password',
  apiToken: 'setup-token',
};

describe('SetupCompletionPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('Blob', MockBlob as unknown as typeof Blob);
    vi.stubGlobal('URL', {
      createObjectURL: createObjectURLMock,
      revokeObjectURL: revokeObjectURLMock,
    });
    apiFetchJSONMock.mockResolvedValue({ resources: [] });
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue({ outcome: 'noop' });
    copyToClipboardMock.mockResolvedValue(true);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('frames setup completion around the canonical infrastructure install workspace', async () => {
    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    expect(screen.getByText('Connect your first monitored system')).toBeInTheDocument();
    expect(screen.getByText('What happens next')).toBeInTheDocument();
    expect(screen.getAllByText('Open Infrastructure Install').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Open Platform connections' })).toBeInTheDocument();
    expect(screen.getByText('Credentials you must save now')).toBeInTheDocument();
    expect(screen.getByText('Shown during setup')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('password')).toBeInTheDocument();
    expect(screen.getByText('What to expect')).toBeInTheDocument();
    expect(screen.getByText('First system first')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Infrastructure Install owns the token, connection URL, TLS/CA settings, and platform-specific commands.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Use the canonical install workspace where Pulse prepares the first-host install token from setup and keeps Platform connections beside it when the first target is API-backed.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'API-backed platforms like Proxmox, TrueNAS, and VMware use Platform connections instead of a dedicated install profile in Infrastructure Install.',
      ),
    ).toBeInTheDocument();

    expect(screen.queryByText('Connection URL (Agent → Pulse)')).not.toBeInTheDocument();
    expect(screen.queryByText('Custom CA certificate path (optional)')).not.toBeInTheDocument();
    expect(screen.queryByText('Windows (PowerShell as Administrator)')).not.toBeInTheDocument();
  });

  it('hands install into the canonical infrastructure workspace', async () => {
    const onComplete = vi.fn();

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    fireEvent.click(screen.getAllByRole('button', { name: 'Open Infrastructure Install' })[0]);

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/install');
  });

  it('hands API-backed starts into the canonical platform connections workspace', async () => {
    const onComplete = vi.fn();

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    fireEvent.click(screen.getByRole('button', { name: 'Open Platform connections' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/platforms');
  });

  it('downloads credentials that point operators to the install workspace instead of inline commands', async () => {
    const anchorClickMock = vi.fn();
    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tagName) => {
      const element = document.createElementNS('http://www.w3.org/1999/xhtml', tagName);
      if (tagName.toLowerCase() === 'a') {
        Object.defineProperty(element, 'click', {
          value: anchorClickMock,
          configurable: true,
        });
      }
      return element as HTMLElement;
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Download credentials' }));

    await waitFor(() => {
      expect(createObjectURLMock).toHaveBeenCalled();
    });

    const content = await getLastDownloadedBlob().text();
    expect(content).toContain('Web Login:');
    expect(content).toContain('Admin API Token:');
    expect(content).toContain('Infrastructure Install Workspace:');
    expect(content).toContain('https://pulse.example.com/settings/infrastructure/install');
    expect(content).toContain('Platform Connections Workspace:');
    expect(content).toContain('https://pulse.example.com/settings/infrastructure/platforms');
    expect(content).toContain(
      'continue with the first-host install token Pulse prepares from setup',
    );
    expect(content).toContain(
      'the first system is API-backed, such as Proxmox, TrueNAS, or VMware',
    );
    expect(content).not.toContain('Example Install Command');
    expect(content).not.toContain('Example Windows Install Command');

    createElementSpy.mockRestore();
  });

  it('keeps credentials visible first and lets operators collapse them after saving', async () => {
    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    expect(screen.getByText('Credentials you must save now')).toBeInTheDocument();
    expect(screen.getByText('Shown during setup')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('password')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Credentials you must save now/i }));

    expect(screen.queryByText('admin')).not.toBeInTheDocument();
    expect(screen.queryByText('password')).not.toBeInTheDocument();
  });

  it('still shows connected systems from polled resources and allows dashboard handoff', async () => {
    const onComplete = vi.fn();
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'agent-1',
          type: 'agent',
          name: 'Tower',
          agent: { agentId: 'agent-1' },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (1 system)')).toBeInTheDocument();
    });

    expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    expect(screen.getByText('First monitored system connected')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview, then return to Infrastructure Install when you want to add more host-installed systems.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Open the dashboard to review your first connected system.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Infrastructure Install stays available any time you want to add more host-installed systems.',
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Go to Dashboard' }).length).toBeGreaterThan(0);
    expect(
      screen.getAllByRole('button', { name: 'Open Infrastructure Install' }).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: 'Open Platform connections' })).not.toBeInTheDocument();
    expect(trackAgentFirstConnectedMock).toHaveBeenCalledWith(
      'setup_wizard_complete',
      'first_agent',
    );

    const nextStepHeading = screen.getByRole('heading', { name: 'Open your first dashboard view' });
    const nextStepCard = nextStepHeading.closest('div.bg-surface.rounded-md.border.border-border.p-6.text-left.mb-6');
    expect(nextStepCard).not.toBeNull();

    fireEvent.click(within(nextStepCard as HTMLElement).getByRole('button', { name: 'Go to Dashboard' }));
    expect(onComplete).toHaveBeenCalledWith('/');
  });

  it('keeps platform connections available for API-backed starts after the first system connects', async () => {
    const onComplete = vi.fn();
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'truenas-1',
          type: 'agent',
          name: 'truenas-main',
          displayName: 'TrueNAS Main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'online',
          lastSeen: 123,
          platformData: {
            truenas: { hostname: 'tn-main.local' },
          },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (1 system)')).toBeInTheDocument();
    });

    expect(screen.getByText('TrueNAS Main')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS')).toBeInTheDocument();
    expect(screen.getByText('First monitored system connected')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview, then return to Platform connections when you want to add more API-backed systems.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Platform connections stays available any time you want to add more API-backed systems, and Infrastructure Install is ready when the next system should run the unified agent.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Platform connections' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Infrastructure Install' })).toBeInTheDocument();
    expect(trackAgentFirstConnectedMock).not.toHaveBeenCalled();

    const nextStepHeading = screen.getByRole('heading', { name: 'Open your first dashboard view' });
    const nextStepCard = nextStepHeading.closest('div.bg-surface.rounded-md.border.border-border.p-6.text-left.mb-6');
    expect(nextStepCard).not.toBeNull();

    fireEvent.click(
      within(nextStepCard as HTMLElement).getByRole('button', {
        name: 'Open Platform connections',
      }),
    );
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/platforms');
  });

  it('derives admitted VMware onboarding from the governed platform manifest', async () => {
    const onComplete = vi.fn();
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'vmware-1',
          type: 'agent',
          name: 'vcsa-prod',
          displayName: 'vCenter Prod',
          platformId: 'vmware-prod',
          platformType: 'vmware-vsphere',
          sourceType: 'api',
          status: 'online',
          lastSeen: 123,
          platformData: {
            vmware: { hostname: 'vcenter.example.local' },
          },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (1 system)')).toBeInTheDocument();
    });

    expect(screen.getByText('vCenter Prod')).toBeInTheDocument();
    expect(screen.getByText('VMware vSphere')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview, then return to Platform connections when you want to add more API-backed systems.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Platform connections' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Open Platform connections' }));
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure/platforms');
  });

  it('keeps both continuation paths visible when install-managed and API-backed systems are already present', async () => {
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'agent-1',
          type: 'agent',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'tower',
          platformType: 'agent',
          sourceType: 'agent',
          status: 'online',
          lastSeen: 123,
          agent: { agentId: 'agent-1' },
        },
        {
          id: 'truenas-1',
          type: 'agent',
          name: 'truenas-main',
          displayName: 'TrueNAS Main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'online',
          lastSeen: 456,
          platformData: {
            truenas: { hostname: 'tn-main.local' },
          },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (2 systems)')).toBeInTheDocument();
    });

    expect(
      screen.getByText(
        'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview, then return to Platform connections or Infrastructure Install when you want to add more systems.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Platform connections and Infrastructure Install both stay available any time you want to expand from this first system.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Platform connections' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Infrastructure Install' })).toBeInTheDocument();
    expect(trackAgentFirstConnectedMock).toHaveBeenCalledWith(
      'setup_wizard_complete',
      'first_agent',
    );
  });

  it('keeps connected governed infrastructure on local operator identity', async () => {
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'pbs-1',
          type: 'pbs',
          name: 'redacted-pbs',
          displayName: 'PBS Main',
          status: 'online',
          agent: { agentId: 'pbs-1' },
          platformData: {
            pbs: { hostname: 'pbs.local', instanceId: 'pbs-main' },
          },
          policy: {
            display: {
              mode: 'governed',
              summary: 'backup server resource; status online; sources pbs',
            },
          },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (1 system)')).toBeInTheDocument();
    });

    expect(screen.getByText('PBS Main')).toBeInTheDocument();
    expect(screen.getByText('Proxmox Backup Server')).toBeInTheDocument();
    expect(
      screen.queryByText('backup server resource; status online; sources pbs'),
    ).not.toBeInTheDocument();
  });

  it('routes relay setup through the canonical settings destination', async () => {
    const onComplete = vi.fn();
    apiFetchJSONMock.mockResolvedValue({
      resources: [
        {
          id: 'agent-1',
          type: 'agent',
          name: 'Tower',
          agent: { agentId: 'agent-1' },
        },
      ],
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    await waitFor(() => {
      expect(screen.getByText('Monitor from Anywhere')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Start Free Trial & Set Up Mobile' }));

    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalled();
      expect(screen.getByRole('button', { name: 'Set Up Relay' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Set Up Relay' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/system-relay');
    expect(navigateMock).toHaveBeenCalledWith('/settings/system-relay');
  });
});
