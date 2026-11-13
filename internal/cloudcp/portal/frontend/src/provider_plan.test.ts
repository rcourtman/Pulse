import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createPortalStore } from './store';
import {
  formatProviderPlanPrice,
  installProviderPlan,
  PROVIDER_PLAN_ROOT_ID,
  renderProviderPlanHTML,
  type ProviderPlanAPI,
  type ProviderPlanState,
  type ProviderPlanView,
} from './provider_plan';
import type { PortalBootstrapData } from './types';

beforeEach(() => {
  // These saved licences describe an active November fixture. Keep its clock
  // fixed so CI cannot age it into grace/expiry; Date and timers stay real.
  vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-11-01T00:00:00Z'));
});

afterEach(() => {
  vi.restoreAllMocks();
});

function evaluationPlan(overrides: Partial<ProviderPlanState> = {}): ProviderPlanState {
  return {
    plan_version: 'msp_eval',
    plan_source: 'license_file',
    evaluation: true,
    license_id: 'lic_msp_eval',
    expires_at: '2026-11-22T20:49:54Z',
    workspace_limit: 2,
    purchase_available: true,
    plans_complete: true,
    plans: [
      { plan_version: 'msp_solo', billing_cycle: 'monthly', unit_amount: 6900, currency: 'usd', workspace_limit: 3 },
      { plan_version: 'msp_solo', billing_cycle: 'annual', unit_amount: 69000, currency: 'usd', workspace_limit: 3 },
      { plan_version: 'msp_starter', billing_cycle: 'monthly', unit_amount: 14900, currency: 'usd', workspace_limit: 5 },
      { plan_version: 'msp_scale', billing_cycle: 'annual', unit_amount: 399000, currency: 'usd', workspace_limit: 40 },
    ],
    ...overrides,
  };
}

function view(overrides: Partial<ProviderPlanView> = {}): ProviderPlanView {
  return { loading: false, error: '', plan: evaluationPlan(), cycle: 'monthly', busy: '', notice: '', ...overrides };
}

