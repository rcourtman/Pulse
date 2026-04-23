import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { State } from '@/types/api';
import { AppLayout } from '@/AppLayout';

HTMLElement.prototype.scrollIntoView = vi.fn();

describe('AppLayout navigation icons', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/dashboard');
  });

  afterEach(() => {
    cleanup();
  });

  const renderLayout = () =>
    render(() => (
      <Router>
        <Route
          path="/dashboard"
          component={() => (
            <AppLayout
              connectionStatus={() => ({
                kind: 'connected',
                label: 'Connected',
                detail: 'Backend and live data stream are connected.',
                tone: 'healthy',
              })}
              lastUpdateText={() => ''}
              versionInfo={() =>
                ({
                  version: '6.0.0-rc.2',
                  channel: 'rc',
                  isDevelopment: false,
                  isDocker: false,
                }) as never
              }
              hasAuth={() => true}
              needsAuth={() => false}
              proxyAuthInfo={() => null}
              handleLogout={() => {}}
              state={() =>
                ({
                  activeAlerts: [{ id: 'alert-1', level: 'warning', acknowledged: false }],
                }) as unknown as State
              }
              tokenScopes={() => ['settings:read']}
              organizations={() => []}
              activeOrgID={() => 'default'}
              orgsLoading={() => false}
              showOrgSwitcher={() => false}
              onSwitchOrg={() => {}}
            >
              <div>Dashboard body</div>
            </AppLayout>
          )}
        />
      </Router>
    ));

  it('renders fresh utility icons for both desktop and mobile navigation trees', () => {
    const { container } = renderLayout();

    const desktopNav = screen.getByRole('tablist', { name: 'Primary navigation' });
    const systemGroup = desktopNav.querySelector('[aria-label="System"]');
    expect(systemGroup).toBeTruthy();

    const desktopTabs = within(systemGroup as HTMLElement).getAllByRole('tab');
    expect(desktopTabs).toHaveLength(3);
    desktopTabs.forEach((tab) => {
      expect(tab.querySelector('svg')).toBeTruthy();
    });

    const mobileTablist = screen.getByRole('tablist', { name: 'Mobile navigation' });
    ['alerts', 'ai', 'settings'].forEach((tabId) => {
      const button = mobileTablist.querySelector<HTMLElement>(`[data-tab-id="${tabId}"]`);
      expect(button).toBeTruthy();
      expect(button?.querySelector('svg')).toBeTruthy();
    });

    expect(container).toHaveTextContent('Dashboard body');
  });

  it('keeps connected brand motion on the logo while the wordmark stays static', () => {
    const { container } = renderLayout();

    const brandLockup = screen.getByTestId('pulse-brand-lockup');
    expect(brandLockup).toHaveClass('animate-pulse-brand');
    expect(brandLockup.querySelector('.pulse-brand-logo')).toBeTruthy();
    const wordmark = brandLockup.querySelector('.pulse-brand-wordmark');
    expect(wordmark).toHaveTextContent('Pulse');
    expect(wordmark).not.toHaveClass('animate-pulse-brand');
    expect(container.querySelector('.animate-pulse-logo')).toBeNull();
  });
});
