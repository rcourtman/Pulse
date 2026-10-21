// Real router, policy reader, entitlement store, billing hook, panel and CSS.
// Only HTTP responses are synthetic, supplied by the offline browser runner.
import { onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { SecurityAPI } from '../src/api/security';
import { ProLicensePanel } from '../src/components/Settings/ProLicensePanel';
import { syncSessionPresentationPolicy } from '../src/stores/sessionPresentationPolicy';
import { SELF_HOSTED_PRO_BILLING_PLAN_ROUTE } from '../src/utils/pricingHandoff';
import '../src/index.css';

function BillingFixture() {
  onMount(async () => {
    syncSessionPresentationPolicy(await SecurityAPI.getStatus());
  });
  return (
    <main class="mx-auto min-w-0 max-w-5xl p-3 sm:p-6">
      <ProLicensePanel />
    </main>
  );
}

render(
  () => (
    <Router>
      <Route path={SELF_HOSTED_PRO_BILLING_PLAN_ROUTE} component={BillingFixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
