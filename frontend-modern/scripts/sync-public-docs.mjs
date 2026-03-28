import { copyFile, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const targetDocsDir = path.join(frontendRoot, 'public', 'docs');

const shippedDocs = [
  { source: path.join(repoRoot, 'docs', 'README.md'), target: 'README.md' },
  { source: path.join(repoRoot, 'docs', 'PRIVACY.md'), target: 'PRIVACY.md' },
  { source: path.join(repoRoot, 'docs', 'CONFIGURATION.md'), target: 'CONFIGURATION.md' },
  { source: path.join(repoRoot, 'docs', 'PROXY_AUTH.md'), target: 'PROXY_AUTH.md' },
  { source: path.join(repoRoot, 'SECURITY.md'), target: 'SECURITY.md' },
];

export async function syncPublicDocs() {
  await mkdir(targetDocsDir, { recursive: true });

  for (const { source, target } of shippedDocs) {
    await copyFile(source, path.join(targetDocsDir, target));
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
