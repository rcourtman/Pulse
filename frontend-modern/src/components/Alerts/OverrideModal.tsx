import { createSignal, Show, For, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import { ThresholdSlider } from '@/components/Dashboard/ThresholdSlider';

interface Override {
  id?: string;  // Full guest ID (e.g. "Main-node1-105")
  guestName: string;
  vmid: number;
  type: string;
  node: string;
  instance?: string;
  disabled?: boolean;  // Completely disable alerts for this guest
  thresholds: {
    cpu?: number;
    memory?: number;
    disk?: number;
    diskRead?: number;
    diskWrite?: number;
    networkIn?: number;
    networkOut?: number;
  };
}

interface OverrideModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (override: Override) => void;
  existingOverride?: Override;
  guests: Array<{ id: string; name: string; vmid: number; type: string; node: string; instance: string }>;
}

export function OverrideModal(props: OverrideModalProps) {
  // Initialize state only when modal opens, not on every render
  const [selectedGuest, setSelectedGuest] = createSignal<string>('');
  const [alertsDisabled, setAlertsDisabled] = createSignal(false);
  
  // Store the select element ref
  let selectRef: HTMLSelectElement | undefined;
  const [thresholds, setThresholds] = createSignal({
    cpu: 80,
    memory: 80,
    disk: 80,
    diskRead: 0,
    diskWrite: 0,
    networkIn: 0,
    networkOut: 0
  });
  
  const [enabledMetrics, setEnabledMetrics] = createSignal({
    cpu: false,
    memory: false,
    disk: false,
    diskRead: false,
    diskWrite: false,
    networkIn: false,
    networkOut: false
  });
  
  // Maintain select value when guests change
  createEffect(() => {
    if (selectRef && selectedGuest()) {
      const currentValue = selectedGuest();
      // Use requestAnimationFrame to ensure DOM has updated
      requestAnimationFrame(() => {
        if (selectRef) {
          selectRef.value = currentValue;
        }
      });
    }
  });
  
  // Reset state when modal opens
  createEffect(() => {
    if (props.isOpen) {
      if (props.existingOverride) {
        setSelectedGuest(`${props.existingOverride.vmid}`);
        setAlertsDisabled(props.existingOverride.disabled || false);
        setThresholds({
          cpu: props.existingOverride.thresholds.cpu || 80,
          memory: props.existingOverride.thresholds.memory || 80,
          disk: props.existingOverride.thresholds.disk || 80,
          diskRead: props.existingOverride.thresholds.diskRead || 0,
          diskWrite: props.existingOverride.thresholds.diskWrite || 0,
          networkIn: props.existingOverride.thresholds.networkIn || 0,
          networkOut: props.existingOverride.thresholds.networkOut || 0
        });
        setEnabledMetrics({
          cpu: props.existingOverride.thresholds.cpu !== undefined,
          memory: props.existingOverride.thresholds.memory !== undefined,
          disk: props.existingOverride.thresholds.disk !== undefined,
          diskRead: props.existingOverride.thresholds.diskRead !== undefined,
          diskWrite: props.existingOverride.thresholds.diskWrite !== undefined,
          networkIn: props.existingOverride.thresholds.networkIn !== undefined,
          networkOut: props.existingOverride.thresholds.networkOut !== undefined
        });
      } else {
        // Reset to defaults for new override
        setSelectedGuest('');
        setAlertsDisabled(false);
        setThresholds({
          cpu: 80,
          memory: 80,
          disk: 80,
          diskRead: 0,
          diskWrite: 0,
          networkIn: 0,
          networkOut: 0
        });
        setEnabledMetrics({
          cpu: false,
          memory: false,
          disk: false,
          diskRead: false,
          diskWrite: false,
          networkIn: false,
          networkOut: false
        });
      }
    }
  });
  
  const handleSave = () => {
    const guest = props.guests.find(g => g.vmid.toString() === selectedGuest());
    if (!guest) return;
    
    const enabledThresholds: Override['thresholds'] = {};
    const enabled = enabledMetrics();
    const thresh = thresholds();
    
    if (enabled.cpu && thresh.cpu !== undefined) enabledThresholds.cpu = thresh.cpu;
    if (enabled.memory && thresh.memory !== undefined) enabledThresholds.memory = thresh.memory;
    if (enabled.disk && thresh.disk !== undefined) enabledThresholds.disk = thresh.disk;
    if (enabled.diskRead && thresh.diskRead) enabledThresholds.diskRead = thresh.diskRead;
    if (enabled.diskWrite && thresh.diskWrite) enabledThresholds.diskWrite = thresh.diskWrite;
    if (enabled.networkIn && thresh.networkIn) enabledThresholds.networkIn = thresh.networkIn;
    if (enabled.networkOut && thresh.networkOut) enabledThresholds.networkOut = thresh.networkOut;
    
    props.onSave({
      id: guest.id,  // Pass the full guest ID
      guestName: guest.name,
      vmid: guest.vmid,
      type: guest.type,
      node: guest.node,
      instance: guest.instance,
      disabled: alertsDisabled(),
      thresholds: enabledThresholds
    });
  };
  
  return (
    <Show when={props.isOpen}>
      <Portal>
        <div class="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center p-4">
          <div class="bg-white dark:bg-gray-800 rounded-lg max-w-2xl w-full max-h-[90vh] overflow-hidden">
            {/* Header */}
            <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h2 class="text-lg font-semibold text-gray-800 dark:text-gray-200">
                {props.existingOverride ? 'Edit Guest Override' : 'Add Guest Override'}
              </h2>
            </div>
            
            {/* Content */}
            <div class="p-6 space-y-6 overflow-y-auto max-h-[calc(90vh-8rem)]">
              {/* Guest Selection */}
              <Show when={!props.existingOverride}>
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Select Guest
                  </label>
                  <select
                    class="w-full px-3 py-2 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                    onChange={(e) => {
                      const value = e.currentTarget.value;
                      setSelectedGuest(value);
                    }}
                    ref={(el) => {
                      selectRef = el;
                    }}
                  >
                    <option value="">Choose a guest...</option>
                    <For each={props.guests}>
                      {(guest) => (
                        <option value={guest.vmid.toString()}>
                          {guest.name} ({guest.vmid}) - {guest.type} on {guest.node}
                        </option>
                      )}
                    </For>
                  </select>
                </div>
              </Show>
              
              {/* Disable Alerts Option */}
              <div class="flex items-center gap-3 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg border border-red-200 dark:border-red-800">
                <input
                  type="checkbox"
                  id="disable-alerts"
                  checked={alertsDisabled()}
                  onChange={(e) => setAlertsDisabled(e.currentTarget.checked)}
                  class="rounded border-gray-300 dark:border-gray-600 text-red-600 focus:ring-red-500"
                />
                <label for="disable-alerts" class="flex-1">
                  <span class="text-sm font-medium text-red-800 dark:text-red-200">
                    Disable all alerts for this guest
                  </span>
                  <p class="text-xs text-red-600 dark:text-red-400 mt-1">
                    No alerts will be generated for this guest, regardless of resource usage
                  </p>
                </label>
              </div>
              
              {/* Threshold Overrides */}
              <div class={`space-y-4 ${alertsDisabled() ? 'opacity-50 pointer-events-none' : ''}`}>
                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Threshold Overrides
                </h3>
                
                {/* CPU */}
                <div class="flex items-start gap-3">
                  <input
                    type="checkbox"
                    checked={enabledMetrics().cpu}
                    onChange={(e) => setEnabledMetrics({...enabledMetrics(), cpu: e.currentTarget.checked})}
                    class="mt-1 rounded border-gray-300 dark:border-gray-600"
                  />
                  <div class="flex-1 space-y-2">
                    <label class="text-sm text-gray-600 dark:text-gray-400">CPU Usage</label>
                    <div class="flex items-center gap-2">
                      <div class="flex-1">
                        <ThresholdSlider
                          value={thresholds().cpu || 80}
                          onChange={(v) => setThresholds({...thresholds(), cpu: v})}
                          type="cpu"
                        />
                      </div>
                      <span class="text-xs text-gray-500 w-10 text-right">
                        {thresholds().cpu || 80}%
                      </span>
                    </div>
                  </div>
                </div>
                
                {/* Memory */}
                <div class="flex items-start gap-3">
                  <input
                    type="checkbox"
                    checked={enabledMetrics().memory}
                    onChange={(e) => setEnabledMetrics({...enabledMetrics(), memory: e.currentTarget.checked})}
                    class="mt-1 rounded border-gray-300 dark:border-gray-600"
                  />
                  <div class="flex-1 space-y-2">
                    <label class="text-sm text-gray-600 dark:text-gray-400">Memory Usage</label>
                    <div class="flex items-center gap-2">
                      <div class="flex-1">
                        <ThresholdSlider
                          value={thresholds().memory || 85}
                          onChange={(v) => setThresholds({...thresholds(), memory: v})}
                          type="memory"
                        />
                      </div>
                      <span class="text-xs text-gray-500 w-10 text-right">
                        {thresholds().memory || 85}%
                      </span>
                    </div>
                  </div>
                </div>
                
                {/* Disk */}
                <div class="flex items-start gap-3">
                  <input
                    type="checkbox"
                    checked={enabledMetrics().disk}
                    onChange={(e) => setEnabledMetrics({...enabledMetrics(), disk: e.currentTarget.checked})}
                    class="mt-1 rounded border-gray-300 dark:border-gray-600"
                  />
                  <div class="flex-1 space-y-2">
                    <label class="text-sm text-gray-600 dark:text-gray-400">Disk Usage</label>
                    <div class="flex items-center gap-2">
                      <div class="flex-1">
                        <ThresholdSlider
                          value={thresholds().disk || 90}
                          onChange={(v) => setThresholds({...thresholds(), disk: v})}
                          type="disk"
                        />
                      </div>
                      <span class="text-xs text-gray-500 w-10 text-right">
                        {thresholds().disk || 90}%
                      </span>
                    </div>
                  </div>
                </div>
                
                {/* I/O Metrics */}
                <div class="grid grid-cols-2 gap-4">
                  <div class="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={enabledMetrics().diskRead}
                      onChange={(e) => setEnabledMetrics({...enabledMetrics(), diskRead: e.currentTarget.checked})}
                      class="mt-1 rounded border-gray-300 dark:border-gray-600"
                    />
                    <div class="flex-1 space-y-2">
                      <label class="text-sm text-gray-600 dark:text-gray-400">Disk Read</label>
                      <select
                        value={thresholds().diskRead}
                        onChange={(e) => setThresholds({...thresholds(), diskRead: parseInt(e.currentTarget.value)})}
                        class="w-full px-2 py-1 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      >
                        <option value="0">Off</option>
                        <option value="10">10 MB/s</option>
                        <option value="50">50 MB/s</option>
                        <option value="100">100 MB/s</option>
                        <option value="500">500 MB/s</option>
                      </select>
                    </div>
                  </div>
                  
                  <div class="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={enabledMetrics().diskWrite}
                      onChange={(e) => setEnabledMetrics({...enabledMetrics(), diskWrite: e.currentTarget.checked})}
                      class="mt-1 rounded border-gray-300 dark:border-gray-600"
                    />
                    <div class="flex-1 space-y-2">
                      <label class="text-sm text-gray-600 dark:text-gray-400">Disk Write</label>
                      <select
                        value={thresholds().diskWrite}
                        onChange={(e) => setThresholds({...thresholds(), diskWrite: parseInt(e.currentTarget.value)})}
                        class="w-full px-2 py-1 text-sm border rounded dark:bg-gray-700 dark:border-gray-600"
                      >
                        <option value="0">Off</option>
                        <option value="10">10 MB/s</option>
                        <option value="50">50 MB/s</option>
                        <option value="100">100 MB/s</option>
                        <option value="500">500 MB/s</option>
                      </select>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            
            {/* Footer */}
            <div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-2">
              <button type="button"
                onClick={props.onClose}
                class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
              >
                Cancel
              </button>
              <button type="button"
                onClick={handleSave}
                disabled={!selectedGuest() && !props.existingOverride}
                class="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Save Override
              </button>
            </div>
          </div>
        </div>
      </Portal>
    </Show>
  );
}