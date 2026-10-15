// Actual setup producer and consuming sections; APIs/clipboard use safe fixtures.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { useNodeModalState } from '../src/components/Settings/useNodeModalState';
import { NodeModalBasicInfoSection } from '../src/components/Settings/NodeModalBasicInfoSection';
import { NodeModalAuthenticationSection } from '../src/components/Settings/NodeModalAuthenticationSection';
import { TokenRevealDialog } from '../src/components/TokenRevealDialog';
import { ToastContainer } from '../src/components/Toast/Toast';
import '../src/index.css';

const Fixture = () => {
  const type = new URLSearchParams(location.search).get('type') === 'pbs' ? 'pbs' : 'pve';
  const [open, setOpen] = createSignal(true);
  const props = {
    get isOpen() {
      return open();
    },
    nodeType: type,
    prefillNode: {
      type,
      name: 'Synthetic alpha',
      host: `https://${type}.example.invalid`,
      verifySSL: true,
    },
    onClose: () => setOpen(false),
    onSave: () => {},
  };
  const state = useNodeModalState(props);
  return (
    <>
      <main class="mx-auto max-w-3xl space-y-4 bg-background p-4 text-base-content">
        <h1 class="text-lg font-semibold">Connect a Proxmox server</h1>
        <p class="text-sm text-muted">
          Offline synthetic setup; no credentials, hosts or installation.
        </p>
        <button
          type="button"
          class="min-h-11 rounded-md border border-border px-4"
          onClick={() => setOpen(!open())}
        >
          {open() ? 'Leave setup' : 'Reopen setup'}
        </button>
        <Show when={open()}>
          <NodeModalBasicInfoSection modalProps={props} state={state} />
          <NodeModalAuthenticationSection modalProps={props} state={state} />
        </Show>
        <output data-testid="cache-state">
          {state.quickSetupCommandReady() ? 'ready' : 'empty'}
        </output>
      </main>
      <TokenRevealDialog />
      <ToastContainer />
    </>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
