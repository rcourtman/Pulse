import { Component, For, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import {
  getZfsDeviceBlockClass,
  getZfsDeviceStateTextClass,
} from '@/features/storageBackups/zfsPresentation';
import {
  getZfsHealthMapDeviceClass,
  getZfsHealthMapErrorSummaryClass,
  getZfsHealthMapMessageClass,
  getZfsHealthMapTooltipStyle,
  ZFS_HEALTH_MAP_ROOT_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_NAME_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_PORTAL_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_STATE_ROW_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_STATE_TEXT_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_TYPE_CLASS,
} from '@/features/storageBackups/zfsHealthMapPresentation';
import type { ZFSPool } from '@/types/api';
import { useZFSHealthMapModel } from './useZFSHealthMapModel';

interface ZFSHealthMapProps {
  pool: ZFSPool;
}

export const ZFSHealthMap: Component<ZFSHealthMapProps> = (props) => {
  const {
    devices,
    hoveredDevice,
    hoveredTooltip,
    tooltipPos,
    isResilvering,
    handleMouseEnter,
    handleMouseLeave,
  } = useZFSHealthMapModel(() => props.pool);

  return (
    <div class={ZFS_HEALTH_MAP_ROOT_CLASS}>
      <For each={devices()}>
        {(device) => (
          <div
            class={getZfsHealthMapDeviceClass(
              getZfsDeviceBlockClass(device),
              isResilvering(device),
            )}
            onMouseEnter={(e) => handleMouseEnter(e, device)}
            onMouseLeave={handleMouseLeave}
          />
        )}
      </For>

      <Show when={hoveredTooltip()}>
        <Portal mount={document.body}>
          <div
            class={ZFS_HEALTH_MAP_TOOLTIP_PORTAL_CLASS}
            style={getZfsHealthMapTooltipStyle(tooltipPos().x, tooltipPos().y)}
          >
            <div class={ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS}>
              <div class={ZFS_HEALTH_MAP_TOOLTIP_NAME_CLASS}>{hoveredTooltip()?.name}</div>
              <div class={ZFS_HEALTH_MAP_TOOLTIP_TYPE_CLASS}>{hoveredTooltip()?.type}</div>
              <div class={ZFS_HEALTH_MAP_TOOLTIP_STATE_ROW_CLASS}>
                <span class={`${ZFS_HEALTH_MAP_TOOLTIP_STATE_TEXT_CLASS} ${getZfsDeviceStateTextClass(hoveredDevice()!)}`}>
                  {hoveredTooltip()?.state}
                </span>
                <Show when={hoveredTooltip()?.hasErrors}>
                  <span class={getZfsHealthMapErrorSummaryClass()}>
                    ({hoveredTooltip()?.errorSummary})
                  </span>
                </Show>
              </div>
              <Show when={hoveredTooltip()?.message}>
                <div class={getZfsHealthMapMessageClass()}>
                  {hoveredTooltip()?.message}
                </div>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </div>
  );
};
