import { copyFile, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const sourceDocsDir = path.join(repoRoot, 'docs');
const targetDocsDir = path.join(frontendRoot, 'public', 'docs');

const shippedDocs = ['README.md', 'PRIVACY.md'];

export async function syncPublicDocs() {
  await mkdir(targetDocsDir, { recursive: true });

  for (const filename of shippedDocs) {
    await copyFile(path.join(sourceDocsDir, filename), path.join(targetDocsDir, filename));
  }

  console.log(`Synced shipped docs to ${targetDocsDir}`);
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
