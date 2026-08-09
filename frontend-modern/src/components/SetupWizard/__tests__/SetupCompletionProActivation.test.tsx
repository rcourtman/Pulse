import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { SetupCompletionPanel } from '../SetupCompletionPanel';
import { shouldShowSetupProActivationPointer } from '../setupCompletionModel';
import type { WizardState } from '../SetupWizard';

const apiFetchJSONMock = vi.fn();

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
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

const mockLicenseEndpoints = ({
  build,
  licenseValid,
}: {
  build?: string;
  licenseValid?: boolean;
}) => {
  apiFetchJSONMock.mockImplementation((path: string) => {
    if (path === '/api/license/runtime-capabilities') {
      return Promise.resolve({
        capabilities: [],
        limits: [],
        runtime: build ? { build, label: `Pulse ${build} runtime` } : undefined,
      });
    }
    if (path === '/api/license/status') {
      return Promise.resolve({
        valid: licenseValid,
        tier: licenseValid ? 'pro' : 'free',
        is_lifetime: false,
        days_remaining: 0,
        features: [],
      });
    }
    return Promise.resolve({ resources: [] });
  });
};

const waitForLicenseProbe = async () => {
  await waitFor(() => {
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/license/status',
      expect.objectContaining({ headers: { 'X-API-Token': 'setup-token' } }),
    );
  });
};

describe('shouldShowSetupProActivationPointer', () => {
  it('shows only for a Pro build with an explicitly invalid license', () => {
    expect(shouldShowSetupProActivationPointer({ runtimeBuild: 'pro', licenseValid: false })).toBe(
      true,
    );
    expect(shouldShowSetupProActivationPointer({ runtimeBuild: 'pro', licenseValid: true })).toBe(
      false,
    );
    expect(
      shouldShowSetupProActivationPointer({ runtimeBuild: 'community', licenseValid: false }),
    ).toBe(false);
    expect(
      shouldShowSetupProActivationPointer({ runtimeBuild: undefined, licenseValid: false }),
    ).toBe(false);
    expect(
      shouldShowSetupProActivationPointer({ runtimeBuild: 'pro', licenseValid: undefined }),
    ).toBe(false);
  });
});

describe('SetupCompletionPanel Pro activation pointer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiFetchJSONMock.mockResolvedValue({ resources: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders the pointer from the preview override without touching license endpoints', () => {
    const onComplete = vi.fn();
    render(() => (
      <SetupCompletionPanel
        state={baseState}
        onComplete={onComplete}
        connectedResourcesOverride={[]}
        proActivationOverride={true}
      />
    ));

    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    expect(screen.getByText('Activate Pulse Pro')).toBeInTheDocument();
    expect(
      screen.getByText(
        'This server is running the Pulse Pro build without an active license. Enter the activation key from your purchase email to unlock Pro features.',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Enter activation key' }));

    expect(onComplete).toHaveBeenCalledWith('/settings/pulse-intelligence/billing/plan');
  });

  it('hides the pointer when the preview override is false', () => {
    render(() => (
      <SetupCompletionPanel
        state={baseState}
        onComplete={vi.fn()}
        connectedResourcesOverride={[]}
        proActivationOverride={false}
      />
    ));

    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    expect(screen.queryByText('Activate Pulse Pro')).not.toBeInTheDocument();
  });

  it('shows the pointer when a Pro build reports no valid license', async () => {
    mockLicenseEndpoints({ build: 'pro', licenseValid: false });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Activate Pulse Pro')).toBeInTheDocument();
    });
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/license/runtime-capabilities',
      expect.objectContaining({ headers: { 'X-API-Token': 'setup-token' } }),
    );
  });

  it('keeps the pointer hidden on a community build', async () => {
    mockLicenseEndpoints({ build: 'community', licenseValid: false });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitForLicenseProbe();
    expect(screen.queryByText('Activate Pulse Pro')).not.toBeInTheDocument();
  });

  it('keeps the pointer hidden on a Pro build with an active license', async () => {
    mockLicenseEndpoints({ build: 'pro', licenseValid: true });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitForLicenseProbe();
    expect(screen.queryByText('Activate Pulse Pro')).not.toBeInTheDocument();
  });

  it('keeps the pointer hidden when the license probe fails', async () => {
    apiFetchJSONMock.mockImplementation((path: string) => {
      if (path === '/api/license/runtime-capabilities' || path === '/api/license/status') {
        return Promise.reject(new Error('probe failed'));
      }
      return Promise.resolve({ resources: [] });
    });

    render(() => <SetupCompletionPanel state={baseState} onComplete={vi.fn()} />);

    await waitForLicenseProbe();
    expect(screen.queryByText('Activate Pulse Pro')).not.toBeInTheDocument();
  });
});
