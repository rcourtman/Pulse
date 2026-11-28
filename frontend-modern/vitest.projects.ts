import fs from 'node:fs';
import path from 'node:path';

export const JEST_DOM_MATCHER_RE =
  /\b(?:toBeInTheDocument|toHaveAttribute|toHaveClass|toHaveTextContent|toBeVisible|toBeDisabled|toBeEnabled|toBeChecked|toHaveValue|toHaveStyle|toHaveFocus|toHaveAccessibleName|toHaveAccessibleDescription|toHaveAccessibleErrorMessage|toContainElement|toContainHTML|toBeEmptyDOMElement|toBeRequired|toBeInvalid|toBeValid|toHaveDescription|toHaveDisplayValue|toHaveErrorMessage|toHaveFormValues|toHaveRole|toBePartiallyChecked)\b/;

export const NON_ENGLISH_LOCALE_RE =
  /\b(?:setActiveLocale|setLocalePreference|loadLocaleCatalog|FIRST_LOCALIZATION_LOCALES|LOCALE_PREFERENCE|translateMessage)\b|navigator\.language|['"`](?:de|es|fr)(?:[-_][A-Za-z]+)?['"`]/;

const DIRECT_DOM_RE = new RegExp(
  [
    '@solidjs/testing-library',
    '@testing-library/',
    'solid-js/web',
    '\\bjsdom\\b',
    '\\bhappy-dom\\b',
    '@vitest-environment',
    '\\b(?:document|window|navigator|sessionStorage)\\s*(?:\\.|\\[|\\?\\.)',
    '\\bglobalThis\\.(?:document|window|navigator|sessionStorage|HTMLElement|Element|Node|CustomEvent|MouseEvent|KeyboardEvent|ResizeObserver|IntersectionObserver|MutationObserver|matchMedia|getComputedStyle|requestAnimationFrame|cancelAnimationFrame|DOMParser|XMLSerializer|CSSStyleSheet|FileReader|CanvasRenderingContext2D)\\b',
    '\\b(?:HTMLElement|HTMLInputElement|HTMLButtonElement|HTMLDivElement|HTMLSelectElement|HTMLTextAreaElement|HTMLFormElement|HTMLAnchorElement|SVGElement|DocumentFragment|ShadowRoot|DOMParser|XMLSerializer|CSSStyleSheet|FileReader|CanvasRenderingContext2D|ResizeObserver|IntersectionObserver|MutationObserver|MediaQueryList)\\b',
    '\\b(?:CustomEvent|MouseEvent|KeyboardEvent|PointerEvent|FocusEvent|InputEvent|ClipboardEvent|DragEvent|TouchEvent|WheelEvent|UIEvent|StorageEvent|PopStateEvent|HashChangeEvent)\\b',
    '\\b(?:matchMedia|getComputedStyle|requestAnimationFrame|cancelAnimationFrame)\\s*\\(',
    JEST_DOM_MATCHER_RE.source,
  ].join('|'),
);

const TYPE_ONLY_IMPORT_RE =
  /^\s*import\s+type\s+(?:\*\s+as\s+[A-Za-z_$][\w$]*|[A-Za-z_$][\w$]*(?:\s*,\s*\{[^}]*\})?|\{[^}]*\})\s+from\s+['"][^'"]+['"]\s*;?/gm;
const TYPE_ONLY_EXPORT_RE =
  /^\s*export\s+type\s+(?:\*(?:\s+as\s+[A-Za-z_$][\w$]*)?|\{[^}]*\})\s+from\s+['"][^'"]+['"]\s*;?/gm;
const NAMED_TYPE_IMPORT_EXPORT_RE =
  /^\s*(?:import|export)\s*\{([^}]+)\}\s+from\s+['"][^'"]+['"]\s*;?/gm;
const IMPORT_SPECIFIER_RE =
  /(?:from\s+|import\s*\(\s*|vi\.(?:mock|doMock|importActual|importMock)\s*\(\s*|require\s*\(\s*|^\s*import\s+)['"]([^'"]+)['"]/gm;
const HOISTED_FACTORY_MOCK_RE =
  /^vi\.mock\s*\(\s*['"]([^'"]+)['"]\s*,\s*(?:async\s*)?\(\s*\)\s*=>/gm;
const DYNAMIC_MOCK_ESCAPE_RE = /\b(?:importOriginal|importActual|doUnmock|unmock|resetModules)\b/;

function stripTypeOnlyModuleEdges(source: string): string {
  return source
    .replace(TYPE_ONLY_IMPORT_RE, '')
    .replace(TYPE_ONLY_EXPORT_RE, '')
    .replace(NAMED_TYPE_IMPORT_EXPORT_RE, (full, body: string) => {
      const specifiers = body
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);
      if (specifiers.length > 0 && specifiers.every((item) => /^type\s+[A-Za-z_$]/.test(item))) {
        return '';
      }
      return full;
    });
}

const SKIP_DIRS = new Set(['node_modules', 'dist', 'coverage', '.git']);
const NON_RUNTIME_LOCAL_EXTS = new Set(['.json', '.md', '.txt']);

export function needsJestDomForSource(filePath: string, source: string): boolean {
  if (filePath.endsWith('.tsx')) return true;
  return JEST_DOM_MATCHER_RE.test(source) || /@testing-library\/jest-dom/.test(source);
}

export function needsNonEnglishLocalesForSource(source: string): boolean {
  return NON_ENGLISH_LOCALE_RE.test(source);
}

function walkTestTsFiles(dir: string, out: string[] = []): string[] {
  if (!fs.existsSync(dir)) return out;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) {
        walkTestTsFiles(path.join(dir, entry.name), out);
      }
    } else if (entry.isFile() && entry.name.endsWith('.test.ts')) {
      out.push(path.join(dir, entry.name));
    }
  }
  return out;
}

