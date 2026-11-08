// @vitest-environment node

import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
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
  cpSync(path.join(path.dirname(checker), 'build-integrity.mjs'), path.join(root, 'scripts/build-integrity.mjs'));
  // The copied CLI uses the same already-declared, locked parser as the real
  // build check. No dependency install or resource loading occurs in fixtures.
  symlinkSync(
    path.resolve(path.dirname(checker), '../node_modules'),
    path.join(root, 'node_modules'),
    'junction',
  );
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
    transformHTML: (html) => html,
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
      const html = f.transformHTML(`<!-- Generated build fixture -->\n${f.commentedMap ? `<!--${mapTag}-->` : mapTag}\n${mapTag.repeat(f.extraMaps)}\n<script type="module" src="/assets/${f.entryFile}" ${f.entryAttribute}="${f.entry}"></script>\n${Object.entries(
        f.preloads,
      )
        .map(
          ([file, hash]) =>
            `<link rel="modulepreload" href="/assets/${file}" ${f.preloadAttribute}="${hash}">`,
        )
        .join('\n')}\n`);
      const htmlBytes = Buffer.isBuffer(html) ? html : Buffer.from(html);
      writeFileSync(path.join(root, 'dist/index.html'), htmlBytes);
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
      expect(readFileSync(path.join(root, 'dist/index.html'))).toEqual(htmlBytes);
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

  it.each(
    [null, [], 'not an object', { integrity: [] }, { integrity: 'not an object' }].map((value) => [
      value,
    ]),
  )('rejects malformed import-map shape %j with a bounded diagnostic', (map) => {
    const f = fixture();
    f.map = map;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toMatch(/importmap.*integrity/i);
  });

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

  it.each([
    ['split map tag', '<script type="importmap"', '<scr<!-- gap -->ipt type="importmap"'],
    ['split map attribute', 'type="importmap"', 'ty<!-- gap -->pe="importmap"'],
    ['split map value', 'type="importmap"', 'type="import<!-- gap -->map"'],
    ['comment as tag separator', '<script type="importmap"', '<script<!-- -->type="importmap"'],
    ['comment inside map tag', '<script type="importmap"', '<script <!-- -->type="importmap"'],
    ['split entry tag', '<script type="module"', '<scr<!-- gap -->ipt type="module"'],
    ['split entry source', 'src="/assets/', 'sr<!-- gap -->c="/assets/'],
    ['split entry integrity', 'integrity="sha384-', 'integri<!-- gap -->ty="sha384-'],
    ['comment inside integrity value', 'integrity="sha384-', 'integrity="sha384-<!-- gap -->'],
    [
      'split preload integrity',
      'href="/assets/vendor-bbbbbbbb.js" integrity=',
      'href="/assets/vendor-bbbbbbbb.js" integri<!-- gap -->ty=',
    ],
  ])('rejects %s without reconstructing browser-inactive controls', (_name, before, after) => {
    const f = fixture();
    f.transformHTML = (html) => html.replace(before, after);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toMatch(/(?:importmap|entry script|integrity)/i);
  });

  it('does not certify controls after an unterminated comment', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      '<!-- unfinished\n' + html.replace('<!-- Generated build fixture -->', '');
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('importmap script block missing');
  });

  it.each([
    ['template', (map) => `<template>${map}</template>`],
    ['attribute', (map) => `<div title='${map}'></div>`],
    [
      'script text',
      (map) => `<script type="application/json">${map.replace('</script>', '<\\/script>')}</script>`,
    ],
    ['textarea', (map) => `<textarea>${map}</textarea>`],
    ['foreign namespace', (map) => `<svg>${map}</svg>`],
  ])('does not treat an import map in %s as document metadata', (_name, wrap) => {
    const f = fixture();
    f.transformHTML = (html) => html.replace(/<script type="importmap">.*?<\/script>/, wrap);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('importmap script block missing');
  });

  it.each([
    ['unquoted rel', (tag) => tag.replace('rel="modulepreload"', 'rel=modulepreload')],
    ['greater-than attribute', (tag) => tag.replace('<link ', '<link title=">" ')],
    ['short comment', (tag) => `<!-->${tag}<!-- -->`],
    ['short dash comment', (tag) => `<!--->${tag}<!-- -->`],
    ['alternate comment close', (tag) => `<!-- a --!>${tag}-->`],
    ['comment opener in attribute', (tag) => `<meta content="<!--">${tag}<!-- -->`],
    [
      'mixed-case rel tokens',
      (tag) => tag.replace('rel="modulepreload"', 'rel="alternate MODULEPRELOAD"'),
    ],
  ])('still inspects a browser-active lazy preload with %s', (_name, wrap) => {
    const f = fixture();
    f.transformHTML = (html) =>
      html +
      wrap(
        `<link rel="modulepreload" href="/assets/lazy-dddddddd.js" integrity="${f.map.integrity['/assets/lazy-dddddddd.js']}">`,
      );
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('modulepreload of lazy chunk');
  });

  it.each(['body', 'head'])(
    'does not certify controls hidden in %s noscript with scripting enabled',
    (position) => {
      const f = fixture();
      f.transformHTML = (html) =>
        position === 'head'
          ? `<html><head><noscript>${html}</noscript></head><body></body></html>`
          : `<body><noscript>${html}</noscript></body>`;
      const result = f.run();
      expect(result.status).toBe(1);
      expect(result.stdout).toContain('importmap script block missing');
    },
  );

  it('fails closed if the parser changes to scripting-disabled tokenization', () => {
    const f = fixture();
    const script = path.join(f.root, 'scripts/build-integrity.mjs');
    const source = readFileSync(script, 'utf8');
    const changed = source.replace('includeNodeLocations: true', 'includeNodeLocations: false');
    expect(changed).not.toBe(source);
    writeFileSync(script, changed);
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('HTML parser scripting mode is unsupported');
  });

  it('refuses unsupported declarative shadow controls rather than ignoring them', () => {
    const f = fixture();
    f.transformHTML = (html) => `<div><template shadowrootmode="open">${html}</template></div>`;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('declarative shadow DOM controls are unverified');
  });

  it('uses the HTML byte encoding rather than pre-decoding every document as UTF-8', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      Buffer.concat([Buffer.from([0xff, 0xfe]), Buffer.from(html, 'utf16le')]);
    expect(f.run().status).toBe(0);
  });

  it('accepts real controls with benign comments and the plain Vite noscript fallback', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      html +
      '<!-- <script type="importmap">not a map</script> --><noscript>You need to enable JavaScript to run this app.</noscript>';
    expect(f.run().status).toBe(0);
  });

  it('never executes inline scripts while inventorying the built document', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      html +
      '<script>document.querySelector(\'script[type="importmap"]\').remove(); console.error("executed");</script>';
    expect(f.run().status).toBe(0);
  });

  it('refuses an external import map even when inline JSON contains matching digests', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      html.replace('type="importmap"', 'type="importmap" src="/assets/map.json"');
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('importmap must be inline');
  });

  it('refuses duplicate module entries rather than choosing a convenient match', () => {
    const f = fixture();
    f.transformHTML = (html) =>
      html +
      `<script type="module" src="/assets/lazy-dddddddd.js" integrity="${f.map.integrity['/assets/lazy-dddddddd.js']}"></script>`;
    const result = f.run();
    expect(result.status).toBe(1);
    expect(result.stdout).toContain('exactly one external module entry');
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
