import { For, Show, type Component } from 'solid-js';
import { ActionIconButton, Button, ButtonLink } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { formControl, formField, formLabel } from '@/components/shared/Form';
import X from 'lucide-solid/icons/x';
import { getTeamOnboardingCopy } from '@/utils/rbacPresentation';
import { settingsTabPath } from '../settingsNavigationModel';
import { useTeamOnboardingState } from './useTeamOnboardingState';

interface TeamOnboardingDialogProps {
  onClose: () => void;
  onReviewUsers: () => void;
  returnFocus: () => HTMLElement | undefined;
}

export const TeamOnboardingDialog: Component<TeamOnboardingDialogProps> = (props) => {
  const state = useTeamOnboardingState();
  const copy = getTeamOnboardingCopy();

  return (
    <Dialog
      isOpen={true}
      onClose={props.onClose}
      ariaLabel={copy.title}
      panelClass="max-w-xl"
      returnFocus={props.returnFocus}
    >
      <div class="flex items-start justify-between gap-4 border-b border-border p-5">
        <div>
          <h3 class="text-lg font-semibold text-base-content">{copy.title}</h3>
          <p class="mt-1 text-sm text-muted">{copy.introduction}</p>
        </div>
        <ActionIconButton label={copy.closeLabel} onClick={props.onClose}>
          <X class="h-4 w-4" />
        </ActionIconButton>
      </div>
      <div class="min-h-0 space-y-5 overflow-y-auto p-5">
        <Show when={state.loading()}>
          <div class="flex items-center gap-2 text-sm text-muted">
            <LoadingSpinner />
            <span role="status">{copy.loading}</span>
          </div>
        </Show>
        <Show when={state.loadError()}>
          <InlineNotice
            tone="danger"
            role="alert"
            actionLabel={copy.retry}
            actionOnClick={() => void state.load()}
          >
            {copy.loadError}
          </InlineNotice>
        </Show>
        <Show when={!state.loading() && !state.loadError()}>
          <Show
            when={state.signInMethods().length > 0}
            fallback={
              <section class="space-y-3">
                <h4 class="font-medium text-base-content">{copy.setupTitle}</h4>
                <p class="text-sm text-muted">{copy.setupBody}</p>
                <div class="flex flex-wrap gap-2">
                  <ButtonLink href={settingsTabPath('security-sso')} onClick={props.onClose}>
                    {copy.configureSSO}
                  </ButtonLink>
                  <ButtonLink
                    href={settingsTabPath('security-auth')}
                    variant="secondary"
                    onClick={props.onClose}
                  >
                    {copy.authenticationSettings}
                  </ButtonLink>
                </div>
              </section>
            }
          >
            <section class="space-y-2">
              <h4 class="font-medium text-base-content">{copy.configuredTitle}</h4>
              <ul class="list-inside list-disc text-sm text-base-content">
                <For each={state.signInMethods()}>{(method) => <li>{method}</li>}</For>
              </ul>
              <p class="text-sm text-muted">{copy.configuredBody}</p>
              <ButtonLink
                href={settingsTabPath('security-sso')}
                variant="ghost"
                onClick={props.onClose}
              >
                {copy.reviewSSO}
              </ButtonLink>
            </section>
            <section class="space-y-3">
              <h4 class="font-medium text-base-content">{copy.shareTitle}</h4>
              <p class="text-sm text-muted">{copy.shareBody}</p>
              <Show
                when={state.signInUrl}
                fallback={<InlineNotice tone="warning">{copy.invalidLink}</InlineNotice>}
              >
                {(url) => (
                  <>
                    <label class={formField}>
                      <span class={formLabel}>{copy.linkLabel}</span>
                      <input
                        type="text"
                        readonly
                        value={url()}
                        onFocus={(event) => event.currentTarget.select()}
                        class={formControl}
                      />
                    </label>
                    <Button onClick={() => void state.copyLink()} isLoading={state.copying()}>
                      {state.copied() ? copy.copied : copy.copy}
                    </Button>
                    <Show when={state.copied()}>
                      <p role="status" class="text-sm text-muted">
                        {copy.copied}
                      </p>
                    </Show>
                    <Show when={state.copyError()}>
                      <InlineNotice tone="warning" role="alert">
                        {copy.copyError}
                      </InlineNotice>
                    </Show>
                  </>
                )}
              </Show>
            </section>
            <section class="space-y-2">
              <h4 class="font-medium text-base-content">{copy.reviewTitle}</h4>
              <p class="text-sm text-muted">{copy.reviewBody}</p>
              <InlineNotice tone="info">{copy.mappings}</InlineNotice>
              <p class="text-sm text-muted">{copy.removal}</p>
            </section>
          </Show>
        </Show>
      </div>
      <div class="flex flex-wrap justify-end gap-3 border-t border-border p-5">
        <Button onClick={props.onClose}>{copy.close}</Button>
        <Show when={!state.loading() && !state.loadError() && state.signInMethods().length > 0}>
          <Button variant="primary" onClick={props.onReviewUsers}>
            {copy.review}
          </Button>
        </Show>
      </div>
    </Dialog>
  );
};
