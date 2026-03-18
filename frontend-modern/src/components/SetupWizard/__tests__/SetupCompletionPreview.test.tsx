import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { SetupCompletionPreview } from '../SetupCompletionPreview';

const navigateMock = vi.fn();
const apiFetchJSONMock = vi.fn();

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/stores/license', () => ({
  loadLicenseStatus: vi.fn(),
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
    apiFetchJSONMock.mockResolvedValue({ resources: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps setup completion preview outside the runtime wizard flow', () => {
    render(() => <SetupCompletionPreview />);

    fireEvent.click(screen.getAllByRole('button', { name: 'Open Infrastructure Install' })[0]);

    expect(navigateMock).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
