import { Show, type Component, type JSX } from 'solid-js';

export type TableCardHeaderProps = {
  title?: JSX.Element;
  actions?: JSX.Element;
  showClearAction?: boolean;
  clearLabel?: string;
  clearAriaLabel?: string;
  onClear?: () => void;
};

export const TABLE_CARD_HEADER_CLASS =
  'flex items-center gap-3 border-b border-border bg-surface-hover px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted';

export const TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS = [
  'ml-auto inline-flex items-center rounded-sm text-[11px] font-medium normal-case tracking-normal text-muted transition-colors',
  'hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-1',
].join(' ');

const TABLE_CARD_HEADER_ACTIONS_CLEAR_BUTTON_CLASS = TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS.replace(
  'ml-auto ',
  '',
);

export const TableCardHeader: Component<TableCardHeaderProps> = (props) => {
  const hasActions = () => Boolean(props.actions || (props.showClearAction && props.onClear));
  const hasContent = () => Boolean(props.title) || hasActions();
  const clearLabel = () => props.clearLabel ?? 'Clear';
  const clearAriaLabel = () => props.clearAriaLabel ?? 'Clear selection';

  return (
    <Show when={hasContent()}>
      <div class={TABLE_CARD_HEADER_CLASS}>
        <Show when={props.title}>
          <span>{props.title}</span>
        </Show>
        <Show when={hasActions()}>
          <div class="ml-auto flex items-center gap-2 normal-case tracking-normal">
            {props.actions}
            <Show when={props.showClearAction && props.onClear}>
              <button
                type="button"
                class={TABLE_CARD_HEADER_ACTIONS_CLEAR_BUTTON_CLASS}
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
        </Show>
      </div>
    </Show>
  );
};

export default TableCardHeader;
