import { Show } from 'solid-js';
import type { Component, JSX } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  getDialogAlignmentClass,
  getDialogPanelClass,
  getDialogViewportClass,
  type DialogLayout,
} from './dialogModel';
import { useDialogState } from './useDialogState';

interface DialogProps {
  isOpen: boolean;
  onClose: () => void;
  children: JSX.Element;
  panelClass?: string;
  layout?: DialogLayout;
  closeOnBackdrop?: boolean;
  ariaLabel?: string;
  ariaLabelledBy?: string;
  ariaDescribedBy?: string;
}

export const Dialog: Component<DialogProps> = (props) => {
  const state = useDialogState(props);

  return (
    <Show when={props.isOpen}>
      <Portal mount={document.body}>
        <div class="fixed inset-0 z-[1000]">
          <div
            class="absolute inset-0 bg-black/60 backdrop-blur-sm transition-opacity duration-300"
            data-dialog-backdrop
            onClick={state.handleBackdropClick}
          />
          <div class={getDialogViewportClass(state.layout())}>
            <div class={getDialogAlignmentClass(state.layout())}>
              <div
                ref={state.setPanelRef}
                role="dialog"
                aria-modal="true"
                aria-label={props.ariaLabel}
                aria-labelledby={props.ariaLabelledBy}
                aria-describedby={props.ariaDescribedBy}
                tabindex="-1"
                class={getDialogPanelClass(state.layout(), props.panelClass)}
                onClick={(event) => event.stopPropagation()}
              >
                {props.children}
              </div>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
};

export default Dialog;
