import { Component, Show } from 'solid-js';
import Smartphone from 'lucide-solid/icons/smartphone';
import X from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyIsReadOnly,
} from '@/stores/sessionPresentationPolicy';
import {
  RELAY_ONBOARDING_DESCRIPTION,
  RELAY_ONBOARDING_DISCONNECTED_LABEL,
  RELAY_ONBOARDING_SETUP_LABEL,
  RELAY_ONBOARDING_TITLE,
  RELAY_ONBOARDING_TRIAL_LABEL,
  RELAY_ONBOARDING_TRIAL_STARTING_LABEL,
  RELAY_ONBOARDING_UPGRADE_LABEL,
} from '@/utils/relayPresentation';
import { useRelayOnboardingCardState } from './useRelayOnboardingCardState';

export const RelayOnboardingCard: Component = () => {
  const state = useRelayOnboardingCardState();

  return (
    <Show
      when={
        !presentationPolicyIsReadOnly() &&
        state.shouldShow() &&
        (!presentationPolicyHidesCommercialSurfaces() || state.hasRelay())
      }
    >
      <Card padding="lg" class="relative overflow-hidden">
        <div class="absolute -right-10 -top-10 h-32 w-32 rounded-full bg-blue-100 dark:bg-blue-900" />
        <div class="absolute -right-16 -bottom-16 h-40 w-40 rounded-full bg-surface-alt" />

        <button
          type="button"
          class="absolute right-3 top-3 inline-flex items-center justify-center rounded-md p-1 hover:text-base-content hover:bg-surface-hover"
          onClick={state.dismiss}
          aria-label="Dismiss relay onboarding"
        >
          <X size={16} strokeWidth={2} />
        </button>

        <div class="relative flex items-start gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm">
            <Smartphone size={20} strokeWidth={2} />
          </div>

          <div class="min-w-0 flex-1">
            <h2 class="text-base font-semibold text-base-content">{RELAY_ONBOARDING_TITLE}</h2>
            <p class="mt-1 text-sm text-muted">
              {RELAY_ONBOARDING_DESCRIPTION}
            </p>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <Show
                when={state.hasRelay()}
                fallback={
                  <>
                    <UpgradeLink
                      class="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
                      destination={getUpgradeActionDestination('relay')}
                      onClick={state.handleUpgradeClick}
                    >
                      {RELAY_ONBOARDING_UPGRADE_LABEL}
                    </UpgradeLink>
                    <button
                      type="button"
                      class="inline-flex items-center rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content shadow-sm hover:bg-surface-hover disabled:opacity-50"
                      onClick={() => void state.handleStartTrial()}
                      disabled={state.trialStarting()}
                    >
                      {state.trialStarting()
                        ? RELAY_ONBOARDING_TRIAL_STARTING_LABEL
                        : RELAY_ONBOARDING_TRIAL_LABEL}
                    </button>
                  </>
                }
              >
                <button
                  type="button"
                  class="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
                  onClick={state.handleSetupRelay}
                >
                  {RELAY_ONBOARDING_SETUP_LABEL}
                </button>
              </Show>

              <Show
                when={
                  state.hasRelay() && state.statusLoaded() && state.status()?.connected === false
                }
              >
                <span class="text-xs text-muted">{RELAY_ONBOARDING_DISCONNECTED_LABEL}</span>
              </Show>
            </div>
          </div>
        </div>
      </Card>
    </Show>
  );
};
