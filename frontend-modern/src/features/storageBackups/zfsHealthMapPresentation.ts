import type { ZFSDevice, ZFSPool } from '@/types/api';

export type ZfsHealthMapTooltipPresentation = {
  name: string;
  type: string;
  state: string;
  message: string;
  errorSummary: string;
  hasErrors: boolean;
};

export const ZFS_HEALTH_MAP_ROOT_CLASS = 'flex items-center gap-0.5';
export const ZFS_HEALTH_MAP_TOOLTIP_PORTAL_CLASS = 'fixed z-[9999] pointer-events-none';
export const ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS =
  'bg-surface text-base-content text-[10px] rounded-md shadow-sm px-2 py-1.5 min-w-[120px] border border-border';
export const ZFS_HEALTH_MAP_TOOLTIP_NAME_CLASS = 'font-medium mb-0.5 text-base-content';
export const ZFS_HEALTH_MAP_TOOLTIP_TYPE_CLASS = 'text-muted mb-1';
export const ZFS_HEALTH_MAP_TOOLTIP_STATE_ROW_CLASS = 'flex items-center gap-2 border-t border-border pt-1';
export const ZFS_HEALTH_MAP_TOOLTIP_STATE_TEXT_CLASS = 'font-semibold';
export const ZFS_HEALTH_MAP_TOOLTIP_TRANSFORM = 'translate(-50%, -100%)';

export const getZfsHealthMapDevices = (pool: ZFSPool): ZFSDevice[] => pool.devices || [];

export const getZfsHealthMapDeviceClass = (
  baseClass: string,
  resilvering: boolean,
): string =>
  `w-2.5 h-3 rounded-sm transition-colors duration-200 ${baseClass} ${resilvering ? 'animate-pulse' : ''}`;

export const getZfsHealthMapErrorSummaryClass = (): string => 'text-red-400';

export const getZfsHealthMapMessageClass = (): string =>
  'text-muted mt-1 italic max-w-[200px] break-words';

export const getZfsHealthMapTooltipStyle = (x: number, y: number) => ({
  left: `${x}px`,
  top: `${y - 8}px`,
  transform: ZFS_HEALTH_MAP_TOOLTIP_TRANSFORM,
});

export const isZfsHealthMapDeviceResilvering = (
  pool: ZFSPool,
  device: ZFSDevice,
): boolean => {
  const scan = pool.scan?.toLowerCase() || '';
  return scan.includes('resilver') || (device.message || '').toLowerCase().includes('resilver');
};

export const getZfsHealthMapTooltipPresentation = (
  device: ZFSDevice,
): ZfsHealthMapTooltipPresentation => {
  const readErrors = device.readErrors || 0;
  const writeErrors = device.writeErrors || 0;
  const checksumErrors = device.checksumErrors || 0;
  const hasErrors = readErrors > 0 || writeErrors > 0 || checksumErrors > 0;

  return {
    name: device.name,
    type: device.type,
    state: device.state,
    message: device.message || '',
    errorSummary: hasErrors ? `E: ${readErrors}/${writeErrors}/${checksumErrors}` : '',
    hasErrors,
  };
};
