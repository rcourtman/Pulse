import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import { usePlatformWindowedItems } from '@/features/platformPage/usePlatformWindowedItems';
import type { WorkloadCoverageRow } from './proxmoxBackupRecoveryModel';

// Summary height is shared; detail height belongs to an identity, not its
// sorted index. Unseen rows use estimates until their actual block is mounted.
export function useProxmoxCoverageWindowing(
  rows: Accessor<readonly WorkloadCoverageRow[]>,
  expanded: Accessor<ReadonlySet<string>>,
  measurementKey: Accessor<unknown>,
) {
  const [summaryHeight, setSummaryHeight] = createSignal(32);
  const [detailHeights, setDetailHeights] = createSignal<ReadonlyMap<string, number>>(new Map());
  createEffect(() => {
    const keys = new Set(rows().map((row) => row.key));
    setDetailHeights((current) => {
      if ([...current.keys()].every((key) => keys.has(key))) return current;
      return new Map([...current].filter(([key]) => keys.has(key)));
    });
  });
  const offsets = createMemo(() => {
    const heights = detailHeights();
    const open = expanded();
    const summary = summaryHeight();
    const result = [0];
    for (const row of rows()) {
      const height = summary + (open.has(row.key) ? (heights.get(row.key) ?? 320) : 0);
      result.push(result[result.length - 1]! + height);
    }
    return result;
  });
  const measure = (root: HTMLElement) => {
    const body = root.querySelector('tbody');
    if (!body) return;
    const summaries = Array.from(body.children).filter(
      (row): row is HTMLElement =>
        row instanceof HTMLElement && row.dataset.proxmoxBackupRow === 'coverage',
    );
    const height = summaries[0]?.getBoundingClientRect().height;
    if (height && Number.isFinite(height)) setSummaryHeight(height);
    setDetailHeights((current) => {
      let next: Map<string, number> | undefined;
      for (const summary of summaries) {
        const key = summary.dataset.proxmoxCoverageKey;
        const detail = summary.nextElementSibling;
        if (!key || detail?.getAttribute('data-inline-detail-for') !== key) continue;
        const measured = detail.getBoundingClientRect().height;
        if (!Number.isFinite(measured) || measured <= 0 || current.get(key) === measured) continue;
        next ??= new Map(current);
        next.set(key, measured);
      }
      return next ?? current;
    });
  };
  const state = usePlatformWindowedItems({
    items: rows,
    estimatedItemHeight: 32,
    itemGeometry: { offsets, measure },
    measurementKey,
  });
  return { ...state, setRootRef: state.setAnchorRef };
}
