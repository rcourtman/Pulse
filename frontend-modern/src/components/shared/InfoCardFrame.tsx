import { splitProps, type JSX } from 'solid-js';

export interface InfoCardFrameProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> {
  class?: string;
}

export const INFO_CARD_FRAME_CLASS = 'rounded border border-border bg-surface p-3 shadow-sm';

export function getInfoCardFrameClass(props: { class?: string } = {}): string {
  return [INFO_CARD_FRAME_CLASS, props.class ?? ''].filter(Boolean).join(' ');
}

export function InfoCardFrame(props: InfoCardFrameProps): JSX.Element {
  const [local, rest] = splitProps(props, ['class']);

  return <div {...rest} class={getInfoCardFrameClass({ class: local.class })} />;
}

export default InfoCardFrame;
