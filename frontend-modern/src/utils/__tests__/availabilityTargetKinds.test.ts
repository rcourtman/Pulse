import { describe, expect, it } from 'vitest';
import {
  AVAILABILITY_TARGET_KINDS,
  AVAILABILITY_TARGET_KIND_VALUES,
  getAvailabilityTargetAddLabel,
  getAvailabilityTargetKindInfo,
  isNetworkGearKind,
  normalizeAvailabilityTargetKind,
  suggestAvailabilityTargetKind,
} from '../availabilityTargetKinds';

describe('availability target kinds', () => {
  it('names every kind the server accepts, once, for every page that shows one', () => {
    expect(AVAILABILITY_TARGET_KIND_VALUES).toEqual([
      'service',
      'machine',
      'router',
      'switch',
      'access_point',
      'device',
    ]);
    expect(AVAILABILITY_TARGET_KINDS.map((kind) => kind.label)).toEqual([
      'Website or service',
      'Machine or server',
      'Router or firewall',
      'Switch',
      'Access point',
      'Other device',
    ]);
    expect(getAvailabilityTargetKindInfo(' Access_Point ')?.noun).toBe('access point');
    expect(getAvailabilityTargetKindInfo('database')).toBeUndefined();
    expect(normalizeAvailabilityTargetKind('Machine')).toBe('machine');
    expect(normalizeAvailabilityTargetKind('vm')).toBeUndefined();
  });

  it('knows which kinds are network gear', () => {
    expect(AVAILABILITY_TARGET_KIND_VALUES.filter((kind) => isNetworkGearKind(kind))).toEqual([
      'router',
      'switch',
      'access_point',
    ]);
    expect(isNetworkGearKind(undefined)).toBe(false);
  });

  it('words the button that saves a new check after what the check is on', () => {
    expect(getAvailabilityTargetAddLabel('machine')).toBe('Add machine check');
    expect(getAvailabilityTargetAddLabel('router')).toBe('Add router check');
    expect(getAvailabilityTargetAddLabel('access_point')).toBe('Add access point check');
    expect(getAvailabilityTargetAddLabel('service')).toBe('Add service/device check');
    expect(getAvailabilityTargetAddLabel('device')).toBe('Add service/device check');
    expect(getAvailabilityTargetAddLabel(undefined)).toBe('Add availability check');
  });

  it('suggests network gear from what the owner called the check', () => {
    const suggest = (name: string) => suggestAvailabilityTargetKind({ name });
    for (const name of ['Router', 'OPNsense', 'pfsense-fw', 'FRITZ!Box 7590', 'UDM Pro']) {
      expect(suggest(name)).toBe('router');
    }
    for (const name of ['Core switch', 'switch-24-rack', 'USW_Switch_8']) {
      expect(suggest(name)).toBe('switch');
    }
    for (const name of ['AP living room', 'ap-garage', 'Wi-Fi upstairs', 'Access Point 2']) {
      expect(suggest(name)).toBe('access_point');
    }
    // A wireless router is the router. A word inside another word is not it.
    expect(suggest('Router AP')).toBe('router');
    for (const name of ['Status page', 'API gateway', 'Nintendo Switch', 'apt-cache', 'Grafana']) {
      expect(suggest(name)).toBeUndefined();
    }
  });

  it('suggests a router for a check on a default gateway, whatever it is called', () => {
    const gatewayAddresses = new Set(['192.168.0.1']);
    expect(
      suggestAvailabilityTargetKind({ name: 'Fritz', address: ' 192.168.0.1 ', gatewayAddresses }),
    ).toBe('router');
    expect(
      suggestAvailabilityTargetKind({ name: 'Fritz', address: '192.168.0.9', gatewayAddresses }),
    ).toBeUndefined();
    // A name that says otherwise is not overruled by the address.
    expect(
      suggestAvailabilityTargetKind({
        name: 'Core switch',
        address: '192.168.0.1',
        gatewayAddresses,
      }),
    ).toBe('switch');
  });
});
