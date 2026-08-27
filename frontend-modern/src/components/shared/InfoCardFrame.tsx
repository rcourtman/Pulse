import { splitProps, type JSX } from 'solid-js';

export interface InfoCardFrameProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> {
  class?: string;
}

export interface InfoCardKeyValueRowProps extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'class' | 'children'
> {
  label: JSX.Element;
  value: JSX.Element;
  class?: string;
  labelClass?: string;
  labelTitle?: string;
  valueClass?: string;
  valueTitle?: string;
  desktopAt?: 'sm' | 'lg';
}

export const INFO_CARD_FRAME_CLASS = 'rounded border border-border bg-surface p-3 shadow-sm';

export const INFO_CARD_KEY_VALUE_ROW_CLASS = 'flex min-w-0 items-start justify-between gap-3';

const INFO_CARD_KEY_VALUE_ROW_DESKTOP_CLASS = {
  sm: 'sm:grid sm:grid-cols-[7rem_minmax(0,1fr)] sm:justify-normal',
  lg: 'lg:grid lg:grid-cols-[7rem_minmax(0,1fr)] lg:justify-normal',
} as const;

const INFO_CARD_KEY_VALUE_VALUE_DESKTOP_CLASS = {
  sm: 'sm:text-left',
  lg: 'lg:text-left',
} as const;

export function getInfoCardFrameClass(props: { class?: string } = {}): string {
  return [INFO_CARD_FRAME_CLASS, props.class ?? ''].filter(Boolean).join(' ');
}

export function getInfoCardKeyValueRowClass(
  props: Pick<InfoCardKeyValueRowProps, 'class' | 'desktopAt'> = {},
): string {
  const desktopAt = props.desktopAt ?? 'lg';
  return [
    INFO_CARD_KEY_VALUE_ROW_CLASS,
    INFO_CARD_KEY_VALUE_ROW_DESKTOP_CLASS[desktopAt],
    props.class ?? '',
  ]
    .filter(Boolean)
    .join(' ');
}

export function InfoCardFrame(props: InfoCardFrameProps): JSX.Element {
  const [local, rest] = splitProps(props, ['class']);

  return <div {...rest} class={getInfoCardFrameClass({ class: local.class })} />;
}

export function InfoCardKeyValueRow(props: InfoCardKeyValueRowProps): JSX.Element {
  const [local, rest] = splitProps(props, [
    'class',
    'desktopAt',
    'label',
    'labelClass',
    'labelTitle',
    'value',
    'valueClass',
    'valueTitle',
  ]);
  const desktopAt = local.desktopAt ?? 'lg';

  return (
    <div {...rest} class={getInfoCardKeyValueRowClass(local)}>
      <span
        class={`min-w-0 shrink-0 text-muted ${local.labelClass ?? ''}`}
        title={local.labelTitle}
      >
        {local.label}
      </span>
      <span
        class={`min-w-0 text-right font-medium text-base-content ${
          INFO_CARD_KEY_VALUE_VALUE_DESKTOP_CLASS[desktopAt]
        } ${local.valueClass ?? ''}`}
        title={local.valueTitle}
      >
        {local.value}
      </span>
    </div>
  );
}

export default InfoCardFrame;
