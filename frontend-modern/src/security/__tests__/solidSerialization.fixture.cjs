// Executed by the normal Vitest suite in a plain Node process. The Solid Vite
// plugin intentionally selects browser conditions for test workers; it must not
// replace the server consumer whose serializer API we are qualifying here.
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { createRequire } = require('node:module');
const { resolve } = require('node:path');
const { createContext, runInContext } = require('node:vm');
const { JSDOM } = require('jsdom');
const solid = require('solid-js');
const server = require('solid-js/web');
const fromServer = createRequire(require.resolve('solid-js/web'));

const versions = Object.fromEntries(
  ['solid-js', 'seroval', 'seroval-plugins'].map((name) => [
    name,
    JSON.parse(
      readFileSync(resolve(__dirname, '../../../node_modules', name, 'package.json'), 'utf8'),
    ).version,
  ]),
);

function fixture() {
  const buffer = new ArrayBuffer(12);
  const bytes = new Uint8Array(buffer, 2, 4);
  bytes.set([0, 42, 128, 255]);
  const values = {
    name: 'Pulse resource',
    date: new Date('2026-10-28T00:00:00Z'),
    bytes,
    view: new DataView(buffer, 2, 4),
    groups: new Map([['alerts', new Set(['node-a', 'node-b'])]]),
    url: new URL('https://example.test/history?period=24h'),
    params: new URLSearchParams('node=node-a&node=node-b'),
    headers: new Headers({ 'x-pulse-fixture': 'resource' }),
  };
  const original = { values, again: values };
  values.parent = original;
  return original;
}

function hydrationContext() {
  const hydration = { r: {} };
  const context = createContext({
    _$HY: hydration,
    URL,
    URLSearchParams,
    Headers,
    Date,
    Map,
    Set,
    ArrayBuffer,
    Uint8Array,
    DataView,
    atob,
  });
  context.self = context;
  return { hydration, context };
}

function scriptContents(html) {
  // Parse only synthetic HTML, with JSDOM's script execution and resource
  // loading left disabled. HTML parsing recognises tag casing and quoted
  // attributes, and does not mistake commented markup for executable scripts.
  const dom = new JSDOM(html);
  try {
    return [...dom.window.document.querySelectorAll('script')].map((script) => script.textContent);
  } finally {
    dom.window.close();
  }
}

function assertSingleScript(html) {
  assert.equal(scriptContents(html).length, 1, 'resource HTML must contain exactly one script');
}

function consumeScripts(html, target = hydrationContext()) {
  const scripts = scriptContents(html);
  assert.ok(scripts.length > 0);
  for (const script of scripts) {
    // Only synthetic fixture data enters these actual-consumer scripts. This
    // context exposes no process, require, filesystem or network capability.
    runInContext(script, target.context, { timeout: 1000 });
  }
  return target;
}

async function renderText(text) {
  return server.renderToStringAsync(
    () => {
      const [resource] = solid.createResource(() => ({ text }));
      return resource().text;
    },
    { renderId: 'text-', nonce: 'text-fixture' },
  );
}

async function expectScriptBoundary(text) {
  const html = await renderText(text);
  assert.ok(!html.includes(text));
  assertSingleScript(html);
  const { hydration, context } = consumeScripts(html);
  assert.equal(hydration.r['text-0'].text, text);
  assert.equal(context.unexpected, undefined);
}

function expectFixture(restored) {
  assert.equal(restored.again, restored.values);
  assert.equal(restored.values.parent, restored);
  assert.equal(restored.values.name, 'Pulse resource');
  assert.equal(restored.values.date.toISOString(), '2026-10-28T00:00:00.000Z');
  assert.deepEqual([...restored.values.bytes], [0, 42, 128, 255]);
  assert.equal(restored.values.bytes.byteOffset, 2);
  assert.equal(restored.values.view.byteLength, 4);
  assert.equal(restored.values.view.buffer, restored.values.bytes.buffer);
  assert.equal(restored.values.view.getUint8(3), 255);
  assert.deepEqual([...restored.values.groups.get('alerts')], ['node-a', 'node-b']);
  assert.equal(restored.values.url.href, 'https://example.test/history?period=24h');
  assert.deepEqual(restored.values.params.getAll('node'), ['node-a', 'node-b']);
  assert.equal(restored.values.headers.get('x-pulse-fixture'), 'resource');
}

