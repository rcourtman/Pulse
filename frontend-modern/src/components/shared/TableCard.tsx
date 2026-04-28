import { splitProps } from 'solid-js';

import { Card, type CardProps } from '@/components/shared/Card';

export type TableCardProps = Omit<CardProps, 'border' | 'padding' | 'tone'>;

export const TABLE_CARD_FRAME_CLASS = 'overflow-hidden';

export function TableCard(props: TableCardProps) {
  const [local, rest] = splitProps(props, ['class']);

  return (
    <Card
      {...rest}
      border={true}
      padding="none"
      tone="card"
      class={`${TABLE_CARD_FRAME_CLASS} ${local.class ?? ''}`.trim()}
    />
  );
}

export default TableCard;
