import { describe, expect, it } from 'vitest';
import {
  AVAILABILITY_TARGET_PRESETS,
  applyAvailabilityTargetPreset,
  availabilityPresetById,
} from '../availabilityTargetPresets';

describe('availabilityTargetPresets', () => {
  it('includes ESPHome as a TCP availability preset', () => {
    expect(availabilityPresetById('esphome-device')).toEqual(
      expect.objectContaining({
        label: 'ESPHome device',
        protocol: 'tcp',
        port: '6053',
      }),
    );
  });

  it('applies preset probe defaults without changing endpoint identity fields', () => {
    const form = {
      id: '',
      name: 'Rack sensor',
      address: 'rack-sensor.local',
      protocol: 'icmp' as const,
      port: '',
      path: '/health',
      enabled: true,
    };

    expect(applyAvailabilityTargetPreset(form, 'mqtt-broker')).toEqual({
      ...form,
      protocol: 'tcp',
      port: '1883',
      path: '',
    });
    expect(applyAvailabilityTargetPreset(form, 'custom')).toBe(form);
  });

  it('keeps preset ids unique', () => {
    const ids = AVAILABILITY_TARGET_PRESETS.map((preset) => preset.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});
