// Isolated non-release E2E issuer. No network clients or production credentials.
import { generateKeyPairSync, randomUUID, sign } from 'node:crypto';
import http from 'node:http';

const encode = value => Buffer.from(JSON.stringify(value)).toString('base64url');
export async function startOfflineLicenseIssuer() {
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  const activationKey = `ppk_live_e2e_${randomUUID()}`;
  const installations = new Map();
  const fingerprints = new Map();
  const grant = installation => {
    const now = Math.floor(Date.now() / 1000);
    const jti = `grt_${randomUUID()}`;
    const claims = { iss: 'pulse-license', aud: 'pulse-relay', sub: installation.id,
      lid: installation.license, iid: installation.id, lv: 1, st: 'active',
      tier: 'msp', plan: 'msp_starter', iat: now, nbf: now, exp: now + 3600,
      jti, email: 'e2e@example.invalid' };
    const input = `${encode({ alg: 'EdDSA', typ: 'JWT' })}.${encode(claims)}`;
    return { jwt: `${input}.${sign(null, Buffer.from(input), privateKey).toString('base64url')}`,
      jti, expires_at: new Date(claims.exp * 1000).toISOString() };
  };
  const server = http.createServer(async (req, res) => {
    const reply = (status, body) => {
      res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
      res.end(JSON.stringify(body));
    };
    if (req.method !== 'POST' || !['/v1/activate', '/v1/grants/status', '/v1/grants/refresh'].includes(req.url)) {
      reply(404, { error: 'unsupported fixture route' }); return;
    }
    try {
      let body = '';
      for await (const chunk of req) {
        body += chunk;
        if (Buffer.byteLength(body) > 16384) { reply(413, { error: 'request too large' }); return; }
      }
      const data = JSON.parse(body);
      if (!data || typeof data.instance_fingerprint !== 'string' || !data.instance_fingerprint.trim() || data.instance_fingerprint.length > 256) {
        reply(400, { error: 'missing fixture fingerprint' }); return;
      }
      if (req.url === '/v1/activate') {
        if (data.activation_key !== activationKey || data.runtime?.build !== 'community') {
          reply(401, { error: 'invalid fixture activation' }); return;
        }
        let installation = fingerprints.get(data.instance_fingerprint);
        if (!installation) {
          if (installations.size >= 1024) { reply(429, { error: 'fixture capacity reached' }); return; }
          installation = { id: `inst_${randomUUID()}`, license: `lic_${randomUUID()}`,
            token: `pit_live_e2e_${randomUUID()}`, fingerprint: data.instance_fingerprint };
          installations.set(installation.id, installation);
          fingerprints.set(installation.fingerprint, installation);
        }
        reply(201, { license: { license_id: installation.license, state: 'active', tier: 'msp', license_version: 1 },
          installation: { installation_id: installation.id, installation_token: installation.token, status: 'active' },
          grant: grant(installation) });
        return;
      }
      const installation = installations.get(data.installation_id);
      if (!installation || req.headers.authorization !== `Bearer ${installation.token}` || data.instance_fingerprint !== installation.fingerprint) {
        reply(401, { error: 'invalid fixture installation binding' }); return;
      }
      reply(200, req.url === '/v1/grants/status'
        ? { license_version: 1, refresh_required: false, server_time: new Date().toISOString(), status_policy: { recommended_check_after_sec: 300 } }
        : { grant: grant(installation) });
    } catch { if (!res.headersSent) reply(400, { error: 'invalid fixture request' }); }
  });
  server.requestTimeout = 5000;
  server.headersTimeout = 5000;
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  return {
    url: `http://127.0.0.1:${server.address().port}`,
    publicKey: Buffer.from(publicKey.export({ format: 'jwk' }).x, 'base64url').toString('base64'),
    activationKey,
    close: () => new Promise((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}
