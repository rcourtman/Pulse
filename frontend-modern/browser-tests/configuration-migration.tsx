// Actual transfer dialogs and flow, plus the production Docs renderer/router.
// The browser driver supplies only synthetic API responses; no server or
// operational credentials are involved in this copy/interaction proof.
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { BackupTransferDialogs } from '../src/components/Settings/BackupTransferDialogs';
import { useBackupTransferFlow } from '../src/components/Settings/useBackupTransferFlow';
import Docs from '../src/pages/Docs';
import '../src/index.css';

function Transfer() {
  const securityStatus = () => null;
  const flow = useBackupTransferFlow({ securityStatus });
  return (
    <main class="p-6">
      <h1>Configuration transfer</h1>
      <button onClick={() => flow.setShowExportDialog(true)}>Create Backup</button>
      <button onClick={() => flow.setShowImportDialog(true)}>Restore Configuration</button>
      <BackupTransferDialogs {...flow} securityStatus={securityStatus} />
    </main>
  );
}

render(
  () => (
    <Router>
      <Route path="/docs/*docPath" component={Docs} />
      <Route path="*" component={Transfer} />
    </Router>
  ),
  document.getElementById('root')!,
);
