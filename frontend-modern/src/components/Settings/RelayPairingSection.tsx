import { Component, For, Show } from 'solid-js';
import type { OnboardingQRResponse } from '@/api/onboarding';
import { Card } from '@/components/shared/Card';
import { formField, formHelpText, labelClass } from '@/components/shared/Form';
import {
  getRelayDiagnosticClass,
  RELAY_CODE_BLOCK_CLASS,
  RELAY_DIAGNOSTICS_TITLE_CLASS,
  RELAY_DIAGNOSTICS_WRAP_CLASS,
  RELAY_PRIMARY_BUTTON_CLASS,
  RELAY_QR_IMAGE_CLASS,
  RELAY_SECONDARY_BUTTON_CLASS,
} from '@/utils/relayPresentation';

interface RelayPairingSectionProps {
  canManage: boolean;
  pairingLoading: boolean;
  pairingPayload: OnboardingQRResponse | null;
  pairingQRCode: string | null;
  saving: boolean;
  showPairing: boolean;
  onCopyPairingPayload: () => void;
  onHidePairing: () => void;
  onPairNewDevice: () => void;
}

export const RelayPairingSection: Component<RelayPairingSectionProps> = (props) => (
  <div class={formField}>
    <label class={labelClass()}>Pair Mobile Device</label>
    <Card tone="muted" padding="md">
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <button
            class={RELAY_PRIMARY_BUTTON_CLASS}
            onClick={props.onPairNewDevice}
            disabled={!props.canManage || props.saving || props.pairingLoading}
          >
            {props.pairingLoading
              ? 'Generating QR code...'
              : props.showPairing
                ? 'Refresh QR Code'
                : 'Pair New Device'}
          </button>
          <Show when={props.showPairing && props.pairingPayload}>
            <>
              <button
                class={RELAY_SECONDARY_BUTTON_CLASS}
                onClick={props.onCopyPairingPayload}
                disabled={!props.canManage || props.pairingLoading}
              >
                Copy Payload
              </button>
              <button
                class={RELAY_SECONDARY_BUTTON_CLASS}
                onClick={props.onHidePairing}
                disabled={!props.canManage || props.pairingLoading}
              >
                Hide QR
              </button>
            </>
          </Show>
        </div>

        <p class={formHelpText}>
          Generate a QR code that provisions a dedicated Pulse Mobile relay access credential.
        </p>

        <Show when={props.showPairing}>
          <div class="space-y-3">
            <Show when={props.pairingLoading}>
              <p class="text-sm text-muted">Preparing pairing payload...</p>
            </Show>

            <Show when={!props.pairingLoading && props.pairingQRCode}>
              <img
                src={props.pairingQRCode!}
                alt="Pulse mobile pairing QR code"
                width="256"
                height="256"
                class={RELAY_QR_IMAGE_CLASS}
              />
            </Show>

            <Show when={props.pairingPayload?.deep_link}>
              <code class={RELAY_CODE_BLOCK_CLASS}>{props.pairingPayload!.deep_link}</code>
            </Show>

            <Show when={(props.pairingPayload?.diagnostics?.length ?? 0) > 0}>
              <div class={RELAY_DIAGNOSTICS_WRAP_CLASS}>
                <p class={RELAY_DIAGNOSTICS_TITLE_CLASS}>Diagnostics</p>
                <For each={props.pairingPayload?.diagnostics ?? []}>
                  {(diagnostic) => (
                    <div class={getRelayDiagnosticClass(diagnostic.severity)}>
                      <p class="font-medium">{diagnostic.message}</p>
                      <p class="mt-0.5 font-mono">
                        {diagnostic.code}
                        <Show when={diagnostic.field}> | field: {diagnostic.field}</Show>
                      </p>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </Show>
      </div>
    </Card>
  </div>
);
