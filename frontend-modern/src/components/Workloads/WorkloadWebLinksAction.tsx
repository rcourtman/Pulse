import { Show, Suspense, createSignal, lazy, type Component } from 'solid-js';
import Link2Icon from 'lucide-solid/icons/link-2';
import { TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS } from '@/components/shared/TableCardHeader';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { WorkloadGuest } from '@/types/workloads';
import type { WorkloadGuestMetadataMap } from './workloadGuestMetadataRecord';
import type { WorkloadWebLinkDrafts } from './workloadWebLinksModel';

// The panel and its row model load on first open, so they stay out of the
// WorkloadsSurface chunk and do no work while the editor is closed.
const WorkloadWebLinksDialog = lazy(() => import('./WorkloadWebLinksDialog'));

export interface WorkloadWebLinksActionProps {
  guests: () => WorkloadGuest[];
  guestMetadata: () => WorkloadGuestMetadataMap;
}

/** Table-header entry point for setting many guest web links at once. */
export const WorkloadWebLinksAction: Component<WorkloadWebLinksActionProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  // Drafts outlive the panel so closing it by accident keeps typed links.
  const [drafts, setDrafts] = createSignal<WorkloadWebLinkDrafts>({});
  let triggerRef: HTMLButtonElement | undefined;
  const preload = () => void WorkloadWebLinksDialog.preload();

  return (
    <Show when={!presentationPolicyIsReadOnly()}>
      <button
        ref={triggerRef}
        type="button"
        class={`${TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS} min-h-6 gap-1`}
        aria-haspopup="dialog"
        onPointerEnter={preload}
        onFocus={preload}
        onClick={() => setOpen(true)}
      >
        <Link2Icon class="h-3.5 w-3.5" aria-hidden="true" />
        Edit links
        <Show when={Object.keys(drafts()).length > 0}>
          <span class="text-amber-600 dark:text-amber-400">(unsaved)</span>
        </Show>
      </button>
      <Show when={open()}>
        <Suspense fallback={null}>
          <WorkloadWebLinksDialog
            guests={props.guests}
            guestMetadata={props.guestMetadata}
            drafts={drafts}
            setDrafts={setDrafts}
            onClose={() => setOpen(false)}
            returnFocus={() => triggerRef}
          />
        </Suspense>
      </Show>
    </Show>
  );
};

export default WorkloadWebLinksAction;
