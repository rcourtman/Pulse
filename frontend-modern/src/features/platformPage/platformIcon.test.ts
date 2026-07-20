import { describe, expect, it } from 'vitest';

import CpuIcon from 'lucide-solid/icons/cpu';
import ServerIcon from 'lucide-solid/icons/server';
import UsersIcon from 'lucide-solid/icons/users';

import { DockerIcon } from '@/components/icons/DockerIcon';
import { KubernetesIcon } from '@/components/icons/KubernetesIcon';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { TrueNASIcon } from '@/components/icons/TrueNASIcon';

import { getPlatformIcon, type PlatformIconKey } from './platformIcon';

const ALL_KEYS: PlatformIconKey[] = [
  'proxmox',
  'docker',
  'kubernetes',
  'truenas',
  'vmware',
  'standalone',
  'systems',
];

describe('platformIcon', () => {
  describe('getPlatformIcon', () => {
    // Brand marks come from the inlined simple-icons SVG components.
    it.each([
      ['proxmox', ProxmoxIcon],
      ['docker', DockerIcon],
      ['kubernetes', KubernetesIcon],
      ['truenas', TrueNASIcon],
    ] as Array<[PlatformIconKey, ReturnType<typeof getPlatformIcon>]>)(
      'returns the brand glyph for %s',
      (key, icon) => {
        expect(getPlatformIcon(key)).toBe(icon);
      },
    );

    // vSphere has no legible square brand glyph, so it keeps a generic CPU
    // mark; standalone/systems are not third-party brands and use semantic
    // generic lucide icons.
    it.each([
      ['vmware', CpuIcon],
      ['standalone', ServerIcon],
      ['systems', UsersIcon],
    ] as Array<[PlatformIconKey, ReturnType<typeof getPlatformIcon>]>)(
      'returns the generic icon for %s',
      (key, icon) => {
        expect(getPlatformIcon(key)).toBe(icon);
      },
    );

    it('resolves an icon for every key in the PlatformIconKey union', () => {
      for (const key of ALL_KEYS) {
        const icon = getPlatformIcon(key);
        expect(icon).toBeDefined();
        expect(typeof icon).toBe('function');
      }
    });

    it('returns the same icon component reference for repeated lookups', () => {
      for (const key of ALL_KEYS) {
        expect(getPlatformIcon(key)).toBe(getPlatformIcon(key));
      }
    });

    it('returns undefined for an unknown key at runtime', () => {
      expect(getPlatformIcon('does-not-exist' as unknown as PlatformIconKey)).toBeUndefined();
    });

    it('every resolved icon accepts a Solid props object (renderable component shape)', () => {
      for (const key of ALL_KEYS) {
        const icon = getPlatformIcon(key)!;
        expect(typeof icon).toBe('function');
        // Solid components have a finite arity (props). A non-component value
        // like a bare object would not be callable.
        expect(icon.length).toBeLessThanOrEqual(1);
      }
    });
  });
});
