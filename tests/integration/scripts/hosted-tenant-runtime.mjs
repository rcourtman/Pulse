import { execFileSync } from 'node:child_process';

export function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

export function runRemote(cloudHost, command) {
  return execFileSync('ssh', [cloudHost, command], {
    encoding: 'utf8',
    stdio: 'pipe',
    maxBuffer: 32 * 1024 * 1024,
  });
}

export function resolveHostedTenantRootDataDir(tenantId) {
  return `/data/tenants/${tenantId}`;
}

export function resolveHostedTenantOrgDataDir(tenantId, orgId) {
  const scopedOrgId = String(orgId || tenantId).trim();
  if (scopedOrgId === '' || scopedOrgId === 'default') {
    return resolveHostedTenantRootDataDir(tenantId);
  }
  return `/data/tenants/${tenantId}/orgs/${scopedOrgId}`;
}

export function restartHostedTenantRuntime(cloudHost, tenantId) {
  const containerName = `pulse-${tenantId}`;
  const script = `
set -eu
container=${shellQuote(containerName)}
docker restart "$container" >/dev/null
deadline=$(( $(date +%s) + 60 ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  state="$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
  case "$state" in
    "running healthy"|"running none")
      exit 0
      ;;
  esac
  sleep 1
done
echo "timed out waiting for $container to become ready" >&2
docker inspect --format '{{json .State}}' "$container" >&2 || true
exit 1
`;
  runRemote(cloudHost, `sh -lc ${shellQuote(script)}`);
}
