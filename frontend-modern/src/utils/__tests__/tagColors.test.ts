import { describe, it, expect } from 'vitest';
import { getTagColorWithSpecial } from '../tagColors';

describe('tagColors', () => {
  describe('getTagColorWithSpecial', () => {
    it('returns consistent colors for special tags in light mode', () => {
      const production = getTagColorWithSpecial('production', false);
      expect(production).toEqual({
        bg: 'rgb(254, 226, 226)',
        text: 'rgb(153, 27, 27)',
        border: 'rgb(239, 68, 68)',
      });

      const staging = getTagColorWithSpecial('STAGING', false); // case insensitive
      expect(staging).toEqual({
        bg: 'rgb(254, 243, 199)',
        text: 'rgb(146, 64, 14)',
        border: 'rgb(245, 158, 11)',
      });
    });

    it('returns consistent colors for special tags in dark mode', () => {
      const backup = getTagColorWithSpecial('backup', true);
      expect(backup).toEqual({
        bg: 'rgb(30, 58, 138)',
        text: 'rgb(191, 219, 254)',
        border: 'rgb(59, 130, 246)',
      });
    });

    it('generates hash-based colors for non-special tags', () => {
      const color1 = getTagColorWithSpecial('mytag', false);
      const color2 = getTagColorWithSpecial('mytag', false);

      // Consistency
      expect(color1).toEqual(color2);

      // Values (H, S, L format)
      expect(color1.bg).toMatch(/hsl\(\d+, 65%, 60%\)/);
      expect(color1.text).toMatch(/hsl\(\d+, 65%, 25%\)/);
      expect(color1.border).toMatch(/hsl\(\d+, 65%, 50%\)/);
    });

    it('generates different colors for different tags', () => {
      const colorA = getTagColorWithSpecial('tagA', false);
      const colorB = getTagColorWithSpecial('tagB', false);

      expect(colorA.bg).not.toBe(colorB.bg);
    });

    it('generates dark mode hash-based colors', () => {
      const color = getTagColorWithSpecial('custom', true);

      expect(color.bg).toMatch(/hsl\(\d+, 55%, 35%\)/);
      expect(color.text).toMatch(/hsl\(\d+, 55%, 85%\)/);
      expect(color.border).toMatch(/hsl\(\d+, 55%, 45%\)/);
    });
  });
});
