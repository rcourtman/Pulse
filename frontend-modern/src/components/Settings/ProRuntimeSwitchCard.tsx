import { Component, For, Show, createResource, createSignal } from 'solid-js';
import KeyRound from 'lucide-solid/icons/key-round';
import { UpdatesAPI } from '@/api/updates';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { CopyCommandBlock } from '@/components/Settings/CopyCommandBlock';
import {
  getProRuntimeSwitchPresentation,
  type ProRuntimeSwitchPresentation,
} from '@/utils/updatesPresentation';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';

const COMMAND_CODE_CLASS =
  'block rounded-md border border-border bg-page p-3 pr-14 font-mono text-sm text-base-content whitespace-pre-wrap break-all';

type PresentationOf<K extends ProRuntimeSwitchPresentation['kind']> = Extract<
  ProRuntimeSwitchPresentation,
  { kind: K }
>;

interface ProRuntimeSwitchCardProps {
  // Only community-build installs ask: the Pro runtime never needs a switch,
  // and source builds have no release to switch to.
  shouldCheck: boolean;
}

// Moves a community install with a paid licence onto the Pulse Pro runtime.
// The backend asks the license server, so a free install sees nothing.
export const ProRuntimeSwitchCard: Component<ProRuntimeSwitchCardProps> = (props) => {
  const [result] = createResource(
    () => props.shouldCheck || undefined,
    () => UpdatesAPI.checkProRuntimeSwitch(),
  );
  const presentation = () =>
    getProRuntimeSwitchPresentation(result.error ? null : (result() ?? null));
  const ofKind = <K extends ProRuntimeSwitchPresentation['kind']>(kind: K) => {
    const current = presentation();
    return current.kind === kind ? (current as PresentationOf<K>) : undefined;
  };

  const [confirming, setConfirming] = createSignal(false);
  const [starting, setStarting] = createSignal(false);

  const startSwitch = async () => {
    setStarting(true);
    try {
      // The progress modal opens from the updater's own status events.
      await UpdatesAPI.applyProRuntimeSwitch();
      setConfirming(false);
    } catch (error) {
      logger.error('Failed to start the Pulse Pro runtime switch', error);
      notificationStore.error(
        error instanceof Error && error.message
          ? error.message
          : 'Unable to start the switch. Please try again.',
      );
    } finally {
      setStarting(false);
    }
  };

  return (
    <>
      <Show when={ofKind('in_place')}>
        {(view) => (
          <CalloutCard
            tone="warning"
            scale="compact"
            icon={<KeyRound class="h-5 w-5" />}
            title={view().title}
            description={view().body}
            data-testid="pro-runtime-switch"
          >
            <Show
              when={confirming()}
              fallback={
                <div class="pt-2">
                  <Button variant="warningSolid" size="sm" onClick={() => setConfirming(true)}>
                    Switch to Pulse Pro {view().version}
                  </Button>
                </div>
              }
            >
              <div class="space-y-2 pt-2">
                <p class="text-xs text-amber-800 dark:text-amber-200">
                  Pulse restarts during the switch. It is recorded in Update history below, where
                  Roll back returns to the community build.
                </p>
                <div class="flex flex-wrap gap-2">
                  <Button
                    variant="warningSolid"
                    size="sm"
                    isLoading={starting()}
                    disabled={starting()}
                    onClick={() => void startSwitch()}
                  >
                    Switch now
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={starting()}
                    onClick={() => setConfirming(false)}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            </Show>
          </CalloutCard>
        )}
      </Show>

      <Show when={ofKind('docker')}>
        {(view) => (
          <CalloutCard
            tone="warning"
            scale="compact"
            icon={<KeyRound class="h-5 w-5" />}
            title={view().title}
            description={view().body}
            data-testid="pro-runtime-switch"
          >
            <ol class="space-y-3 pt-2">
              <For each={view().steps}>
                {(step, index) => (
                  <li class="space-y-1.5">
                    <p class="text-xs font-medium text-amber-900 dark:text-amber-100">
                      {index() + 1}. {step.title}
                    </p>
                    <Show when={step.command}>
                      {(command) => (
                        <CopyCommandBlock command={command()} codeClass={COMMAND_CODE_CLASS} />
                      )}
                    </Show>
                    <Show when={step.note}>
                      <p class="text-xs text-amber-800 dark:text-amber-200">{step.note}</p>
                    </Show>
                  </li>
                )}
              </For>
            </ol>
            <p class="pt-1 text-xs text-amber-800 dark:text-amber-200">{view().note}</p>
          </CalloutCard>
        )}
      </Show>

      <Show when={ofKind('older_release')}>
        {(view) => (
          <CalloutCard
            tone="info"
            scale="compact"
            icon={<KeyRound class="h-5 w-5" />}
            title={view().title}
            description={view().body}
            data-testid="pro-runtime-switch"
          />
        )}
      </Show>

      <Show when={ofKind('prerelease_only')}>
        {(view) => (
          <CalloutCard
            tone="info"
            scale="compact"
            icon={<KeyRound class="h-5 w-5" />}
            title={view().title}
            description={view().body}
            data-testid="pro-runtime-switch"
          />
        )}
      </Show>

      <Show when={ofKind('unavailable')}>
        {(view) => (
          <p class="text-xs text-muted" data-testid="pro-runtime-switch">
            {view().body}
          </p>
        )}
      </Show>
    </>
  );
};
