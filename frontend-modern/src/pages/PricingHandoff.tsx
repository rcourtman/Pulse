import { Navigate, useLocation } from '@solidjs/router';
import { Show, createMemo, onMount } from 'solid-js';
import { PageHeader } from '@/components/shared/PageHeader';
import {
  getPricingRouteDestination,
  handoffToExternalPricing,
  isExternalPricingDestination,
  isSelfHostedPurchaseStartDestination,
} from '@/utils/pricingHandoff';
import { t } from '@/i18n';

export default function PricingHandoff() {
  const location = useLocation();
  const destination = createMemo(() => getPricingRouteDestination(location.search));
  const externalDestination = createMemo(() => isExternalPricingDestination(destination()));
  const selfHostedPurchaseStartDestination = createMemo(() =>
    isSelfHostedPurchaseStartDestination(destination()),
  );
  const pulseAccountDestination = createMemo(() => selfHostedPurchaseStartDestination());
  const handoffTitle = createMemo(() =>
    pulseAccountDestination()
      ? t('pricing.handoff.title.pulseAccount')
      : t('pricing.handoff.title.publicPricing'),
  );
  const handoffLinkLabel = createMemo(() =>
    pulseAccountDestination()
      ? t('pricing.handoff.link.pulseAccount')
      : t('pricing.handoff.link.publicPricing'),
  );

  onMount(() => {
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
            title={handoffTitle()}
            description={
              <>
                {t('pricing.handoff.description.beforeLink')}{' '}
                <a href={destination()} class="text-blue-600 hover:underline dark:text-blue-400">
                  {handoffLinkLabel()}
                </a>
                {t('pricing.handoff.description.afterLink')}
              </>
            }
            descriptionVisibility="always"
            updateDocumentTitle={false}
            class="!items-center !justify-center text-center sm:!flex-col"
            titleClass="!text-lg !font-semibold"
            descriptionClass="!font-normal"
          />
        </div>
      </div>
    </Show>
  );
}
