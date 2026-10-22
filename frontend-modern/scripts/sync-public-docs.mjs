import { lstat, readFile, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const targetDocsDir = path.join(frontendRoot, 'public', 'docs');

// Match the shipped-doc mirror checks. The existing public tree selects what
// ships; copying every file from docs/ would expose internal source documents.
const rootSourcedDocs = new Set(['SECURITY.md', 'TERMS.md', 'ARCHITECTURE.md', 'CONTRIBUTING.md']);

async function requirePath(parts, directory) {
  for (let index = 0; index < parts.length; index += 1) {
    const file = path.join(repoRoot, ...parts.slice(0, index + 1));
    const stat = await lstat(file);
    const isDirectory = index < parts.length - 1 || directory;
    if (isDirectory ? !stat.isDirectory() : !stat.isFile()) {
      throw new Error(
        `Shipped docs require a regular ${isDirectory ? 'directory' : 'file'}: ${file}`,
      );
    }
  }
}

async function collectShippedDocs(directory, prefix = []) {
  const docs = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const parts = [...prefix, entry.name];
    const file = path.join(directory, entry.name);
    if (entry.isSymbolicLink()) {
      throw new Error(`Shipped docs must not follow a symlink: ${file}`);
    }
    if (entry.isDirectory()) {
      docs.push(...(await collectShippedDocs(file, parts)));
    } else if (entry.name.endsWith('.md')) {
      if (!entry.isFile()) {
        throw new Error(`Shipped docs require a regular file: ${file}`);
      }
      docs.push(parts);
    }
  }
  return docs;
}

export async function syncPublicDocs() {
  await requirePath(['frontend-modern', 'public', 'docs'], true);
  const shippedDocs = await collectShippedDocs(targetDocsDir);
  if (shippedDocs.length === 0) {
    throw new Error(`No shipped Markdown guides found in ${targetDocsDir}`);
  }

  // Read and validate the complete selection before changing any copy. A
  // missing source must fail the build, not leave a partly refreshed help tree.
  const copies = [];
  for (const parts of shippedDocs) {
    const sourceParts =
      parts.length === 1 && rootSourcedDocs.has(parts[0]) ? parts : ['docs', ...parts];
    await requirePath(sourceParts, false);
    const source = await readFile(path.join(repoRoot, ...sourceParts));
    const target = path.join(targetDocsDir, ...parts);
    if (!source.equals(await readFile(target))) {
      copies.push({ target, content: source });
    }
  }

  for (const { target, content } of copies) {
    await writeFile(target, content);
  }

  console.log(`Synced ${shippedDocs.length} shipped docs to ${targetDocsDir}`);
}

const isMainModule = process.argv[1]
  ? pathToFileURL(process.argv[1]).href === import.meta.url
  : false;

if (isMainModule) {
  syncPublicDocs().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  });
}
