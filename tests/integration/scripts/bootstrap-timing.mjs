// Bounded paired probes for the Settings bootstrap diagnostic. Never retain
// headers, cookies, bodies, query strings, tenant IDs or exception messages.
const paths = new Set(['/api/state/summary', '/api/license/runtime-capabilities']);

export function observeBootstrapTiming(page, now = Date.now) {
  const rows = [];
  const pending = [];
  const seen = new Set();
  const starts = new Map();
  const onRequest = (request) => {
    const path = new URL(request.url()).pathname;
    if (!paths.has(path) || seen.has(path) || request.method() !== 'GET') return;
    seen.add(path);
    const started = now();
    const row = { transport: 'browser', path, started, status: null, duration: null };
    rows.push(row);
    starts.set(request, row);
    // page.request uses the same cookie session without the browser network
    // scheduler. This extra request can perturb startup: timings are diagnostic,
    // not a controlled effect size or a substitute for the original assertion.
    pending.push((async () => {
      const probe = { transport: 'http', path, started: now(), status: null, duration: null };
      rows.push(probe);
      let response;
      try {
        response = await page.request.get(path, { timeout: 5000, maxRedirects: 0 });
        probe.status = response.status();
      } catch {
        // Null means no observed response, not an HTTP error status.
      } finally {
        probe.duration = now() - probe.started;
        await response?.dispose().catch(() => {});
      }
    })());
  };
  const onResponse = (response) => {
    const row = starts.get(response.request());
    if (!row) return;
    row.status = response.status();
    row.duration = now() - row.started;
  };
  page.on('request', onRequest);
  page.on('response', onResponse);
  return async () => {
    page.off('request', onRequest);
    page.off('response', onResponse);
    await Promise.all(pending);
    return rows;
  };
}
