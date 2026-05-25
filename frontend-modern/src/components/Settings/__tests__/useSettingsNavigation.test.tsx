import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { useSettingsNavigation } from '../useSettingsNavigation';

const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const sessionPresentationPolicyResolvedMock = vi.hoisted(() => vi.fn(() => true));
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
  sessionPresentationPolicyResolved: () => sessionPresentationPolicyResolvedMock(),
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
    sessionPresentationPolicyResolvedMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
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

  it('does not strip infrastructure onboarding queries before presentation policy resolves', async () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    sessionPresentationPolicyResolvedMock.mockReturnValue(false);

    renderHarness('/settings/infrastructure', '?add=pick');

    await waitFor(() => {
      expect(navigateSpy).not.toHaveBeenCalledWith('/settings/infrastructure', {
        replace: true,
        scroll: false,
      });
    });
  });

  it('does not derive selected agents from retired infrastructure deep links', async () => {
    renderHarness('/settings/infrastructure/platforms/proxmox/pbs');

    await waitFor(() => {
      expect(screen.getByTestId('selected-agent')).toHaveTextContent('pve');
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('does not translate retired infrastructure setup paths', async () => {
    renderHarness('/settings/infrastructure/install');

    await waitFor(() => {
      expect(navigateSpy).not.toHaveBeenCalled();
    });
  });

  it('does not translate retired workloads settings paths', async () => {
    renderHarness('/settings/workloads/docker');

    await waitFor(() => {
      expect(navigateSpy).not.toHaveBeenCalled();
    });
  });

  it('does not translate remaining retired settings aliases', async () => {
    for (const path of [
      '/settings/operations/reporting',
      '/settings/integrations/api',
      '/settings/system-pro',
    ]) {
      cleanup();
      navigateSpy.mockReset();
      renderHarness(path);

      await waitFor(() => {
        expect(navigateSpy).not.toHaveBeenCalled();
      });
    }
  });
});
