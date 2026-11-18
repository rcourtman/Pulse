import { createEffect, createSignal, on, Show, type ParentComponent } from 'solid-js';
import { Button } from '@/components/shared/Button';

interface SettingsAccessRecoveryProps {
  failed: boolean;
  checking: boolean;
  retry: () => Promise<void>;
}

export const SettingsAccessRecovery: ParentComponent<SettingsAccessRecoveryProps> = (props) => {
  let content: HTMLDivElement | undefined;
  const [announcement, setAnnouncement] = createSignal('');
  createEffect(
    on(
      () => props.checking,
      (checking) => {
        if (checking) setAnnouncement('');
      },
    ),
  );

  const retryAccessCheck = async () => {
    if (props.checking) return;
    // Retry disables its button, and a successful check removes it. Start
    // from the surviving content so Tab continues here on either outcome.
    // Never move focus in response to a late completion.
    content?.focus({ preventScroll: true });
    await props.retry();
    setAnnouncement(
      props.failed ? 'Settings access still could not be verified.' : 'Settings access checked.',
    );
  };

  return (
    <div
      ref={content}
      role="region"
      aria-label="Settings content"
      tabIndex={-1}
      class="focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-blue-500"
    >
      <div role="status" class="sr-only">
        {props.checking && props.failed ? 'Checking settings access.' : announcement()}
      </div>
      <Show when={props.failed}>
        <div
          role="alert"
          class="mb-4 space-y-3 rounded-md border border-border bg-surface-hover p-4 text-sm"
        >
          <p class="font-medium text-base-content">Settings access could not be verified</p>
          <p class="text-muted">
            Restricted settings are hidden until a fresh check succeeds. Retrying does not change
            your permissions.
          </p>
          <Button
            variant="secondary"
            size="settingsAction"
            disabled={props.checking}
            onClick={() => void retryAccessCheck()}
          >
            {props.checking ? 'Checking access…' : 'Retry access check'}
          </Button>
        </div>
      </Show>
      {props.children}
    </div>
  );
};
