import { Show, createMemo } from 'solid-js';
import { AlertTriangle } from 'lucide-solid';
import { InlineNotice } from '@/components/shared/InlineNotice';
import type { IdentityConflictHost } from './dockerIdentityConflict';

type DockerIdentityConflictNoticeProps = {
  hosts: IdentityConflictHost[];
};

const hostLabel = (host: IdentityConflictHost): string =>
  host.hostnames.length > 1 ? `${host.name} (${host.hostnames.join(', ')})` : host.name;

// Warns when the server detects reports from two different machines being
// folded into one Docker host record, usually cloned VMs that still share the
// same /etc/machine-id. Unlike the outdated-agent notice this is not gated on
// presentation policy, because it flags that the data on this page is
// unreliable, which matters to read-only viewers too.
export function DockerIdentityConflictNotice(props: DockerIdentityConflictNoticeProps) {
  const count = createMemo(() => props.hosts.length);

  const message = createMemo(() => {
    if (count() === 1) {
      const host = props.hosts[0];
      const names = host.hostnames.length > 1 ? ` (${host.hostnames.join(', ')})` : '';
      return (
        `Two machines appear to share the identity of ${host.name}${names}. ` +
        `They are likely cloned VMs with the same /etc/machine-id, so Pulse sees them as one host and their reports overwrite each other. ` +
        `Give one of them a fresh machine-id and restart its agent to monitor them separately.`
      );
    }
    const labels = props.hosts.map(hostLabel).join('; ');
    return (
      `${count()} hosts are each receiving reports from more than one machine. ` +
      `They are likely cloned VMs with the same /etc/machine-id, so their reports overwrite each other. ` +
      `Give the clones fresh machine-ids and restart their agents to monitor them separately. Affected hosts are ${labels}.`
    );
  });

  return (
    <Show when={count() > 0}>
      <InlineNotice
        role="status"
        data-testid="docker-identity-conflict-notice"
        tone="warning"
        icon={<AlertTriangle aria-hidden="true" />}
      >
        {message()}
      </InlineNotice>
    </Show>
  );
}
