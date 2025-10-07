import type { Component } from 'solid-js';
import { siDocker } from 'simple-icons';

interface DockerIconProps {
  class?: string;
  title?: string;
}

export const DockerIcon: Component<DockerIconProps> = (props) => (
  <svg
    class={props.class}
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    aria-label={props.title ?? 'Docker'}
  >
    <title>{props.title ?? 'Docker'}</title>
    <path d={siDocker.path} fill="currentColor" />
  </svg>
);
