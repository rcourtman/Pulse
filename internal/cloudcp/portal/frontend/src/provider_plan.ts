import { escapeAttribute, escapeText } from './billing_view';
import type { PortalStore } from './store';
import type { PortalAccountSummary } from './types';

// Plan and purchase for a provider-hosted MSP platform. The control plane
// relays to the licence server; nothing here talks to Pulse or Stripe
// directly, and the plan list is whatever the licence server sells now.

export interface ProviderPlanOption {
  plan_version: string;
  billing_cycle: 'monthly' | 'annual' | string;
  unit_amount: number;
  currency: string;
  workspace_limit: number;
}

export interface ProviderPlanState {
  plan_version: string;
  plan_source: string;
  evaluation: boolean;
  license_id?: string;
  expires_at?: string;
  workspace_limit: number;
  purchase_available: boolean;
  plans: ProviderPlanOption[];
  plans_error?: string;
}

export interface ProviderPlanView {
  loading: boolean;
  error: string;
  plan: ProviderPlanState | null;
  cycle: 'monthly' | 'annual';
  busy: string;
  notice: string;
}

export const PROVIDER_PLAN_ROOT_ID = 'provider-plan-root';

const PLAN_NAMES: Record<string, string> = {
  msp_eval: 'Free evaluation',
  msp_solo: 'Solo',
  msp_starter: 'Starter',
  msp_growth: 'Growth',
  msp_scale: 'Scale',
};

export function providerPlanName(planVersion: string): string {
  return PLAN_NAMES[planVersion] || planVersion;
}

export function formatProviderPlanPrice(option: ProviderPlanOption): string {
  var amount = Math.round(Number(option.unit_amount || 0)) / 100;
  var whole = amount % 1 === 0;
  var currency = String(option.currency || 'usd').toUpperCase();
  var symbol = currency === 'USD' ? '$' : currency + ' ';
  var formatted = symbol + (whole ? amount.toFixed(0) : amount.toFixed(2)).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return formatted + (option.billing_cycle === 'annual' ? '/yr' : '/mo');
}

