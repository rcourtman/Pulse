import { Navigate, useLocation } from '@solidjs/router';
import { Show, createMemo, onMount } from 'solid-js';
import { PageHeader } from '@/components/shared/PageHeader';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getPricingRouteDestination,
  handoffToExternalPricing,
  isExternalPricingDestination,
  isSelfHostedPurchaseStartDestination,
} from '@/utils/pricingHandoff';

export default function PricingHandoff() {
  const location = useLocation();
  const destination = createMemo(() => getPricingRouteDestination(location.search));
  const externalDestination = createMemo(() => isExternalPricingDestination(destination()));
  const selfHostedPurchaseStartDestination = createMemo(() =>
    isSelfHostedPurchaseStartDestination(destination()),
  );
  const pulseAccountDestination = createMemo(() => selfHostedPurchaseStartDestination());
  const handoffLabel = createMemo(() =>
    pulseAccountDestination() ? 'Pulse Account' : 'pricing',
  );
  const handoffLinkLabel = createMemo(() =>
    pulseAccountDestination() ? 'continue to Pulse Account' : 'continue to the public pricing site',
  );

  onMount(() => {
    trackPaywallViewed('pricing', 'pricing_handoff');
    if (externalDestination() || selfHostedPurchaseStartDestination()) {
      handoffToExternalPricing(destination());
    }
  });

  return (
    <Show
      when={externalDestination() || selfHostedPurchaseStartDestination()}
      fallback={<Navigate href={destination()} />}
    >
      <div class="flex min-h-[50vh] items-center justify-center">
        <div class="max-w-xl space-y-2 text-center">
          <PageHeader
            title={`Redirecting to ${handoffLabel()}`}
            description={
              <>
                If the handoff does not start automatically,{' '}
                <a href={destination()} class="text-blue-600 hover:underline dark:text-blue-400">
                  {handoffLinkLabel()}
                </a>
                .
              </>
            }
            class="items-center text-center"
            titleClass="text-lg"
            descriptionClass="text-sm"
          />
        </div>
      </div>
    </Show>
  );
}
