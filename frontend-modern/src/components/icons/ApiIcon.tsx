import type { Component } from 'solid-js';

interface ApiIconProps {
  class?: string;
}

export const ApiIcon: Component<ApiIconProps> = (props) => (
  <svg
    class={props.class}
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M4 7v10" />
    <path d="M20 7v10" />
    <path d="M9 12h6" />
    <path d="M12 9v6" />
    <rect x="2.5" y="5" width="19" height="14" rx="2" />
  </svg>
);

export default ApiIcon;