function formatDate(value: string | undefined): string {
  if (!value) return '';
  var date = new Date(value);
  if (isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

function clientWorkspaces(count: number): string {
  return count === 1 ? '1 client workspace' : String(count) + ' client workspaces';
}

// A paid licence is signed to the paid period end plus 14 days of grace and
// re-signed as the subscription renews, so one that ends inside that window
// means the subscription has stopped renewing.
const PAID_GRACE_MS = 14 * 24 * 60 * 60 * 1000;

function paidLicenceInGrace(plan: ProviderPlanState, now: number): boolean {
  if (!plan.expires_at) return false;
  var expiresAt = new Date(plan.expires_at).getTime();
  return !isNaN(expiresAt) && expiresAt - now <= PAID_GRACE_MS;
}

function renderCurrentPlan(plan: ProviderPlanState, canManage: boolean, busy: string, inUse: number, now: number): string {
  var expires = formatDate(plan.expires_at);
  var title = providerPlanName(plan.plan_version);
  var description: string;
  var meta: string[] = [inUse >= 0 ? String(inUse) + ' of ' + clientWorkspaces(plan.workspace_limit) + ' in use' : clientWorkspaces(plan.workspace_limit)];
  var cta = '';
  if (plan.evaluation) {
    description = expires
      ? 'Your evaluation covers ' + clientWorkspaces(plan.workspace_limit) + ' until ' + expires + '. Buy a plan to keep your clients monitored after that and to add more.'
      : 'Your evaluation covers ' + clientWorkspaces(plan.workspace_limit) + '. Buy a plan to add more.';
    if (expires) meta.push('Expires ' + expires);
  } else {
    // The renewal date lives in Manage billing; the licence date only
    // matters, and only shows, once the subscription has stopped renewing.
    description = paidLicenceInGrace(plan, now)
      ? 'Your subscription has not renewed. Your clients keep this plan until ' + expires + '. Open Manage billing to renew or update your payment method.'
      : 'Up to ' + clientWorkspaces(plan.workspace_limit) + '. Renews automatically while your subscription is active.';
    if (canManage) {
      cta = '<button class="btn-secondary billing-action-button" type="button" data-provider-plan-action="manage-billing"' +
        (busy ? ' disabled' : '') + '>' + (busy === 'manage-billing' ? 'Opening…' : 'Manage billing') + '</button>';
    }
  }
  return (
    '<article class="billing-action-row" data-provider-plan-current="' + escapeAttribute(plan.plan_version) + '">' +
      '<div class="billing-action-main">' +
        '<div class="billing-action-copy">' +
          '<h3>' + escapeText(title) + '</h3>' +
          '<p>' + escapeText(description) + '</p>' +
        '</div>' +
        '<div class="billing-action-meta">' + escapeText(meta.join(' • ')) + '</div>' +
      '</div>' +
      (cta ? '<div class="billing-action-cta">' + cta + '</div>' : '') +
    '</article>'
  );
}

function renderCycleToggle(cycle: 'monthly' | 'annual', hasAnnual: boolean): string {
  if (!hasAnnual) return '';
  function button(value: 'monthly' | 'annual', label: string) {
    var active = value === cycle;
    return '<button class="' + (active ? 'btn-primary' : 'btn-secondary') + ' btn-compact" type="button" data-provider-plan-action="select-cycle" data-provider-plan-cycle="' + value + '" aria-pressed="' + (active ? 'true' : 'false') + '">' + label + '</button>';
  }
  return '<div class="billing-action-meta" role="group" aria-label="Billing period">' + button('monthly', 'Monthly') + ' ' + button('annual', 'Annual, 2 months free') + '</div>';
}

function renderOffer(option: ProviderPlanOption, canManage: boolean, busy: string): string {
  var name = providerPlanName(option.plan_version);
  var action = canManage
    ? '<button class="btn-primary billing-action-button" type="button" data-provider-plan-action="buy" data-provider-plan-version="' + escapeAttribute(option.plan_version) + '" data-provider-plan-cycle="' + escapeAttribute(option.billing_cycle) + '"' + (busy ? ' disabled' : '') + '>' +
      (busy === 'buy:' + option.plan_version ? 'Opening checkout…' : 'Buy ' + escapeText(name)) + '</button>'
    : '';
  return (
    '<article class="billing-action-row" data-provider-plan-offer="' + escapeAttribute(option.plan_version) + '">' +
      '<div class="billing-action-main">' +
        '<div class="billing-action-copy">' +
          '<h3>' + escapeText(name) + '</h3>' +
          '<p>Up to ' + escapeText(clientWorkspaces(option.workspace_limit)) + '.</p>' +
        '</div>' +
        '<div class="billing-action-meta">' + escapeText(formatProviderPlanPrice(option)) + '</div>' +
      '</div>' +
      (action ? '<div class="billing-action-cta">' + action + '</div>' : '') +
    '</article>'
  );
}

export function renderProviderPlanHTML(view: ProviderPlanView, canManage: boolean, inUse = -1, now = Date.now()): string {
  var parts: string[] = ['<div class="billing-section-intro"><h2>Plan</h2></div>'];
  if (view.notice) {
    parts.push('<p class="billing-action-meta" role="status">' + escapeText(view.notice) + '</p>');
  }
  if (view.loading && !view.plan) {
    parts.push('<p class="billing-action-meta">Loading your plan…</p>');
    return parts.join('');
  }
  if (view.error && !view.plan) {
    parts.push('<p class="billing-action-meta" role="alert">' + escapeText(view.error) + '</p>');
    return parts.join('');
  }
  var plan = view.plan;
  if (!plan) return parts.join('');
  parts.push(renderCurrentPlan(plan, canManage, view.busy, inUse, now));

  if (plan.evaluation) {
    if (plan.plans_error) {
      parts.push('<p class="billing-action-meta" role="alert">' + escapeText(plan.plans_error) + '</p>');
    } else if (plan.purchase_available) {
      var hasAnnual = plan.plans.some(function(option) { return option.billing_cycle === 'annual'; });
      var cycle = hasAnnual ? view.cycle : 'monthly';
      var offers = plan.plans.filter(function(option) {
        return option.billing_cycle === cycle && option.workspace_limit > plan!.workspace_limit;
      });
      parts.push('<div class="billing-section-intro"><h3>Choose a plan</h3><p>You pay per client workspace, never per monitored system. Every client workspace is full Pulse.</p></div>');
      parts.push(renderCycleToggle(cycle, hasAnnual));
      parts.push(offers.map(function(option) { return renderOffer(option, canManage, view.busy); }).join(''));
      if (!canManage) {
        parts.push('<p class="billing-action-meta">Only an owner or admin of this account can buy a plan.</p>');
      }
    }
  } else if (canManage) {
    parts.push('<p class="billing-action-meta">Change plan, update your payment method or cancel renewal from Manage billing.</p>');
  }

  if (canManage && plan.license_id) {
    parts.push(
      '<p class="billing-action-meta">' + (plan.evaluation ? 'Just paid? ' : 'Changed your plan? ') +
        '<button class="btn-secondary btn-compact" type="button" data-provider-plan-action="refresh"' + (view.busy ? ' disabled' : '') + '>' +
          (view.busy === 'refresh' ? 'Checking…' : (plan.evaluation ? 'Apply my purchase now' : 'Apply it now')) +
        '</button>' +
      '</p>'
    );
  }
  return parts.join('');
}

export interface ProviderPlanAPI {
  fetchPlan(accountID: string): Promise<ProviderPlanState>;
  startCheckout(accountID: string, planVersion: string, billingCycle: string): Promise<{ url?: string }>;
  openBillingPortal(accountID: string): Promise<{ url?: string }>;
  refreshLicense(accountID: string): Promise<{ status?: string; changed?: boolean; restart_scheduled?: boolean }>;
}

export interface ProviderPlanDeps {
  api: ProviderPlanAPI;
  store: PortalStore;
  showToast: (message: string, isError?: boolean) => void;
  navigate?: (url: string) => void;
  locationSearch?: () => string;
  replaceSearch?: (search: string) => void;
  setTimeoutFn?: (fn: () => void, ms: number) => unknown;
}

export interface ProviderPlanController {
  load(): Promise<void>;
  render(): void;
  view(): ProviderPlanView;
}

function providerAccount(accounts: PortalAccountSummary[] | undefined): PortalAccountSummary | null {
  var list = Array.isArray(accounts) ? accounts : [];
  for (var i = 0; i < list.length; i += 1) {
    if (list[i] && list[i].can_manage) return list[i];
  }
  return list[0] || null;
}

const CHECKOUT_RETURN_PARAM = 'provider_msp_checkout';
const APPLY_POLL_MS = 5000;
const APPLY_POLL_ATTEMPTS = 12;
// Most visits to Manage billing change nothing, so a return checks twice (the
// plan-change webhook can trail the redirect) rather than polling.
const BILLING_RETURN_ATTEMPTS = 2;
// The control plane restarts to apply a changed licence; give it a moment,
// then poll the plan until it answers with the new one.
const RESTART_SETTLE_MS = 8000;
const RESTART_POLL_MS = 3000;
const RESTART_POLL_ATTEMPTS = 20;

export function installProviderPlan(deps: ProviderPlanDeps): ProviderPlanController {
  var navigate = deps.navigate || function(url: string) { window.location.assign(url); };
  var later = deps.setTimeoutFn || function(fn: () => void, ms: number) { return setTimeout(fn, ms); };
  var view: ProviderPlanView = { loading: false, error: '', plan: null, cycle: 'monthly', busy: '', notice: '' };

  function account(): PortalAccountSummary | null {
    var bootstrap = deps.store.getBootstrap();
    if (bootstrap.provider_hosted_mode !== true || !bootstrap.authenticated) return null;
    return providerAccount(bootstrap.accounts);
  }

  function render() {
    var root = document.getElementById(PROVIDER_PLAN_ROOT_ID);
    var current = account();
    if (!root || !current) return;
    var workspaces = Array.isArray(current.workspaces) ? current.workspaces : [];
    root.innerHTML = renderProviderPlanHTML(view, current.can_manage === true, workspaces.length);
  }

  async function load() {
    var current = account();
    if (!current) return;
    view.loading = true;
    render();
    try {
      view.plan = await deps.api.fetchPlan(current.id);
      view.error = '';
    } catch (error) {
      view.error = error instanceof Error && error.message ? error.message : 'Your plan could not be loaded.';
    } finally {
      view.loading = false;
      render();
    }
  }

  function waitForAppliedPlan(previousPlan: string, attempt: number) {
    var current = account();
    if (!current) return;
    deps.api.fetchPlan(current.id).then(function(plan) {
      if (plan.plan_version !== previousPlan) {
        view.plan = plan;
        view.error = '';
        view.notice = 'Your ' + providerPlanName(plan.plan_version) + ' plan is active.';
        render();
        return;
      }
      throw new Error('plan not applied yet');
    }).catch(function() {
      if (attempt + 1 >= RESTART_POLL_ATTEMPTS) {
        view.notice = 'Your plan should be active now. Open this page again if it still shows the old plan.';
        render();
        return;
      }
      later(function() { waitForAppliedPlan(previousPlan, attempt + 1); }, RESTART_POLL_MS);
    });
  }

  function applyRestart() {
    var previousPlan = view.plan ? view.plan.plan_version : '';
    view.notice = 'Your plan is being applied. This takes a few seconds.';
    render();
    later(function() { waitForAppliedPlan(previousPlan, 0); }, RESTART_SETTLE_MS);
  }

  async function refresh(manual: boolean): Promise<boolean> {
    var current = account();
    if (!current) return false;
    view.busy = 'refresh';
    render();
    try {
      var result = await deps.api.refreshLicense(current.id);
      if (result.restart_scheduled) {
        applyRestart();
        return true;
      }
      if (manual) {
        deps.showToast(result.status === 'active' ? 'Your plan is already up to date.' : 'No payment has reached this platform yet. It can take a minute after checkout.');
      }
      return false;
    } catch (error) {
      if (manual) deps.showToast(error instanceof Error ? error.message : 'Refresh failed.', true);
      return false;
    } finally {
      view.busy = '';
      render();
    }
  }

  function pollAfterCheckout(attempt: number) {
    void refresh(false).then(function(applied) {
      if (applied) return;
      if (attempt + 1 >= APPLY_POLL_ATTEMPTS) {
        view.notice = 'Payment received. If your plan has not changed in a few minutes, use Apply my purchase now.';
        render();
        return;
      }
      later(function() { pollAfterCheckout(attempt + 1); }, APPLY_POLL_MS);
    });
  }

  function checkAfterBillingReturn(attempt: number) {
    void refresh(false).then(function(applied) {
      if (applied || attempt + 1 >= BILLING_RETURN_ATTEMPTS) return;
      later(function() { checkAfterBillingReturn(attempt + 1); }, APPLY_POLL_MS);
    });
  }

  document.addEventListener('click', function(event) {
    var target = event.target instanceof HTMLElement ? event.target.closest('[data-provider-plan-action]') as HTMLElement | null : null;
    if (!target) return;
    var current = account();
    if (!current) return;
    event.preventDefault();
    var action = target.getAttribute('data-provider-plan-action') || '';
    switch (action) {
      case 'select-cycle':
        view.cycle = target.getAttribute('data-provider-plan-cycle') === 'annual' ? 'annual' : 'monthly';
        render();
        return;
      case 'buy': {
        var planVersion = target.getAttribute('data-provider-plan-version') || '';
        var cycle = target.getAttribute('data-provider-plan-cycle') || 'monthly';
        view.busy = 'buy:' + planVersion;
        render();
        deps.api.startCheckout(current.id, planVersion, cycle).then(function(response) {
          if (response && response.url) {
            navigate(response.url);
            return;
          }
          throw new Error('Checkout is unavailable right now.');
        }).catch(function(error) {
          view.busy = '';
          render();
          deps.showToast(error instanceof Error ? error.message : 'Checkout is unavailable right now.', true);
        });
        return;
      }
      case 'manage-billing':
        view.busy = 'manage-billing';
        render();
        deps.api.openBillingPortal(current.id).then(function(response) {
          if (response && response.url) {
            navigate(response.url);
            return;
          }
          throw new Error('Billing is unavailable right now.');
        }).catch(function(error) {
          view.busy = '';
          render();
          deps.showToast(error instanceof Error ? error.message : 'Billing is unavailable right now.', true);
        });
        return;
      case 'refresh':
        void refresh(true);
        return;
    }
  });

  // The shell re-renders on bootstrap changes; put the panel back.
  deps.store.subscribeBootstrap(function() {
    render();
  });

  var search = deps.locationSearch ? deps.locationSearch() : window.location.search;
  var params = new URLSearchParams(search || '');
  var returned = params.get(CHECKOUT_RETURN_PARAM);
  if (returned && account()) {
    params.delete(CHECKOUT_RETURN_PARAM);
    var remaining = params.toString();
    if (deps.replaceSearch) {
      deps.replaceSearch(remaining ? '?' + remaining : '');
    } else if (window.history && typeof window.history.replaceState === 'function') {
      window.history.replaceState(null, '', window.location.pathname + (remaining ? '?' + remaining : '') + window.location.hash);
    }
    deps.store.setActiveShellSection('billing');
    if (returned === 'complete') {
      view.notice = 'Payment received. Applying your plan…';
      pollAfterCheckout(0);
    } else if (returned === 'billing') {
      checkAfterBillingReturn(0);
    } else {
      view.notice = 'Checkout was cancelled. Nothing was charged.';
    }
  }

  return {
    load: load,
    render: render,
    view: function() { return view; },
  };
}
