// Real setup owners and consumers. Synthetic GETs only; no command or token mint.
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { Show } from 'solid-js';
import { InfrastructureInstallerSection } from '../src/components/Settings/InfrastructureInstallerSection';
import { InfrastructureOperationsStateProvider } from '../src/components/Settings/useInfrastructureOperationsState';
import { NodeModalBasicInfoSection } from '../src/components/Settings/NodeModalBasicInfoSection';
import { NodeModalAuthenticationSection } from '../src/components/Settings/NodeModalAuthenticationSection';
import { useNodeModalState } from '../src/components/Settings/useNodeModalState';
import '../src/index.css';

function NodeSetup() {
  const props = {
    isOpen: true,
    nodeType: 'pve' as const,
    prefillNode: {
      type: 'pve' as const,
      name: 'Synthetic Proxmox node',
      host: 'https://pve.example.invalid',
      verifySSL: true,
    },
    setupHandoffDisabled: () => true,
    setupHandoffDisabledReason: 'Synthetic preview: command generation is disabled.',
    onClose: () => {},
    onSave: () => {},
  };
  const state = useNodeModalState(props);
  return (
    <>
      <h1 class="text-xl font-semibold">Connect Proxmox VE</h1>
      <NodeModalBasicInfoSection modalProps={props} state={state} />
      <NodeModalAuthenticationSection modalProps={props} state={state} />
    </>
  );
}

render(
  () => (
    <Router>
      <Route
        path="/*"
        component={() => {
          const query = new URLSearchParams(location.search);
          return (
            <main class="mx-auto max-w-4xl space-y-4 p-3 text-base-content">
              <Show
                when={query.get('surface') === 'node'}
                fallback={
                  <InfrastructureOperationsStateProvider embedded>
                    <InfrastructureInstallerSection
                      focus={query.get('focus') === 'docker' ? 'docker' : 'agent'}
                    />
                  </InfrastructureOperationsStateProvider>
                }
              >
                <NodeSetup />
              </Show>
            </main>
          );
        }}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
