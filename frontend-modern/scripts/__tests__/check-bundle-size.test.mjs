// @vitest-environment node

import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it } from 'vitest';

const checker = fileURLToPath(new URL('../check-bundle-size.mjs', import.meta.url));
const temporaryRoots = [];
const digest = (content, algorithm = 'sha384') =>
  `${algorithm}-${createHash(algorithm).update(content).digest('base64')}`;

function fixture(algorithm = 'sha384') {
  const root = mkdtempSync(path.join(os.tmpdir(), 'pulse-bundle-check-'));
  temporaryRoots.push(root);
  mkdirSync(path.join(root, 'scripts'));
  mkdirSync(path.join(root, 'dist/assets'), { recursive: true });
  const script = path.join(root, 'scripts/check-bundle-size.mjs');
  cpSync(checker, script);
  const files = {
    'index-aaaaaaaa.js': 'import "./vendor-bbbbbbbb.js"; import("./lazy-dddddddd.js");\n',
    'vendor-bbbbbbbb.js': 'export { shared } from "./shared-cccccccc.js";\n',
    'shared-cccccccc.js': 'export const shared = true;\n',
    'lazy-dddddddd.js': 'export const lazy = true;\n',
  };
  for (const [name, bytes] of Object.entries(files)) {
    writeFileSync(path.join(root, 'dist/assets', name), bytes);
  }
  const f = {
    root,
    files,
    map: {
      integrity: Object.fromEntries(
        Object.entries(files).map(([name, bytes]) => [`/assets/${name}`, digest(bytes, algorithm)]),
      ),
    },
    entry: digest(files['index-aaaaaaaa.js'], algorithm),
    entryAttribute: 'integrity',
    preloadAttribute: 'integrity',
    entryFile: 'index-aaaaaaaa.js',
    extraMaps: 0,
    commentedMap: false,
    preloads: {
      'vendor-bbbbbbbb.js': digest(files['vendor-bbbbbbbb.js'], algorithm),
      'shared-cccccccc.js': digest(files['shared-cccccccc.js'], algorithm),
    },
    baseline: {
      chunks: { index: { baselineGzip: 1000 } },
      totals: { js: { baselineGzip: 10000 }, css: { baselineGzip: 10000 } },
    },
    run() {
      const mapTag = `<script type="importmap">${JSON.stringify(f.map)}</script>`;
      const html = `${f.commentedMap ? `<!--${mapTag}-->` : mapTag}\n${mapTag.repeat(f.extraMaps)}\n<script type="module" src="/assets/${f.entryFile}" ${f.entryAttribute}="${f.entry}"></script>\n${Object.entries(
        f.preloads,
      )
        .map(
          ([file, hash]) =>
            `<link rel="modulepreload" href="/assets/${file}" ${f.preloadAttribute}="${hash}">`,
        )
        .join('\n')}\n`;
      writeFileSync(path.join(root, 'dist/index.html'), html);
      const baseline = JSON.stringify(f.baseline);
      writeFileSync(path.join(root, '.bundlesize.json'), baseline);
      const result = spawnSync(process.execPath, [script], {
        encoding: 'utf8',
        cwd: root,
        timeout: 15000,
      });
      expect(result.error).toBeUndefined();
      expect(result.signal).toBeNull();
      // The checker is read-only even when rejecting a build.
      expect(readFileSync(path.join(root, 'dist/index.html'), 'utf8')).toBe(html);
      expect(readFileSync(path.join(root, '.bundlesize.json'), 'utf8')).toBe(baseline);
      expect(result.stderr).toBe('');
      return result;
    },
  };
  return f;
}

afterEach(() => {
  while (temporaryRoots.length) rmSync(temporaryRoots.pop(), { recursive: true, force: true });
});

