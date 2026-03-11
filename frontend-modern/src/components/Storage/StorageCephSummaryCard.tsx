import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EnhancedStorageBar } from '@/components/Storage/EnhancedStorageBar';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import {
  CEPH_SUMMARY_CARD_BAR_WRAP_CLASS,
  CEPH_SUMMARY_CARD_CLASS,
  CEPH_SUMMARY_CARD_CLUSTER_COUNT_CLASS,
  CEPH_SUMMARY_CARD_GRID_CLASS,
  CEPH_SUMMARY_CARD_HEADER_CLASS,
  CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS,
  CEPH_SUMMARY_CARD_HEADING_CLASS,
  CEPH_SUMMARY_CARD_INFO_WRAP_CLASS,
  CEPH_SUMMARY_CARD_MESSAGE_CLASS,
  CEPH_SUMMARY_CARD_TOP_LEFT_CLASS,
  CEPH_SUMMARY_CARD_TOP_RIGHT_CLASS,
  CEPH_SUMMARY_CARD_TOP_ROW_CLASS,
  CEPH_SUMMARY_CARD_TOTAL_CLASS,
  CEPH_SUMMARY_CARD_TITLE_CLASS,
  CEPH_SUMMARY_CARD_USAGE_CLASS,
} from '@/features/storageBackups/cephSummaryCardPresentation';
import { useStorageCephSummaryCardModel } from './useStorageCephSummaryCardModel';

type StorageCephSummaryCardProps = {
  summary: CephSummaryStats;
};

export const StorageCephSummaryCard: Component<StorageCephSummaryCardProps> = (props) => {
  const model = useStorageCephSummaryCardModel({
    summary: () => props.summary,
  });

  return (
    <Card padding="md" tone="card">
      <div class={CEPH_SUMMARY_CARD_TOP_ROW_CLASS}>
        <div class={CEPH_SUMMARY_CARD_TOP_LEFT_CLASS}>
          <div class={CEPH_SUMMARY_CARD_HEADING_CLASS}>
            {model.header().heading}
          </div>
          <div class={CEPH_SUMMARY_CARD_CLUSTER_COUNT_CLASS}>
            {model.header().clusterCountLabel}
          </div>
        </div>
        <div class={CEPH_SUMMARY_CARD_TOP_RIGHT_CLASS}>
          <div class={CEPH_SUMMARY_CARD_TOTAL_CLASS}>
            {model.header().totalLabel}
          </div>
          <div class={CEPH_SUMMARY_CARD_USAGE_CLASS}>
            {model.header().usageLabel}
          </div>
        </div>
      </div>
      <div class={CEPH_SUMMARY_CARD_GRID_CLASS}>
        <For each={model.clusterCards()}>
          {(cluster) => (
            <div class={CEPH_SUMMARY_CARD_CLASS}>
              <div class={CEPH_SUMMARY_CARD_HEADER_CLASS}>
                <div class={CEPH_SUMMARY_CARD_INFO_WRAP_CLASS}>
                  <div class={CEPH_SUMMARY_CARD_TITLE_CLASS}>
                    {cluster.title}
                  </div>
                  <Show when={cluster.healthMessage}>
                    <div class={CEPH_SUMMARY_CARD_MESSAGE_CLASS} title={cluster.healthMessage}>
                      {cluster.healthMessage}
                    </div>
                  </Show>
                </div>
                <span class={`${CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS} ${cluster.healthClass}`}>
                  {cluster.healthLabel}
                </span>
              </div>
              <div class={CEPH_SUMMARY_CARD_BAR_WRAP_CLASS}>
                <EnhancedStorageBar
                  used={cluster.usedBytes}
                  free={cluster.freeBytes}
                  total={cluster.totalBytes}
                />
              </div>
            </div>
          )}
        </For>
      </div>
    </Card>
  );
};

export default StorageCephSummaryCard;
