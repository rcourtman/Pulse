// Read-only admission for the built JavaScript bytes and their browser controls.
// Shared by the bundle checker and BOTH pre-promotion build/embed boundaries.
// Parsing never executes scripts or loads resources; only owned inventory files
// are hashed. Valid output is read, not rewritten.
import { lstatSync, readdirSync, readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join, posix } from 'node:path';
import { JSDOM, VirtualConsole } from 'jsdom';

/**
 * Verify the deployment-installability preload invariant against the built
 * index.html (see the subsystem contract clause on modulepreload posture):
 *   - the import map integrity block covers every JS asset in dist/assets,
 *     and its values and entry/preload attributes match the actual bytes;
 *   - modulepreload links reference only the entry chunk's static imports —
 *     preloading lazy route chunks (preloadDynamicChunks: true) would fetch
 *     the whole app at cold start and defeat route-level code splitting.
 * Returns a list of violation messages (empty on pass).
 */
export function checkPreloadPosture(distDir) {
  const distAssets = join(distDir, 'assets');
  const errors = [];
  let html;
  try {
    html = readFileSync(join(distDir, 'index.html'));
  } catch {
    errors.push('dist/index.html not found. Run "npx vite build" first.');
    return errors;
  }

  // Only inventory filenames may be read, never a path supplied by HTML.
  // Hash raw bytes: decoding a chunk as UTF-8 can replace invalid sequences.
  const assets = new Map();
  function readAssets(directory, prefix = '') {
    if (!lstatSync(directory).isDirectory()) throw new Error('irregular asset directory');
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const relative = `${prefix}${entry.name}`;
      const file = join(directory, entry.name);
      if (entry.isDirectory()) readAssets(file, `${relative}/`);
      else if (entry.name.endsWith('.js')) {
        if (!entry.isFile()) throw new Error('irregular JavaScript asset');
        assets.set(relative, readFileSync(file));
      }
    }
  }
  try {
    readAssets(distAssets);
  } catch {
    errors.push('dist/assets/ JavaScript inventory is missing, unreadable or irregular');
    return errors;
  }
  const hashes = new Map();
  function checkIntegrity(file, value, context) {
    if (!assets.has(file)) {
      errors.push(`${context} references a missing or invalid JS asset`);
      return;
    }
    const tokens = typeof value === 'string' && value.trim() ? value.trim().split(/\s+/) : [];
    if (
      tokens.length === 0 ||
      tokens.some((token) => !/^(sha256|sha384|sha512)-[A-Za-z0-9+/]+={0,2}$/.test(token))
    ) {
      errors.push(`${context} has invalid or unsupported integrity metadata`);
      return;
    }
    // Generated output uses one digest. If it ever supplies alternatives,
    // require all of them to match, not just a weaker matching algorithm.
    // This deliberately validates a strict subset of browser SRI metadata.
    for (const token of tokens) {
      const algorithm = token.slice(0, token.indexOf('-'));
      const key = `${file}:${algorithm}`;
      if (!hashes.has(key)) {
        hashes.set(
          key,
          `${algorithm}-${createHash(algorithm).update(assets.get(file)).digest('base64')}`,
        );
      }
      if (token !== hashes.get(key)) {
        errors.push(`${context} integrity mismatch against built asset bytes`);
        return;
      }
    }
  }

  // Parse elements, never rewritten HTML. Deleting or masking comments can
  // invent tags/attributes, and regexes can mistake inert text for controls.
  // No runScripts/resources options: this is an inert, offline document, not
  // a preview. Do not emit parser diagnostics containing build-source text.
  // The locked jsdom/parse5 pair uses scripting-enabled tokenization with
  // node locations, independently of execution. Verify that behavior instead
  // of silently counting controls hidden inside noscript after a parser update.
  const parserOptions = {
    includeNodeLocations: true,
    virtualConsole: new VirtualConsole(),
  };
  const probe = new JSDOM('<noscript><p></p></noscript>', parserOptions);
  try {
    if (probe.window.document.querySelector('p')) {
      errors.push('HTML parser scripting mode is unsupported; build controls are unverified');
      return errors;
    }
  } finally {
    probe.window.close();
  }
  const dom = new JSDOM(html, parserOptions);
  let importmaps;
  let entries;
  let preloads;
  try {
    const document = dom.window.document;
    const htmlElements = (tag) =>
      [...document.querySelectorAll(tag)].filter(
        (element) => element.namespaceURI === 'http://www.w3.org/1999/xhtml',
      );
    // Declarative shadow DOM is unsupported by this parser. It is absent
    // from Vite output: do not silently ignore potentially active controls.
    if (document.querySelector('template[shadowrootmode]')) {
      errors.push('index.html declarative shadow DOM controls are unverified');
      return errors;
    }
    const scripts = htmlElements('script');
    const asciiLower = (value) =>
      (value ?? '').replace(/[A-Z]/g, (character) => character.toLowerCase());
    importmaps = scripts
      .filter((element) => asciiLower(element.getAttribute('type')) === 'importmap')
      .map((element) => ({ text: element.textContent, external: element.hasAttribute('src') }));
    entries = scripts
      .filter(
        (element) =>
          asciiLower(element.getAttribute('type')) === 'module' && element.hasAttribute('src'),
      )
      .map((element) => ({
        src: element.getAttribute('src'),
        integrity: element.getAttribute('integrity'),
      }));
    preloads = htmlElements('link')
      .filter((element) =>
        asciiLower(element.getAttribute('rel'))
          .split(/[\t\n\f\r ]+/)
          .includes('modulepreload'),
      )
      .map((element) => ({
        href: element.getAttribute('href'),
        integrity: element.getAttribute('integrity'),
      }));
  } finally {
    dom.window.close();
  }
  let integrity = null;
  if (importmaps.length === 0) {
    errors.push('importmap script block missing from index.html (dynamic-import SRI unenforced)');
  } else if (importmaps.length !== 1) {
    errors.push('index.html must contain exactly one importmap script block');
  } else if (importmaps[0].external) {
    errors.push('index.html importmap must be inline, not an external script');
  } else {
    let parsed;
    try {
      parsed = JSON.parse(importmaps[0].text);
    } catch {
      errors.push('importmap block in index.html is not valid JSON');
    }
    if (parsed !== undefined) {
      if (
        !parsed ||
        typeof parsed !== 'object' ||
        Array.isArray(parsed) ||
        !parsed.integrity ||
        typeof parsed.integrity !== 'object' ||
        Array.isArray(parsed.integrity) ||
        Object.keys(parsed.integrity).length === 0
      ) {
        errors.push(
          'importmap in index.html has no integrity block (dynamic-import SRI unenforced)',
        );
      } else {
        integrity = parsed.integrity;
      }
    }
  }
  if (integrity) {
    for (const file of assets.keys()) {
      if (!Object.hasOwn(integrity, `/assets/${file}`)) {
        errors.push(`importmap integrity missing entry for /assets/${file}`);
      } else {
        checkIntegrity(file, integrity[`/assets/${file}`], `importmap /assets/${file}`);
      }
    }
  }

  if (entries.length !== 1) {
    errors.push('index.html must contain exactly one external module entry script');
    return errors;
  }
  const entryMatch = entries[0].src.match(/^\/assets\/(.+\.js)$/);
  if (!entryMatch) {
    errors.push('module entry script not found in index.html');
    return errors;
  }
  const entryFile = entryMatch[1];
  checkIntegrity(
    entryFile,
    entries[0].integrity,
    `entry script /assets/${entryFile}`,
  );

  if (!assets.has(entryFile)) {
    errors.push(`entry chunk ${entryFile} not found in dist/assets/`);
    return errors;
  }
  const entrySource = assets.get(entryFile).toString('utf8');

  // Static imports/re-exports in Rollup output: import ... from"./x.js",
  // import"./x.js", export ... from"./x.js". Dynamic imports never match:
  // their specifier follows an opening parenthesis, not the keyword.
  const staticImportRe =
    /\b(?:import|export)\s*(?:[\w$*{},\s]+?\s*from\s*)?["']\.\/([^"']+\.js)["']/g;
  function staticImportsOf(source, importer) {
    const specifiers = [];
    let m;
    while ((m = staticImportRe.exec(source))) {
      specifiers.push(posix.join(posix.dirname(importer), m[1]));
    }
    return specifiers;
  }

  // Vite preloads the entry's TRANSITIVE static import graph, so walk it:
  // a chunk is preloadable iff it is reachable from the entry through static
  // imports alone. Lazy route chunks are only reachable dynamically and stay
  // excluded, as do their own static-only dependencies.
  const allowedPreloads = new Set([entryFile]);
  const queue = staticImportsOf(entrySource, entryFile);
  while (queue.length > 0) {
    const file = queue.pop();
    if (allowedPreloads.has(file)) continue;
    allowedPreloads.add(file);
    if (!assets.has(file)) {
      errors.push(`statically imported chunk ${file} not found in dist/assets/`);
      continue;
    }
    queue.push(...staticImportsOf(assets.get(file).toString('utf8'), file));
  }

  for (const preload of preloads) {
    const href = preload.href?.match(/^\/assets\/(.+)$/);
    if (!href) {
      errors.push('modulepreload link with unrecognized href in index.html');
      continue;
    }
    if (!allowedPreloads.has(href[1])) {
      errors.push(
        `modulepreload of lazy chunk /assets/${href[1]} — only the entry's static imports may be preloaded (keep preloadDynamicChunks: false in vite.config.ts)`,
      );
    }
    checkIntegrity(
      href[1],
      preload.integrity,
      `modulepreload of /assets/${href[1]}`,
    );
  }

  return errors;
}
