// @vitest-environment node

import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { createContext, runInContext } from 'node:vm';
import { describe, expect, it } from 'vitest';

// Vite resolves Solid's browser export. Deliberately use the real Node server
// consumer: its resource renderer constructs Seroval's Serializer, installs the
// default web plugins and emits hydration code. A Seroval-only round trip cannot
// establish that a security override remains compatible with that consumer.
const require = createRequire(import.meta.url);
const solid = require('solid-js') as typeof import('solid-js');
const server = require('solid-js/web') as typeof import('solid-js/web');
const fromServer = createRequire(require.resolve('solid-js/web'));

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
  Object.assign(values, { parent: original });
  return original;
}

type Fixture = ReturnType<typeof fixture>;

function hydrationContext() {
  const hydration = { r: {} as Record<string, unknown> };
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
  });
  return { hydration, context };
}

function consumeScripts(html: string, target = hydrationContext()) {
  const scripts = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)];
  expect(scripts.length).toBeGreaterThan(0);
  for (const [, script] of scripts) {
    // Only synthetic fixture data goes into these real-consumer scripts. The
    // context exposes no process, require, filesystem or network capability.
    runInContext(script, target.context, { timeout: 1000 });
  }
  return target;
}

function expectFixture(restored: Fixture) {
  expect(restored.again).toBe(restored.values);
  expect((restored.values as Fixture['values'] & { parent: Fixture }).parent).toBe(restored);
  expect(restored.values.name).toBe('Pulse resource');
  expect(restored.values.date.toISOString()).toBe('2026-10-28T00:00:00.000Z');
  expect([...restored.values.bytes]).toEqual([0, 42, 128, 255]);
  expect(restored.values.bytes.byteOffset).toBe(2);
  expect(restored.values.view.byteLength).toBe(4);
  expect(restored.values.view.buffer).toBe(restored.values.bytes.buffer);
  expect(restored.values.view.getUint8(3)).toBe(255);
  expect([...restored.values.groups.get('alerts')!]).toEqual(['node-a', 'node-b']);
  expect(restored.values.url.href).toBe('https://example.test/history?period=24h');
  expect(restored.values.params.getAll('node')).toEqual(['node-a', 'node-b']);
  expect(restored.values.headers.get('x-pulse-fixture')).toBe('resource');
}

describe('actual Solid resource serialization consumer', () => {
  it('uses the exact committed server, serializer and unchanged plugin graph', () => {
    expect(server.isServer).toBe(true);
    const lock = JSON.parse(
      readFileSync(new URL('../../../package-lock.json', import.meta.url), 'utf8'),
    ) as { packages: Record<string, { version: string }> };
    for (const name of ['solid-js', 'seroval', 'seroval-plugins']) {
      const installed = JSON.parse(
        readFileSync(
          new URL(`../../../node_modules/${name}/package.json`, import.meta.url),
          'utf8',
        ),
      ) as { version: string };
      expect(installed.version).toBe(lock.packages[`node_modules/${name}`].version);
    }
    expect(fromServer.resolve('seroval')).toBe(require.resolve('seroval'));
    expect(fromServer.resolve('seroval-plugins/web')).toBe(require.resolve('seroval-plugins/web'));
  });

  it('renders and revives cyclic resources, typed views and default web plugins', async () => {
    const errors: unknown[] = [];
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => fixture());
        return resource()!.values.name;
      },
      { renderId: 'sync-', nonce: 'pulse-fixture', onError: (error) => errors.push(error) },
    );
    expect(errors).toEqual([]);
    expect(html).toContain('Pulse resource');
    expect(html).toContain('<script nonce="pulse-fixture">');
    const { hydration } = consumeScripts(html);
    expectFixture((await hydration.r['sync-0']) as Fixture);
  });

  it('settles a real async resource through Suspense rather than a serializer imitation', async () => {
    const errors: unknown[] = [];
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
    expect(errors).toEqual([]);
    expect(html).toContain('Pulse resource');
    expect(html).not.toContain('Waiting for resource');
    const { hydration } = consumeScripts(html);
    expectFixture((await hydration.r['async-0']) as Fixture);
  });

  it('flushes a late resource through the actual streaming Serializer API', async () => {
    let resolve!: (value: Fixture) => void;
    const pending = new Promise<Fixture>((done) => {
      resolve = done;
    });
    const chunks: string[] = [];
    const errors: unknown[] = [];
    const stream = server.renderToStream(
      () => {
        solid.createResource(() => pending);
        return 'Monitoring shell';
      },
      {
        renderId: 'stream-',
        nonce: 'stream-fixture',
        onCompleteShell: () => resolve(fixture()),
        onError: (error) => errors.push(error),
      },
    );
    await stream.pipeTo(new WritableStream({ write: (chunk) => void chunks.push(String(chunk)) }));
    expect(errors).toEqual([]);
    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks[0]).toContain('Monitoring shell');
    const { hydration } = consumeScripts(chunks.join(''));
    expectFixture((await hydration.r['stream-0']) as Fixture);
  });

  it('keeps script-like resource text as data across the real HTML/script boundary', async () => {
    const text = '</script><script>globalThis.unexpected = true</script> & \u2028\u2029';
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => ({ text }));
        return resource()!.text;
      },
      { renderId: 'text-', nonce: 'text-fixture' },
    );
    expect(html).not.toContain(text);
    expect([...html.matchAll(/<script\b/g)]).toHaveLength(1);
    const { hydration, context } = consumeScripts(html);
    expect(hydration.r['text-0']).toEqual({ text });
    expect(context.unexpected).toBeUndefined();
  });

  it('does not overwrite another render scope in a shared hydration registry', async () => {
    const target = hydrationContext();
    for (const renderId of ['first-', 'second-']) {
      const html = await server.renderToStringAsync(
        () => {
          const [resource] = solid.createResource(() => ({ scope: renderId }));
          return resource()!.scope;
        },
        { renderId },
      );
      consumeScripts(html, target);
    }
    expect(target.hydration.r['first-0']).toEqual({ scope: 'first-' });
    expect(target.hydration.r['second-0']).toEqual({ scope: 'second-' });
  });

  it('preserves Solid noScripts rendering without emitting hydration payloads', async () => {
    const html = await server.renderToStringAsync(
      () => {
        const [resource] = solid.createResource(() => ({ name: 'Pulse without scripts' }));
        return resource()!.name;
      },
      { noScripts: true },
    );
    expect(html).toBe('Pulse without scripts');
  });
});
