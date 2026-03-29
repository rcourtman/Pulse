import { beforeEach, describe, expect, it, vi } from 'vitest';

import { installShell } from './shell';
import { createPortalStore } from './store';
import type { PortalBootstrapData } from './types';

const bootstrapDefaults: Omit<PortalBootstrapData, 'authenticated' | 'email' | 'accounts'> = {
  has_self_hosted_commercial: false,
  public_site_url: 'https://pulserelay.pro',
  support_email: 'support@pulserelay.pro',
  commercial_api_base_url: '/api/portal/commercial',
  portal_path: '/portal',
  bootstrap_path: '/api/portal/bootstrap',
  magic_link_request_path: '/api/public/magic-link/request',
  signup_path: '/signup',
  logout_path: '/auth/logout',
  account_api_base_path: '/api/accounts',
  portal_api_base_path: '/api/portal',
};

describe('shell runtime', function() {
  beforeEach(function() {
    document.body.innerHTML = `
      <div id="portal-user-info"></div>
      <div id="portal-app-root"></div>
    `;
    vi.restoreAllMocks();
  });

  it('reveals the active task in the compact mobile nav strip when the section changes', function() {
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: [
        {
          id: 'acct_1',
          name: 'Acme MSP',
          kind: 'msp',
          kind_label: 'MSP',
          role: 'owner',
          can_manage: true,
          has_billing: true,
          members: [],
          workspaces: [],
        },
      ],
    });
    var scrollIntoView = vi.fn();
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    });

    installShell({ store: store });

    var navGroup = document.querySelector('.portal-tab-bar') as HTMLElement | null;
    expect(navGroup).not.toBeNull();
    if (!navGroup) return;

    Object.defineProperty(navGroup, 'scrollWidth', {
      configurable: true,
      get: function() {
        return 640;
      },
    });
    Object.defineProperty(navGroup, 'clientWidth', {
      configurable: true,
      get: function() {
        return 220;
      },
    });

    scrollIntoView.mockClear();
    store.setActiveShellSection('access');

    var activeLink = document.querySelector('.portal-tab.active') as HTMLElement | null;
    expect(activeLink?.getAttribute('data-shell-section')).toBe('access');
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', inline: 'center' });
  });

  it('opens hosted accounts on workspaces by default', function() {
    var store = createPortalStore(bootstrapDefaults, {
      authenticated: true,
      email: 'owner@example.com',
      accounts: [
        {
          id: 'acct_default',
          name: 'Acme MSP',
          kind: 'msp',
          kind_label: 'MSP',
          role: 'owner',
          can_manage: true,
          has_billing: true,
          members: [],
          workspaces: [],
        },
      ],
    });

    installShell({ store: store });

    var activeLink = document.querySelector('.portal-tab.active') as HTMLElement | null;
    expect(activeLink?.getAttribute('data-shell-section')).toBe('workspaces');
  });
});
