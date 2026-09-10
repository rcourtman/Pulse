export const historyFixture = `
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { AlertsAPI } from '/src/api/alerts';
import { useAlertHistoryState } from '/src/features/alerts/useAlertHistoryState';
import { AlertHistoryAdministrationCard } from '/src/features/alerts/AlertHistoryAdministrationCard';
import '/src/index.css';
const row = {id:'old',type:'cpu',level:'warning',startTime:new Date().toISOString(),lastSeen:new Date().toISOString(),resourceId:'host',resourceName:'Fixture host',message:'Old history',acknowledged:false};
let calls = 0;
AlertsAPI.getHistory = () => ++calls === 1 ? Promise.resolve([row]) : new Promise(resolve => { window.finishHistory = () => resolve([row]); });
AlertsAPI.clearHistory = async () => {};
function Fixture() {
 const state = useAlertHistoryState({activeAlerts:()=>({}),getResource:()=>undefined,allResources:()=>[]});
 window.snapshot = () => ({rows:state.alertHistory().length,loading:state.loading()});
 return <main class="p-4">
 <button onClick={()=>state.setTimeFilter('24h')}>Refresh range</button>
 <p>Rows: {state.alertHistory().length}</p>
 <AlertHistoryAdministrationCard state={state}/>
 </main>;
}
render(() => <Router><Route path="*" component={Fixture}/></Router>, document.getElementById('root'));
`;
