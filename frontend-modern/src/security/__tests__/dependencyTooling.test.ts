// @vitest-environment node

import { readFileSync } from 'node:fs';
import * as prettier from 'prettier';
import { describe, expect, it } from 'vitest';

const manifest = JSON.parse(
  readFileSync(new URL('../../../package.json', import.meta.url), 'utf8'),
) as { devDependencies: { prettier: string } };

describe('reviewed formatter behaviour', () => {
  it('uses the exact locked formatter rather than a stale local install', () => {
    expect(prettier.version).toBe(manifest.devDependencies.prettier);
  });

  it('preserves spaces around copied code when Markdown contains dollar variables', async () => {
    // Prettier 3.9.8's inline-math parser removed spaces after $BAR below.
    // This is plain shell-variable documentation, not a mathematical formula.
    const markdown =
      '**Uses $FOO** from `a.sh` and `b.sh`, plus `$BAR` from `c.sh`, before anything else runs here.\n';
    expect(
      await prettier.format(markdown, {
        parser: 'markdown',
        proseWrap: 'preserve',
      }),
    ).toBe(markdown);
  });
});
