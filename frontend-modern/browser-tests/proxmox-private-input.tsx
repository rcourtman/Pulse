// Real settings state, setup section and existing token dialog; synthetic APIs.
import { createSignal, Show } from 'solid-js';
import type { NodeConfig } from '../src/types/nodes';
import { render } from 'solid-js/web';
import { useNodeModalState } from '../src/components/Settings/useNodeModalState';
import { NodeModalSetupGuideSection } from '../src/components/Settings/NodeModalSetupGuideSection';
import { TokenRevealDialog } from '../src/components/TokenRevealDialog';
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
      name: `${type} fixture`,
      host: `https://${type}.example:${type === 'pbs' ? 8007 : 8006}`,
      verifySSL: true,
    } as NodeConfig,
    onClose() {
      setOpen(false);
    },
    onSave: async () => {},
  };
  const state = useNodeModalState(props);
  return (
    <main class="mx-auto max-w-2xl p-4 bg-base text-base-content">
      <h1 class="text-lg mb-3">Connect a Proxmox server</h1>
      <button
        type="button"
        class="border border-border p-3 mb-3"
        onClick={() => {
          setOpen(!open());
          if (open())
            state.updateField('host', `https://${type}.example:${type === 'pbs' ? 8007 : 8006}`);
        }}
      >
        {open() ? 'Close setup' : 'Reopen setup'}
      </button>
      <Show when={open()}>
        <NodeModalSetupGuideSection modalProps={props} state={state} />
      </Show>
      <output data-testid="cache-state">
        {state.quickSetupCommandReady() ? 'ready' : 'empty'}
      </output>
      <TokenRevealDialog />
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
