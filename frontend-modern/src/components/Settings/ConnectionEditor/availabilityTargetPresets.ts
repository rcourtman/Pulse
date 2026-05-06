import type { AvailabilityProbeProtocol } from '@/api/availabilityTargets';

export const CUSTOM_AVAILABILITY_PRESET_ID = 'custom';

export type AvailabilityTargetPresetID =
  | typeof CUSTOM_AVAILABILITY_PRESET_ID
  | 'ping-device'
  | 'mqtt-broker'
  | 'esphome-device'
  | 'esphome-dashboard';

export interface AvailabilityTargetPreset {
  id: AvailabilityTargetPresetID;
  label: string;
  protocol: AvailabilityProbeProtocol;
  port?: string;
  path?: string;
  addressPlaceholder: string;
  portPlaceholder?: string;
}

export interface AvailabilityPresetFields {
  protocol: AvailabilityProbeProtocol;
  port: string;
  path: string;
}

export const AVAILABILITY_TARGET_PRESETS: readonly AvailabilityTargetPreset[] = [
  {
    id: 'ping-device',
    label: 'Pingable device',
    protocol: 'icmp',
    addressPlaceholder: 'device.local',
  },
  {
    id: 'mqtt-broker',
    label: 'MQTT broker',
    protocol: 'tcp',
    port: '1883',
    addressPlaceholder: 'mqtt.local',
    portPlaceholder: '1883',
  },
  {
    id: 'esphome-device',
    label: 'ESPHome device',
    protocol: 'tcp',
    port: '6053',
    addressPlaceholder: 'sensor.local',
    portPlaceholder: '6053',
  },
  {
    id: 'esphome-dashboard',
    label: 'ESPHome dashboard',
    protocol: 'http',
    port: '6052',
    addressPlaceholder: 'http://esphome.local',
    portPlaceholder: '6052',
  },
] as const;

export const availabilityPresetById = (presetId: string): AvailabilityTargetPreset | undefined =>
  AVAILABILITY_TARGET_PRESETS.find((preset) => preset.id === presetId);

export const applyAvailabilityTargetPreset = <T extends AvailabilityPresetFields>(
  form: T,
  presetId: string,
): T => {
  const preset = availabilityPresetById(presetId);
  if (!preset) return form;

  return {
    ...form,
    protocol: preset.protocol,
    port: preset.port ?? '',
    path: preset.path ?? '',
  };
};
