import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export const frontendRoot = __dirname;
export const repoRoot = path.resolve(frontendRoot, '../../../../');
export const distRoot = path.resolve(frontendRoot, '../dist');
export const manifestPath = path.join(distRoot, 'build_manifest.json');

export function createPortalBuildOptions(overrides = {}) {
  const {
    outfile = path.join(distRoot, 'portal_app.js'),
    plugins = [],
    write = true,
    logLevel = 'info',
    sourcemap = false,
    minify = false,
  } = overrides;

  return {
    absWorkingDir: frontendRoot,
    entryPoints: ['src/index.ts'],
    outfile,
    bundle: true,
    loader: {
      '.woff2': 'dataurl',
      '.woff': 'dataurl',
      '.ttf': 'dataurl',
      '.otf': 'dataurl',
    },
    format: 'iife',
    platform: 'browser',
    target: ['es2020'],
    legalComments: 'none',
    sourcemap,
    minify,
    logLevel,
    write,
    plugins,
  };
}
