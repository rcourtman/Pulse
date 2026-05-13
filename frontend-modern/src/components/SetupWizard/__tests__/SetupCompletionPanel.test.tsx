import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SetupCompletionPanel } from '../SetupCompletionPanel';
import type { WizardState } from '../SetupWizard';

const copyToClipboardMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const createObjectURLMock = vi.fn(() => 'blob:mock-url');
const revokeObjectURLMock = vi.fn();

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

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: (...args: unknown[]) => copyToClipboardMock(...args),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/utils/url', () => ({
  getPulseBaseUrl: () => 'https://pulse.example.com',
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
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
    copyToClipboardMock.mockResolvedValue(true);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('frames setup completion around the canonical add-infrastructure picker', async () => {
    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    expect(screen.getByText('Choose your first infrastructure source')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Install Pulse Agent' })).toBeInTheDocument();
    expect(screen.getByText('Credentials you must save now')).toBeInTheDocument();
    expect(screen.getByText('Shown during setup')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('password')).toBeInTheDocument();
    expect(screen.getByText('Source choices')).toBeInTheDocument();
    expect(screen.getByText('Platform API')).toBeInTheDocument();
    expect(screen.getByText('Use both')).toBeInTheDocument();
    expect(
      screen.getByText('Inventory and health from Proxmox, TrueNAS, VMware, PBS, or PMG.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Node-local telemetry for standalone hosts, services, Docker, and Kubernetes.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('What happens next')).not.toBeInTheDocument();
    expect(screen.queryByText('What to expect')).not.toBeInTheDocument();
    expect(screen.queryByText('First system first')).not.toBeInTheDocument();

    expect(screen.queryByText('Connection URL (Agent → Pulse)')).not.toBeInTheDocument();
    expect(screen.queryByText('Custom CA certificate path (optional)')).not.toBeInTheDocument();
    expect(screen.queryByText('Windows (PowerShell as Administrator)')).not.toBeInTheDocument();
  });

  it('hands the primary action into the canonical source picker', async () => {
    const onComplete = vi.fn();

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=pick');
  });

  it('keeps a direct Pulse Agent handoff for operators who already know the first source', async () => {
    const onComplete = vi.fn();

    render(() => <SetupCompletionPanel state={baseState} onComplete={onComplete} />);

    fireEvent.click(screen.getByRole('button', { name: 'Install Pulse Agent' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=agent');
  });

  it('downloads credentials that point operators to the unified source picker instead of inline commands', async () => {
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
    expect(content).toContain('Infrastructure:');
    expect(content).toContain('https://pulse.example.com/settings/infrastructure?add=pick');
    expect(content).toContain('Use Add infrastructure to choose');
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

  it('still shows connected systems from polled resources and allows Infrastructure handoff', async () => {
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
        'Your admin account is ready and Pulse is already receiving telemetry. Open Infrastructure to inspect the first system, then return to Add infrastructure when you want another Pulse Agent or platform API source.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Open Infrastructure to review your first connected system.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Add infrastructure stays available for more Pulse Agent systems or platform API inventory when a platform manages the estate.',
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Open Infrastructure' }).length).toBeGreaterThan(
      0,
    );
    expect(screen.getAllByRole('button', { name: 'Add infrastructure' }).length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: 'Install Pulse Agent' })).not.toBeInTheDocument();

    const nextStepHeading = screen.getByRole('heading', { name: 'Open Infrastructure' });
    const nextStepCard = nextStepHeading.closest('[aria-label="Setup next step"]');
    expect(nextStepCard).not.toBeNull();

    fireEvent.click(
      within(nextStepCard as HTMLElement).getByRole('button', { name: 'Open Infrastructure' }),
    );
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure');
  });

  it('keeps add infrastructure available for API-backed starts after the first system connects', async () => {
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
        'Your admin account is ready and Pulse is already receiving telemetry. Open Infrastructure to inspect the first system, then return to Add infrastructure when you want another platform API or Pulse Agent source.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Add infrastructure stays available for more API-backed systems or Pulse Agent telemetry when a system needs node-local coverage.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Infrastructure' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Install Pulse Agent' })).not.toBeInTheDocument();

    const nextStepHeading = screen.getByRole('heading', { name: 'Open Infrastructure' });
    const nextStepCard = nextStepHeading.closest('[aria-label="Setup next step"]');
    expect(nextStepCard).not.toBeNull();

    fireEvent.click(
      within(nextStepCard as HTMLElement).getByRole('button', {
        name: 'Add infrastructure',
      }),
    );
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=pick');
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
        'Your admin account is ready and Pulse is already receiving telemetry. Open Infrastructure to inspect the first system, then return to Add infrastructure when you want another platform API or Pulse Agent source.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure' }));
    expect(onComplete).toHaveBeenCalledWith('/settings/infrastructure?add=pick');
  });

  it('keeps add infrastructure visible when agent and API-backed systems are already present', async () => {
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
        'Your admin account is ready and Pulse is already receiving telemetry. Open Infrastructure to inspect the first system, then return to Add infrastructure when you want another platform API or Agent source.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Add infrastructure stays available any time you want to expand from this first system with another API source, Agent source, or both.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Install Pulse Agent' })).not.toBeInTheDocument();
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

  it('does not surface a Monitor from Anywhere trial CTA in the setup completion panel', async () => {
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

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Connected (1 system)')).toBeInTheDocument();
    });

    expect(screen.queryByText('Monitor from Anywhere')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start free trial/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /set up relay/i })).not.toBeInTheDocument();
  });
});
