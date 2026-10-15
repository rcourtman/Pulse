// Real presentation consumers with synthetic, offline observations only.
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { BackupStatusCell } from '../src/components/Workloads/GuestRowCells';
import { AddressProbeStep } from '../src/components/Settings/ConnectionEditor/AddressProbeStep';
import type { ConnectionEditorState } from '../src/components/Settings/ConnectionEditor/useConnectionEditor';
import '../src/index.css';
const state = {
  address: () => 'intended.example.invalid',
  setAddress: () => {},
  phase: () => 'error',
  errorMessage: () => 'HTTP 403 Forbidden',
  candidates: () => [],
  probedMs: () => 0,
} as unknown as ConnectionEditorState;
function Fixture() {
  return (
    <main class="mx-auto max-w-xl space-y-6 p-4 text-base-content">
      <h1 class="text-xl font-semibold">Backup evidence and connection guidance</h1>
      <BackupStatusCell
        lastBackup={Date.parse('2026-10-12T00:30:00Z')}
        guestName="synthetic-guest"
        guestReadEvidence={{
          type: 'qemu',
          platformType: 'proxmox-pve',
          diskStatusReason: 'agent-redirect',
          guestAgentStatus: 'unavailable',
        }}
      />
      <AddressProbeStep state={state} onSelectCandidate={() => {}} />
    </main>
  );
}
render(
  () => (
    <Router>
      <Route path="/*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
