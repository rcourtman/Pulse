import type { Component } from 'solid-js';

interface PulseLogoIconProps {
  class?: string;
  title?: string;
}

/**
 * Monochrome Pulse logo icon (three concentric circles).
 * Uses a mask to cut transparent gaps so the ring and center dot
 * are visible even at small sizes with a single color.
 */
export const PulseLogoIcon: Component<PulseLogoIconProps> = (props) => (
  <svg
    class={props.class}
    role="img"
    viewBox="0 0 256 256"
    xmlns="http://www.w3.org/2000/svg"
    aria-label={props.title ?? 'Pulse'}
  >
    <title>{props.title ?? 'Pulse'}</title>
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
