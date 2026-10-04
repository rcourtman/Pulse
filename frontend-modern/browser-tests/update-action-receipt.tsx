// Exercise the production Docker update button with an unresolved governed action.
import { render } from 'solid-js/web';
import { UpdateButton } from '../src/components/shared/ContainerUpdateBadge';
import { markContainerQueued } from '../src/stores/containerUpdates';
import { markSystemSettingsLoadedWithDefaults } from '../src/stores/systemSettings';
import '../src/index.css';

markSystemSettingsLoadedWithDefaults();
markContainerQueued('agent-fixture-1', 'container-fixture-1', 'action-fixture-1');

render(
  () => (
    <main class="mx-auto max-w-lg p-6">
      <h1 class="mb-4 text-lg font-semibold">Container update receipt</h1>
      <UpdateButton
        agentId="agent-fixture-1"
        containerId="container-fixture-1"
        containerName="edge"
        resourceId="docker:container:edge"
        updateStatus={{
          updateAvailable: false,
          currentDigest: 'sha256:current',
          lastChecked: Date.now(),
        }}
      />
    </main>
  ),
  document.getElementById('root')!,
);
