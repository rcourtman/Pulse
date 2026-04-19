import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { WelcomeStep } from '../steps/WelcomeStep';

const apiFetchMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const copyToClipboardMock = vi.fn();
const showErrorMock = vi.fn();
const showSuccessMock = vi.fn();

vi.mock('@/utils/apiClient', () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: (...args: unknown[]) => copyToClipboardMock(...args),
}));

vi.mock('@/utils/toast', () => ({
  showError: (...args: unknown[]) => showErrorMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

describe('WelcomeStep', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    copyToClipboardMock.mockResolvedValue(true);
    apiFetchJSONMock.mockResolvedValue({
      bootstrapTokenPath: '/etc/pulse/.bootstrap_token',
      isDocker: false,
      inContainer: false,
      lxcCtid: '',
      dockerContainerName: '',
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('frames first-run setup as a clear three-step journey', async () => {
    render(() => (
      <WelcomeStep
        onNext={vi.fn()}
        bootstrapToken=""
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/security/status');
    });

    expect(screen.getByText('Unlock this Pulse server')).toBeInTheDocument();
    expect(screen.getByText('Create the admin account')).toBeInTheDocument();
    expect(screen.getByText('Install the first host')).toBeInTheDocument();
    expect(screen.getByText('What this token does')).toBeInTheDocument();
    expect(
      screen.getByText(
        'This one-time bootstrap token only unlocks first-run setup on this Pulse server. Run the command above and paste the token string it prints. It is not your admin password and it is not the API token you will use after setup.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Verify bootstrap token →' })).toBeInTheDocument();
  });

  it('renders environment-aware Docker unlock guidance when the runtime reports a container name', async () => {
    apiFetchJSONMock.mockResolvedValue({
      bootstrapTokenPath: '/srv/pulse/bootstrap.token',
      isDocker: true,
      inContainer: false,
      lxcCtid: '',
      dockerContainerName: 'pulse-main',
    });

    render(() => (
      <WelcomeStep
        onNext={vi.fn()}
        bootstrapToken=""
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Docker deployment')).toBeInTheDocument();
    });

    expect(
      screen.getByText(
        'Pulse appears to be running in Docker as container "pulse-main". Run the command on the Docker host to print the one-time setup token from that container.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('docker exec pulse-main /app/pulse bootstrap-token'),
    ).toBeInTheDocument();
  });

  it('blocks encrypted bootstrap snapshot pastes with a specific error', async () => {
    const onNext = vi.fn();

    render(() => (
      <WelcomeStep
        onNext={onNext}
        bootstrapToken='{"version":2,"token_ciphertext":"cipher","token_hash":"hash"}'
        setBootstrapToken={vi.fn()}
        isUnlocked={false}
        setIsUnlocked={vi.fn()}
      />
    ));

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/security/status');
    });

    fireEvent.click(screen.getByRole('button', { name: 'Verify bootstrap token →' }));

    expect(showErrorMock).toHaveBeenCalledWith(
      'That looks like the encrypted .bootstrap_token file contents, not the raw setup token. Run the command above and paste the token string it prints.',
    );
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(onNext).not.toHaveBeenCalled();
  });
});
