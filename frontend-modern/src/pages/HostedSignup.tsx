import { For, Show, createMemo, createSignal } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { PageHeader } from '@/components/shared/PageHeader';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { HostedSignupAPI, type HostedAPIError } from '@/api/hostedSignup';
import { getUpgradeActionDestination } from '@/stores/license';
import { logger } from '@/utils/logger';
import {
  HOSTED_SIGNUP_PRESENTATION,
  getCloudPlanForTier,
  getCloudPlanPricePresentation,
} from '@/utils/cloudPlans';

type SignupStatus = 'idle' | 'submitting' | 'success' | 'unavailable' | 'error';

function isValidEmail(value: string): boolean {
  const email = value.trim();
  if (email === '' || email.includes(' ')) {
    return false;
  }
  const at = email.indexOf('@');
  if (at <= 0 || at >= email.length - 1) {
    return false;
  }
  const domain = email.slice(at + 1);
  const dot = domain.indexOf('.');
  return dot > 0 && dot < domain.length - 1;
}

function unavailableError(err: HostedAPIError, status: number): boolean {
  if (status === 404 || status === 501 || status === 503) {
    return true;
  }
  return [
    'orgs_unavailable',
    'provisioner_not_available',
    'magic_links_unavailable',
    'public_url_missing',
  ].includes(err.code);
}

