import type { Component } from 'solid-js';

interface AlertsIconProps {
  class?: string;
  title?: string;
}

// Path data sourced from Lucide "Bell" icon (https://lucide.dev/) -- licensed under ISC
export const AlertsIcon: Component<AlertsIconProps> = (props) => (
  <svg
    class={props.class}
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    aria-label={props.title ?? 'Alerts'}
  >
    <title>{props.title ?? 'Alerts'}</title>
    <path
      d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <path
      d="M13.73 21a2 2 0 0 1-3.46 0"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
);
