import { describe, expect, it } from 'vitest';
import { getTagColorWithSpecial } from '../tagColors';

describe('tagColors', () => {
  describe('getTagColorWithSpecial', () => {
    it('matches Proxmox fallback colors for tags', () => {
      expect(getTagColorWithSpecial('production', false)).toEqual({
        bg: 'rgb(191.3, 176.60000000000002, 238.89999999999998)',
        text: '#000000',
        border: 'rgb(191.3, 176.60000000000002, 238.89999999999998)',
      });

      expect(getTagColorWithSpecial('STAGING', false)).toEqual({
        bg: 'rgb(103.10000000000001, 103.10000000000001, 175.9)',
        text: '#ffffff',
        border: 'rgb(103.10000000000001, 103.10000000000001, 175.9)',
      });
    });

    it('is deterministic for the same tag', () => {
      const color1 = getTagColorWithSpecial('mytag', false);
      const color2 = getTagColorWithSpecial('mytag', true);

      expect(color1).toEqual(color2);
      expect(color1).toEqual({
        bg: 'rgb(228.39999999999998, 164.7, 128.3)',
        text: '#000000',
        border: 'rgb(228.39999999999998, 164.7, 128.3)',
      });
    });

    it('generates different colors for different tags', () => {
      const colorA = getTagColorWithSpecial('tagA', false);
      const colorB = getTagColorWithSpecial('tagB', false);

      expect(colorA.bg).not.toBe(colorB.bg);
    });

    it('prefers proxmox-supplied colors over generated fallback colors', () => {
      expect(
        getTagColorWithSpecial('Production', false, {
          production: '#112233',
        }),
      ).toEqual({
        bg: 'rgb(17, 34, 51)',
        text: '#ffffff',
        border: 'rgb(17, 34, 51)',
      });
    });

    it('falls back to generated colors when proxmox color is invalid', () => {
      expect(
        getTagColorWithSpecial('backup', false, {
          backup: 'not-a-color',
        }),
      ).toEqual({
        bg: 'rgb(108.00000000000001, 149.3, 143.7)',
        text: '#ffffff',
        border: 'rgb(108.00000000000001, 149.3, 143.7)',
      });
    });
  });
});
