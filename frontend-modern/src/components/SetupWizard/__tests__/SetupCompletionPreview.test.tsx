import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { SetupCompletionPreview } from '../SetupCompletionPreview';

const navigateMock = vi.fn();
const apiFetchJSONMock = vi.fn();
let locationSearch = '';

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ search: locationSearch }),
    useNavigate: () => navigateMock,
  };
});

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(),
  entitlements: () => ({ relay: false }),
  getUpgradeActionUrlOrFallback: () => 'https://pulse.example.com/upgrade',
  startProTrial: vi.fn(),
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: vi.fn(),
  showError: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

describe('SetupCompletionPreview', () => {
  beforeEach(() => {
    navigateMock.mockReset();
    locationSearch = '';
    apiFetchJSONMock.mockResolvedValue({ resources: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps setup completion preview outside the runtime wizard flow', () => {
    render(() => <SetupCompletionPreview />);

    expect(apiFetchJSONMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByRole('button', { name: 'Open Infrastructure Install' })[0]);

    expect(navigateMock).toHaveBeenCalledWith('/settings/infrastructure/install');
  });

  it('renders the VMware-connected preview scenario without polling runtime state', () => {
    locationSearch = '?scenario=vmware-api-backed';

    render(() => <SetupCompletionPreview />);

    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    expect(screen.getByText('First monitored system connected')).toBeInTheDocument();
    expect(screen.getByText('VMware vSphere')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open Platform connections' })).toBeInTheDocument();
  });
});
