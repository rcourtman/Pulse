import { Show, type Component } from 'solid-js';

import {
  DetailSectionTable,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
} from './DetailSectionTable';

export interface DrawerAttentionItem {
  id: string;
  message: string;
  severity?: string;
  acknowledged?: boolean;
}

interface DrawerAttentionSectionProps {
  items: DrawerAttentionItem[];
}

const severityLabel = (item: DrawerAttentionItem): string => {
  if (item.acknowledged) return 'Acknowledged';
  return item.severity?.toLowerCase() === 'critical' ? 'Critical' : 'Warning';
};

export const DrawerAttentionSection: Component<DrawerAttentionSectionProps> = (props) => {
  const visibleItems = () => props.items.slice(0, 3);
  const remainingCount = () => Math.max(0, props.items.length - visibleItems().length);
  const sections = () =>
    compactDetailSections([
      {
        label: 'Needs attention',
        rows: compactDetailRows([
          ...visibleItems().map((item) =>
            makeDetailRow(severityLabel(item), item.message, {
              tone: item.acknowledged
                ? 'muted'
                : item.severity?.toLowerCase() === 'critical'
                  ? 'danger'
                  : 'warning',
              title: item.message,
              wrap: true,
            }),
          ),
          remainingCount() > 0
            ? makeDetailRow('Also', `${remainingCount()} more active`, { tone: 'warning' })
            : null,
        ]),
      },
    ]);

  return (
    <Show when={props.items.length > 0}>
      <div data-testid="drawer-attention-section">
        <DetailSectionTable sections={sections()} />
      </div>
    </Show>
  );
};
