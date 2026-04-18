import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { useSettingsNavigation } from '../useSettingsNavigation';

const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

function renderHarness(pathname = '/settings', search = '', hash = '') {
  return render(() => {
    const navigation = useSettingsNavigation({
      navigate: navigateSpy,
      location: {
        pathname,
        search,
        hash,
      },
    });

    return (
      <>
        <button type="button" onClick={() => navigation.setActiveTab('infrastructure-systems')}>
          open infrastructure settings
        </button>
        <div data-testid="selected-agent">{navigation.selectedAgent()}</div>
      </>
    );
  });
}

describe('useSettingsNavigation', () => {
  afterEach(() => {
    cleanup();
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
  });

  it('lands /settings on reporting inventory when the session is read-only', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderHarness('/settings');

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', {
        replace: true,
        scroll: false,
      });
    });
  });

  it('routes setup-oriented infrastructure tab clicks back to systems when the session is read-only', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderHarness('/settings/system-general');

    fireEvent.click(screen.getByRole('button', { name: 'open infrastructure settings' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', {
      scroll: false,
    });
  });

  it('syncs the selected proxmox agent from canonical deep links on initial load', async () => {
    renderHarness('/settings/infrastructure/platforms/proxmox/pbs');

    await waitFor(() => {
      expect(screen.getByTestId('selected-agent')).toHaveTextContent('pbs');
    });
  });

  it('defaults the selected proxmox agent to pve on the base proxmox route', async () => {
    renderHarness('/settings/infrastructure/platforms/proxmox');

    await waitFor(() => {
      expect(screen.getByTestId('selected-agent')).toHaveTextContent('pve');
    });
  });
});
