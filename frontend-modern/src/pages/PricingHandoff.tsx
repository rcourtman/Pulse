import { Navigate, useLocation } from '@solidjs/router';
import { Show, createMemo, onMount } from 'solid-js';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getPricingRouteDestination,
  handoffToExternalPricing,
  isExternalPricingDestination,
} from '@/utils/pricingHandoff';

export default function PricingHandoff() {
  const location = useLocation();
  const destination = createMemo(() => getPricingRouteDestination(location.search));
  const externalDestination = createMemo(() => isExternalPricingDestination(destination()));

  onMount(() => {
    trackPaywallViewed('pricing', 'pricing_handoff');
    if (externalDestination()) {
      handoffToExternalPricing(destination());
    }
  });

  return (
    <Show when={externalDestination()} fallback={<Navigate href={destination()} />}>
      <div class="flex min-h-[50vh] items-center justify-center">
        <div class="space-y-2 text-center">
          <h1 class="text-lg font-semibold text-base-content">Redirecting to pricing</h1>
          <p class="text-sm text-muted">
            If the handoff does not start automatically,{' '}
            <a
              href={destination()}
              class="text-blue-600 hover:underline dark:text-blue-400"
            >
              continue to the public pricing site
            </a>
            .
          </p>
        </div>
      </div>
    </Show>
  );
}
