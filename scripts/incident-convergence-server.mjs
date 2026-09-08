// Loopback synthetic HTTP responses; never an installed backend or destination.
import { createServer } from '../frontend-modern/node_modules/vite/dist/node/index.js';
import solid from '../frontend-modern/node_modules/vite-plugin-solid/dist/esm/index.mjs';
import { fileURLToPath } from 'node:url';
import { fixture } from './incident-browser-fixture.mjs';
export async function startIncidentFixture() {
  const root = fileURLToPath(new URL('../frontend-modern', import.meta.url));
  process.chdir(root);
  const pending = [];
  const source = fixture.replace(
    'new Promise((resolve, reject) => pending.push({resolve, reject}))',
    "fetch('/fixture/incidents?request=' + pending.push({}), { cache: 'no-store' }).then(r => r.json())");
  const server = await createServer({ root, configFile: false,
    optimizeDeps: { noDiscovery: true, entries: [], esbuildOptions: { target: 'esnext' } },
    esbuild: { target: 'esnext' }, resolve: { alias: { '@': root + '/src' } },
    plugins: [solid(), { name: 'incident-convergence',
      configureServer(s) { s.middlewares.use((req, res, next) => {
        if (req.url?.startsWith('/fixture/incidents?request=')) { pending.push(res); console.log(JSON.stringify({ stage: 'incident-request', index: pending.length - 1 })); }
        else if (req.url === '/qualification') { res.setHeader('Content-Type', 'text/html'); res.end('<div id="root"></div><script type="module" src="/incident-fixture.tsx"></script>'); }
        else next();
      }); },
      resolveId(id) { if (id === '/incident-fixture.tsx') return id; },
      load(id) { if (id === '/incident-fixture.tsx') return source; },
    }], server: { host: '127.0.0.1', port: 5198, strictPort: true },
  });
  await server.listen();
  return { count: () => pending.length,
    finish(index, id) {
      const res = pending[index];
      if (!res) throw new Error('Missing pending request ' + index);
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify([{ id, message: id, level: 'warning', status: 'open', openedAt: '2026-09-08T07:20:00Z', events: [] }]));
      console.log(JSON.stringify({ stage: 'incident-response-sent', index, id }));
    }, async close() { for (const res of pending) if (!res.writableEnded) res.end('[]'); await server.close(); },
  };
}
