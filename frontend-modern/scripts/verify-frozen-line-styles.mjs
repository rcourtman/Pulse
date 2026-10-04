import { createHash } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const snapshotPath = 'qualification/final-line-styles.json';
const stylesheetPath = 'src/final-line-styles.generated.css';
const digest = (bytes) => createHash('sha256').update(bytes).digest('hex');

function inputs(root) {
  const files = ['index.html', 'tailwind.config.js'];
  const visit = (relative) => {
    for (const entry of readdirSync(path.join(root, relative), { withFileTypes: true })) {
      if (entry.isSymbolicLink()) throw new Error('Frozen line styles reject symlink inputs');
      const name = `${relative}/${entry.name}`;
      if (entry.isDirectory()) {
        if (!['__tests__', '__fixtures__'].includes(entry.name)) visit(name);
      } else if (
        /\.(?:css|ts|tsx|js|jsx)$/.test(name) &&
        !/\.(?:test|spec|stories)\./.test(name) &&
        name !== stylesheetPath
      ) {
        files.push(name);
      }
    }
  };
  visit('src');
  return files.sort();
}

export function frozenLineStyleInputs(root = frontendRoot) {
  return Object.fromEntries(
    inputs(root).map((name) => [name, digest(readFileSync(path.join(root, name)))]),
  );
}

// The final v6.4 line is frozen product source, not the main development
// toolchain. Do not silently ship missing styles after editing an input.
export function verifyFrozenLineStyles(root = frontendRoot) {
  const snapshot = JSON.parse(readFileSync(path.join(root, snapshotPath), 'utf8'));
  if (
    snapshot.version !== 1 ||
    snapshot.stylesheet?.path !== stylesheetPath ||
    !/^[a-f0-9]{64}$/.test(snapshot.stylesheet?.sha256 ?? '') ||
    !/^[a-f0-9]{40}$/.test(snapshot.generated_from?.source_sha ?? '')
  ) {
    throw new Error('Invalid frozen line stylesheet provenance');
  }
  if (digest(readFileSync(path.join(root, stylesheetPath))) !== snapshot.stylesheet.sha256) {
    throw new Error('Frozen line stylesheet changed; exact-source regeneration is required');
  }
  const actual = frozenLineStyleInputs(root);
  const expected = snapshot.inputs;
  if (
    !expected ||
    Object.keys(expected).sort().join('\n') !== Object.keys(actual).join('\n') ||
    Object.entries(actual).some(([name, hash]) => expected[name] !== hash)
  ) {
    throw new Error(
      'Frozen line style inputs changed; regenerate and qualify the exact candidate before building',
    );
  }
  return snapshot;
}
