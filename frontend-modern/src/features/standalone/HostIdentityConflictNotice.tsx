import { Show, createMemo } from 'solid-js';
import { AlertTriangle } from 'lucide-solid';
import { InlineNotice } from '@/components/shared/InlineNotice';
import type { HostIdentityConflictHost } from './hostIdentityConflict';

type HostIdentityConflictNoticeProps = {
  hosts: HostIdentityConflictHost[];
};

// The flapping evidence: distinct hostnames when the clones are named apart,
// otherwise the alternating report IPs (template deployments often reuse
// hostnames too, e.g. pve01 at two different sites).
const conflictDetail = (host: HostIdentityConflictHost): string => {
  if (host.hostnames.length > 1) return host.hostnames.join(', ');
  if (host.reportIps.length > 1) return host.reportIps.join(', ');
  return '';
};

const hostLabel = (host: HostIdentityConflictHost): string => {
  const detail = conflictDetail(host);
  return detail ? `${host.name} (${detail})` : host.name;
};

// Warns when the server detects reports from two different machines being
// folded into one host agent record, usually template-deployed machines that
// still share the same /etc/machine-id. Unlike the outdated-agent notice this
// is not gated on presentation policy, because it flags that the data on this
// page is unreliable, which matters to read-only viewers too.
export function HostIdentityConflictNotice(props: HostIdentityConflictNoticeProps) {
  const count = createMemo(() => props.hosts.length);

  const message = createMemo(() => {
    if (count() === 1) {
      const host = props.hosts[0];
      const detail = conflictDetail(host);
      const names = detail ? ` (${detail})` : '';
      return (
        `Two machines appear to share the identity of ${host.name}${names}. ` +
        `They are likely cloned from the same template with the same /etc/machine-id, so Pulse sees them as one host and their reports overwrite each other. ` +
        `Give one of them a fresh machine-id and restart its agent to monitor them separately.`
      );
    }
    const labels = props.hosts.map(hostLabel).join(' · ');
    return (
      `${count()} hosts are each receiving reports from more than one machine. ` +
      `They are likely cloned from the same template with the same /etc/machine-id, so their reports overwrite each other. ` +
      `Give the clones fresh machine-ids and restart their agents to monitor them separately. Affected hosts are ${labels}.`
    );
  });

  return (
    <Show when={count() > 0}>
      <InlineNotice
        role="status"
        data-testid="host-identity-conflict-notice"
        tone="warning"
        icon={<AlertTriangle aria-hidden="true" />}
      >
        {message()}
      </InlineNotice>
    </Show>
  );
}
