import { Show, createSignal, onCleanup, onMount, type Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';

export const APP_BOOTSTRAP_SLOW_DELAY_MS = 10_000;

interface AppBootstrapStatusProps {
  onRetry?: () => void;
  slowDelayMs?: number;
}

export const AppBootstrapStatus: Component<AppBootstrapStatusProps> = (props) => {
  const [isSlow, setIsSlow] = createSignal(false);

  onMount(() => {
    const timeout = window.setTimeout(
      () => setIsSlow(true),
      props.slowDelayMs ?? APP_BOOTSTRAP_SLOW_DELAY_MS,
    );
    onCleanup(() => window.clearTimeout(timeout));
  });

  const retry = () => {
    if (props.onRetry) {
      props.onRetry();
      return;
    }
    window.location.reload();
  };

  return (
    <main
      class="flex min-h-screen items-center justify-center bg-base px-4 py-8 text-base-content"
      aria-labelledby="app-bootstrap-title"
    >
      <div class="w-full max-w-md rounded-md border border-border bg-surface p-6 text-center shadow-sm sm:p-8">
        <LoadingSpinner size="xl" tone="info" class="mb-4" />
        <h1 id="app-bootstrap-title" class="text-lg font-semibold tracking-tight">
          Connecting to Pulse
        </h1>
        <p
          class="mt-2 text-sm leading-6 text-muted"
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >
          <Show when={isSlow()} fallback="Checking your session and preparing the workspace.">
            Pulse is taking longer than expected to respond. The server may still be starting or
            busy.
          </Show>
        </p>
        <Show when={isSlow()}>
          <div class="mt-5 border-t border-border pt-5">
            <p class="mb-4 text-sm leading-6 text-muted">
              You can keep waiting, or retry the connection.
            </p>
            <Button variant="primary" size="md" onClick={retry}>
              Retry connection
            </Button>
          </div>
        </Show>
      </div>
    </main>
  );
};

export default AppBootstrapStatus;
