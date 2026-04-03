import { Show, type Component, type JSX } from 'solid-js';

type SummaryTableCardHeaderProps = {
  title: JSX.Element;
  showClearAction?: boolean;
  clearLabel?: string;
  clearAriaLabel?: string;
  onClear?: () => void;
};

const HEADER_CLASS =
  'flex items-center gap-3 border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted';

const CLEAR_BUTTON_CLASS = [
  'ml-auto inline-flex items-center rounded-sm text-[11px] font-medium normal-case tracking-normal text-muted transition-colors',
  'hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1',
].join(' ');

export const SummaryTableCardHeader: Component<SummaryTableCardHeaderProps> = (props) => {
  const clearLabel = () => props.clearLabel ?? 'Clear';
  const clearAriaLabel = () => props.clearAriaLabel ?? 'Clear selection';

  return (
    <div class={HEADER_CLASS}>
      <span>{props.title}</span>
      <Show when={props.showClearAction && props.onClear}>
        <button
          type="button"
          class={CLEAR_BUTTON_CLASS}
          aria-label={clearAriaLabel()}
          onClick={(event) => {
            event.stopPropagation();
            props.onClear?.();
          }}
        >
          {clearLabel()}
        </button>
      </Show>
    </div>
  );
};

export default SummaryTableCardHeader;
