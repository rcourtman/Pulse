import { For, Show } from 'solid-js';
import {
  getZfsPoolErrorOverlayClass,
  getZfsPoolStateTextClass,
} from '@/features/storageBackups/zfsPresentation';
import {
  getZfsErrorTextClass,
  getZfsScanTextClass,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import {
  STORAGE_BAR_LABEL_TEXT_CLASS,
  STORAGE_BAR_LABEL_WRAP_CLASS,
  STORAGE_BAR_PROGRESS_CLASS,
  STORAGE_BAR_PULSE_OVERLAY_CLASS,
  STORAGE_BAR_ROOT_CLASS,
  STORAGE_BAR_TOOLTIP_LABEL_CLASS,
  STORAGE_BAR_TOOLTIP_TITLE_CLASS,
  STORAGE_BAR_TOOLTIP_VALUE_CLASS,
  STORAGE_BAR_TOOLTIP_WRAP_CLASS,
  STORAGE_BAR_ZFS_HEADING_CLASS,
  STORAGE_BAR_ZFS_SECTION_CLASS,
  STORAGE_BAR_ZFS_STATE_LABEL_CLASS,
  STORAGE_BAR_ZFS_STATE_ROW_CLASS,
  getStorageBarTooltipRowClass,
  getStorageBarTooltipTitle,
  getStorageBarZfsHeadingLabel,
} from '@/features/storageBackups/storageBarPresentation';
import { useTooltip } from '@/hooks/useTooltip';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import type { ZFSPool } from '@/types/api';
import { useEnhancedStorageBarModel } from './useEnhancedStorageBarModel';

interface EnhancedStorageBarProps {
  used: number;
  total: number;
  free: number;
  zfsPool?: ZFSPool;
}

export function EnhancedStorageBar(props: EnhancedStorageBarProps) {
  const tip = useTooltip();
  const { usagePercent, barColor, label, tooltipRows, zfsSummary } = useEnhancedStorageBarModel({
    used: () => props.used,
    total: () => props.total,
    free: () => props.free,
    zfsPool: () => props.zfsPool,
  });

  return (
    <div class={STORAGE_BAR_ROOT_CLASS}>
      <ProgressBar
        value={usagePercent()}
        class={STORAGE_BAR_PROGRESS_CLASS}
        fillClass={barColor()}
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
        overlays={
          <>
            <Show when={zfsSummary()?.isScrubbing || zfsSummary()?.isResilvering}>
              <div class={STORAGE_BAR_PULSE_OVERLAY_CLASS} />
            </Show>

            <Show when={zfsSummary()?.hasErrors}>
              <div class={getZfsPoolErrorOverlayClass(Boolean(zfsSummary()?.hasErrors))} />
            </Show>
          </>
        }
        label={
          <span class={STORAGE_BAR_LABEL_WRAP_CLASS}>
            <span class={STORAGE_BAR_LABEL_TEXT_CLASS}>
              {label()}
            </span>
          </span>
        }
      />

      {/* Tooltip */}
      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class={STORAGE_BAR_TOOLTIP_WRAP_CLASS}>
          <div class={STORAGE_BAR_TOOLTIP_TITLE_CLASS}>
            {getStorageBarTooltipTitle()}
          </div>

          <For each={tooltipRows()}>
            {(row) => (
              <div class={getStorageBarTooltipRowClass(row.bordered)}>
                <span class={STORAGE_BAR_TOOLTIP_LABEL_CLASS}>{row.label}</span>
                <span class={STORAGE_BAR_TOOLTIP_VALUE_CLASS}>{row.value}</span>
              </div>
            )}
          </For>

          <Show when={zfsSummary()}>
            <div class={STORAGE_BAR_ZFS_SECTION_CLASS}>
              <div class={STORAGE_BAR_ZFS_HEADING_CLASS}>{getStorageBarZfsHeadingLabel()}</div>
              <div class={STORAGE_BAR_ZFS_STATE_ROW_CLASS}>
                <span class={STORAGE_BAR_ZFS_STATE_LABEL_CLASS}>State</span>
                <span class={getZfsPoolStateTextClass(Boolean(zfsSummary()?.hasErrors))}>
                  {zfsSummary()?.state}
                </span>
              </div>
              <Show when={zfsSummary()?.scan}>
                <div class={`${getZfsScanTextClass()} mt-0.5 max-w-[200px] break-words`}>
                  {zfsSummary()?.scan}
                </div>
              </Show>
              <Show when={zfsSummary()?.errorSummary}>
                <div class={`${getZfsErrorTextClass()} mt-0.5`}>
                  {zfsSummary()?.errorSummary}
                </div>
              </Show>
            </div>
          </Show>
        </div>
      </TooltipPortal>
    </div>
  );
}