export default function HostedSignup() {
  const location = useLocation();
  const [email, setEmail] = createSignal('');
  const [orgName, setOrgName] = createSignal('');
  const [status, setStatus] = createSignal<SignupStatus>('idle');
  const [message, setMessage] = createSignal('');

  const [requestingMagicLink, setRequestingMagicLink] = createSignal(false);
  const [magicLinkEmail, setMagicLinkEmail] = createSignal('');
  const [magicLinkMessage, setMagicLinkMessage] = createSignal('');

  const selectedPlan = createMemo(() => {
    const params = new URLSearchParams(location.search);
    return getCloudPlanForTier(params.get('tier'));
  });
  const selectedPlanPrice = createMemo(() => getCloudPlanPricePresentation(selectedPlan()));
  const cloudPortalDestination = createMemo(() => getUpgradeActionDestination('cloud'));

  const canSubmit = createMemo(() => {
    return isValidEmail(email()) && orgName().trim().length >= 3 && status() !== 'submitting';
  });

  const submitSignup = async (event: Event) => {
    event.preventDefault();
    setMessage('');

    const cleanEmail = email().trim().toLowerCase();
    const cleanOrgName = orgName().trim();
    if (!isValidEmail(cleanEmail)) {
      setStatus('error');
      setMessage('Enter a valid email address.');
      return;
    }
    if (cleanOrgName.length < 3) {
      setStatus('error');
      setMessage('Organization name must be at least 3 characters.');
      return;
    }

    setStatus('submitting');
    const result = await HostedSignupAPI.signup({
      email: cleanEmail,
      org_name: cleanOrgName,
      tier: selectedPlan().tier,
    });

    if (result.ok) {
      const checkoutURL = result.data.checkout_url?.trim();
      if (checkoutURL) {
        window.location.assign(checkoutURL);
        return;
      }
      setStatus('success');
      setMessage(result.data.message || 'Check your email for a sign-in link.');
      return;
    }

    logger.warn('[HostedSignup] Signup failed', result);
    if (unavailableError(result.error, result.status)) {
      setStatus('unavailable');
      setMessage('Hosted signup is not enabled on this deployment yet.');
      return;
    }
    setStatus('error');
    setMessage(result.error.message || 'Signup failed. Please try again.');
  };

  const requestMagicLink = async (event: Event) => {
    event.preventDefault();
    setMagicLinkMessage('');

    const cleanEmail = magicLinkEmail().trim().toLowerCase();
    if (!isValidEmail(cleanEmail)) {
      setMagicLinkMessage('Enter a valid email address first.');
      return;
    }

    setRequestingMagicLink(true);
    try {
      const result = await HostedSignupAPI.requestMagicLink(cleanEmail);
      if (result.ok) {
        setMagicLinkMessage(
          result.data.message ||
            "If that email is registered, you'll receive a sign-in link shortly.",
        );
      } else {
        setMagicLinkMessage(result.error.message || 'Failed to request sign-in link.');
      }
    } finally {
      setRequestingMagicLink(false);
    }
  };

  return (
    <div class="space-y-6">
      <PageHeader
        title={`${HOSTED_SIGNUP_PRESENTATION.pageTitlePrefix} ${selectedPlan().name}`}
        description={HOSTED_SIGNUP_PRESENTATION.pageDescription}
      />

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card padding="lg" class="space-y-4">
          <h2 class="text-lg font-semibold text-base-content">
            {HOSTED_SIGNUP_PRESENTATION.workspaceHeading}
          </h2>

          <form class="space-y-3" onSubmit={submitSignup}>
            <label class="block text-sm font-medium text-base-content" for="hosted-email">
              Work Email
            </label>
            <input
              id="hosted-email"
              type="email"
              class="w-full rounded-md border border-border bg-base px-3 py-2 text-sm text-base-content outline-none focus:border-blue-600"
              value={email()}
              onInput={(e) => setEmail(e.currentTarget.value)}
              placeholder="you@company.com"
              autocomplete="email"
              required
            />

            <label class="block text-sm font-medium text-base-content" for="hosted-org-name">
              Organization Name
            </label>
            <input
              id="hosted-org-name"
              type="text"
              class="w-full rounded-md border border-border bg-base px-3 py-2 text-sm text-base-content outline-none focus:border-blue-600"
              value={orgName()}
              onInput={(e) => setOrgName(e.currentTarget.value)}
              placeholder="Acme Infrastructure"
              autocomplete="organization"
              required
            />

            <button
              type="submit"
              class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canSubmit()}
            >
              <Show
                when={status() === 'submitting'}
                fallback={HOSTED_SIGNUP_PRESENTATION.createWorkspaceLabel}
              >
                {HOSTED_SIGNUP_PRESENTATION.creatingWorkspaceLabel}
              </Show>
            </button>
          </form>

          <Show when={message()}>
            <div
              classList={{
                'rounded-md px-3 py-2 text-sm': true,
                'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/30 dark:text-emerald-200':
                  status() === 'success',
                'bg-amber-100 text-amber-900 dark:bg-amber-900/30 dark:text-amber-200':
                  status() === 'unavailable',
                'bg-rose-100 text-rose-900 dark:bg-rose-900/30 dark:text-rose-200':
                  status() === 'error',
              }}
            >
              {message()}
            </div>
          </Show>

          <Show when={status() === 'unavailable'}>
            <UpgradeLink
              destination={cloudPortalDestination()}
              class="inline-flex items-center text-sm font-medium text-blue-700 hover:underline dark:text-blue-300"
            >
              Open hosted signup portal
            </UpgradeLink>
          </Show>
        </Card>

        <Card padding="lg" class="space-y-4">
          <h2 class="text-lg font-semibold text-base-content">
            {HOSTED_SIGNUP_PRESENTATION.planHeading}
          </h2>
          <div class="rounded-md border border-border bg-surface-alt p-4">
            <div class="flex items-baseline justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-base-content">{selectedPlan().name}</p>
                <p class="text-xs text-muted">{selectedPlanPrice().annualSummary}</p>
              </div>
              <div class="text-right">
                <p class="text-sm font-semibold text-base-content">
                  {selectedPlanPrice().monthlyPrice}
                  {selectedPlanPrice().cadence}
                </p>
                <Show when={selectedPlanPrice().compareAtMonthlyPrice}>
                  <p class="text-xs text-muted line-through">
                    {selectedPlanPrice().compareAtMonthlyPrice}
                    {selectedPlanPrice().cadence}
                  </p>
                </Show>
              </div>
            </div>
            <dl class="mt-3 grid grid-cols-2 gap-3 text-sm text-base-content">
              <div>
                <dt class="text-xs uppercase tracking-wide text-muted">Monitored Systems</dt>
                <dd class="mt-1 font-medium">{selectedPlan().monitoredSystems}</dd>
              </div>
              <div>
                <dt class="text-xs uppercase tracking-wide text-muted">Support</dt>
                <dd class="mt-1 font-medium">{selectedPlan().support}</dd>
              </div>
            </dl>
          </div>
          <h3 class="text-sm font-semibold text-base-content">
            {HOSTED_SIGNUP_PRESENTATION.nextHeading}
          </h3>
          <ol class="list-decimal space-y-2 pl-5 text-sm text-base-content">
            <For each={HOSTED_SIGNUP_PRESENTATION.nextSteps}>{(step) => <li>{step}</li>}</For>
          </ol>

          <div class="border-t border-border pt-4">
            <h3 class="text-sm font-semibold text-base-content">
              {HOSTED_SIGNUP_PRESENTATION.existingAccountHeading}
            </h3>
            <p class="mt-1 text-sm text-muted">
              {HOSTED_SIGNUP_PRESENTATION.existingAccountDescription}
            </p>
            <form class="mt-3 space-y-3" onSubmit={requestMagicLink}>
              <input
                id="hosted-magic-link-email"
                type="email"
                class="w-full rounded-md border border-border bg-base px-3 py-2 text-sm text-base-content outline-none focus:border-blue-600"
                value={magicLinkEmail()}
                onInput={(e) => setMagicLinkEmail(e.currentTarget.value)}
                placeholder="you@company.com"
                autocomplete="email"
                required
              />
              <button
                type="submit"
                class="w-full inline-flex items-center justify-center rounded-md border border-border bg-surface px-4 py-2 text-sm font-semibold text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                disabled={requestingMagicLink()}
              >
                <Show
                  when={requestingMagicLink()}
                  fallback={HOSTED_SIGNUP_PRESENTATION.emailSignInLinkLabel}
                >
                  {HOSTED_SIGNUP_PRESENTATION.sendingSignInLinkLabel}
                </Show>
              </button>
            </form>
            <Show when={magicLinkMessage()}>
              <div class="mt-2 text-xs text-muted">{magicLinkMessage()}</div>
            </Show>
          </div>
        </Card>
      </div>
    </div>
  );
}