const cases = {
  async 'exact-installed-graph'() {
    const lock = JSON.parse(readFileSync(resolve(__dirname, '../../../package-lock.json'), 'utf8'));
    for (const [name, version] of Object.entries(versions)) {
      assert.equal(version, lock.packages[`node_modules/${name}`].version);
    }
  },
  async 'synchronous-resource'() {
    const errors = [];
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => fixture());
        return resource().values.name;
      },
      { renderId: 'sync-', nonce: 'pulse-fixture', onError: (error) => errors.push(error) },
    );
    assert.deepEqual(errors, []);
    assert.ok(html.includes('Pulse resource'));
    assert.ok(html.includes('<script nonce="pulse-fixture">'));
    const { hydration } = consumeScripts(html);
    expectFixture(await hydration.r['sync-0']);
  },
  async 'async-suspense'() {
    const errors = [];
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => Promise.resolve(fixture()));
        return solid.createComponent(solid.Suspense, {
          fallback: 'Waiting for resource',
          get children() {
            return resource()?.values.name;
          },
        });
      },
      { renderId: 'async-', onError: (error) => errors.push(error) },
    );
    assert.deepEqual(errors, []);
    assert.ok(html.includes('Pulse resource'));
    assert.ok(!html.includes('Waiting for resource'));
    const { hydration } = consumeScripts(html);
    expectFixture(await hydration.r['async-0']);
  },
  async 'late-stream'() {
    let resolveResource;
    const pending = new Promise((done) => {
      resolveResource = done;
    });
    const chunks = [];
    const errors = [];
    const stream = server.renderToStream(
      () => {
        solid.createResource(() => pending);
        return 'Monitoring shell';
      },
      {
        renderId: 'stream-',
        nonce: 'stream-fixture',
        onCompleteShell: () => resolveResource(fixture()),
        onError: (error) => errors.push(error),
      },
    );
    await stream.pipeTo(
      new WritableStream({
        write: (chunk) => {
          chunks.push(typeof chunk === 'string' ? chunk : Buffer.from(chunk).toString('utf8'));
        },
      }),
    );
    assert.deepEqual(errors, []);
    assert.ok(chunks.length > 1);
    assert.ok(chunks[0].includes('Monitoring shell'));
    assert.ok(chunks.join('').includes('<script nonce="stream-fixture">'));
    const { hydration } = consumeScripts(chunks.join(''));
    expectFixture(await hydration.r['stream-0']);
  },
  async 'script-boundary'() {
    await expectScriptBoundary(
      '</script><script>globalThis.unexpected = true</script> & \u2028\u2029',
    );
  },
  async 'script-boundary-uppercase'() {
    await expectScriptBoundary(
      '</SCRIPT><SCRIPT>globalThis.unexpected = true</SCRIPT> & \u2028\u2029',
    );
  },
  async 'script-boundary-mixed-case'() {
    await expectScriptBoundary(
      '</ScRiPt><sCrIpT>globalThis.unexpected = true</sCrIpT> & \u2028\u2029',
    );
  },
  async 'script-oracle-uppercase'() {
    const { context } = consumeScripts(
      '<SCRIPT nonce="case-fixture">globalThis.casing = "upper"</SCRIPT>',
    );
    assert.equal(context.casing, 'upper');
  },
  async 'script-oracle-mixed-case'() {
    const { context } = consumeScripts(
      '<sCrIpT nonce="case-fixture">globalThis.casing = "mixed"</ScRiPt>',
    );
    assert.equal(context.casing, 'mixed');
  },
  async 'script-oracle-quoted-attribute'() {
    const { context } = consumeScripts(
      '<script data-label="a > b">globalThis.quotedAttribute = true</script>',
    );
    assert.equal(context.quotedAttribute, true);
  },
  async 'script-oracle-comment'() {
    const { context } = consumeScripts(
      '<!-- <script>globalThis.commentRan = true</script> -->' +
        '<script>globalThis.actualScriptRan = true</script>',
    );
    assert.equal(context.actualScriptRan, true);
    assert.equal(context.commentRan, undefined);
  },
  async 'script-oracle-rejects-additional-tags'() {
    const html = await renderText('Safe resource control');
    assertSingleScript(html);
    for (const tag of ['script', 'SCRIPT', 'ScRiPt']) {
      const injected = `${html}<${tag}>globalThis.unexpected = true</${tag}>`;
      assert.throws(() => assertSingleScript(injected), { actual: 2, expected: 1 });
    }
  },
  async 'render-scopes'() {
    const target = hydrationContext();
    for (const renderId of ['first-', 'second-']) {
      const html = await server.renderToStringAsync(
        () => {
          const [resource] = solid.createResource(() => ({ scope: renderId }));
          return resource().scope;
        },
        { renderId },
      );
      consumeScripts(html, target);
    }
    assert.equal(target.hydration.r['first-0'].scope, 'first-');
    assert.equal(target.hydration.r['second-0'].scope, 'second-');
  },
  async 'no-scripts'() {
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => ({ name: 'Pulse without scripts' }));
        return resource().name;
      },
      { noScripts: true },
    );
    assert.equal(html, 'Pulse without scripts');
  },
};

async function main() {
  const name = process.argv[2];
  assert.ok(Object.hasOwn(cases, name), 'unknown consumer case');
  assert.equal(server.isServer, true, 'must execute the real Node server consumer');
  assert.equal(fromServer.resolve('seroval'), require.resolve('seroval'));
  assert.equal(fromServer.resolve('seroval-plugins/web'), require.resolve('seroval-plugins/web'));
  await cases[name]();
  process.stdout.write(JSON.stringify({ case: name, passed: true, versions }) + '\n');
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
