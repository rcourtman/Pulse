import { createHash } from 'node:crypto';

// Model a completed compiler output, not the validator: expectations still
// compare every published byte and exercise the real scripts in child processes.
export function completeBuild(files, { entry, preloads = [], styles = [] }) {
  const integrity = Object.fromEntries(
    Object.entries(files)
      .filter(([name]) => name.startsWith('assets/') && name.endsWith('.js'))
      .map(([name, bytes]) => [
        `/${name}`,
        `sha384-${createHash('sha384').update(bytes).digest('base64')}`,
      ]),
  );
  return {
    ...files,
    'index.html': [
      '<!doctype html>',
      `<script type="importmap">${JSON.stringify({ integrity })}</script>`,
      `<script type="module" src="/assets/${entry}" integrity="${integrity[`/assets/${entry}`]}"></script>`,
      ...preloads.map((name) =>
        `<link rel="modulepreload" href="/assets/${name}" integrity="${integrity[`/assets/${name}`]}">`),
      ...styles.map((name) => `<link rel="stylesheet" href="/assets/${name}">`),
      '',
    ].join('\n'),
  };
}