describe('renderProviderPlanHTML', () => {
  it('shows the evaluation, its expiry and the plans it can grow into', () => {
    const html = renderProviderPlanHTML(view(), true);
    expect(html).toContain('Free evaluation');
    expect(html).toContain('2 client workspaces');
    expect(html).toContain('Expires');
    expect(renderProviderPlanHTML(view(), true, 1)).toContain('1 of 2 client workspaces in use');
    expect(html).toContain('data-provider-plan-offer="msp_solo"');
    expect(html).toContain('Buy Solo');
    expect(html).toContain('$69/mo');
    expect(html).toContain('Buy Starter');
    // Annual-only plans stay behind the annual toggle.
    expect(html).not.toContain('data-provider-plan-offer="msp_scale"');
    expect(html).toContain('Annual, 2 months free');
  });

  it('switches every offer to annual pricing', () => {
    const html = renderProviderPlanHTML(view({ cycle: 'annual' }), true);
    expect(html).toContain('$690/yr');
    expect(html).toContain('$3,990/yr');
    expect(html).not.toContain('Buy Starter');
  });

  it('tells a read-only member who can buy instead of offering buttons', () => {
    const html = renderProviderPlanHTML(view(), false);
    expect(html).not.toContain('data-provider-plan-action="buy"');
    expect(html).not.toContain('Apply my purchase now');
    expect(html).toContain('Only an owner or admin of this account can buy a plan.');
  });

  it('points a paying provider to Manage billing and offers no second purchase', () => {
    const html = renderProviderPlanHTML(view({
      plan: evaluationPlan({ plan_version: 'msp_solo', evaluation: false, plan_source: 'renewed_license', workspace_limit: 3 }),
    }), true);
    expect(html).toContain('<h3>Solo</h3>');
    expect(html).toContain('Up to 3 client workspaces');
    expect(html).toContain('data-provider-plan-action="manage-billing"');
    expect(html).not.toContain('data-provider-plan-action="buy"');
    expect(html).toContain('Change plan, update your payment method or cancel renewal');
    expect(html).toContain('Changed your plan?');
    expect(html).not.toContain('Just paid?');
  });

  it('keeps the licence date out of a renewing plan and warns once renewal has stopped', () => {
    const now = Date.parse('2026-10-01T00:00:00Z');
    const renewing = renderProviderPlanHTML(view({
      plan: evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3, expires_at: '2026-11-06T00:00:00Z' }),
    }), true, 2, now);
    // A subscription cancelled for the period end looks the same here, so the
    // copy points to Manage billing instead of promising a renewal.
    expect(renewing).toContain('Open Manage billing to see when your subscription will renew or end.');
    expect(renewing).not.toContain('Renews automatically');
    expect(renewing).toContain('2 of 3 client workspaces in use');
    expect(renewing).not.toContain('2026');
    expect(renewing).not.toContain('Nov');

    const lapsed = renderProviderPlanHTML(view({
      plan: evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3, expires_at: '2026-10-10T00:00:00Z' }),
    }), true, 2, now);
    expect(lapsed).toContain('Your subscription has not renewed.');
    expect(lapsed).toContain('Open Manage billing to renew');
    expect(lapsed).not.toContain('Renews automatically');
  });

  it.each([
    { now: '2026-11-08T20:49:53.999Z', status: 'active' },
    { now: '2026-11-08T20:49:54Z', status: 'grace' },
    { now: '2026-11-22T20:49:53.999Z', status: 'grace' },
    { now: '2026-11-22T20:49:54Z', status: 'ended' },
    { now: '2027-01-01T00:00:00Z', status: 'ended' },
  ])('renders the paid plan as $status at $now through the default clock', ({ now, status }) => {
    vi.mocked(Date.now).mockReturnValue(Date.parse(now));
    const html = renderProviderPlanHTML(view({
      plan: evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3 }),
    }), true);
    const root = document.createElement('div');
    root.innerHTML = html;
    // After expiry the offers below this article still say "Up to 3". Judge
    // the current plan, not the independently available replacement plans.
    const current = root.querySelector('[data-provider-plan-current="msp_solo"]')!.textContent!;
    expect(current.includes('Up to 3 client workspaces')).toBe(status === 'active');
    expect(current.includes('Your subscription has not renewed.')).toBe(status === 'grace');
    expect(current.includes('Your Solo plan ended on')).toBe(status === 'ended');
    expect(html.includes('data-provider-plan-action="buy"')).toBe(status === 'ended');
    expect(html).toContain('data-provider-plan-action="manage-billing"');
    expect(html).not.toContain('Renews automatically');
  });

  it.each([
    { now: '2026-11-22T20:49:53.999Z', ended: false },
    { now: '2026-11-22T20:49:54Z', ended: true },
  ])('preserves evaluation expiry at $now through the default clock', ({ now, ended }) => {
    vi.mocked(Date.now).mockReturnValue(Date.parse(now));
    const html = renderProviderPlanHTML(view(), true);
    expect(html.includes('Your evaluation ended on')).toBe(ended);
    expect(html.includes('Your evaluation covers')).toBe(!ended);
    expect(html).toContain('data-provider-plan-action="buy"');
    expect(html).not.toContain('Your subscription has not renewed.');
  });

  // The control plane keeps serving the portal on a lapsed licence precisely
  // so the provider can buy here; the panel must say so and offer the plans.
  it('tells a provider whose evaluation ended what happened and how to buy', () => {
    const now = Date.parse('2026-12-10T00:00:00Z');
    const html = renderProviderPlanHTML(view({
      plan: evaluationPlan({ expires_at: '2026-11-22T20:49:54Z', lapsed: true }),
    }), true, 2, now);
    expect(html).toContain('Your evaluation ended on');
    expect(html).toContain('lost their MSP features, and no new clients can be added');
    expect(html).toContain('data-provider-plan-action="buy" data-provider-plan-version="msp_solo"');
    expect(html).not.toContain('Your evaluation covers');
  });

  it('offers the same plan again after a paid plan lapses', () => {
    const now = Date.parse('2026-12-10T00:00:00Z');
    const html = renderProviderPlanHTML(view({
      plan: evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3, expires_at: '2026-11-20T00:00:00Z', lapsed: true }),
    }), true, 3, now);
    expect(html).toContain('Your Solo plan ended on');
    expect(html).toContain('data-provider-plan-action="buy" data-provider-plan-version="msp_solo"');
    expect(html).toContain('data-provider-plan-action="buy" data-provider-plan-version="msp_starter"');
    expect(html).toContain('data-provider-plan-action="manage-billing"');
    expect(html).not.toContain('Renews automatically');
  });

  it('keeps the evaluation visible when plans cannot be loaded', () => {
    const html = renderProviderPlanHTML(view({ plan: evaluationPlan({ plans: [], purchase_available: false, plans_error: 'Plans are unavailable right now.' }) }), true);
    expect(html).toContain('Free evaluation');
    expect(html).toContain('Plans are unavailable right now.');
    expect(html).not.toContain('Choose a plan');
  });

  it.each([false, undefined])('keeps usable offers beside an incomplete/unknown %s catalogue warning', (complete) => {
    const html = renderProviderPlanHTML(view({ plan: evaluationPlan({ plans_complete: complete, plans_error: 'One price lookup failed.' }) }), true);
    expect(html).toContain('Buy Solo');
    expect(html).toContain('$69/mo');
    expect(html).toContain('Apply my purchase now');
    expect(html).toContain('One price lookup failed.');
    expect(html).toContain('data-provider-plan-action="reload-plans"');
  });

  it('distinguishes a complete closed catalogue from an unknown empty one', () => {
    const closed = renderProviderPlanHTML(view({ plan: evaluationPlan({ plans: [], purchase_available: false }) }), true);
    expect(closed).not.toContain('data-provider-plan-catalogue-warning');
    expect(closed).not.toContain('data-provider-plan-action="reload-plans"');
    const unknown = renderProviderPlanHTML(view({ plan: evaluationPlan({ plans: [], purchase_available: false, plans_complete: undefined }) }), true);
    expect(unknown).toContain('Full plan list not confirmed');
    expect(unknown).toContain('Some plans or billing periods may be missing');
  });

  it('defaults to a known annual offer when no monthly offer is listed', () => {
    const html = renderProviderPlanHTML(view({ plan: evaluationPlan({ plans_complete: false, plans: [evaluationPlan().plans[1]] }) }), true);
    const root = document.createElement('div'); root.innerHTML = html;
    expect(html).toContain('$690/yr');
    expect(root.querySelector<HTMLButtonElement>('[data-provider-plan-cycle="monthly"]')!.disabled).toBe(true);
    expect(root.querySelector('[data-provider-plan-cycle="monthly"]')!.textContent).toBe('Monthly (not listed)');
    expect(root.querySelector('[data-provider-plan-cycle="annual"]')!.getAttribute('aria-pressed')).toBe('true');
    expect(html).toContain('data-provider-plan-action="buy"');
  });

  it('makes a missing annual period explicit without hiding monthly offers', () => {
    const html = renderProviderPlanHTML(view({ cycle: 'annual', plan: evaluationPlan({ plans_complete: false, plans: [evaluationPlan().plans[0]] }) }), false);
    const root = document.createElement('div'); root.innerHTML = html;
    expect(html).toContain('$69/mo');
    expect(root.querySelector<HTMLButtonElement>('[data-provider-plan-cycle="annual"]')!.disabled).toBe(true);
    expect(html).toContain('Annual (not listed)');
    expect(html).toContain('Only an owner or admin');
    expect(html).not.toContain('data-provider-plan-action="buy"');
  });

  it('does not show stale offers after a transport failure, but keeps saved licence recovery', () => {
    const html = renderProviderPlanHTML(view({ error: 'Your plan could not be loaded.' }), true);
    expect(html).toContain('Free evaluation');
    expect(html).toContain('Apply my purchase now');
    expect(html).not.toContain('data-provider-plan-offer');
    expect(html).toContain('Your plan could not be loaded.');
    expect(html).toContain('data-provider-plan-action="reload-plans"');
  });

  it('formats amounts in the listed currency', () => {
    expect(formatProviderPlanPrice({ plan_version: 'msp_solo', billing_cycle: 'monthly', unit_amount: 6950, currency: 'eur', workspace_limit: 3 })).toBe('EUR 69.50/mo');
  });
});

