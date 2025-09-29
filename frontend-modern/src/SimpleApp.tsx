import { createSignal, onMount } from 'solid-js';

export default function SimpleApp() {
  const [status, setStatus] = createSignal('Initializing...');
  const [data, setData] = createSignal<any>(null);
  const [wsStatus, setWsStatus] = createSignal('Not connected');

  onMount(() => {
    setStatus('Testing API connection...');

    // Test API
    fetch('/api/health')
      .then((res) => {
        setStatus(`API Status: ${res.status}`);
        return res.json();
      })
      .then((d) => {
        setData(d);
        setStatus('API Connected! Testing WebSocket...');

        // Test WebSocket
        const ws = new WebSocket(`ws://${window.location.host}/ws`);

        ws.onopen = () => {
          setWsStatus('WebSocket CONNECTED');
          setStatus('Everything working!');
        };

        ws.onerror = (e) => {
          setWsStatus('WebSocket ERROR');
          console.error('WS Error:', e);
        };

        ws.onclose = (e) => {
          setWsStatus(`WebSocket CLOSED: ${e.code} - ${e.reason}`);
        };

        ws.onmessage = (e) => {
          setWsStatus('WebSocket receiving data!');
          try {
            const msg = JSON.parse(e.data);
            setData((prev) => ({ ...prev, lastMessage: msg.type }));
          } catch (err) {
            console.error('Parse error:', err);
          }
        };
      })
      .catch((err) => {
        setStatus(`API Error: ${err}`);
        console.error(err);
      });
  });

  return (
    <div style={{ padding: '20px', 'font-family': 'monospace' }}>
      <h1>Pulse System Test</h1>
      <hr />
      <p>
        <strong>Status:</strong> {status()}
      </p>
      <p>
        <strong>WebSocket:</strong> {wsStatus()}
      </p>
      <hr />
      <h3>API Data:</h3>
      <pre>{JSON.stringify(data(), null, 2)}</pre>
    </div>
  );
}
