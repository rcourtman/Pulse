// @vitest-environment node

import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { describe, expect, it } from 'vitest';

// Vite's browser resolution picks Prettier's parser-free standalone bundle.
// Exercise the Node formatter used by repository tooling, not that UI bundle.
const prettier = createRequire(import.meta.url)('prettier') as typeof import('prettier');
const require = createRequire(import.meta.url);

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

describe('patched transitive dependency compatibility', () => {
  it.each([
    ['solid-js', 'seroval'],
    ['seroval-plugins/web', 'seroval'],
    ['@tailwindcss/typography', 'postcss-selector-parser'],
    ['postcss', 'source-map-js'],
    ['@tailwindcss/node', 'source-map-js'],
    ['magicast', 'source-map-js'],
  ])('routes %s through the shared patched %s', (consumer, dependency) => {
    // Catch a stale/nested install that would evade an otherwise correct
    // top-level lock assertion. These are the real installed consumers.
    const fromConsumer = createRequire(require.resolve(consumer));
    expect(fromConsumer.resolve(dependency)).toBe(require.resolve(dependency));
  });

  it('round-trips cyclic values, typed arrays and the existing URL plugin', () => {
    const { toJSON, fromJSON } = require('seroval') as typeof import('seroval');
    const { URLPlugin } = require('seroval-plugins/web') as typeof import('seroval-plugins/web');
    const values = {
      name: 'Pulse',
      date: new Date('2026-10-06T00:00:00Z'),
      bytes: new Uint8Array([0, 42, 255]),
      groups: new Map([['alerts', new Set(['node-a', 'node-b'])]]),
      url: new URL('https://example.test/history?period=24h'),
    };
    const original = { values, again: values };
    Object.assign(values, { parent: original });
    const options = { plugins: [URLPlugin] };
    const restored = fromJSON(toJSON(original, options), options) as typeof original;
    expect(restored.values).toEqual(values);
    expect(restored.again).toBe(restored.values);
    expect((restored.values as typeof values & { parent: typeof original }).parent).toBe(restored);
  });

  it('keeps typography trailing pseudos outside the zero-specificity selector wrapper', () => {
    type CSS = Record<string, Record<string, unknown>>;
    const variants: Record<string, string> = {};
    const components: CSS[] = [];
    const css = {
      'h1::before, h2::before': { content: '"heading"' },
      'pre code': { color: 'inherit' },
      '> ul > li': { color: 'inherit' },
    };
    const typography = require('@tailwindcss/typography') as () => {
      handler: (api: {
        addVariant: (name: string, selector: string) => void;
        addComponents: (styles: CSS[]) => void;
        theme: (name: string) => unknown;
        prefix: (selector: string) => string;
      }) => void;
    };
    // Run the public plugin, which consumes selector-parser through its
    // commonTrailingPseudos helper, rather than a hand-written parser imitation.
    typography().handler({
      addVariant: (name, selector) => {
        variants[name] = selector;
      },
      addComponents: (styles) => {
        components.push(...styles);
      },
      theme: (name) => (name === 'typography' ? { DEFAULT: { css } } : undefined),
      prefix: (selector) => selector,
    });
    const notProse = ':not(:where([class~="not-prose"],[class~="not-prose"] *))';
    expect(components[0]['.prose']).toEqual({
      [`:where(h1, h2)${notProse}::before`]: css['h1::before, h2::before'],
      [`:where(pre code)${notProse}`]: css['pre code'],
      [`:where(.prose > ul > li)${notProse}`]: css['> ul > li'],
    });
    expect(variants['prose-code']).toBe(`& :is(:where(code)${notProse})`);
  });

  it('preserves named source mappings and their source content', () => {
    const { SourceMapGenerator, SourceMapConsumer } =
      require('source-map-js') as typeof import('source-map-js');
    const source = '.status {\n  color: red;\n}\n';
    const map = new SourceMapGenerator({ file: 'output.css' });
    map.addMapping({
      generated: { line: 2, column: 2 },
      original: { line: 2, column: 2 },
      source: 'input.css',
      name: 'status-colour',
    });
    map.setSourceContent('input.css', source);
    const consumer = new SourceMapConsumer(map.toJSON());
    expect(consumer.originalPositionFor({ line: 2, column: 2 })).toEqual({
      source: 'input.css',
      line: 2,
      column: 2,
      name: 'status-colour',
    });
    expect(consumer.sourceContentFor('input.css')).toBe(source);
    expect(
      consumer.generatedPositionFor({ source: 'input.css', line: 2, column: 2 }),
    ).toMatchObject({
      line: 2,
      column: 2,
    });
  });

  it('preserves original CSS positions through the actual PostCSS consumer', async () => {
    const postcss = require('postcss') as typeof import('postcss').default;
    const { SourceMapConsumer } = require('source-map-js') as typeof import('source-map-js');
    const source = '.status {\n  color: red;\n}\n';
    const result = await postcss([
      {
        postcssPlugin: 'compatibility-colour',
        Declaration(declaration) {
          if (declaration.prop === 'color') declaration.value = 'green';
        },
      },
    ]).process(source, {
      from: '/fixture/input.css',
      to: '/fixture/output.css',
      map: { inline: false, annotation: false },
    });
    expect(result.css).toContain('color: green');
    expect(result.map).toBeDefined();
    const consumer = new SourceMapConsumer(result.map!.toJSON());
    expect(consumer.originalPositionFor({ line: 2, column: 2 })).toMatchObject({
      source: 'input.css',
      line: 2,
      column: 2,
    });
    expect(consumer.sourceContentFor('input.css')).toBe(source);
  });
});
