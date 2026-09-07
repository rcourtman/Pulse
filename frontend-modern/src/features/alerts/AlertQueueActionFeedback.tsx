import { Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { Button } from '@/components/shared/Button';

// Keep the live region mounted before its content changes, and retain a focus
// destination when the user clears the message. This is view-local, not history.
export function AlertQueueActionFeedback(props: { message: string | null; onClear: () => void }) {
  let region!: HTMLDivElement;
  return (
    <div ref={region} role="region" aria-label="Notification recovery feedback" tabIndex={-1}>
      <div role="status" aria-live="polite" aria-atomic="true">
        <Show when={props.message}>
          <Card tone="warning" padding="sm">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <p class="min-w-0 flex-1 basis-72 break-words text-sm text-base-content">
                {props.message}
              </p>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  region.focus();
                  props.onClear();
                }}
              >
                Clear recovery message
              </Button>
            </div>
          </Card>
        </Show>
      </div>
    </div>
  );
}
