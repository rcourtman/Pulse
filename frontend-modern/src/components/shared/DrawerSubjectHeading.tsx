import { Show, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';

interface DrawerSubjectHeadingProps {
  headingId: string;
  title: string;
  /** Omitted only for a record without a status of its own. */
  statusVariant?: StatusIndicatorVariant;
  statusLabel?: string;
  trailing?: JSX.Element;
}

// Every drawer names its subject the same way: the row's status dot, then the
// name. Names wrap rather than truncate because operators compare names that
// differ at their end, and a phone has no hover title to reveal the rest.
export const DrawerSubjectHeading: Component<DrawerSubjectHeadingProps> = (props) => (
  <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
    <div class="flex min-w-0 items-center gap-2">
      <Show when={props.statusVariant}>
        {(variant) => (
          <StatusDot
            size="sm"
            variant={variant()}
            title={props.statusLabel}
            ariaLabel={props.statusLabel}
          />
        )}
      </Show>
      <h2
        id={props.headingId}
        class="text-sm font-semibold text-base-content min-w-0 wrap-anywhere m-0"
        title={props.title}
      >
        {props.title}
      </h2>
    </div>
    {props.trailing}
  </div>
);
