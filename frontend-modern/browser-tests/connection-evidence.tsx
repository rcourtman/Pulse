import { Show, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ConnectionEditor } from '../src/components/Settings/ConnectionEditor/ConnectionEditor';
import { TrueNASCredentialSlot } from '../src/components/Settings/ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from '../src/components/Settings/ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import { useTrueNASSettingsPanelState } from '../src/components/Settings/useTrueNASSettingsPanelState';
import { useVMwareSettingsPanelState } from '../src/components/Settings/useVMwareSettingsPanelState';
import { ToastContainer } from '../src/components/Toast/Toast';
import '../src/index.css';

// Real consumers, persistent Settings-owned hooks, API clients, notifications
// and production CSS. The driver supplies only synthetic HTTP responses. No
// platform, credential, saving, installation or live monitoring is exercised.
const type = new URLSearchParams(location.search).get('platform');
if (type !== 'truenas' && type !== 'vmware') throw new Error('Expected platform');
render(() => {
  const [visible, setVisible] = createSignal(true);
  const [targetIndex, setTargetIndex] = createSignal(0);
  const nas = useTrueNASSettingsPanelState({ canLoad: () => type === 'truenas' });
  const vmware = useVMwareSettingsPanelState({ canLoad: () => type === 'vmware' });
  const leave = () => setVisible(false);
  return (
    <>
      <main class="mx-auto max-w-5xl bg-background p-4 text-base-content">
        <h1 class="mb-4 text-lg font-semibold">Offline {type} connection editor</h1>
        <p class="mb-4 text-sm text-muted">
          Synthetic appliance and credentials; no live monitoring.
        </p>
        <button
          class="mb-4 min-h-11 rounded-md border border-border px-4"
          onClick={() => setVisible(!visible())}
        >
          {visible() ? 'Leave editor' : 'Reopen editor'}
        </button>
        <button
          class="mb-4 ml-2 min-h-11 rounded-md border border-border px-4"
          onClick={() => setTargetIndex((i) => 1 - i)}
        >
          {targetIndex() === 0 ? 'Switch to beta' : 'Switch to alpha'}
        </button>
        <Show
          when={type === 'truenas' ? nas.connections().length > 0 : vmware.connections().length > 0}
        >
          <Show when={visible()}>
            <ConnectionEditor
              mode="edit"
              initialType={type}
              onClose={leave}
              onSaved={leave}
              renderCredentialSlot={() =>
                type === 'truenas' ? (
                  <TrueNASCredentialSlot
                    state={nas}
                    editingConnection={nas.connections()[targetIndex()]}
                    onCancel={leave}
                    onSaved={leave}
                  />
                ) : (
                  <VMwareCredentialSlot
                    state={vmware}
                    editingConnection={vmware.connections()[targetIndex()]}
                    onCancel={leave}
                    onSaved={leave}
                  />
                )
              }
            />
          </Show>
        </Show>
      </main>
      <ToastContainer />
    </>
  );
}, document.getElementById('root')!);
