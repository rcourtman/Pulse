// Shared synthetic fixture mounting the production incident hook and panel.
export const fixture = `
import { render } from 'solid-js/web';
import { createSignal, Show } from 'solid-js';
import { AlertsAPI } from '/src/api/alerts';
import { notificationStore } from '/src/stores/notifications';
import { useAlertResourceIncidentsState } from '/src/features/alerts/useAlertResourceIncidentsState';
import { AlertResourceIncidentsPanel } from '/src/features/alerts/AlertResourceIncidentsPanel';
import '/src/index.css';
const pending = [];
const [errors, setErrors] = createSignal(0);
notificationStore.error = () => setErrors(n => n + 1);
AlertsAPI.getIncidentsForResource = () => new Promise((resolve, reject) => pending.push({resolve, reject}));
window.finish = (i, status) => status === 'error' ? pending[i].reject(new Error('scripted read failure')) : pending[i].resolve(status === 'empty' ? [] : [{id:status, message:status, level:'warning', status:'open', openedAt:new Date().toISOString(), events:[]}]);
window.count = () => pending.length;
function Panel() {
  const s = useAlertResourceIncidentsState();
  window.snapshot = () => ({incidents:s.resourceIncidents(), loading:s.resourceIncidentLoading(), error:s.resourceIncidentError()});
  return <section><button onClick={() => s.openResourceIncidentPanel('host','Host','row')}>Open row</button><button onClick={s.refreshResourceIncidentPanel}>Overlap refresh</button><button onClick={s.resetResourceIncidentsState}>Reset</button><output style="display:block;overflow-wrap:anywhere" data-testid="state">{JSON.stringify(window.snapshot())}</output><AlertResourceIncidentsPanel state={s}/></section>;
}
function Fixture() {
  const [mounted, setMounted] = createSignal(true);
  return <main class="p-4"><h1>Incident lifecycle qualification fixture</h1><button onClick={() => setMounted(false)}>Unmount</button><output data-testid="errors">{errors()}</output><Show when={mounted()}><Panel/></Show></main>;
}
render(() => <Fixture/>, document.getElementById('root'));
`;
