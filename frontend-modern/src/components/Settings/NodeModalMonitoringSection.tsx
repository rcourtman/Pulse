import { Component, Show } from 'solid-js';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { controlClass, formCheckbox, formField, formHelpText, labelClass } from '@/components/shared/Form';
import { TogglePrimitive } from '@/components/shared/Toggle';
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

  return (
    <>
      <div>
        <SectionHeader title="SSL settings" size="sm" class="mb-4" titleClass="text-base-content" />
        <div class="space-y-3">
          <label class="flex items-center gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().verifySSL}
              onChange={(event) => state.updateField('verifySSL', event.currentTarget.checked)}
              class={formCheckbox}
            />
            Verify SSL certificate
          </label>

          <div class={formField}>
            <label class={labelClass()}>SSL Fingerprint (optional)</label>
            <input
              type="text"
              value={state.formData().fingerprint}
              onInput={(event) => state.updateField('fingerprint', event.currentTarget.value)}
              placeholder="AA:BB:CC:DD:EE:FF:..."
              class={controlClass('font-mono')}
            />
            <p class={formHelpText}>
              Useful when connecting to servers with self-signed certificates.
            </p>
          </div>
        </div>
      </div>

      <div>
        <SectionHeader
          title="Monitoring coverage"
          size="sm"
          class="mb-2"
          titleClass="text-base-content"
        />
        <p class="text-sm text-muted">{getNodeMonitoringCoverageCopy(modalProps.nodeType)}</p>
      </div>

      <Show when={modalProps.nodeType === 'pve'}>
        <div class="space-y-4">
          <SectionHeader
            title="Advanced monitoring"
            size="sm"
            class="mb-3"
            titleClass="text-base-content"
          />
          <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-sm">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="font-medium text-base-content">Monitor physical disk health (SMART)</p>
                <p class="mt-1 text-xs text-muted">
                  This will spin up idle HDDs; leave disabled if you rely on drive standby.
                </p>
              </div>
              <TogglePrimitive
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
                <label class="text-xs text-muted">Poll every</label>
                <select
                  class="rounded border bg-surface px-2 py-1 text-xs text-base-content"
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
                </select>
              </div>
            </Show>
          </div>

          <Show when={state.showTemperatureMonitoringSection()}>
            <div class="rounded-md border border-border bg-surface p-3 text-sm shadow-sm">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="font-medium text-base-content">Temperature monitoring</p>
                  <p class="mt-1 text-xs text-muted">
                    Uses the Pulse sensors key or proxy to read CPU/NVMe temperatures for this
                    node. Disable if you don't need temperature data or haven't deployed the proxy
                    yet.
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
                <p class="mt-3 rounded border border-blue-200 bg-blue-50 p-2 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200">
                  Pulse will skip SSH temperature polling for this node. Existing dashboard
                  readings will stop refreshing.
                </p>
              </Show>
              <Show when={modalProps.temperatureMonitoringLocked}>
                <p class="mt-3 rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                  {getTemperatureMonitoringLockedCopy()}
                </p>
              </Show>
            </div>
          </Show>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pmg'}>
        <div class="space-y-3">
          <SectionHeader
            title="Data collection"
            size="sm"
            class="mb-1"
            titleClass="text-base-content"
          />
          <p class="text-xs text-muted">
            Control which PMG data sets Pulse ingests. Disable individual collectors if you want
            to limit API usage.
          </p>

          <label class="flex items-start gap-2 text-sm text-base-content">
            <input
              type="checkbox"
              checked={state.formData().monitorMailStats}
              onChange={(event) => state.updateField('monitorMailStats', event.currentTarget.checked)}
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
