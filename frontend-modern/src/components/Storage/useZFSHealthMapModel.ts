import { createSignal } from 'solid-js';
import type { ZFSDevice, ZFSPool } from '@/types/api';
import {
  getZfsHealthMapDevices,
  getZfsHealthMapTooltipPresentation,
  isZfsHealthMapDeviceResilvering,
} from '@/features/storageBackups/zfsHealthMapPresentation';

export const useZFSHealthMapModel = (pool: () => ZFSPool) => {
  const [hoveredDevice, setHoveredDevice] = createSignal<ZFSDevice | null>(null);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const devices = () => getZfsHealthMapDevices(pool());
  const isResilvering = (device: ZFSDevice) => isZfsHealthMapDeviceResilvering(pool(), device);
  const hoveredTooltip = () =>
    hoveredDevice() ? getZfsHealthMapTooltipPresentation(hoveredDevice()!) : null;

  const handleMouseEnter = (e: MouseEvent, device: ZFSDevice) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setHoveredDevice(device);
  };

  const handleMouseLeave = () => {
    setHoveredDevice(null);
  };

  return {
    devices,
    hoveredDevice,
    hoveredTooltip,
    tooltipPos,
    isResilvering,
    handleMouseEnter,
    handleMouseLeave,
  };
};