describe('built JavaScript integrity checks', () => {
  it.each(['sha256', 'sha384', 'sha512'])(
    'accepts correct %s bytes and transitive static preloads',
    (algorithm) => {
      const f = fixture(algorithm);
      const result = f.run();
      expect(result.status).toBe(0);
      expect(result.stdout).toContain('All chunks within budget.');
    },
  );

  it.each([
    [
      'stale import-map digest',
      (f) => {
        f.map.integrity['/assets/lazy-dddddddd.js'] = digest('old lazy chunk');
      },
    ],
    [
      'empty import-map digest',
      (f) => {
        f.map.integrity['/assets/lazy-dddddddd.js'] = '';
      },
    ],
    [
      'unsupported import-map digest',
      (f) => {
        f.map.integrity['/assets/lazy-dddddddd.js'] = 'sha1-deadbeef';
      },
    ],
    [
      'empty entry digest',
      (f) => {
        f.entry = '';
      },
    ],
    [
      'empty preload digest',
      (f) => {
        f.preloads['vendor-bbbbbbbb.js'] = '';
      },
    ],
    [
      'malformed digest',
      (f) => {
        f.map.integrity['/assets/lazy-dddddddd.js'] = 'sha384-not-a-digest';
      },
    ],
    [
      'stale entry digest',
      (f) => {
        f.entry = digest('old entry');
      },
    ],
    [
      'stale preload digest',
      (f) => {
        f.preloads['vendor-bbbbbbbb.js'] = digest('old vendor');
      },
    ],
    [
      'changed lazy chunk bytes',
      (f) => {
        writeFileSync(
          path.join(f.root, 'dist/assets/lazy-dddddddd.js'),
          'export const lazy = false;\n',
        );
      },
    ],
    [
      'non-string digest',
      (f) => {
        f.map.integrity['/assets/lazy-dddddddd.js'] = { hash: digest(f.files['lazy-dddddddd.js']) };
      },
    ],
  ])('rejects %s instead of certifying only metadata presence', (_name, change) => {
    const f = fixture();
    change(f);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toMatch(/(?:integrity.*mismatch|invalid or unsupported integrity)/i);
  });

  it.each([null, [], 'not an object', { integrity: [] }, { integrity: 'not an object' }])(
    'rejects malformed import-map shape %j with a bounded diagnostic',
    (map) => {
      const f = fixture();
      f.map = map;
      const result = f.run();
      expect(result.status).toBe(1);
      expect(result.stdout).toMatch(/importmap.*integrity/i);
    },
  );

  it('does not let a matching weaker digest override a stale stronger digest', () => {
    const f = fixture();
    f.map.integrity['/assets/lazy-dddddddd.js'] =
      `${digest(f.files['lazy-dddddddd.js'], 'sha256')} ${digest('old lazy chunk', 'sha512')}`;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toMatch(/integrity.*mismatch/i);
  });

  it('accepts all-correct metadata with multiple supported algorithms', () => {
    const f = fixture();
    f.map.integrity['/assets/lazy-dddddddd.js'] = ['sha256', 'sha384', 'sha512']
      .map((algorithm) => digest(f.files['lazy-dddddddd.js'], algorithm))
      .join(' \t');
    expect(f.run().status).toBe(0);
  });

  it('intentionally rejects conflicting weaker alternatives in generated output', () => {
    const f = fixture();
    f.map.integrity['/assets/lazy-dddddddd.js'] =
      `${digest('old lazy chunk', 'sha256')} ${digest(f.files['lazy-dddddddd.js'], 'sha512')}`;
    expect(f.run().status).toBe(1);
  });

  it('compares raw asset bytes rather than decoded text', () => {
    const f = fixture();
    const bytes = Buffer.concat([Buffer.from(f.files['lazy-dddddddd.js']), Buffer.from([0xff])]);
    writeFileSync(path.join(f.root, 'dist/assets/lazy-dddddddd.js'), bytes);
    f.map.integrity['/assets/lazy-dddddddd.js'] = digest(bytes);
    expect(f.run().status).toBe(0);
    f.map.integrity['/assets/lazy-dddddddd.js'] = digest(bytes.toString('utf8'));
    expect(f.run().status).toBe(1);
  });

  it.each(['entryAttribute', 'preloadAttribute'])(
    'does not mistake data-integrity for %s',
    (attribute) => {
      const f = fixture();
      f[attribute] = 'data-integrity';
      const result = f.run();
      expect(result.status).toBe(1);
      expect(result.stdout).toContain('invalid or unsupported integrity metadata');
    },
  );

  it('rejects duplicate import maps rather than checking only the first', () => {
    const f = fixture();
    f.extraMaps = 1;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('exactly one importmap');
  });

  it('does not treat a commented-out import map as enforced metadata', () => {
    const f = fixture();
    f.commentedMap = true;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('importmap script block missing');
  });

  it('rejects an entry outside the built JS inventory even if its bytes and hash match', () => {
    const f = fixture();
    const bytes = 'export const outside = true;\n';
    writeFileSync(path.join(f.root, 'dist/outside.js'), bytes);
    f.entryFile = '../outside.js';
    f.entry = digest(bytes);
    f.preloads = {};
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('missing or invalid JS asset');
  });

  it('still rejects missing import-map coverage', () => {
    const f = fixture();
    delete f.map.integrity['/assets/lazy-dddddddd.js'];
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain(
      'importmap integrity missing entry for /assets/lazy-dddddddd.js',
    );
  });

  it('still rejects a lazy route preload even with a correct digest', () => {
    const f = fixture();
    f.preloads['lazy-dddddddd.js'] = f.map.integrity['/assets/lazy-dddddddd.js'];
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('modulepreload of lazy chunk');
  });

  it('still enforces the unchanged size budget for correct build bytes', () => {
    const f = fixture();
    f.baseline.chunks.index.baselineGzip = 1;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('bundle size violation');
    expect(result.stdout).not.toContain('preload posture violation');
  });
});
