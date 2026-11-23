import { Show, type Component } from 'solid-js';

interface PulseLogoIconProps {
  class?: string;
  title?: string;
  decorative?: boolean;
  'aria-hidden'?: boolean | 'true' | 'false';
}

/**
 * Monochrome Pulse logo icon (three concentric circles).
 * Uses a mask to cut transparent gaps so the ring and center dot
 * are visible even at small sizes with a single color.
 */
export const PulseLogoIcon: Component<PulseLogoIconProps> = (props) => {
  const isDecorative = () =>
    Boolean(props.decorative) || props['aria-hidden'] === true || props['aria-hidden'] === 'true';

  return (
    <svg
      class={props.class}
      role={isDecorative() ? undefined : 'img'}
      viewBox="0 0 256 256"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden={isDecorative() ? 'true' : undefined}
      aria-label={isDecorative() ? undefined : (props.title ?? 'Pulse')}
    >
      <Show when={!isDecorative()}>
        <title>{props.title ?? 'Pulse'}</title>
      </Show>
      <defs>
        <mask id="pulse-logo-mask">
          {/* Start with everything visible */}
          <rect width="256" height="256" fill="white" />
          {/* Cut out the gap between outer disc and ring */}
          <circle cx="128" cy="128" r="100" fill="black" />
          {/* Add the ring back */}
          <circle cx="128" cy="128" r="91" fill="white" />
          {/* Cut out the gap between ring and center dot */}
          <circle cx="128" cy="128" r="68" fill="black" />
          {/* Add the center dot back */}
          <circle cx="128" cy="128" r="30" fill="white" />
        </mask>
      </defs>
      <circle cx="128" cy="128" r="122" fill="currentColor" mask="url(#pulse-logo-mask)" />
    </svg>
  );
};
