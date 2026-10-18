import { describe, expect, it } from 'vitest';
import {
  getDiskTemperatureByTypeItems,
  resolveDiskTemperatureTriggerForType,
  resolveTrueNASDiskTemperatureDefault,
} from '../trueNASDiskTemperature';

const factoryPolicy = {
  agentDiskTemperature: 55,
  diskTempByType: { nvme: 70, sas: 65, sata: 55 },
};

describe('TrueNAS disk temperature in the thresholds editor', () => {
  it('gives each disk its type trigger, and unknown types the agent default', () => {
    expect(resolveDiskTemperatureTriggerForType(factoryPolicy, 'NVMe')).toBe(70);
    expect(resolveDiskTemperatureTriggerForType(factoryPolicy, 'sas')).toBe(65);
    expect(resolveDiskTemperatureTriggerForType(factoryPolicy, 'sata')).toBe(55);
    expect(resolveDiskTemperatureTriggerForType(factoryPolicy, 'hdd')).toBe(55);
    expect(
      resolveDiskTemperatureTriggerForType({ ...factoryPolicy, agentDiskTemperature: 60 }, ''),
    ).toBe(60);
  });

  it('switches every type off with the agent Disk Temp default', () => {
    const off = { ...factoryPolicy, agentDiskTemperature: -1 };
    expect(resolveDiskTemperatureTriggerForType(off, 'nvme')).toBe(0);
    expect(getDiskTemperatureByTypeItems(off)).toEqual([]);
  });

  it('lets a TrueNAS-wide value, Off included, replace the type trigger', () => {
    expect(resolveTrueNASDiskTemperatureDefault(undefined, factoryPolicy, 'nvme')).toBe(70);
    expect(resolveTrueNASDiskTemperatureDefault(62, factoryPolicy, 'nvme')).toBe(62);
    expect(resolveTrueNASDiskTemperatureDefault(-1, factoryPolicy, 'nvme')).toBe(-1);
  });

  it('names the per-type triggers', () => {
    expect(
      getDiskTemperatureByTypeItems({
        ...factoryPolicy,
        diskTempByType: { nvme: 75, sas: 65, sata: 55 },
      }),
    ).toEqual(['NVMe 75°C', 'SAS 65°C', 'SATA 55°C']);
  });
});
