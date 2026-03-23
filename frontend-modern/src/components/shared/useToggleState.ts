import type { ToggleChangeEvent } from '@/components/shared/toggleModel';

export interface ToggleRuntimeProps {
  checked: boolean;
  disabled?: boolean;
  onToggle?: () => void;
  onChange?: (event: ToggleChangeEvent) => void;
}

export function useToggleState(props: ToggleRuntimeProps) {
  const isDisabled = () => Boolean(props.disabled);

  const handleClick = () => {
    if (isDisabled()) {
      return;
    }

    const next = !props.checked;
    let defaultPrevented = false;

    const event: ToggleChangeEvent = {
      currentTarget: { checked: next },
      preventDefault() {
        defaultPrevented = true;
      },
      stopPropagation() {
        /* noop for synthetic toggle event */
      },
      get defaultPrevented() {
        return defaultPrevented;
      },
    };

    props.onChange?.(event);
    if (!event.defaultPrevented) {
      props.onToggle?.();
    }
  };

  return {
    handleClick,
    isDisabled,
  };
}
