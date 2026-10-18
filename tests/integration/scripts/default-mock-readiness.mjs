// The shared fixture's existing admission requirements. Keep browser setup
// and the early CI probe on one policy; readiness is not application acceptance.
const hasSource = (resource, source) =>
  Array.isArray(resource?.sources) && resource.sources.includes(source);

export function defaultMockInventoryReady(state) {
  const resources = Array.isArray(state?.resources) ? state.resources : [];
  const infrastructure = Array.isArray(state?.connectedInfrastructure)
    ? state.connectedInfrastructure : [];
  return resources.some(r => r?.name === 'nvme-primary' && r.type === 'storage' && hasSource(r, 'vmware')) &&
    resources.some(r => r?.type === 'k8s-cluster' && hasSource(r, 'kubernetes')) &&
    resources.some(r => r?.name === 'tank' && hasSource(r, 'truenas')) &&
    resources.some(r => ['vm', 'system-container'].includes(r?.type) && hasSource(r, 'proxmox')) &&
    resources.some(r => r?.name === 'esxi-01.lab.local' && r.type === 'agent' && hasSource(r, 'vmware')) &&
    resources.some(r => r?.type === 'docker-host') &&
    resources.some(r => r?.type === 'pbs') &&
    resources.some(r => r?.type === 'pmg') &&
    infrastructure.some(entry => entry?.name === 'esxi-01.lab.local');
}

function hasDeepSeries(points) {
  if (!Array.isArray(points)) return false;
  const timestamps = points.map(point => Number(point?.timestamp))
    .filter(Number.isFinite).sort((left, right) => left - right);
  return timestamps.length >= 2 &&
    timestamps[timestamps.length - 1] - timestamps[0] > 5 * 24 * 60 * 60 * 1000;
}

export function defaultMockHistoryReady(charts) {
  return Object.values(charts?.pools ?? {}).some(pool => hasDeepSeries(pool?.used)) &&
    Object.values(charts?.disks ?? {}).some(disk => hasDeepSeries(disk?.temperature));
}
