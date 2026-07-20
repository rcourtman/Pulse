import type { ClusterEndpoint, ClusterEndpointOverridePayload, NodeConfig } from '@/types/nodes';
import type { SecurityStatus } from '@/types/config';
import type { NodeModalNodeType } from '@/utils/nodeModalPresentation';

export interface NodeModalProps {
  isOpen: boolean;
  resetKey?: number;
  onClose: () => void;
  nodeType: NodeModalNodeType;
  editingNode?: NodeConfig;
  prefillNode?: Partial<NodeConfig>;
  onSave: (nodeData: Partial<NodeConfig>) => void;
  showBackToDiscovery?: boolean;
  onBackToDiscovery?: () => void;
  securityStatus?: Partial<SecurityStatus>;
  temperatureMonitoringEnabled?: boolean;
  temperatureMonitoringLocked?: boolean;
  savingTemperatureSetting?: boolean;
  onToggleTemperatureMonitoring?: (enabled: boolean) => Promise<void> | void;
  setupHandoffDisabled?: () => boolean;
  setupHandoffDisabledReason?: string;
}

// Build the write-only clusterEndpointOverrides payload from the form's
// per-member address record, including only members whose value actually
// changed from the saved override. Returns undefined when nothing changed so
// the PUT stays a no-op for endpoints (partial-update semantics).
export const buildClusterEndpointOverridesPayload = (
  endpoints: ClusterEndpoint[] | undefined,
  overrides: Record<string, string>,
): ClusterEndpointOverridePayload[] | undefined => {
  if (!endpoints?.length) return undefined;
  const changed = endpoints
    .filter((endpoint) => {
      const value = overrides[endpoint.nodeName];
      if (value === undefined) return false;
      return value.trim() !== (endpoint.ipOverride ?? '');
    })
    .map((endpoint) => ({
      nodeName: endpoint.nodeName,
      ipOverride: (overrides[endpoint.nodeName] ?? '').trim(),
    }));
  return changed.length > 0 ? changed : undefined;
};

// Mirror a saved clusterEndpointOverrides payload onto locally cached
// endpoints so the editor reflects the save without a full nodes reload.
export const applyClusterEndpointOverridesLocally = (
  endpoints: ClusterEndpoint[] | undefined,
  overrides: ClusterEndpointOverridePayload[] | undefined,
): ClusterEndpoint[] | undefined => {
  if (!endpoints?.length || !overrides?.length) return endpoints;
  const byNodeName = new Map(overrides.map((override) => [override.nodeName, override.ipOverride]));
  return endpoints.map((endpoint) => {
    const value = byNodeName.get(endpoint.nodeName);
    if (value === undefined) return endpoint;
    return { ...endpoint, ipOverride: value || undefined };
  });
};

export const deriveNameFromHost = (host: string): string => {
  let value = host.trim();
  if (!value) {
    return '';
  }

  try {
    const url = value.includes('://') ? new URL(value) : new URL(`https://${value}`);
    value = url.hostname || value;
  } catch {
    value = value.replace(/^https?:\/\//, '');
  }

  value = value.replace(/\/.*$/, '').replace(/^\[(.*)\]$/, '$1');
  value = value.replace(/\s+/g, '-');

  return value;
};

export const PVE_MANUAL_PERMISSION_COMMAND = `# Apply monitoring permissions - use built-in PVEAuditor role
pveum aclmod / -user pulse-monitor@pve -role PVEAuditor

# Gather additional privileges for VM metrics
EXTRA_PRIVS=()

# Sys.Audit (Ceph, cluster status)
if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
  EXTRA_PRIVS+=("Sys.Audit")
else
  if pveum role add PulseTmpSysAudit -privs Sys.Audit 2>/dev/null; then
    EXTRA_PRIVS+=("Sys.Audit")
    pveum role delete PulseTmpSysAudit 2>/dev/null
  fi
fi

# VM guest agent (PVE 9+) / monitor (PVE 8) privileges
HAS_GUEST_AGENT_AUDIT=false
if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
  HAS_GUEST_AGENT_AUDIT=true
elif pveum role add PulseTmpGuestAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
  HAS_GUEST_AGENT_AUDIT=true
  pveum role delete PulseTmpGuestAudit 2>/dev/null
fi

if [ "$HAS_GUEST_AGENT_AUDIT" = true ]; then
  EXTRA_PRIVS+=("VM.GuestAgent.Audit")

  if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.FileRead"; then
    EXTRA_PRIVS+=("VM.GuestAgent.FileRead")
  elif pveum role add PulseTmpGuestFileRead -privs VM.GuestAgent.FileRead 2>/dev/null; then
    EXTRA_PRIVS+=("VM.GuestAgent.FileRead")
    pveum role delete PulseTmpGuestFileRead 2>/dev/null
  fi
else
  if pveum role list 2>/dev/null | grep -q "VM.Monitor"; then
    EXTRA_PRIVS+=("VM.Monitor")
  elif pveum role add PulseTmpVMMonitor -privs VM.Monitor 2>/dev/null; then
    EXTRA_PRIVS+=("VM.Monitor")
    pveum role delete PulseTmpVMMonitor 2>/dev/null
  fi
fi

if [ \${#EXTRA_PRIVS[@]} -gt 0 ]; then
  PRIV_STRING="$(IFS=,; echo "\${EXTRA_PRIVS[*]}")"
  pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null
  pveum aclmod / -user pulse-monitor@pve -role PulseMonitor
fi`;
