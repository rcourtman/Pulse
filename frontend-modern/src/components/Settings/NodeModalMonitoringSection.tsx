import { Component, createMemo, createUniqueId, Show } from 'solid-js';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { FormSelect } from '@/components/shared/FormSelect';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  controlClass,
  formCheckbox,
  formField,
  formHelpText,
  labelClass,
} from '@/components/shared/Form';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { InlineNotice } from '@/components/shared/InlineNotice';
import {
  getNodeMonitoringCoverageCopy,
  getTemperatureMonitoringLockedCopy,
} from '@/utils/nodeModalPresentation';

interface NodeModalMonitoringSectionProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

export const NodeModalMonitoringSection: Component<NodeModalMonitoringSectionProps> = (props) => {
  const { modalProps, state } = props;
  const descriptionId = createUniqueId();
  const tlsPolicy = createMemo(() => {
    // Match the TLS client's precedence, not the checkbox alone. This is a
    // description of form inputs, never evidence that a peer was verified.
    if (state.formData().fingerprint !== '') {
      return {
        title: 'Certificate fingerprint required',
        description:
          'The fingerprint takes precedence over the checkbox. This endpoint must present the matching certificate, even if its certificate chain is trusted or verification is unchecked.',
      };
    }
    if (state.formData().verifySSL) {
      return {
        title: 'Trusted certificate chain required',
        description:
          'With no fingerprint set, this endpoint must present a valid, hostname-matching certificate with a chain trusted by the Pulse runtime.',
      };
    }
    return {
      title: 'Certificate identity not verified',
      description:
        'With no fingerprint and verification unchecked, these settings do not authenticate the server. Encryption alone does not protect credentials from an impersonated endpoint.',
    };
  });

  return (
    <>
      <section aria-label="TLS trust">
        <SectionHeader title="SSL settings" size="sm" class="mb-4" titleClass="text-base-content" />
        <div class="space-y-3">
          <label class="flex min-h-11 items-center gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().verifySSL}
              onChange={(event) => state.updateField('verifySSL', event.currentTarget.checked)}
              aria-describedby={`${descriptionId}-tls-policy`}
              class={formCheckbox}
            />
            Verify SSL certificate
          </label>

          <div class={formField}>
            <label for={`${descriptionId}-ssl-fingerprint`} class={labelClass()}>
              SSL Fingerprint (optional)
            </label>
            <input
              id={`${descriptionId}-ssl-fingerprint`}
              type="text"
              value={state.formData().fingerprint}
              onInput={(event) => state.updateField('fingerprint', event.currentTarget.value)}
              aria-describedby={`${descriptionId}-fingerprint-help ${descriptionId}-tls-policy`}
              placeholder="AA:BB:CC:DD:EE:FF:..."
              class={controlClass('font-mono')}
            />
            <p id={`${descriptionId}-fingerprint-help`} class={formHelpText}>
              Pins the SHA-256 fingerprint of this endpoint's certificate instead of checking its
              certificate chain. Certificate renewal can change the fingerprint, including with ACME
              or Let's Encrypt. Independently verify the replacement through an already-trusted
              administration channel before updating a pin. Do not disable verification or clear a
              pin to bypass an error.
            </p>
          </div>
          <InlineNotice
            tone={
              state.formData().fingerprint === '' && !state.formData().verifySSL ? 'danger' : 'info'
            }
            role="status"
            aria-label="Form TLS policy"
            aria-live="polite"
            aria-atomic="true"
          >
            <p class="font-medium">{tlsPolicy().title}</p>
            <p id={`${descriptionId}-tls-policy`}>{tlsPolicy().description}</p>
          </InlineNotice>
          <p class={formHelpText}>
            Form settings only, not a connection-test result.
            <Show when={modalProps.nodeType === 'pve'}>
              {' '}
              Cluster members may have their own saved fingerprints.
            </Show>
          </p>
        </div>
      </section>

      {/* For PVE the Collection scope section below carries both the
          coverage description and the per-collector toggles, so the
          standalone Monitoring coverage paragraph would be redundant. PBS
          and PMG keep the standalone paragraph because they have no toggles. */}
      <Show when={modalProps.nodeType !== 'pve'}>
        <div>
          <SectionHeader
            title="Monitoring coverage"
            size="sm"
            class="mb-2"
            titleClass="text-base-content"
          />
          <p class="text-sm text-muted">{getNodeMonitoringCoverageCopy(modalProps.nodeType)}</p>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pve'}>
        <section aria-label="Collection scope" class="space-y-3">
          <SectionHeader
            title="Collection scope"
            size="sm"
            class="mb-1"
            titleClass="text-base-content"
          />
          <p class="text-sm text-muted">
            Choose which resources Pulse collects from this Proxmox VE connection. Guest details
            depend on guest-agent availability and permissions. Some readings can be unavailable or
            temporarily deferred. Disable collectors you do not need to reduce API load or trim
            workload noise.
          </p>
          <InlineNotice tone="warning" role="note" aria-label="Guest-agent backup precaution">
            <span id={`${descriptionId}-backup-precaution`}>
              Guest-agent reads share the channel used by freeze-enabled backups. If a guest becomes
              unresponsive around a backup, stop Pulse monitoring and leave it stopped until
              independent checks confirm thaw and writes to every filesystem covered by the backup.
              Do not run live diagnostics or restart the guest agent during the incident. An OK
              backup or a running VM does not prove recovery.
            </span>
          </InlineNotice>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              aria-labelledby={`${descriptionId}-vms-label`}
              aria-describedby={`${descriptionId}-vms-help ${descriptionId}-backup-precaution`}
              checked={state.formData().monitorVMs}
              onChange={(event) => state.updateField('monitorVMs', event.currentTarget.checked)}
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div id={`${descriptionId}-vms-label`}>Virtual machines</div>
              <p id={`${descriptionId}-vms-help`} class="text-xs text-muted mt-1">
                VM state and resource usage. Guest memory and filesystem details depend on available
                guest-agent readings.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              aria-labelledby={`${descriptionId}-containers-label`}
              aria-describedby={`${descriptionId}-containers-help`}
              checked={state.formData().monitorContainers}
              onChange={(event) =>
                state.updateField('monitorContainers', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div id={`${descriptionId}-containers-label`}>Containers</div>
              <p id={`${descriptionId}-containers-help`} class="text-xs text-muted mt-1">
                LXC container state and resource usage.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              aria-labelledby={`${descriptionId}-storage-label`}
              aria-describedby={`${descriptionId}-storage-help`}
              checked={state.formData().monitorStorage}
              onChange={(event) => state.updateField('monitorStorage', event.currentTarget.checked)}
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div id={`${descriptionId}-storage-label`}>Storage</div>
              <p id={`${descriptionId}-storage-help`} class="text-xs text-muted mt-1">
                Per-storage capacity, status, and content type breakdown.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              aria-labelledby={`${descriptionId}-backups-label`}
              aria-describedby={`${descriptionId}-backups-help`}
              checked={state.formData().monitorBackups}
              onChange={(event) => state.updateField('monitorBackups', event.currentTarget.checked)}
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div id={`${descriptionId}-backups-label`}>Backups</div>
              <p id={`${descriptionId}-backups-help`} class="text-xs text-muted mt-1">
                vzdump schedules and recent backup activity.
              </p>
            </div>
          </label>
        </section>

        <div class="space-y-4">
          <SectionHeader
            title="Advanced monitoring"
            size="sm"
            class="mb-3"
            titleClass="text-base-content"
          />
          <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-xs">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="font-medium text-base-content">Monitor physical disk health (SMART)</p>
                <p id={`${descriptionId}-physical-disks-help`} class="mt-1 text-xs text-muted">
                  This will spin up idle HDDs. Leave disabled if you rely on drive standby.
                </p>
              </div>
              <TogglePrimitive
                ariaDescribedBy={`${descriptionId}-physical-disks-help`}
                checked={state.formData().monitorPhysicalDisks}
                onChange={(event) =>
                  state.updateField('monitorPhysicalDisks', event.currentTarget.checked)
                }
                ariaLabel={
                  state.formData().monitorPhysicalDisks
                    ? 'Disable physical disk monitoring'
                    : 'Enable physical disk monitoring'
                }
              />
            </div>
            <Show when={state.formData().monitorPhysicalDisks}>
              <div class="mt-3 flex items-center gap-2 border-t border-border pt-3">
                <span class="text-xs text-muted">Poll every</span>
                <FormSelect
                  label="Physical disk health polling interval"
                  labelClass="sr-only"
                  fieldBaseClass="contents"
                  selectBaseClass="rounded-sm border bg-surface px-2 py-1 text-xs text-base-content"
                  value={state.formData().physicalDiskPollingMinutes}
                  onChange={(event) =>
                    state.updateField(
                      'physicalDiskPollingMinutes',
                      parseInt(event.currentTarget.value, 10),
                    )
                  }
                >
                  <option value={5}>5 minutes</option>
                  <option value={15}>15 minutes</option>
                  <option value={30}>30 minutes</option>
                  <option value={60}>1 hour</option>
                </FormSelect>
              </div>
            </Show>
          </div>

          <Show when={state.showTemperatureMonitoringSection()}>
            <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-xs">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-medium text-base-content">Temperature monitoring</p>
                  <p class="mt-1 text-xs text-muted">
                    Uses the Pulse sensors key or proxy to read CPU/NVMe temperatures for this node.
                    Disable if you don't need temperature data or haven't deployed the proxy yet.
                  </p>
                </div>
                <TogglePrimitive
                  checked={state.temperatureMonitoringEnabledValue()}
                  onChange={(event) => {
                    modalProps.onToggleTemperatureMonitoring?.(event.currentTarget.checked);
                  }}
                  disabled={
                    modalProps.savingTemperatureSetting || modalProps.temperatureMonitoringLocked
                  }
                  ariaLabel={
                    state.temperatureMonitoringEnabledValue()
                      ? 'Disable temperature monitoring'
                      : 'Enable temperature monitoring'
                  }
                />
              </div>
              <Show when={!state.temperatureMonitoringEnabledValue()}>
                <p class="mt-3 rounded-sm border border-blue-200 bg-blue-50 p-2 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900/25 dark:text-blue-200">
                  Pulse will skip SSH temperature polling for this node. Existing monitoring
                  readings will stop refreshing.
                </p>
              </Show>
              <Show when={modalProps.temperatureMonitoringLocked}>
                <p class="mt-3 rounded-sm border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900/25 dark:text-amber-200">
                  {getTemperatureMonitoringLockedCopy()}
                </p>
              </Show>
            </div>
          </Show>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pbs'}>
        <div class="space-y-3">
          <SectionHeader
            title="Collection scope"
            size="sm"
            class="mb-1"
            titleClass="text-base-content"
          />
          <p class="text-xs text-muted">
            Control which Proxmox Backup Server surfaces Pulse ingests. Disable collectors you don't
            need.
          </p>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorDatastores}
              onChange={(event) =>
                state.updateField('monitorDatastores', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Datastores</div>
              <p class="text-xs text-muted mt-1">
                Datastore capacity, usage, and deduplication status.
              </p>
            </div>
          </label>

          <Show when={state.formData().monitorDatastores}>
            <div class={formField}>
              <label for="pbs-excluded-datastores" class={labelClass()}>
                Excluded datastores (optional)
              </label>
              <input
                id="pbs-excluded-datastores"
                type="text"
                value={state.formData().excludeDatastores}
                onInput={(event) =>
                  state.updateField('excludeDatastores', event.currentTarget.value)
                }
                placeholder="exthdd1500gb, removable-*"
                class={controlClass('font-mono')}
              />
              <p class={formHelpText}>
                Comma-separated names or patterns. Use <code>name*</code>, <code>*name</code>, or{' '}
                <code>*name*</code>. Pulse will not query matching removable or unmounted
                datastores.
              </p>
            </div>
          </Show>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorSyncJobs}
              onChange={(event) =>
                state.updateField('monitorSyncJobs', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Sync jobs</div>
              <p class="text-xs text-muted mt-1">Remote sync schedules and their last-run state.</p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorVerifyJobs}
              onChange={(event) =>
                state.updateField('monitorVerifyJobs', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Verify jobs</div>
              <p class="text-xs text-muted mt-1">Backup verification schedules and outcomes.</p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorPruneJobs}
              onChange={(event) =>
                state.updateField('monitorPruneJobs', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Prune jobs</div>
              <p class="text-xs text-muted mt-1">
                Retention policy runs and the snapshots they remove.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorGarbageJobs}
              onChange={(event) =>
                state.updateField('monitorGarbageJobs', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Garbage collection</div>
              <p class="text-xs text-muted mt-1">
                GC runs that reclaim orphaned chunks from the datastore.
              </p>
            </div>
          </label>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pmg'}>
        <div class="space-y-3">
          <SectionHeader
            title="Collection scope"
            size="sm"
            class="mb-1"
            titleClass="text-base-content"
          />
          <p class="text-xs text-muted">
            Control which PMG data sets Pulse ingests. Disable individual collectors if you want to
            limit API usage.
          </p>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorMailStats}
              onChange={(event) =>
                state.updateField('monitorMailStats', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Mail statistics &amp; trends</div>
              <p class="text-xs text-muted mt-1">
                Total mail volume, inbound/outbound breakdown, spam and virus counts.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorQueues}
              onChange={(event) => state.updateField('monitorQueues', event.currentTarget.checked)}
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Queue health insights</div>
              <p class="text-xs text-muted mt-1">
                Track Postfix queue depth and rejection trends to spot delivery bottlenecks.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorQuarantine}
              onChange={(event) =>
                state.updateField('monitorQuarantine', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Quarantine totals</div>
              <p class="text-xs text-muted mt-1">
                Mirror PMG quarantine sizes for spam, virus, and attachment buckets.
              </p>
            </div>
          </label>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorDomainStats}
              onChange={(event) =>
                state.updateField('monitorDomainStats', event.currentTarget.checked)
              }
              class={formCheckbox + ' mt-0.5'}
            />
            <div>
              <div>Domain-level statistics</div>
              <p class="text-xs text-muted mt-1">
                Gather per-domain metrics for deeper mail routing analysis.
              </p>
            </div>
          </label>
        </div>
      </Show>
    </>
  );
};