function resolveLocalModule(srcRoot: string, fromFile: string, specifier: string): string | null {
  let target: string;
  if (specifier.startsWith('@/')) {
    target = path.join(srcRoot, specifier.slice(2));
  } else if (specifier.startsWith('.')) {
    target = path.resolve(path.dirname(fromFile), specifier);
  } else {
    return null;
  }

  const ext = path.extname(target);
  if (NON_RUNTIME_LOCAL_EXTS.has(ext)) {
    return '';
  }

  const candidates = [
    target,
    `${target}.ts`,
    `${target}.tsx`,
    `${target}.js`,
    `${target}.mjs`,
    `${target}.cjs`,
    path.join(target, 'index.ts'),
    path.join(target, 'index.tsx'),
    path.join(target, 'index.js'),
  ];
  for (const candidate of candidates) {
    try {
      if (fs.statSync(candidate).isFile()) return candidate;
    } catch {
      // Try next candidate.
    }
  }
  return null;
}

/**
 * Partition `src/**\/*.test.ts` files into provably DOM-free unit tests that
 * can run in Vitest's `node` environment instead of instantiating `jsdom`.
 *
 * Classification is strictly conservative: `.test.tsx` files, tests with an
 * explicit `@vitest-environment` directive, tests or transitive runtime
 * dependencies referencing browser/DOM APIs, and any unresolvable local
 * runtime imports remain in the `jsdom` project.
 */
export function partitionVitestFiles(frontendRoot: string): { nodeFiles: string[] } {
  const srcRoot = path.join(frontendRoot, 'src');
  const testFiles = walkTestTsFiles(srcRoot).sort();

  const sourceCache = new Map<string, string>();
  const directDomCache = new Map<string, boolean>();
  const depsCache = new Map<string, { unresolvedOrDom: boolean; localDeps: string[] }>();

  const readSource = (filePath: string): string => {
    let cached = sourceCache.get(filePath);
    if (cached === undefined) {
      cached = fs.readFileSync(filePath, 'utf8');
      sourceCache.set(filePath, cached);
    }
    return cached;
  };

  const hasDirectDom = (filePath: string): boolean => {
    let cached = directDomCache.get(filePath);
    if (cached === undefined) {
      cached = DIRECT_DOM_RE.test(readSource(filePath));
      directDomCache.set(filePath, cached);
    }
    return cached;
  };

  const getRuntimeDeps = (filePath: string): { unresolvedOrDom: boolean; localDeps: string[] } => {
    let cached = depsCache.get(filePath);
    if (cached !== undefined) return cached;

    const stripped = stripTypeOnlyModuleEdges(readSource(filePath));
    const localDeps: string[] = [];
    let unresolvedOrDom = false;

    IMPORT_SPECIFIER_RE.lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = IMPORT_SPECIFIER_RE.exec(stripped)) !== null) {
      const specifier = match[1];
      if (
        specifier === 'solid-js/web' ||
        specifier === '@solidjs/testing-library' ||
        specifier.startsWith('@testing-library/')
      ) {
        unresolvedOrDom = true;
        break;
      }
      if (specifier.startsWith('@/') || specifier.startsWith('.')) {
        const resolved = resolveLocalModule(srcRoot, filePath, specifier);
        if (resolved === null) {
          unresolvedOrDom = true;
          break;
        }
        if (resolved !== '') {
          localDeps.push(resolved);
        }
      }
    }

    cached = { unresolvedOrDom, localDeps };
    depsCache.set(filePath, cached);
    return cached;
  };

  const getHoistedFactoryMockedLocalDeps = (entryFile: string): Set<string> => {
    const mocked = new Set<string>();
    const source = readSource(entryFile);
    if (DYNAMIC_MOCK_ESCAPE_RE.test(source)) {
      return mocked;
    }
    HOISTED_FACTORY_MOCK_RE.lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = HOISTED_FACTORY_MOCK_RE.exec(source)) !== null) {
      const specifier = match[1];
      if (specifier.startsWith('@/') || specifier.startsWith('.')) {
        const resolved = resolveLocalModule(srcRoot, entryFile, specifier);
        if (resolved) {
          mocked.add(resolved);
        }
      }
    }
    return mocked;
  };

  const requiresJsdom = (entryFile: string): boolean => {
    const mockedLocalDeps = getHoistedFactoryMockedLocalDeps(entryFile);
    const visited = new Set<string>([entryFile]);
    const stack = [entryFile];
    while (stack.length > 0) {
      const current = stack.pop()!;
      if (hasDirectDom(current)) return true;
      const { unresolvedOrDom, localDeps } = getRuntimeDeps(current);
      if (unresolvedOrDom) return true;
      for (const dep of localDeps) {
        if (mockedLocalDeps.has(dep)) continue;
        if (!visited.has(dep)) {
          visited.add(dep);
          stack.push(dep);
        }
      }
    }
    return false;
  };

  const nodeFiles: string[] = [];
  for (const filePath of testFiles) {
    if (!requiresJsdom(filePath)) {
      nodeFiles.push(path.relative(frontendRoot, filePath).split(path.sep).join('/'));
    }
  }

  return { nodeFiles };
}
