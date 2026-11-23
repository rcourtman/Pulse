import { A, Navigate, useLocation } from '@solidjs/router';
import { Show, createMemo, onMount } from 'solid-js';
import { PulseBrandMark } from '@/components/Brand/PulseBrandMark';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
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
      <div class="flex min-h-[60vh] items-center justify-center px-4 py-8">
        <div class="w-full max-w-md space-y-5 rounded-lg border border-border bg-surface p-6 text-center shadow-xs sm:p-8">
          <div class="flex items-center justify-center gap-2">
            <PulseBrandMark class="h-8 w-8" />
            <span class="text-lg font-semibold text-base-content">Pulse</span>
          </div>
          <div class="flex flex-col items-center gap-3">
            <LoadingSpinner size="md" tone="info" />
            <PageHeader
              title={handoffTitle()}
              description={
                <>
                  {t('pricing.handoff.description.beforeLink')}{' '}
                  <a
                    href={destination()}
                    class="font-medium text-blue-600 hover:underline focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400"
                  >
                    {handoffLinkLabel()}
                  </a>
                  {t('pricing.handoff.description.afterLink')}
                </>
              }
              descriptionVisibility="always"
              updateDocumentTitle={false}
              class="items-center! justify-center! text-center sm:flex-col!"
              titleClass="w-full text-center text-lg! font-semibold!"
              descriptionClass="text-sm! font-normal!"
            />
          </div>
          <div class="border-t border-border pt-4">
            <A
              href="/"
              class="inline-flex min-h-10 items-center justify-center rounded-md px-3 text-sm font-medium text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
            >
              Return to Pulse
            </A>
          </div>
        </div>
      </div>
    </Show>
  );
}
