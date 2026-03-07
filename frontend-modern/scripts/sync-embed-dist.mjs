import { cp, mkdir, rm, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..');
const sourceDir = path.join(repoRoot, 'frontend-modern', 'dist');
const targetDir = path.join(repoRoot, 'internal', 'api', 'frontend-modern', 'dist');

async function ensureSourceExists() {
  try {
    const sourceStat = await stat(sourceDir);
    if (!sourceStat.isDirectory()) {
      throw new Error(`${sourceDir} is not a directory`);
    }
  } catch (error) {
    throw new Error(`frontend build output missing at ${sourceDir}`, { cause: error });
  }
}

async function syncEmbedDir() {
  await ensureSourceExists();
  await rm(targetDir, { force: true, recursive: true });
  await mkdir(path.dirname(targetDir), { recursive: true });
  await cp(sourceDir, targetDir, { recursive: true });
  console.log(`Synced frontend embed assets to ${targetDir}`);
}

syncEmbedDir().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