function providerBootstrap(canManage = true): PortalBootstrapData {
  return {
    authenticated: true,
    email: 'owner@example.com',
    has_self_hosted_commercial: false,
    email_sign_in_available: false,
    provider_hosted_mode: true,
    public_site_url: 'https://pulserelay.pro',
    support_email: 'support@pulserelay.pro',
    commercial_api_base_url: '',
    commercial_api_base_path: '',
    portal_path: '/portal',
    bootstrap_path: '/api/portal/bootstrap',
    magic_link_request_path: '',
    signup_path: '',
    logout_path: '/auth/logout',
    account_api_base_path: '/api/accounts',
    portal_api_base_path: '/api/portal',
    accounts: [{
      id: 'a_1', name: 'Provider MSP', kind: 'msp', kind_label: 'MSP', role: canManage ? 'owner' : 'read_only',
      can_manage: canManage, has_billing: false, workspaces: [], members: [],
    }],
  } as unknown as PortalBootstrapData;
}

describe('installProviderPlan', () => {
  let api: ProviderPlanAPI;
  let navigate: ReturnType<typeof vi.fn<(url: string) => void>>;
  let toast: ReturnType<typeof vi.fn<(message: string, isError?: boolean) => void>>;
  let timers: Array<() => void>;
  let clickListeners: EventListenerOrEventListenerObject[];

  beforeEach(() => {
    // Controllers bind document clicks for the lifetime of a page. Each test
    // starts a new page fixture, so remove its handlers before the next one.
    clickListeners = [];
    const addEventListener = document.addEventListener.bind(document);
    vi.spyOn(document, 'addEventListener').mockImplementation((type, listener, options) => {
      addEventListener(type, listener, options);
      if (type === 'click') clickListeners.push(listener);
    });
    document.body.innerHTML = '<div id="' + PROVIDER_PLAN_ROOT_ID + '"></div>';
    navigate = vi.fn<(url: string) => void>();
    toast = vi.fn<(message: string, isError?: boolean) => void>();
    timers = [];
    api = {
      fetchPlan: vi.fn().mockResolvedValue(evaluationPlan()),
      startCheckout: vi.fn().mockResolvedValue({ url: 'https://checkout.stripe.com/c/pay/cs_test' }),
      openBillingPortal: vi.fn().mockResolvedValue({ url: 'https://billing.stripe.com/p/session/test' }),
      refreshLicense: vi.fn().mockResolvedValue({ status: 'no_paid_subscription' }),
    };
  });

  afterEach(() => {
    clickListeners.forEach((listener) => document.removeEventListener('click', listener));
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  function install(bootstrap: PortalBootstrapData, search = '') {
    const store = createPortalStore(bootstrap, bootstrap);
    const controller = installProviderPlan({
      api, store, showToast: toast, navigate,
      locationSearch: () => search,
      replaceSearch: () => {},
      setTimeoutFn: (fn: () => void) => { timers.push(fn); return 0; },
    });
    return { store, controller };
  }

  function click(selector: string) {
    const el = document.querySelector(selector) as HTMLElement | null;
    expect(el, selector).not.toBeNull();
    el!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  }

  async function flush() {
    for (let i = 0; i < 5; i += 1) await Promise.resolve();
  }

  it('loads the plan and sends Buy to checkout for this account', async () => {
    const { controller } = install(providerBootstrap());
    await controller.load();
    expect(document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML).toContain('Buy Solo');
    click('[data-provider-plan-action="buy"][data-provider-plan-version="msp_solo"]');
    await flush();
    expect(api.startCheckout).toHaveBeenCalledWith('a_1', 'msp_solo', 'monthly');
    expect(navigate).toHaveBeenCalledWith('https://checkout.stripe.com/c/pay/cs_test');
  });

  it('enables only a deliberate catalogue reload after the server retry window', async () => {
    const partial = evaluationPlan({ plans_complete: false, plans_retry_after_seconds: 60 });
    api.fetchPlan = vi.fn().mockResolvedValueOnce(partial).mockResolvedValueOnce(evaluationPlan());
    const { controller } = install(providerBootstrap());
    await controller.load();
    const reload = document.querySelector<HTMLButtonElement>('[data-provider-plan-action="reload-plans"]')!;
    const buy = document.querySelector<HTMLButtonElement>('[data-provider-plan-action="buy"]')!;
    expect(reload.disabled).toBe(true);
    click('[data-provider-plan-action="reload-plans"]'); await flush();
    expect(api.fetchPlan).toHaveBeenCalledTimes(1);
    expect(timers).toHaveLength(1);
    vi.mocked(Date.now).mockReturnValue(Date.parse('2026-11-01T00:01:00Z'));
    buy.focus(); timers.shift()!();
    expect(reload.disabled).toBe(false);
    expect(document.activeElement).toBe(buy);
    expect(document.querySelector('[data-provider-plan-action="buy"]')).toBe(buy);
    expect(api.fetchPlan).toHaveBeenCalledTimes(1);
    click('[data-provider-plan-action="reload-plans"]'); await flush();
    expect(api.fetchPlan).toHaveBeenCalledTimes(2);
    expect(document.querySelector('[data-provider-plan-catalogue-warning]')).toBeNull();
    expect(controller.view().plan!.plans_complete).toBe(true);
    expect(api.startCheckout).not.toHaveBeenCalled();
    expect(api.refreshLicense).not.toHaveBeenCalled();
  });

  it('admits one pending plan read and keeps deliberate retry after a route error', async () => {
    let reject!: (reason: Error) => void;
    api.fetchPlan = vi.fn().mockImplementationOnce(() => new Promise((_, r) => { reject = r; })).mockResolvedValueOnce(evaluationPlan());
    const { controller } = install(providerBootstrap());
    const pending = controller.load();
    await controller.load();
    expect(api.fetchPlan).toHaveBeenCalledTimes(1);
    reject(new Error('Plan request failed.')); await pending;
    expect(document.body.textContent).toContain('Plan request failed.');
    expect(timers).toHaveLength(1);
    vi.mocked(Date.now).mockReturnValue(Date.parse('2026-11-01T00:01:00Z')); timers.shift()!();
    click('[data-provider-plan-action="reload-plans"]'); await flush();
    expect(controller.view().error).toBe('');
    expect(document.body.textContent).toContain('Buy Solo');
    expect(api.fetchPlan).toHaveBeenCalledTimes(2);
  });

  it('permits read-only catalogue recovery without purchase or licence writes', async () => {
    api.fetchPlan = vi.fn().mockResolvedValueOnce(evaluationPlan({ plans_complete: false })).mockResolvedValueOnce(evaluationPlan());
    const { controller } = install(providerBootstrap(false)); await controller.load();
    click('[data-provider-plan-action="reload-plans"]'); await flush();
    expect(api.fetchPlan).toHaveBeenCalledTimes(2);
    expect(api.startCheckout).not.toHaveBeenCalled();
    expect(api.openBillingPortal).not.toHaveBeenCalled();
    expect(api.refreshLicense).not.toHaveBeenCalled();
    expect(document.querySelector('[data-provider-plan-action="buy"]')).toBeNull();
  });

  it('opens the billing portal for a paying provider', async () => {
    api.fetchPlan = vi.fn().mockResolvedValue(evaluationPlan({ plan_version: 'msp_starter', evaluation: false, workspace_limit: 5 }));
    const { controller } = install(providerBootstrap());
    await controller.load();
    click('[data-provider-plan-action="manage-billing"]');
    await flush();
    expect(api.openBillingPortal).toHaveBeenCalledWith('a_1');
    expect(navigate).toHaveBeenCalledWith('https://billing.stripe.com/p/session/test');
  });

  it('updates the manual apply view when a paid licence is already running', async () => {
    const paid = evaluationPlan({ plan_version: 'msp_solo', evaluation: false, license_id: 'lic_msp_paid', workspace_limit: 3 });
    api.fetchPlan = vi.fn()
      .mockResolvedValueOnce(evaluationPlan())
      .mockResolvedValueOnce(paid);
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: paid.plan_version, license_id: paid.license_id, expires_at: paid.expires_at,
    });
    const { controller } = install(providerBootstrap());
    await controller.load();
    click('[data-provider-plan-action="refresh"]');
    await flush();
    expect(controller.view().notice).toBe('Your Solo plan is active.');
    expect(controller.view().plan).toEqual(paid);
    expect(toast).not.toHaveBeenCalled();
  });

  it.each(['no_paid_subscription', 'unknown', undefined])(
    'does not infer payment history or discard a saved paid plan from refresh status %s',
    async (status) => {
      const paid = evaluationPlan({ plan_version: 'msp_solo', evaluation: false, license_id: 'lic_paid', workspace_limit: 3 });
      api.fetchPlan = vi.fn().mockResolvedValue(paid);
      api.refreshLicense = vi.fn().mockResolvedValue({ status });
      const { controller } = install(providerBootstrap());
      await controller.load();
      const savedHTML = document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML;
      click('[data-provider-plan-action="refresh"]');
      await flush();
      expect(toast).toHaveBeenCalledExactlyOnceWith('No updated plan could be confirmed. If you just paid, it can take a minute after checkout.');
      expect(controller.view().plan).toEqual(paid);
      expect(document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML).toBe(savedHTML);
      expect(api.refreshLicense).toHaveBeenCalledExactlyOnceWith('a_1');
      expect(api.fetchPlan).toHaveBeenCalledTimes(1);
      expect(navigate).not.toHaveBeenCalled();
      expect(timers).toHaveLength(0);
    },
  );

  it('keeps the checkout-delay hint and evaluation offers after an unconfirmed manual refresh', async () => {
    const { controller } = install(providerBootstrap());
    await controller.load();
    const savedHTML = document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML;
    click('[data-provider-plan-action="refresh"]');
    await flush();
    expect(toast).toHaveBeenCalledExactlyOnceWith('No updated plan could be confirmed. If you just paid, it can take a minute after checkout.');
    expect(controller.view().plan).toEqual(evaluationPlan());
    expect(document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML).toBe(savedHTML);
    expect(api.refreshLicense).toHaveBeenCalledExactlyOnceWith('a_1');
    expect(timers).toHaveLength(0);
  });

  it('keeps the existing retry message when an active refresh cannot be confirmed', async () => {
    api.refreshLicense = vi.fn().mockResolvedValue({ status: 'active' });
    const { controller } = install(providerBootstrap());
    await controller.load();
    click('[data-provider-plan-action="refresh"]');
    await flush();
    expect(toast).toHaveBeenCalledExactlyOnceWith('The updated plan could not be confirmed yet. Try again shortly.');
    expect(controller.view().plan).toEqual(evaluationPlan());
    expect(timers).toHaveLength(0);
  });

  it('applies a purchase after checkout returns and shows the new plan once the platform restarts', async () => {
    api.refreshLicense = vi.fn()
      .mockResolvedValueOnce({ status: 'no_paid_subscription' })
      .mockResolvedValueOnce({ status: 'active', changed: true, restart_scheduled: true, plan_version: 'msp_solo' });
    const { store, controller } = install(providerBootstrap(), '?provider_msp_checkout=complete');
    await controller.load();
    expect(store.getShellState().activeSection).toBe('billing');
    expect(controller.view().notice).toBe('Checking for your paid plan…');
    await flush();
    expect(api.refreshLicense).toHaveBeenCalledTimes(1);
    timers.shift()!();
    await flush();
    expect(api.refreshLicense).toHaveBeenCalledTimes(2);
    expect(controller.view().notice).toContain('being applied');

    // While the control plane restarts the plan is still the old one, then
    // the new plan appears without a page reload.
    api.fetchPlan = vi.fn()
      .mockResolvedValueOnce(evaluationPlan())
      .mockResolvedValueOnce(evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3 }));
    timers.shift()!();
    await flush();
    timers.shift()!();
    await flush();
    expect(controller.view().notice).toBe('Your Solo plan is active.');
    expect(document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML).toContain('Manage billing');
  });

  it('does not claim payment from the checkout return when no paid subscription appears', async () => {
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=complete');
    await controller.load();
    await flush();
    for (let attempt = 1; attempt < 12; attempt += 1) {
      expect(timers).toHaveLength(1);
      timers.shift()!();
      await flush();
    }
    expect(api.refreshLicense).toHaveBeenCalledTimes(12);
    expect(timers).toHaveLength(0);
    expect(controller.view().notice).toContain('could not be confirmed yet');
    expect(controller.view().notice).not.toContain('Payment received');
    expect(controller.view().plan?.evaluation).toBe(true);
  });

  it('confirms an already-applied paid licence without requiring another restart', async () => {
    const paid = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_msp_paid', workspace_limit: 3,
    });
    api.fetchPlan = vi.fn().mockResolvedValue(paid);
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: paid.plan_version, license_id: paid.license_id, expires_at: paid.expires_at,
    });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=complete');
    await controller.load();
    await flush();
    expect(controller.view().notice).toBe('Your Solo plan is active.');
    expect(controller.view().plan).toEqual(paid);
    expect(timers).toHaveLength(0);
  });

  it('waits for the matching running plan even when refresh reports an active licence', async () => {
    const paid = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_msp_paid', workspace_limit: 3,
    });
    api.fetchPlan = vi.fn()
      .mockResolvedValueOnce(evaluationPlan())
      .mockResolvedValueOnce(evaluationPlan())
      .mockResolvedValueOnce(paid);
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: paid.plan_version, license_id: paid.license_id, expires_at: paid.expires_at,
    });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=complete');
    await controller.load();
    await flush();
    expect(controller.view().notice).toBe('Checking for your paid plan…');
    expect(timers).toHaveLength(1);
    timers.shift()!();
    await flush();
    expect(controller.view().notice).toBe('Your Solo plan is active.');
  });

  it('does not announce the old plan when refresh beats the initial plan request', async () => {
    let resolveInitial!: (plan: ProviderPlanState) => void;
    api.fetchPlan = vi.fn()
      .mockImplementationOnce(() => new Promise<ProviderPlanState>((resolve) => { resolveInitial = resolve; }))
      .mockResolvedValueOnce(evaluationPlan())
      .mockResolvedValueOnce(evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3 }));
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: true, restart_scheduled: true, plan_version: 'msp_solo',
    });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=complete');
    const initialLoad = controller.load();
    await flush();
    expect(controller.view().notice).toContain('being applied');

    timers.shift()!();
    await flush();
    expect(controller.view().notice).not.toContain('plan is active');
    expect(timers).toHaveLength(1);

    timers.shift()!();
    await flush();
    expect(controller.view().notice).toBe('Your Solo plan is active.');
    resolveInitial(evaluationPlan());
    await initialLoad;
    expect(controller.view().plan?.plan_version).toBe('msp_solo');
  });

  it('waits for the renewed licence date when the plan name is unchanged', async () => {
    const original = evaluationPlan({ plan_version: 'msp_solo', evaluation: false, expires_at: '2026-10-10T00:00:00Z' });
    const renewed = evaluationPlan({ plan_version: 'msp_solo', evaluation: false, expires_at: '2026-11-10T00:00:00Z' });
    let served = original;
    api.fetchPlan = vi.fn().mockImplementation(() => Promise.resolve(served));
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: true, restart_scheduled: true,
      plan_version: 'msp_solo', expires_at: renewed.expires_at,
    });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    await flush();
    timers.shift()!();
    await flush();
    expect(controller.view().notice).not.toContain('plan is active');
    served = renewed;
    timers.shift()!();
    await flush();
    expect(controller.view().plan?.expires_at).toBe(renewed.expires_at);
    expect(controller.view().notice).toBe('Your Solo plan is active.');
  });

  it('applies a plan changed in Manage billing when the provider comes back', async () => {
    api.fetchPlan = vi.fn().mockResolvedValue(evaluationPlan({ plan_version: 'msp_solo', evaluation: false, workspace_limit: 3 }));
    api.refreshLicense = vi.fn()
      .mockResolvedValueOnce({ status: 'active', changed: false })
      .mockResolvedValueOnce({ status: 'active', changed: true, restart_scheduled: true, plan_version: 'msp_starter' });
    const { store, controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    expect(store.getShellState().activeSection).toBe('billing');
    expect(controller.view().notice).toBe('');
    await flush();
    expect(api.refreshLicense).toHaveBeenCalledTimes(1);
    timers.shift()!();
    await flush();
    expect(api.refreshLicense).toHaveBeenCalledTimes(2);
    expect(controller.view().notice).toContain('being applied');
    expect(toast).not.toHaveBeenCalled();
  });

  it('confirms a paid plan already applied by the background refresh on a billing return', async () => {
    const oldPlan = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_old',
      workspace_limit: 3, expires_at: '2026-10-10T00:00:00Z',
    });
    const newPlan = evaluationPlan({
      plan_version: 'msp_starter', evaluation: false, license_id: 'lic_new',
      workspace_limit: 5, expires_at: '2026-11-10T00:00:00Z',
    });
    let resolveRefresh!: (result: Awaited<ReturnType<ProviderPlanAPI['refreshLicense']>>) => void;
    api.refreshLicense = vi.fn().mockImplementation(() => new Promise((resolve) => { resolveRefresh = resolve; }));
    api.fetchPlan = vi.fn().mockResolvedValueOnce(oldPlan).mockResolvedValueOnce(newPlan);
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    expect(controller.view().plan).toEqual(oldPlan);
    expect(controller.view().notice).toBe('');

    resolveRefresh({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: newPlan.plan_version, license_id: newPlan.license_id, expires_at: newPlan.expires_at,
    });
    await flush();
    expect(controller.view().plan).toEqual(newPlan);
    expect(controller.view().notice).toBe('Your Starter plan is active.');
    expect(timers).toHaveLength(0);
    expect(toast).not.toHaveBeenCalled();
  });

  it('does not announce an unchanged paid plan from a Manage billing visit', async () => {
    const paid = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_paid', workspace_limit: 3,
    });
    api.fetchPlan = vi.fn().mockResolvedValue(paid);
    api.refreshLicense = vi.fn().mockResolvedValue({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: paid.plan_version, license_id: paid.license_id, expires_at: paid.expires_at,
    });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    await flush();
    expect(controller.view().notice).toBe('');
    expect(timers).toHaveLength(1);
    timers.shift()!();
    await flush();
    expect(controller.view().notice).toBe('');
    expect(controller.view().plan).toEqual(paid);
    expect(timers).toHaveLength(0);
  });

  it('confirms a renewed paid period but not an unmatched refresh on a billing return', async () => {
    const oldPlan = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_paid',
      workspace_limit: 3, expires_at: '2026-10-10T00:00:00Z',
    });
    const renewed = { ...oldPlan, expires_at: '2026-11-10T00:00:00Z' };
    let served = oldPlan;
    api.fetchPlan = vi.fn().mockImplementation(() => Promise.resolve(served));
    let resolveRefresh!: (result: Awaited<ReturnType<ProviderPlanAPI['refreshLicense']>>) => void;
    api.refreshLicense = vi.fn().mockImplementationOnce(() => new Promise((resolve) => { resolveRefresh = resolve; }))
      .mockResolvedValue({
        status: 'active', changed: false, restart_scheduled: false,
        plan_version: renewed.plan_version, license_id: renewed.license_id, expires_at: renewed.expires_at,
      });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    resolveRefresh({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: renewed.plan_version, license_id: renewed.license_id, expires_at: renewed.expires_at,
    });
    await flush();
    expect(controller.view().plan).toEqual(oldPlan);
    expect(controller.view().notice).toBe('');
    expect(timers).toHaveLength(1);
    served = renewed;
    timers.shift()!();
    await flush();
    expect(controller.view().plan).toEqual(renewed);
    expect(controller.view().notice).toBe('Your Solo plan is active.');
    expect(timers).toHaveLength(0);
  });

  it('keeps the second billing check when refresh finishes before the first plan load', async () => {
    const oldPlan = evaluationPlan({
      plan_version: 'msp_solo', evaluation: false, license_id: 'lic_old', workspace_limit: 3,
    });
    const newPlan = evaluationPlan({
      plan_version: 'msp_starter', evaluation: false, license_id: 'lic_new', workspace_limit: 5,
    });
    let served = oldPlan;
    api.fetchPlan = vi.fn().mockImplementation(() => Promise.resolve(served));
    api.refreshLicense = vi.fn().mockImplementation(() => Promise.resolve({
      status: 'active', changed: false, restart_scheduled: false,
      plan_version: served.plan_version, license_id: served.license_id, expires_at: served.expires_at,
    }));
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await flush();
    expect(controller.view().plan).toEqual(oldPlan);
    expect(controller.view().notice).toBe('');
    expect(timers).toHaveLength(1);

    served = newPlan;
    timers.shift()!();
    await flush();
    expect(controller.view().plan).toEqual(newPlan);
    expect(controller.view().notice).toBe('Your Starter plan is active.');
    expect(timers).toHaveLength(0);
  });

  it('checks a Manage billing return only twice when nothing changed', async () => {
    api.refreshLicense = vi.fn().mockResolvedValue({ status: 'active', changed: false });
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=billing');
    await controller.load();
    await flush();
    timers.shift()!();
    await flush();
    expect(api.refreshLicense).toHaveBeenCalledTimes(2);
    expect(timers).toHaveLength(0);
    expect(controller.view().notice).toBe('');
  });

  it('does not infer a charge outcome from a cancelled checkout return', () => {
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=cancelled');
    expect(controller.view().notice).toBe('Checkout was not completed here. Check your current plan below.');
    expect(api.refreshLicense).not.toHaveBeenCalled();
  });

  it('ignores an unrecognised checkout return value', () => {
    const { controller } = install(providerBootstrap(), '?provider_msp_checkout=unknown');
    expect(controller.view().notice).toBe('');
    expect(api.refreshLicense).not.toHaveBeenCalled();
  });

  it('does nothing outside provider-hosted mode', async () => {
    const bootstrap = providerBootstrap();
    bootstrap.provider_hosted_mode = false;
    const { controller } = install(bootstrap);
    await controller.load();
    expect(api.fetchPlan).not.toHaveBeenCalled();
    expect(document.getElementById(PROVIDER_PLAN_ROOT_ID)!.innerHTML).toBe('');
  });
});
