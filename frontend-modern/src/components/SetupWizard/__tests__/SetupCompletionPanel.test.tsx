import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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

vi.mock('@/stores/license', () => ({
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  entitlements: () => ({ relay: false }),
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
  trackAgentFirstConnected: vi.fn(),
  trackPaywallViewed: vi.fn(),
  trackUpgradeClicked: vi.fn(),
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

    expect(screen.getByText('Unified Resource Inventory')).toBeInTheDocument();
    expect(screen.getAllByText('Open Infrastructure Install').length).toBeGreaterThan(0);
    expect(screen.getByText('What Pulse Builds')).toBeInTheDocument();
    expect(screen.getByText('Unified by default')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Infrastructure Operations owns token generation, connection URL, TLS/CA, and platform-specific install commands.',
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

    fireEvent.click(screen.getByRole('button', { name: /Your Credentials Save these/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Download credentials' }));

    await waitFor(() => {
      expect(createObjectURLMock).toHaveBeenCalled();
    });

    const content = await getLastDownloadedBlob().text();
    expect(content).toContain('Web Login:');
    expect(content).toContain('Admin API Token:');
    expect(content).toContain('Infrastructure Install Workspace:');
    expect(content).toContain('https://pulse.example.com/settings/infrastructure/install');
    expect(content).not.toContain('Example Install Command');
    expect(content).not.toContain('Example Windows Install Command');

    createElementSpy.mockRestore();
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
      expect(screen.getByText('Connected (1 agent)')).toBeInTheDocument();
    });

    expect(screen.getByText('Tower')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Go to Dashboard' }).length).toBeGreaterThan(0);

    fireEvent.click(screen.getAllByRole('button', { name: 'Go to Dashboard' })[0]);
    expect(onComplete).toHaveBeenCalledWith('/');
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
