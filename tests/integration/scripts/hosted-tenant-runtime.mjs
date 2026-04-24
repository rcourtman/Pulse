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

export function hostedTenantContainerName(tenantId) {
  const normalizedTenantId = String(tenantId || '').trim();
  if (!normalizedTenantId) {
    throw new Error('tenantId is required');
  }
  return `pulse-${normalizedTenantId}`;
}

function remoteFailureOutput(error) {
  const remoteOutput = ['stderr', 'stdout']
    .map((field) => error?.[field])
    .filter((value) => value !== undefined && value !== null && String(value).trim() !== '')
    .map((value) => String(value).trim())
    .join('\n');
  if (remoteOutput) {
    return remoteOutput;
  }
  return String(error?.message || '').trim();
}

export function hostedTenantRuntimeExistsScript(tenantId) {
  const containerName = hostedTenantContainerName(tenantId);
  return `
set -eu
container=${shellQuote(containerName)}
if ! docker inspect "$container" >/dev/null 2>&1; then
  echo "hosted tenant runtime container $container does not exist" >&2
  echo "create or restore an active hosted tenant before running hosted mobile proof seeding" >&2
  exit 72
fi
docker inspect --format '{{.Name}} {{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container"
`;
}

export function assertHostedTenantRuntimeExists(cloudHost, tenantId, runner = runRemote) {
  const containerName = hostedTenantContainerName(tenantId);
  try {
    return runner(cloudHost, `sh -lc ${shellQuote(hostedTenantRuntimeExistsScript(tenantId))}`);
  } catch (error) {
    const detail = remoteFailureOutput(error);
    throw new Error([
      `hosted tenant runtime ${containerName} is unavailable on ${cloudHost}`,
      'Hosted mobile proof seeding requires an active hosted tenant container before it mutates approval or token data.',
      detail,
    ].filter(Boolean).join('\n'));
  }
}

export function restartHostedTenantRuntime(cloudHost, tenantId, runner = runRemote) {
  const containerName = hostedTenantContainerName(tenantId);
  const script = `
set -eu
container=${shellQuote(containerName)}
if ! docker inspect "$container" >/dev/null 2>&1; then
  echo "hosted tenant runtime container $container does not exist" >&2
  echo "create or restore an active hosted tenant before restarting hosted mobile proof state" >&2
  exit 72
fi
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
  try {
    runner(cloudHost, `sh -lc ${shellQuote(script)}`);
  } catch (error) {
    const detail = remoteFailureOutput(error);
    throw new Error([
      `failed to restart hosted tenant runtime ${containerName} on ${cloudHost}`,
      detail,
    ].filter(Boolean).join('\n'));
  }
}
