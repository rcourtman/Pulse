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

function evaluationPlan(overrides: Partial<ProviderPlanState> = {}): ProviderPlanState {
  return {
    plan_version: 'msp_eval',
    plan_source: 'license_file',
    evaluation: true,
    license_id: 'lic_msp_eval',
    expires_at: '2026-11-22T20:49:54Z',
    workspace_limit: 2,
    purchase_available: true,
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
    expect(renewing).toContain('Renews automatically');
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

  beforeEach(() => {
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
