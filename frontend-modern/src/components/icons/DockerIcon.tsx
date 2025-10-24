import type { Component } from 'solid-js';
import Container from 'lucide-solid/icons/container';

interface DockerIconProps {
  class?: string;
  title?: string;
}

export const DockerIcon: Component<DockerIconProps> = (props) => (
  <Container class={props.class} aria-label={props.title ?? 'Docker'} />
);
