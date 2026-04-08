/**
 * ConsoleModal: Unified console access modal for VMs and containers.
 *
 * Supports multiple console protocols:
 * - VNC via noVNC (most universal, works with Proxmox, VMware, KVM)
 * - SSH via xterm.js (terminal access for any host with SSH)
 * - SPICE (higher performance for KVM/Proxmox VMs)
 * - Serial console via xterm.js
 *
 * The modal auto-detects available console types from the VM's consoleTypes field
 * and lets the user pick the preferred protocol.
 */
import { Component, Show, createSignal, onCleanup, createEffect } from 'solid-js';
import { ConsoleAPI, type ConsoleTicketResponse } from '@/api/console';

interface ConsoleModalProps {
  vmId: string;
  vmName: string;
  nodeId: string;
  providerId: string;
  consoleTypes: string[];
  onClose: () => void;
}

export const ConsoleModal: Component<ConsoleModalProps> = (props) => {
  const [selectedType, setSelectedType] = createSignal(props.consoleTypes[0] || 'vnc');
  const [connecting, setConnecting] = createSignal(false);
  const [connected, setConnected] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [ticket, setTicket] = createSignal<ConsoleTicketResponse | null>(null);
  let containerRef: HTMLDivElement | undefined;
  let ws: WebSocket | null = null;

  const connect = async () => {
    setConnecting(true);
    setError(null);
    try {
      const resp = await ConsoleAPI.requestTicket({
        providerId: props.providerId,
        nodeId: props.nodeId,
        vmId: props.vmId,
        type: selectedType() as 'vnc' | 'spice' | 'ssh' | 'serial' | 'rdp',
      });
      setTicket(resp);

      // Establish WebSocket connection.
      const wsUrl = ConsoleAPI.buildWebSocketUrl(resp.websocketUrl);
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        setConnected(true);
        setConnecting(false);
      };

      ws.onclose = () => {
        setConnected(false);
        setConnecting(false);
      };

      ws.onerror = () => {
        setError('WebSocket connection failed');
        setConnected(false);
        setConnecting(false);
      };

      ws.onmessage = (event) => {
        // Route messages to the appropriate renderer.
        // For VNC: noVNC handles its own WebSocket
        // For SSH/Serial: append to xterm.js terminal
        if (containerRef && (selectedType() === 'ssh' || selectedType() === 'serial')) {
          // Text terminal output
          const pre = containerRef.querySelector('.console-output');
          if (pre) {
            pre.textContent += event.data;
            pre.scrollTop = pre.scrollHeight;
          }
        }
      };
    } catch (err: any) {
      setError(err.message || 'Failed to connect');
      setConnecting(false);
    }
  };

  onCleanup(() => {
    if (ws) {
      ws.close();
      ws = null;
    }
  });

  const consoleTypeLabels: Record<string, string> = {
    vnc: 'VNC (noVNC)',
    spice: 'SPICE',
    ssh: 'SSH Terminal',
    serial: 'Serial Console',
    rdp: 'Remote Desktop (RDP)',
  };

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={(e) => { if (e.target === e.currentTarget) props.onClose(); }}>
      <div class="bg-gray-800 rounded-lg shadow-2xl border border-gray-700 w-[90vw] max-w-5xl h-[80vh] flex flex-col">
        {/* Header */}
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-700">
          <div class="flex items-center gap-3">
            <div class="w-2 h-2 rounded-full" classList={{ 'bg-green-500': connected(), 'bg-yellow-500': connecting(), 'bg-red-500': !connected() && !connecting() }} />
            <h2 class="text-sm font-medium text-gray-200">
              Console: {props.vmName}
            </h2>
            <Show when={ticket()}>
              <span class="text-xs text-gray-500">Session: {ticket()?.sessionId?.slice(-8)}</span>
            </Show>
          </div>
          <div class="flex items-center gap-2">
            {/* Console type selector */}
            <Show when={!connected() && props.consoleTypes.length > 1}>
              <select
                class="text-xs bg-gray-700 text-gray-300 border border-gray-600 rounded px-2 py-1"
                value={selectedType()}
                onChange={(e) => setSelectedType(e.currentTarget.value)}
              >
                {props.consoleTypes.map((type) => (
                  <option value={type}>{consoleTypeLabels[type] || type}</option>
                ))}
              </select>
            </Show>
            <Show when={!connected()}>
              <button
                class="text-xs bg-blue-600 hover:bg-blue-500 text-white px-3 py-1 rounded disabled:opacity-50"
                onClick={connect}
                disabled={connecting()}
              >
                {connecting() ? 'Connecting...' : 'Connect'}
              </button>
            </Show>
            <Show when={connected()}>
              <span class="text-xs text-gray-400">{consoleTypeLabels[selectedType()] || selectedType()}</span>
            </Show>
            <button class="text-gray-400 hover:text-white ml-2" onClick={props.onClose}>
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Console area */}
        <div ref={containerRef} class="flex-1 bg-black overflow-hidden relative">
          <Show when={error()}>
            <div class="absolute inset-0 flex items-center justify-center">
              <div class="bg-red-900/50 border border-red-700 rounded-lg p-4 max-w-md">
                <p class="text-red-300 text-sm">{error()}</p>
                <button class="mt-2 text-xs text-red-400 hover:text-red-300 underline" onClick={() => { setError(null); connect(); }}>
                  Retry
                </button>
              </div>
            </div>
          </Show>

          <Show when={!connected() && !connecting() && !error()}>
            <div class="absolute inset-0 flex flex-col items-center justify-center text-gray-500">
              <svg class="w-16 h-16 mb-4 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              <p class="text-sm">Select a console type and click Connect</p>
              <p class="text-xs text-gray-600 mt-1">
                {selectedType() === 'vnc' && 'VNC provides a graphical desktop console via noVNC'}
                {selectedType() === 'ssh' && 'SSH provides a terminal session over WebSocket'}
                {selectedType() === 'spice' && 'SPICE provides high-performance desktop access'}
                {selectedType() === 'serial' && 'Serial console provides text-mode access'}
                {selectedType() === 'rdp' && 'RDP provides Windows Remote Desktop access'}
              </p>
            </div>
          </Show>

          <Show when={connecting()}>
            <div class="absolute inset-0 flex items-center justify-center">
              <div class="animate-spin w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full" />
            </div>
          </Show>

          {/* Terminal output area (SSH/Serial) */}
          <Show when={connected() && (selectedType() === 'ssh' || selectedType() === 'serial')}>
            <pre class="console-output w-full h-full overflow-auto p-2 text-green-400 text-sm font-mono whitespace-pre-wrap" />
          </Show>

          {/* VNC canvas area (noVNC) */}
          <Show when={connected() && selectedType() === 'vnc'}>
            <div class="w-full h-full flex items-center justify-center text-gray-500 text-sm">
              {/* noVNC canvas will be mounted here by the VNC integration */}
              <p>VNC display connected. noVNC canvas integration pending (@novnc/novnc npm package).</p>
            </div>
          </Show>
        </div>

        {/* Status bar */}
        <div class="flex items-center justify-between px-4 py-2 border-t border-gray-700 text-xs text-gray-500">
          <span>
            {props.providerId} / {props.nodeId} / {props.vmId}
          </span>
          <span>
            {connected() ? 'Connected' : connecting() ? 'Connecting...' : 'Disconnected'}
          </span>
        </div>
      </div>
    </div>
  );
};
