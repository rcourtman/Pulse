import { JSX, createUniqueId } from 'solid-js';
import {
  getToggleContainerClass,
  getToggleDescriptionClass,
  getToggleKnobClass,
  getToggleLabelClass,
  getToggleTrackClass,
  resolveToggleSize,
  type ToggleSize,
} from '@/components/shared/toggleModel';
export type { ToggleChangeEvent } from '@/components/shared/toggleModel';
import { type ToggleRuntimeProps, useToggleState } from '@/components/shared/useToggleState';

interface BaseToggleProps extends ToggleRuntimeProps {
  size?: ToggleSize;
  class?: string;
  title?: string;
  ariaLabel?: string;
  ariaLabelledBy?: string;
  ariaDescribedBy?: string;
}

export function TogglePrimitive(props: BaseToggleProps): JSX.Element {
  const size = resolveToggleSize(props.size);
  const state = useToggleState(props);

  return (
    <button
      type="button"
      class={getToggleTrackClass(size, props.checked, state.isDisabled(), props.class)}
      onClick={state.handleClick}
      disabled={props.disabled}
      title={props.title}
      aria-pressed={props.checked ? 'true' : 'false'}
      aria-label={props.ariaLabel}
      aria-labelledby={props.ariaLabel ? undefined : props.ariaLabelledBy}
      aria-describedby={props.ariaDescribedBy}
    >
      <span class={getToggleKnobClass(size, props.checked, state.isDisabled())} />
    </button>
  );
}

interface LabeledToggleProps extends BaseToggleProps {
  label?: JSX.Element;
  description?: JSX.Element;
  containerClass?: string;
  locked?: boolean;
  lockedMessage?: string;
}

export function Toggle(props: LabeledToggleProps) {
  const size = resolveToggleSize(props.size, 'md');
  const labelId = createUniqueId();
  const descriptionId = createUniqueId();

  return (
    <div class={getToggleContainerClass(props.containerClass)}>
      <TogglePrimitive
        {...props}
        size={size}
        ariaLabelledBy={props.label ? labelId : props.ariaLabelledBy}
        ariaDescribedBy={props.description ? descriptionId : props.ariaDescribedBy}
      />
      {(props.label || props.description) && (
        <span class={getToggleLabelClass()}>
          {props.label && <span id={labelId}>{props.label}</span>}
          {props.description && (
            <span id={descriptionId} class={getToggleDescriptionClass()}>
              {props.description}
            </span>
          )}
        </span>
      )}
    </div>
  );
}

export default Toggle;
