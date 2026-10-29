// @vitest-environment node

import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const fixture = fileURLToPath(new URL('./solidSerialization.fixture.cjs', import.meta.url));
const lock = JSON.parse(
  readFileSync(new URL('../../../package-lock.json', import.meta.url), 'utf8'),
) as { packages: Record<string, { version: string }> };

// vite-plugin-solid deliberately adds browser conditions to Vitest workers,
// even node-environment tests. A plain Node child exercises the actual server
// export without changing the app/test resolver or mocking its Serializer.
// Each case asserts the installed tuple as well as the real consumer result.
describe('actual Solid resource serialization consumer', () => {
  it.each([
    'exact-installed-graph',
    'synchronous-resource',
    'async-suspense',
    'late-stream',
    'script-boundary',
    'render-scopes',
    'no-scripts',
  ])('preserves %s on the exact committed graph', (name) => {
    const result = JSON.parse(
      execFileSync(process.execPath, [fixture, name], { encoding: 'utf8', timeout: 10_000 }),
    ) as { case: string; passed: boolean; versions: Record<string, string> };
    expect(result.case).toBe(name);
    expect(result.passed).toBe(true);
    for (const dependency of ['solid-js', 'seroval', 'seroval-plugins']) {
      expect(result.versions[dependency]).toBe(lock.packages[`node_modules/${dependency}`].version);
    }
  });
});
