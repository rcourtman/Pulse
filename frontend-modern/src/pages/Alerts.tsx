import { createSignal, Show, For, createMemo, createEffect, onMount } from 'solid-js';
import { ThresholdSlider } from '@/components/Dashboard/ThresholdSlider';
import { EmailProviderSelect } from '@/components/Alerts/EmailProviderSelect';
import { WebhookConfig } from '@/components/Alerts/WebhookConfig';
import { CustomRulesTab } from '@/components/Alerts/CustomRulesTab';
import { useWebSocket } from '@/App';
import { showSuccess, showError } from '@/utils/toast';
import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import type { EmailConfig } from '@/api/notifications';
import type { HysteresisThreshold, AlertThresholds } from '@/types/alerts';
import type { Alert, State } from '@/types/api';

type AlertTab = 'overview' | 'thresholds' | 'destinations' | 'schedule' | 'history' | 'custom-rules';

// Webhook interface matching WebhookConfig component
interface Webhook {
  id?: string;
  name: string;
  url: string;
  method: string;
  service: string;
  headers: Record<string, string>;
  enabled: boolean;
}

// Store reference interfaces
interface DestinationsRef {
  emailConfig?: () => EmailConfig;
}

interface ScheduleConfig {
  enabled?: boolean;
  quietHours?: {
    enabled: boolean;
    start: string;
    end: string;
    days: Record<string, boolean>;
    timezone?: string;
  };
  cooldown?: number;
  groupingWindow?: number;
  grouping?: {
    enabled: boolean;
    window: number;
    byNode?: boolean;
    byGuest?: boolean;
  };
  maxAlertsHour?: number;
  escalation?: {
    enabled: boolean;
  };
}

interface ScheduleRef {
  setScheduleConfig?: (config: ScheduleConfig) => void;
  getScheduleConfig?: () => ScheduleConfig | undefined;
}

// Override interface for both guests and nodes
interface Override {
  id: string;  // Full ID (e.g. "Main-delly-105" for guest, "node-delly" for node)
  name: string;  // Display name
  type: 'guest' | 'node';
  resourceType?: string;  // VM, CT, or Node
  vmid?: number;  // Only for guests
  node?: string;  // Node name (for guests), undefined for nodes themselves
  instance?: string;
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


export function Alerts() {
  const { state, activeAlerts } = useWebSocket();
  const [activeTab, setActiveTab] = createSignal<AlertTab>('overview');
  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  
  // Store references to child component data
  let destinationsRef: DestinationsRef = {};
  let scheduleRef: ScheduleRef = {};
  
  const [overrides, setOverrides] = createSignal<Override[]>([]);
  
  // Load existing alert configuration on mount (only once)
  onMount(async () => {
    try {
      const config = await AlertsAPI.getConfig();
        if (config.guestDefaults) {
          // Extract trigger values from potentially hysteresis thresholds
          setGuestDefaults({
            cpu: getTriggerValue(config.guestDefaults.cpu) || 80,
            memory: getTriggerValue(config.guestDefaults.memory) || 85,
            disk: getTriggerValue(config.guestDefaults.disk) || 90,
            diskRead: getTriggerValue(config.guestDefaults.diskRead) || 150,
            diskWrite: getTriggerValue(config.guestDefaults.diskWrite) || 150,
            networkIn: getTriggerValue(config.guestDefaults.networkIn) || 200,
            networkOut: getTriggerValue(config.guestDefaults.networkOut) || 200
          });
        }
        
        if (config.nodeDefaults) {
          setNodeDefaults({
            cpu: getTriggerValue(config.nodeDefaults.cpu) || 80,
            memory: getTriggerValue(config.nodeDefaults.memory) || 85,
            disk: getTriggerValue(config.nodeDefaults.disk) || 90
          });
        }
        
        if (config.storageDefault) {
          setStorageDefault(getTriggerValue(config.storageDefault) || 85);
        }
        if (config.overrides) {
          // Convert overrides object to array format
          const overridesList: Override[] = [];
          
          Object.entries(config.overrides).forEach(([key, thresholds]) => {
            // Check if it's a node override by looking for matching node
            const node = (state.nodes || []).find((n) => n.id === key);
            if (node) {
              overridesList.push({
                id: key,
                name: node.name,
                type: 'node',
                resourceType: 'Node',
                thresholds: extractTriggerValues(thresholds)
              });
            } else {
              // Find the guest by matching the full ID
              const guest = [...(state.vms || []), ...(state.containers || [])].find((g) => g.id === key);
              if (guest) {
                overridesList.push({
                  id: key,
                  name: guest.name,
                  type: 'guest',
                  resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
                  vmid: guest.vmid,
                  node: guest.node,
                  instance: guest.instance,
                  thresholds: extractTriggerValues(thresholds)
                });
              }
            }
          });
          setOverrides(overridesList);
        }
        // Pass schedule config to schedule tab if it exists
        if (config.schedule && scheduleRef.setScheduleConfig) {
          // Convert days array to Record if needed
          const scheduleConfig: ScheduleConfig = {
            ...config.schedule,
            quietHours: config.schedule.quietHours ? {
              ...config.schedule.quietHours,
              days: Array.isArray(config.schedule.quietHours.days) 
                ? {
                    '0': config.schedule.quietHours.days.includes(0),
                    '1': config.schedule.quietHours.days.includes(1),
                    '2': config.schedule.quietHours.days.includes(2),
                    '3': config.schedule.quietHours.days.includes(3),
                    '4': config.schedule.quietHours.days.includes(4),
                    '5': config.schedule.quietHours.days.includes(5),
                    '6': config.schedule.quietHours.days.includes(6),
                  }
                : config.schedule.quietHours.days
            } : undefined
          };
          scheduleRef.setScheduleConfig(scheduleConfig);
        }
    } catch (err) {
      console.error('Failed to load alert configuration:', err);
    }
  });

  // Get all guests from state - memoize to prevent unnecessary updates
  const allGuests = createMemo(() => {
    const vms = state.vms || [];
    const containers = state.containers || [];
    return [...vms, ...containers].map(g => ({
      id: g.id,
      name: g.name,
      vmid: g.vmid,
      type: g.type === 'qemu' ? 'VM' : 'LXC',
      node: g.node,
      instance: g.instance
    }));
  }, [], { equals: (prev, next) => {
    // Only update if the actual guest list changed
    if (prev.length !== next.length) return false;
    return prev.every((p, i) => p.vmid === next[i].vmid && p.name === next[i].name);
  }});
  
  // Helper function to extract trigger value from threshold
  const getTriggerValue = (threshold: number | HysteresisThreshold | undefined): number => {
    if (typeof threshold === 'number') {
      return threshold; // Legacy format
    }
    if (threshold && typeof threshold === 'object' && 'trigger' in threshold) {
      return threshold.trigger; // New hysteresis format
    }
    return 0; // Default fallback
  };

  // Helper to extract trigger values for all thresholds
  const extractTriggerValues = (thresholds: AlertThresholds): Record<string, number> => {
    const result: Record<string, number> = {};
    Object.entries(thresholds).forEach(([key, value]) => {
      result[key] = getTriggerValue(value);
    });
    return result;
  };

  // Threshold states - using trigger values for display
  const [guestDefaults, setGuestDefaults] = createSignal({
    cpu: 80,
    memory: 85,
    disk: 90,
    diskRead: 150,
    diskWrite: 150,
    networkIn: 200,
    networkOut: 200
  });

  const [nodeDefaults, setNodeDefaults] = createSignal({
    cpu: 80,
    memory: 85,
    disk: 90
  });

  const [storageDefault, setStorageDefault] = createSignal(85);
  
  const tabs: { id: AlertTab; label: string; icon: string }[] = [
    { 
      id: 'overview', 
      label: 'Overview',
      icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6'
    },
    { 
      id: 'thresholds', 
      label: 'Thresholds',
      icon: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6'
    },
    { 
      id: 'destinations', 
      label: 'Notifications',
      icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9'
    },
    { 
      id: 'schedule', 
      label: 'Schedule',
      icon: 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z'
    },
    { 
      id: 'history', 
      label: 'History',
      icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'
    }
  ];
  
  return (
    <div class="space-y-4">
      {/* Header with better styling */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-800 dark:text-gray-200">Alert Configuration</h1>
          <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
            Configure monitoring thresholds and notification settings
          </p>
        </div>
      </div>
      
      {/* Save notification bar - only show when there are unsaved changes */}
      <Show when={hasUnsavedChanges() && activeTab() !== 'overview' && activeTab() !== 'history'}>
        <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3 sm:p-4">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div class="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
              </svg>
              <span class="text-sm font-medium">You have unsaved changes</span>
            </div>
            <div class="flex gap-2 w-full sm:w-auto">
              <button 
                class="flex-1 sm:flex-initial px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                onClick={async () => {
                  try {
                    // Save alert configuration with hysteresis format
                    const createHysteresisThreshold = (trigger: number, clearMargin: number = 5) => ({
                      trigger,
                      clear: Math.max(0, trigger - clearMargin)
                    });
                    
                    const alertConfig = {
                      enabled: true,
                      guestDefaults: {
                        cpu: createHysteresisThreshold(guestDefaults().cpu),
                        memory: createHysteresisThreshold(guestDefaults().memory),
                        disk: createHysteresisThreshold(guestDefaults().disk),
                        diskRead: createHysteresisThreshold(guestDefaults().diskRead),
                        diskWrite: createHysteresisThreshold(guestDefaults().diskWrite),
                        networkIn: createHysteresisThreshold(guestDefaults().networkIn),
                        networkOut: createHysteresisThreshold(guestDefaults().networkOut)
                      },
                      nodeDefaults: {
                        cpu: createHysteresisThreshold(nodeDefaults().cpu),
                        memory: createHysteresisThreshold(nodeDefaults().memory),
                        disk: createHysteresisThreshold(nodeDefaults().disk)
                      },
                      storageDefault: createHysteresisThreshold(storageDefault()),
                      minimumDelta: 2.0,
                      suppressionWindow: 5,
                      hysteresisMargin: 5.0,
                      overrides: overrides().reduce((acc, o) => {
                        // Convert thresholds to hysteresis format
                        const hysteresisThresholds: AlertThresholds = {};
                        Object.entries(o.thresholds).forEach(([metric, value]) => {
                          hysteresisThresholds[metric] = createHysteresisThreshold(value as number);
                        });
                        acc[o.id] = hysteresisThresholds;
                        return acc;
                      }, {} as Record<string, AlertThresholds>),
                      schedule: scheduleRef.getScheduleConfig ? scheduleRef.getScheduleConfig() : {
                        quietHours: {
                          enabled: false,
                          start: "22:00",
                          end: "08:00",
                          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                          days: {
                            monday: true,
                            tuesday: true,
                            wednesday: true,
                            thursday: true,
                            friday: true,
                            saturday: false,
                            sunday: false
                          }
                        },
                        cooldown: 5,
                        groupingWindow: 30
                      },
                      // Add missing required fields
                      aggregation: {
                        enabled: true,
                        timeWindow: 10,
                        countThreshold: 3,
                        similarityWindow: 5.0
                      },
                      flapping: {
                        enabled: true,
                        threshold: 5,
                        window: 10,
                        suppressionTime: 30,
                        minStability: 0.8
                      },
                      ioNormalization: {
                        enabled: true,
                        vmDiskMax: 500.0,
                        containerDiskMax: 300.0,
                        networkMax: 1000.0
                      }
                    };
                    
                    await AlertsAPI.updateConfig(alertConfig);
                    
                    // Save email config if on destinations tab
                    if (activeTab() === 'destinations' && destinationsRef.emailConfig) {
                      await NotificationsAPI.updateEmailConfig(destinationsRef.emailConfig());
                    }
                    
                    setHasUnsavedChanges(false);
                    showSuccess('Configuration saved successfully!');
                  } catch (err) {
                    console.error('Failed to save configuration:', err);
                    showError(err instanceof Error ? err.message : 'Failed to save configuration');
                  }
                }}
              >
                Save Changes
              </button>
              <button 
                class="flex-1 sm:flex-initial px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                onClick={() => {
                  // Reset any changes made
                  window.location.reload();
                }}
              >
                Discard
              </button>
            </div>
          </div>
        </div>
      </Show>
      
      {/* Tab Navigation - modern style */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
        <div class="p-1">
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5 w-full overflow-x-auto">
            <For each={tabs}>
              {(tab) => (
                <button
                  class={`flex-1 px-3 py-2 text-xs sm:text-sm font-medium rounded-md transition-all whitespace-nowrap ${
                    activeTab() === tab.id
                      ? 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100'
                  }`}
                  onClick={() => setActiveTab(tab.id)}
                >
                  {tab.label}
                </button>
              )}
            </For>
          </div>
        </div>
        <div class="border-t border-gray-200 dark:border-gray-700"></div>
        
        {/* Tab Content */}
        <div class="p-3 sm:p-6">
          <Show when={activeTab() === 'overview'}>
            <OverviewTab overrides={overrides()} activeAlerts={activeAlerts} />
          </Show>
          
          <Show when={activeTab() === 'thresholds'}>
            <ThresholdsTab 
              overrides={overrides}
              setOverrides={setOverrides}
              allGuests={allGuests}
              state={state}
              guestDefaults={guestDefaults()}
              setGuestDefaults={setGuestDefaults}
              nodeDefaults={nodeDefaults()}
              setNodeDefaults={setNodeDefaults}
              storageDefault={storageDefault}
              setStorageDefault={setStorageDefault}
              activeAlerts={activeAlerts}
            />
          </Show>
          
          <Show when={activeTab() === 'destinations'}>
            <DestinationsTab 
              ref={destinationsRef}
              hasUnsavedChanges={hasUnsavedChanges}
              setHasUnsavedChanges={setHasUnsavedChanges}
            />
          </Show>
          
          <Show when={activeTab() === 'schedule'}>
            <ScheduleTab 
              ref={scheduleRef} 
              hasUnsavedChanges={hasUnsavedChanges}
              setHasUnsavedChanges={setHasUnsavedChanges}
            />
          </Show>
          
          <Show when={activeTab() === 'history'}>
            <HistoryTab />
          </Show>
          
          {/* Custom Rules Tab */}
          <Show when={activeTab() === 'custom-rules'}>
            <CustomRulesTab
              rules={[]}
              onUpdateRules={() => {}}
              onHasChanges={setHasUnsavedChanges}
            />
          </Show>
        </div>
      </div>
      
    </div>
  );
}

// Overview Tab - Shows current alert status
function OverviewTab(props: { overrides: Override[]; activeAlerts: Record<string, Alert> }) {
  // Get alert stats from actual active alerts
  const alertStats = createMemo(() => {
    const alerts = Object.values(props.activeAlerts);
    return {
      active: alerts.filter(a => !a.acknowledged).length,
      acknowledged: alerts.filter(a => a.acknowledged).length,
      total24h: alerts.length, // In real app, would filter by time
      overrides: props.overrides.length
    };
  });
  
  return (
    <div class="space-y-6">
      {/* Stats Cards */}
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 sm:p-4 shadow-sm">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Active Alerts</p>
              <p class="text-xl sm:text-2xl font-semibold text-red-600 dark:text-red-400">{alertStats().active}</p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-red-100 dark:bg-red-900/50 rounded-full flex items-center justify-center">
              <svg width="16" height="16" class="sm:w-5 sm:h-5 text-red-600 dark:text-red-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
                <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
              </svg>
            </div>
          </div>
        </div>
        
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 sm:p-4 shadow-sm">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Acknowledged</p>
              <p class="text-xl sm:text-2xl font-semibold text-yellow-600 dark:text-yellow-400">{alertStats().acknowledged}</p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-yellow-100 dark:bg-yellow-900/50 rounded-full flex items-center justify-center">
              <svg width="16" height="16" class="sm:w-5 sm:h-5 text-yellow-600 dark:text-yellow-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M9 11L12 14L22 4"></path>
                <path d="M21 12V19C21 20.1046 20.1046 21 19 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3.89543 3 5 3H16"></path>
              </svg>
            </div>
          </div>
        </div>
        
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 sm:p-4 shadow-sm">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Last 24 Hours</p>
              <p class="text-xl sm:text-2xl font-semibold text-gray-700 dark:text-gray-300">{alertStats().total24h}</p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-gray-200 dark:bg-gray-600 rounded-full flex items-center justify-center">
              <svg width="16" height="16" class="sm:w-5 sm:h-5 text-gray-600 dark:text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <polyline points="12 6 12 12 16 14"></polyline>
              </svg>
            </div>
          </div>
        </div>
        
        <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 sm:p-4 shadow-sm">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">Guest Overrides</p>
              <p class="text-xl sm:text-2xl font-semibold text-blue-600 dark:text-blue-400">{alertStats().overrides}</p>
            </div>
            <div class="w-8 h-8 sm:w-10 sm:h-10 bg-blue-100 dark:bg-blue-900/50 rounded-full flex items-center justify-center">
              <svg width="16" height="16" class="sm:w-5 sm:h-5 text-blue-600 dark:text-blue-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
              </svg>
            </div>
          </div>
        </div>
      </div>
      
      {/* Recent Alerts */}
      <div>
        <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Active Alerts</h3>
        <Show 
          when={Object.keys(props.activeAlerts).length > 0}
          fallback={
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              <p class="text-sm">No active alerts</p>
              <p class="text-xs mt-1">Alerts will appear here when thresholds are exceeded</p>
            </div>
          }
        >
          <div class="space-y-2">
            <For each={Object.values(props.activeAlerts)}>
              {(alert) => (
                <div class={`border rounded-lg p-4 ${
                  alert.level === 'critical' 
                    ? 'border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900/20' 
                    : 'border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900/20'
                }`}>
                  <div class="flex items-start justify-between">
                    <div class="flex-1">
                      <div class="flex items-center gap-2">
                        <span class={`text-sm font-medium ${
                          alert.level === 'critical' ? 'text-red-700 dark:text-red-400' : 'text-yellow-700 dark:text-yellow-400'
                        }`}>
                          {alert.resourceName}
                        </span>
                        <span class="text-xs text-gray-600 dark:text-gray-400">
                          ({alert.type})
                        </span>
                        <Show when={alert.acknowledged}>
                          <span class="px-2 py-0.5 text-xs bg-yellow-200 dark:bg-yellow-800 text-yellow-800 dark:text-yellow-200 rounded">
                            Acknowledged
                          </span>
                        </Show>
                      </div>
                      <p class="text-sm text-gray-700 dark:text-gray-300 mt-1">
                        {alert.message}
                      </p>
                      <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                        Started: {new Date(alert.startTime).toLocaleString()}
                      </p>
                    </div>
                    <div class="flex gap-2 ml-4">
                      <Show when={!alert.acknowledged}>
                        <button 
                          class="px-3 py-1 text-xs bg-yellow-600 text-white rounded hover:bg-yellow-700 transition-colors"
                          onClick={() => {
                            // API call to acknowledge alert
                            AlertsAPI.acknowledge(alert.id)
                              .catch((err: unknown) => console.error('Failed to acknowledge alert:', err));
                          }}
                        >
                          Acknowledge
                        </button>
                      </Show>
                      <button 
                        class="px-3 py-1 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                        onClick={() => {
                          // API call to clear alert
                          AlertsAPI.clearAlert(alert.id)
                            .catch((err: unknown) => console.error('Failed to clear alert:', err));
                        }}
                      >
                        Clear
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}

// Add Override Form Component
function AddOverrideForm(props: {
  guests: Array<{ id: string; name: string; vmid: number; type: string; node: string; instance: string }>;
  nodes: Array<{ id: string; name: string }>;
  existingOverrides: Override[];
  onAdd: (override: Override) => void;
}) {
  const [resourceType, setResourceType] = createSignal<'guest' | 'node'>('guest');
  const [selectedResource, setSelectedResource] = createSignal('');
  const [thresholds, setThresholds] = createSignal({
    cpu: 80,
    memory: 85, 
    disk: 90
  });
  const [enabledThresholds, setEnabledThresholds] = createSignal({
    cpu: false,
    memory: false,
    disk: false
  });

  const availableResources = createMemo(() => {
    const existingIds = new Set(props.existingOverrides.map(o => o.id));
    
    if (resourceType() === 'node') {
      return props.nodes.filter(n => !existingIds.has(n.id));
    } else {
      return props.guests.filter(g => !existingIds.has(g.id));
    }
  });

  const handleAdd = () => {
    const resourceId = selectedResource();
    if (!resourceId) return;

    const activeThresholds: Record<string, number> = {};
    const thresholdValues = thresholds();
    Object.entries(enabledThresholds()).forEach(([key, enabled]) => {
      if (enabled && key in thresholdValues) {
        activeThresholds[key] = thresholdValues[key as keyof typeof thresholdValues];
      }
    });

    if (Object.keys(activeThresholds).length === 0) return;

    let newOverride: Override;
    
    if (resourceType() === 'node') {
      const node = props.nodes.find(n => n.id === resourceId);
      if (!node) return;
      
      newOverride = {
        id: node.id,
        name: node.name,
        type: 'node',
        resourceType: 'Node',
        thresholds: activeThresholds
      };
    } else {
      const guest = props.guests.find(g => g.id === resourceId);
      if (!guest) return;
      
      newOverride = {
        id: guest.id,
        name: guest.name,
        type: 'guest',
        resourceType: guest.type === 'qemu' ? 'VM' : 'CT',
        vmid: guest.vmid,
        node: guest.node,
        instance: guest.instance,
        thresholds: activeThresholds
      };
    }

    props.onAdd(newOverride);
    
    // Reset form
    setSelectedResource('');
    setThresholds({ cpu: 80, memory: 85, disk: 90 });
    setEnabledThresholds({ cpu: false, memory: false, disk: false });
  };

  return (
    <div class="space-y-4">
      <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 flex items-center gap-2">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 2L2 7v10c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-10-5z"/>
        </svg>
        Add New Override
      </h4>
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Resource Selection */}
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              Select Resource
            </label>
            
            {/* Resource Type Toggle */}
            <div class="flex p-1 bg-gray-100 dark:bg-gray-700 rounded-lg mb-4">
              <button
                onClick={() => {
                  setResourceType('guest');
                  setSelectedResource('');
                }}
                class={`flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors ${
                  resourceType() === 'guest'
                    ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
                }`}
              >
                Guests
              </button>
              <button
                onClick={() => {
                  setResourceType('node');
                  setSelectedResource('');
                }}
                class={`flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors ${
                  resourceType() === 'node'
                    ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
                }`}
              >
                Nodes
              </button>
            </div>
            
            <select
              value={selectedResource()}
              onChange={(e) => setSelectedResource(e.target.value)}
              class="w-full px-4 py-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                     bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-transparent
                     transition-colors"
            >
              <option value="">Choose a {resourceType()}...</option>
              <For each={availableResources()}>
                {(resource) => (
                  <option value={resource.id}>
                    {resource.name}
                    {resourceType() === 'guest' && 'vmid' in resource && ` (${resource.vmid})`}
                  </option>
                )}
              </For>
            </select>
          </div>
        </div>

        {/* Threshold Configuration */}
        <div class="space-y-4">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Configure Thresholds
          </label>
          <div class="space-y-4">
            <For each={['cpu', 'memory', 'disk']}>
              {(metric) => (
                <div>
                  <div class="flex items-center justify-between mb-1">
                    <label class="text-xs font-medium text-gray-600 dark:text-gray-400">
                      {metric === 'cpu' ? 'CPU' : metric === 'memory' ? 'Memory' : 'Disk'} Usage
                    </label>
                    <span class="text-xs text-gray-500">{thresholds()[metric as keyof typeof thresholds]}%</span>
                  </div>
                  <ThresholdSlider
                    value={thresholds()[metric as keyof typeof thresholds]}
                    onChange={(value) => {
                      setThresholds({
                        ...thresholds(),
                        [metric]: value
                      });
                      // Auto-enable this threshold when value changes
                      setEnabledThresholds({
                        ...enabledThresholds(),
                        [metric]: true
                      });
                    }}
                    type={metric as 'cpu' | 'memory' | 'disk'}
                  />
                </div>
              )}
            </For>
          </div>
        </div>
      </div>

      <div class="flex justify-end pt-2">
        <button
          onClick={handleAdd}
          disabled={!selectedResource()}
          class="px-6 py-2.5 text-sm font-medium bg-gradient-to-r from-blue-600 to-blue-700 
                 text-white rounded-lg hover:from-blue-700 hover:to-blue-800 
                 disabled:opacity-50 disabled:cursor-not-allowed transition-all
                 shadow-sm hover:shadow-md"
        >
          <span class="flex items-center gap-2">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="12" y1="5" x2="12" y2="19"></line>
              <line x1="5" y1="12" x2="19" y2="12"></line>
            </svg>
            Add Override
          </span>
        </button>
      </div>
    </div>
  );
}

// Override Item Component
function OverrideItem(props: {
  override: Override;
  onUpdate: (override: Override) => void;
  onRemove: () => void;
}) {
  const [editing, setEditing] = createSignal(false);
  const [editValues, setEditValues] = createSignal({ ...props.override.thresholds });

  const handleSave = () => {
    props.onUpdate({
      ...props.override,
      thresholds: { ...editValues() }
    });
    setEditing(false);
  };

  return (
    <div class="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:shadow-md transition-shadow">
      <div class="flex items-start justify-between">
        <div class="flex-1">
          <div class="flex items-center gap-2 mb-3">
            <p class="text-sm font-semibold text-gray-800 dark:text-gray-200">
              {props.override.name}
              <Show when={props.override.type === 'guest'}>
                <span class="text-xs text-gray-500 ml-1">({props.override.vmid})</span>
              </Show>
            </p>
            <span class={`text-xs px-2 py-0.5 rounded-full ${
              props.override.type === 'node' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
              props.override.resourceType === 'VM' ? 'bg-indigo-100 dark:bg-indigo-900/50 text-indigo-700 dark:text-indigo-300' :
              'bg-teal-100 dark:bg-teal-900/50 text-teal-700 dark:text-teal-300'
            }`}>
              {props.override.type === 'node' ? 'Node' : props.override.resourceType}
            </span>
          </div>

          <Show when={!editing()}>
            <div class="flex gap-4">
              <For each={Object.entries(props.override.thresholds).filter(([_, v]) => v)}>
                {([key, value]) => (
                  <div class="flex items-center gap-1 text-sm">
                    <span class="text-gray-600 dark:text-gray-400">{key.toUpperCase()}:</span>
                    <span class="font-medium text-gray-800 dark:text-gray-200">{value as number}%</span>
                  </div>
                )}
              </For>
            </div>
          </Show>

          <Show when={editing()}>
            <div class="space-y-3 mt-4">
              <For each={Object.entries(props.override.thresholds).filter(([_, v]) => v)}>
                {([key]) => (
                  <div class="space-y-2">
                    <div class="flex items-center justify-between">
                      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                        {key === 'cpu' ? 'CPU' : key === 'memory' ? 'Memory' : 'Disk'} Usage
                      </label>
                      <span class="text-sm text-gray-500">{editValues()[key as keyof typeof editValues]}%</span>
                    </div>
                    <ThresholdSlider
                      value={editValues()[key as keyof typeof editValues]}
                      onChange={(value) => setEditValues({
                        ...editValues(),
                        [key]: value
                      })}
                      type={key === 'cpu' ? 'cpu' : key === 'memory' ? 'memory' : 'disk'}
                    />
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>

        <div class="flex items-center gap-2 ml-4">
          <Show when={!editing()}>
            <button
              onClick={() => {
                setEditValues({ ...props.override.thresholds });
                setEditing(true);
              }}
              class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400"
            >
              Edit
            </button>
            <button
              onClick={props.onRemove}
              class="text-sm text-red-600 hover:text-red-700 dark:text-red-400"
            >
              Remove
            </button>
          </Show>
          <Show when={editing()}>
            <button
              onClick={handleSave}
              class="px-3 py-1 text-sm bg-green-600 text-white rounded hover:bg-green-700"
            >
              Save
            </button>
            <button
              onClick={() => {
                setEditing(false);
                setEditValues({ ...props.override.thresholds });
              }}
              class="px-3 py-1 text-sm bg-gray-500 text-white rounded hover:bg-gray-600"
            >
              Cancel
            </button>
          </Show>
        </div>
      </div>
    </div>
  );
}

// Thresholds Tab - Improved design  
interface ThresholdsTabProps {
  allGuests: () => Array<{ id: string; name: string; vmid: number; type: string; node: string; instance: string }>;
  state: State;
  guestDefaults: Record<string, number>;
  nodeDefaults: Record<string, number>;
  storageDefault: () => number;
  overrides: () => Override[];
  setGuestDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  setNodeDefaults: (value: Record<string, number> | ((prev: Record<string, number>) => Record<string, number>)) => void;
  setStorageDefault: (value: number) => void;
  setOverrides: (value: Override[]) => void;
  activeAlerts: Record<string, Alert>;
}

function ThresholdsTab(props: ThresholdsTabProps) {
  return (
    <div class="space-y-8">
      {/* Step 1: Global Default Thresholds */}
      <div>
        <div class="flex items-center justify-between mb-6">
          <div>
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Default Alert Thresholds</h3>
            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
              These thresholds apply to all resources unless overridden below
            </p>
          </div>
          <button 
            onClick={() => {
              props.setGuestDefaults({
                cpu: 80,
                memory: 85,
                disk: 90,
                diskRead: 150,
                diskWrite: 150,
                networkIn: 200,
                networkOut: 200
              });
              props.setNodeDefaults({
                cpu: 80,
                memory: 85,
                disk: 90
              });
              props.setStorageDefault(85);
              // Changed
            }}
            class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 font-medium">
            Reset All to Defaults
          </button>
        </div>
        
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* VMs & Containers */}
          <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-blue-500">
                <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
                <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
                <line x1="6" y1="6" x2="6.01" y2="6"></line>
                <line x1="6" y1="18" x2="6.01" y2="18"></line>
              </svg>
              VMs & Containers
            </h4>
            
            <div class="space-y-4">
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">CPU Usage</label>
                  <span class="text-xs text-gray-500">{props.guestDefaults.cpu}%</span>
                </div>
                <ThresholdSlider
                  value={props.guestDefaults.cpu}
                  onChange={(v) => {
                    props.setGuestDefaults((prev) => ({...prev, cpu: v}));
                    // Changed
                  }}
                  type="cpu"
                />
              </div>
              
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">Memory Usage</label>
                  <span class="text-xs text-gray-500">{props.guestDefaults.memory}%</span>
                </div>
                <ThresholdSlider
                  value={props.guestDefaults.memory}
                  onChange={(v) => {
                    props.setGuestDefaults((prev) => ({...prev, memory: v}));
                    // Changed
                  }}
                  type="memory"
                />
              </div>
              
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">Disk Usage</label>
                  <span class="text-xs text-gray-500">{props.guestDefaults.disk}%</span>
                </div>
                <ThresholdSlider
                  value={props.guestDefaults.disk}
                  onChange={(v) => {
                    props.setGuestDefaults((prev) => ({...prev, disk: v}));
                    // Changed
                  }}
                  type="disk"
                />
              </div>
            </div>
          </div>

          {/* Proxmox Nodes */}
          <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-green-500">
                <path d="M22 12h-4l-3 9L9 3l-3 9H2"></path>
              </svg>
              Proxmox Nodes
            </h4>
            
            <div class="space-y-4">
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">CPU Usage</label>
                  <span class="text-xs text-gray-500">{props.nodeDefaults.cpu}%</span>
                </div>
                <ThresholdSlider
                  value={props.nodeDefaults.cpu}
                  onChange={(v) => {
                    props.setNodeDefaults((prev) => ({...prev, cpu: v}));
                    // Changed
                  }}
                  type="cpu"
                />
              </div>
              
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">Memory Usage</label>
                  <span class="text-xs text-gray-500">{props.nodeDefaults.memory}%</span>
                </div>
                <ThresholdSlider
                  value={props.nodeDefaults.memory}
                  onChange={(v) => {
                    props.setNodeDefaults((prev) => ({...prev, memory: v}));
                    // Changed
                  }}
                  type="memory"
                />
              </div>
              
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">Disk Usage</label>
                  <span class="text-xs text-gray-500">{props.nodeDefaults.disk}%</span>
                </div>
                <ThresholdSlider
                  value={props.nodeDefaults.disk}
                  onChange={(v) => {
                    props.setNodeDefaults((prev) => ({...prev, disk: v}));
                    // Changed
                  }}
                  type="disk"
                />
              </div>
            </div>
          </div>

          {/* Storage */}
          <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-5 shadow-sm">
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-amber-500">
                <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
                <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path>
                <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path>
              </svg>
              Storage
            </h4>
            
            <div class="space-y-4">
              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-400">Usage Threshold</label>
                  <span class="text-xs text-gray-500">{props.storageDefault()}%</span>
                </div>
                <ThresholdSlider
                  value={props.storageDefault()}
                  onChange={(v) => {
                    props.setStorageDefault(v);
                    // Changed
                  }}
                  type="disk"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      
      {/* Step 2: Resource-Specific Overrides */}
      <div>
        <div class="mb-6">
          <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Custom Overrides</h3>
          <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
            Override default thresholds for specific VMs, containers, or nodes
          </p>
        </div>
        
        {/* Existing Overrides List */}
        <Show when={props.overrides().length > 0}>
          <div class="space-y-3 mb-6">
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
              Active Overrides ({props.overrides().length})
            </h4>
            <div class="space-y-2">
              <For each={props.overrides()}>
                {(override) => (
                  <OverrideItem
                    override={override}
                    onUpdate={(updatedOverride) => {
                      props.setOverrides(props.overrides().map((o: Override) => 
                        o.id === override.id ? updatedOverride : o
                      ));
                      // Changed
                    }}
                    onRemove={() => {
                      props.setOverrides(props.overrides().filter((o) => o.id !== override.id));
                      // Changed
                    }}
                  />
                )}
              </For>
            </div>
          </div>
        </Show>
        
        {/* Add New Override Form */}
        <div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-6 hover:border-blue-400 dark:hover:border-blue-500 transition-colors">
          <div class="flex items-center gap-3 mb-4">
            <div class="w-8 h-8 rounded-full bg-blue-100 dark:bg-blue-900/50 flex items-center justify-center">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-blue-600 dark:text-blue-400">
                <line x1="12" y1="5" x2="12" y2="19"></line>
                <line x1="5" y1="12" x2="19" y2="12"></line>
              </svg>
            </div>
            <div>
              <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Add Custom Override</h4>
              <p class="text-xs text-gray-600 dark:text-gray-400">Set different thresholds for a specific resource</p>
            </div>
          </div>
          
          <AddOverrideForm 
            guests={props.allGuests()}
            nodes={props.state.nodes || []}
            existingOverrides={props.overrides()}
            onAdd={(override) => {
              props.setOverrides([...props.overrides(), override]);
              // Changed
            }}
          />
        </div>
      </div>
    </div>
  );
}

// Destinations Tab - Notification settings
interface DestinationsTabProps {
  ref: DestinationsRef;
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
}

// Local email config with UI-specific fields
interface UIEmailConfig {
  enabled: boolean;
  provider: string;
  smtpHost: string;
  smtpPort: number;
  username: string;
  password: string;
  from: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
  replyTo: string;
  maxRetries: number;
  retryDelay: number;
  rateLimit: number;
}

function DestinationsTab(props: DestinationsTabProps) {
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>({
    enabled: false,
    provider: '',
    smtpHost: '',
    smtpPort: 587,
    username: '',
    password: '',
    from: '',
    to: [] as string[],
    tls: true,
    startTLS: false,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 5,
    rateLimit: 60
  });
  
  // Expose emailConfig to parent (convert to API format)
  onMount(() => {
    if (props.ref) {
      props.ref.emailConfig = () => {
        const config = emailConfig();
        return {
          enabled: config.enabled,
          provider: config.provider,
          server: config.smtpHost,
          port: config.smtpPort,
          username: config.username,
          password: config.password,
          from: config.from,
          to: config.to,
          tls: config.tls,
          starttls: config.startTLS
        } as EmailConfig;
      };
    }
  });
  
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([]);
  const [testingEmail, setTestingEmail] = createSignal(false);
  const [testingWebhook, setTestingWebhook] = createSignal<string | null>(null);
  
  // Load email config on mount
  onMount(async () => {
    try {
      const config = await NotificationsAPI.getEmailConfig();
      // Map API config to local format
      setEmailConfig({
        enabled: config.enabled,
        provider: config.provider,
        smtpHost: config.server,
        smtpPort: config.port,
        username: config.username,
        password: config.password || '',
        from: config.from,
        to: config.to,
        tls: config.tls,
        startTLS: config.starttls,
        replyTo: '',
        maxRetries: 3,
        retryDelay: 5,
        rateLimit: 60
      });
    } catch (err) {
      console.error('Failed to load email config:', err);
    }
    
    // Load webhooks
    try {
      const hooks = await NotificationsAPI.getWebhooks();
      // Map to local Webhook type
      setWebhooks(hooks.map(h => ({
        ...h,
        service: 'custom' // Default service type
      })));
    } catch (err) {
      console.error('Failed to load webhooks:', err);
    }
  });
  
  const testEmailConfig = async () => {
    setTestingEmail(true);
    try {
      await NotificationsAPI.testNotification({ type: 'email' });
      alert('Test email sent successfully! Check your inbox.');
    } catch (err) {
      alert('Failed to send test email');
    } finally {
      setTestingEmail(false);
    }
  };
  
  const testWebhook = async (webhookId: string) => {
    setTestingWebhook(webhookId);
    try {
      await NotificationsAPI.testNotification({ type: 'webhook', webhookId });
      alert('Test webhook sent successfully!');
    } catch (err) {
      alert(`Failed to send test webhook: ${err instanceof Error ? err.message : 'Unknown error'}`);
    } finally {
      setTestingWebhook(null);
    }
  };
  
  return (
    <div class="space-y-6">
      {/* Email Configuration */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 shadow-sm">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 flex items-center gap-2">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
              <polyline points="22,6 12,13 2,6"></polyline>
            </svg>
            Email Notifications
          </h3>
          <label class="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              checked={emailConfig().enabled}
              onChange={(e) => {
                setEmailConfig({...emailConfig(), enabled: e.currentTarget.checked});
                // Changed
              }}
              class="sr-only peer" 
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
        </div>
        
        <div class={`${!emailConfig().enabled ? 'opacity-50 pointer-events-none' : ''}`}>
          <EmailProviderSelect
            config={emailConfig()}
            onChange={(config) => {
              setEmailConfig(config);
              // Changed
            }}
            onTest={testEmailConfig}
            testing={testingEmail()}
          />
        </div>
      </div>
      
      {/* Webhook Configuration */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 shadow-sm">
        <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 flex items-center gap-2 mb-4">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"></path>
            <path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"></path>
          </svg>
          Webhooks
        </h3>
        
        <WebhookConfig
          webhooks={webhooks()}
          onAdd={(webhook) => {
            setWebhooks([...webhooks(), {
              ...webhook,
              id: Date.now().toString()
            }]);
            props.setHasUnsavedChanges(true);
          }}
          onUpdate={(webhook) => {
            setWebhooks(webhooks().map(w => 
              w.id === webhook.id ? webhook : w
            ));
            props.setHasUnsavedChanges(true);
          }}
          onDelete={(id) => {
            setWebhooks(webhooks().filter(w => w.id !== id));
            props.setHasUnsavedChanges(true);
          }}
          onTest={testWebhook}
          testing={testingWebhook()}
        />
      </div>
    </div>
  );
}

// History Tab - Alert history
// Schedule Tab - Quiet hours, cooldown, and grouping
interface ScheduleTabProps {
  ref: ScheduleRef;
  hasUnsavedChanges: () => boolean;
  setHasUnsavedChanges: (value: boolean) => void;
}

function ScheduleTab(props: ScheduleTabProps) {
  const [quietHours, setQuietHours] = createSignal({
    enabled: false,
    start: '22:00',
    end: '08:00',
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    days: {
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false
    } as Record<string, boolean>
  });
  
  const [cooldown, setCooldown] = createSignal({
    enabled: true,
    minutes: 30,
    maxAlerts: 3
  });
  
  const [grouping, setGrouping] = createSignal({
    enabled: true,
    window: 5,
    byNode: true,
    byGuest: false
  });
  
  const [escalation, setEscalation] = createSignal({
    enabled: false,
    levels: [
      { after: 15, notify: 'email' },
      { after: 30, notify: 'webhook' },
      { after: 60, notify: 'all' }
    ]
  });
  
  const timezones = [
    'UTC',
    'America/New_York',
    'America/Chicago',
    'America/Denver',
    'America/Los_Angeles',
    'Europe/London',
    'Europe/Paris',
    'Europe/Berlin',
    'Asia/Tokyo',
    'Asia/Shanghai',
    'Australia/Sydney'
  ];
  
  const days = [
    { id: 'monday', label: 'M', fullLabel: 'Monday' },
    { id: 'tuesday', label: 'T', fullLabel: 'Tuesday' },
    { id: 'wednesday', label: 'W', fullLabel: 'Wednesday' },
    { id: 'thursday', label: 'T', fullLabel: 'Thursday' },
    { id: 'friday', label: 'F', fullLabel: 'Friday' },
    { id: 'saturday', label: 'S', fullLabel: 'Saturday' },
    { id: 'sunday', label: 'S', fullLabel: 'Sunday' }
  ];
  
  // Expose schedule config via ref
  if (props.ref) {
    props.ref.getScheduleConfig = () => ({
      quietHours: quietHours(),
      cooldown: cooldown().enabled ? cooldown().minutes : 5,
      groupingWindow: grouping().enabled && grouping().window ? grouping().window * 60 : 30, // Convert minutes to seconds
      maxAlertsHour: cooldown().enabled && cooldown().maxAlerts ? cooldown().maxAlerts : 10,
      escalation: escalation(),
      grouping: grouping()
    });
    
    props.ref.setScheduleConfig = (config: ScheduleConfig) => {
      if (config.quietHours) {
        const qh = config.quietHours;
        setQuietHours(prev => ({
          ...prev,
          ...qh,
          // Ensure timezone is preserved if not provided
          timezone: qh.timezone || prev.timezone
        }));
      }
      if (config.cooldown !== undefined) {
        setCooldown({
          enabled: config.cooldown > 0,
          minutes: config.cooldown,
          maxAlerts: 3
        });
      }
      if (config.groupingWindow !== undefined) {
        const gw = config.groupingWindow;
        setGrouping(prev => ({
          ...prev,
          enabled: gw > 0,
          window: Math.floor(gw / 60), // Convert seconds to minutes
          // Preserve existing grouping preferences
          byNode: config.grouping?.byNode !== undefined ? config.grouping.byNode : prev.byNode,
          byGuest: config.grouping?.byGuest !== undefined ? config.grouping.byGuest : prev.byGuest
        }));
      }
      if (config.maxAlertsHour !== undefined) {
        setCooldown(prev => ({ ...prev, maxAlerts: config.maxAlertsHour! }));
      }
      if (config.escalation !== undefined) {
        setEscalation(prev => ({
          ...prev,
          enabled: config.escalation!.enabled
        }));
      }
      if (config.grouping !== undefined) {
        setGrouping(prev => ({
          ...prev,
          enabled: config.grouping!.enabled,
          window: config.grouping!.window,
          byNode: config.grouping!.byNode ?? prev.byNode,
          byGuest: config.grouping!.byGuest ?? prev.byGuest
        }));
      }
    };
  }
  
  return (
    <div class="space-y-4">
      {/* Header */}
      <div class="mb-6">
        <h2 class="text-lg font-semibold text-gray-800 dark:text-gray-200">Notification Schedule</h2>
        <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">Control when and how alerts are delivered</p>
      </div>
      
      {/* Quiet Hours */}
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 sm:p-6">
        <div class="flex items-start sm:items-center justify-between gap-4 flex-col sm:flex-row">
          <div class="flex items-start gap-3">
            <div class="w-10 h-10 bg-blue-100 dark:bg-blue-900/50 rounded-lg flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">Quiet Hours</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">Pause non-critical alerts during specific times</p>
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={quietHours().enabled}
              onChange={(e) => {
                setQuietHours({ ...quietHours(), enabled: e.currentTarget.checked });
                // Changed
              }}
              class="sr-only peer"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
        </div>
        
        <Show when={quietHours().enabled}>
          <div class="space-y-4 mt-4 pt-4 border-t border-gray-200 dark:border-gray-600">
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Start Time</label>
                <input
                  type="time"
                  value={quietHours().start}
                  onChange={(e) => {
                    setQuietHours({ ...quietHours(), start: e.currentTarget.value });
                    // Changed
                  }}
                  class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-600 dark:border-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">End Time</label>
                <input
                  type="time"
                  value={quietHours().end}
                  onChange={(e) => {
                    setQuietHours({ ...quietHours(), end: e.currentTarget.value });
                    // Changed
                  }}
                  class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-600 dark:border-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Timezone</label>
                <select
                  value={quietHours().timezone}
                  onChange={(e) => {
                    setQuietHours({ ...quietHours(), timezone: e.currentTarget.value });
                    // Changed
                  }}
                  class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-600 dark:border-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                >
                  <For each={timezones}>
                    {(tz) => <option value={tz}>{tz}</option>}
                  </For>
                </select>
              </div>
            </div>
            
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Quiet Days</label>
              <div class="grid grid-cols-7 gap-1">
                <For each={days}>
                  {(day) => (
                    <button
                      onClick={() => {
                        const currentDays = quietHours().days;
                        setQuietHours({
                          ...quietHours(),
                          days: { ...currentDays, [day.id]: !currentDays[day.id] }
                        });
                        // Changed
                      }}
                      title={day.fullLabel}
                      class={`px-2 py-2 text-xs rounded-lg transition-all duration-200 font-medium ${
                        quietHours().days[day.id]
                          ? 'bg-blue-500 text-white shadow-sm'
                          : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'
                      }`}
                    >
                      {day.label}
                    </button>
                  )}
                </For>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                <Show when={quietHours().days.monday && quietHours().days.tuesday && quietHours().days.wednesday && quietHours().days.thursday && quietHours().days.friday && !quietHours().days.saturday && !quietHours().days.sunday}>
                  Weekdays only
                </Show>
                <Show when={!quietHours().days.monday && !quietHours().days.tuesday && !quietHours().days.wednesday && !quietHours().days.thursday && !quietHours().days.friday && quietHours().days.saturday && quietHours().days.sunday}>
                  Weekends only
                </Show>
              </p>
            </div>
          </div>
        </Show>
      </div>
      
      {/* Cooldown Period */}
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 sm:p-6">
        <div class="flex items-start sm:items-center justify-between gap-4 flex-col sm:flex-row">
          <div class="flex items-start gap-3">
            <div class="w-10 h-10 bg-amber-100 dark:bg-amber-900/50 rounded-lg flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">Alert Cooldown</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">Limit alert frequency to prevent spam</p>
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={cooldown().enabled}
              onChange={(e) => {
                setCooldown({ ...cooldown(), enabled: e.currentTarget.checked });
                // Changed
              }}
              class="sr-only peer"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
        </div>
        
        <Show when={cooldown().enabled}>
          <div class="space-y-4 mt-4 pt-4 border-t border-gray-200 dark:border-gray-600">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Cooldown Period
                </label>
                <div class="relative">
                  <input
                    type="number"
                    min="5"
                    max="120"
                    value={cooldown().minutes}
                    onChange={(e) => {
                      setCooldown({ ...cooldown(), minutes: parseInt(e.currentTarget.value) });
                      // Changed
                    }}
                    class="w-full px-3 py-2 pr-16 text-sm border rounded-lg dark:bg-gray-600 dark:border-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  />
                  <span class="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500 dark:text-gray-400">minutes</span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Minimum time between alerts for the same issue
                </p>
              </div>
              
              <div>
                <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Max Alerts/Hour
                </label>
                <div class="relative">
                  <input
                    type="number"
                    min="1"
                    max="10"
                    value={cooldown().maxAlerts}
                    onChange={(e) => {
                      setCooldown({ ...cooldown(), maxAlerts: parseInt(e.currentTarget.value) });
                      // Changed
                    }}
                    class="w-full px-3 py-2 pr-16 text-sm border rounded-lg dark:bg-gray-600 dark:border-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  />
                  <span class="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-500 dark:text-gray-400">alerts</span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Per guest/metric combination
                </p>
              </div>
            </div>
          </div>
        </Show>
      </div>
      
      {/* Alert Grouping */}
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 sm:p-6">
        <div class="flex items-start sm:items-center justify-between gap-4 flex-col sm:flex-row">
          <div class="flex items-start gap-3">
            <div class="w-10 h-10 bg-green-100 dark:bg-green-900/50 rounded-lg flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-green-600 dark:text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">Smart Grouping</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">Bundle similar alerts together</p>
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={grouping().enabled}
              onChange={(e) => {
                setGrouping({ ...grouping(), enabled: e.currentTarget.checked });
                // Changed
              }}
              class="sr-only peer"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
        </div>
        
        <Show when={grouping().enabled}>
          <div class="space-y-4 mt-4 pt-4 border-t border-gray-200 dark:border-gray-600">
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                Grouping Window
              </label>
              <div class="flex items-center gap-3">
                <input
                  type="range"
                  min="1"
                  max="30"
                  value={grouping().window}
                  onChange={(e) => {
                    setGrouping({ ...grouping(), window: parseInt(e.currentTarget.value) });
                    // Changed
                  }}
                  class="flex-1"
                />
                <div class="w-16 px-2 py-1 text-sm text-center bg-gray-100 dark:bg-gray-600 rounded">
                  {grouping().window} min
                </div>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                Alerts within this window will be grouped together
              </p>
            </div>
            
            <div>
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Grouping Strategy</label>
              <div class="grid grid-cols-2 gap-2">
                <label class={`relative flex items-center p-3 rounded-lg cursor-pointer border-2 transition-all ${
                  grouping().byNode
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                    : 'border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'
                }`}>
                  <input
                    type="checkbox"
                    checked={grouping().byNode}
                    onChange={(e) => {
                      setGrouping({ ...grouping(), byNode: e.currentTarget.checked });
                      // Changed
                    }}
                    class="sr-only"
                  />
                  <div class="flex items-center gap-2">
                    <div class={`w-4 h-4 rounded border-2 flex items-center justify-center ${
                      grouping().byNode
                        ? 'border-blue-500 bg-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    }`}>
                      <Show when={grouping().byNode}>
                        <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                      </Show>
                    </div>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">By Node</span>
                  </div>
                </label>
                <label class={`relative flex items-center p-3 rounded-lg cursor-pointer border-2 transition-all ${
                  grouping().byGuest
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                    : 'border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'
                }`}>
                  <input
                    type="checkbox"
                    checked={grouping().byGuest}
                    onChange={(e) => {
                      setGrouping({ ...grouping(), byGuest: e.currentTarget.checked });
                      // Changed
                    }}
                    class="sr-only"
                  />
                  <div class="flex items-center gap-2">
                    <div class={`w-4 h-4 rounded border-2 flex items-center justify-center ${
                      grouping().byGuest
                        ? 'border-blue-500 bg-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    }`}>
                      <Show when={grouping().byGuest}>
                        <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                        </svg>
                      </Show>
                    </div>
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">By Guest</span>
                  </div>
                </label>
              </div>
            </div>
          </div>
        </Show>
      </div>
      
      {/* Escalation Rules */}
      <div class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4 sm:p-6">
        <div class="flex items-start sm:items-center justify-between gap-4 flex-col sm:flex-row">
          <div class="flex items-start gap-3">
            <div class="w-10 h-10 bg-red-100 dark:bg-red-900/50 rounded-lg flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-red-600 dark:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-800 dark:text-gray-200">Alert Escalation</h3>
              <p class="text-sm text-gray-600 dark:text-gray-400">Notify additional contacts for persistent issues</p>
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={escalation().enabled}
              onChange={(e) => {
                setEscalation({ ...escalation(), enabled: e.currentTarget.checked });
                // Changed
              }}
              class="sr-only peer"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          </label>
        </div>
        
        <Show when={escalation().enabled}>
          <div class="space-y-3 mt-4 pt-4 border-t border-gray-200 dark:border-gray-600">
            <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">Define escalation levels for unresolved alerts:</p>
            <For each={escalation().levels}>
              {(level, index) => (
                <div class="flex items-center gap-2 p-3 bg-white dark:bg-gray-600 rounded-lg border border-gray-200 dark:border-gray-500">
                  <div class="flex-1 grid grid-cols-1 sm:grid-cols-2 gap-2 items-center">
                    <div class="flex items-center gap-2">
                      <span class="text-xs font-medium text-gray-600 dark:text-gray-400 whitespace-nowrap">After</span>
                      <input
                        type="number"
                        min="5"
                        max="180"
                        value={level.after}
                        onChange={(e) => {
                          const newLevels = [...escalation().levels];
                          newLevels[index()] = { ...level, after: parseInt(e.currentTarget.value) };
                          setEscalation({ ...escalation(), levels: newLevels });
                          // Changed
                        }}
                        class="w-16 px-2 py-1 text-sm border rounded dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      />
                      <span class="text-xs text-gray-600 dark:text-gray-400">min</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-xs font-medium text-gray-600 dark:text-gray-400 whitespace-nowrap">notify</span>
                      <select
                        value={level.notify}
                        onChange={(e) => {
                          const newLevels = [...escalation().levels];
                          newLevels[index()] = { ...level, notify: e.currentTarget.value };
                          setEscalation({ ...escalation(), levels: newLevels });
                          // Changed
                        }}
                        class="flex-1 px-2 py-1 text-sm border rounded dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                      >
                        <option value="email">Email</option>
                        <option value="webhook">Webhooks</option>
                        <option value="all">All Channels</option>
                      </select>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const newLevels = escalation().levels.filter((_, i) => i !== index());
                      setEscalation({ ...escalation(), levels: newLevels });
                      // Changed
                    }}
                    class="p-1.5 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                    title="Remove escalation level"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                    </svg>
                  </button>
                </div>
              )}
            </For>
            
            <button
              onClick={() => {
                const lastLevel = escalation().levels[escalation().levels.length - 1];
                const newAfter = lastLevel ? lastLevel.after + 30 : 15;
                setEscalation({
                  ...escalation(),
                  levels: [...escalation().levels, { after: newAfter, notify: 'all' }]
                });
                // Changed
              }}
              class="w-full py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:border-gray-400 dark:hover:border-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700 transition-all duration-200 flex items-center justify-center gap-2"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
              </svg>
              Add Escalation Level
            </button>
          </div>
        </Show>
      </div>
      
      {/* Configuration Summary */}
      <div class="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4 border border-blue-200 dark:border-blue-800">
        <h3 class="text-sm font-semibold text-blue-900 dark:text-blue-200 mb-2 flex items-center gap-2">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Current Configuration Summary
        </h3>
        <div class="space-y-1 text-xs text-blue-800 dark:text-blue-300">
          <Show when={quietHours().enabled}>
            <p>• Quiet hours active from {quietHours().start} to {quietHours().end} ({quietHours().timezone})</p>
          </Show>
          <Show when={cooldown().enabled}>
            <p>• {cooldown().minutes} minute cooldown between alerts, max {cooldown().maxAlerts} alerts per hour</p>
          </Show>
          <Show when={grouping().enabled}>
            <p>• Grouping alerts within {grouping().window} minute windows
              <Show when={grouping().byNode || grouping().byGuest}>
                {' '}by {[grouping().byNode && 'node', grouping().byGuest && 'guest'].filter(Boolean).join(' and ')}
              </Show>
            </p>
          </Show>
          <Show when={escalation().enabled && escalation().levels.length > 0}>
            <p>• {escalation().levels.length} escalation level{escalation().levels.length > 1 ? 's' : ''} configured</p>
          </Show>
          <Show when={!quietHours().enabled && !cooldown().enabled && !grouping().enabled && !escalation().enabled}>
            <p>• All notification controls are disabled - alerts will be sent immediately</p>
          </Show>
        </div>
      </div>
    </div>
  );
}

// History Tab - Comprehensive alert table
function HistoryTab() {
  const { state, activeAlerts } = useWebSocket();
  
  // Filter states with localStorage persistence
  const [timeFilter, setTimeFilter] = createSignal(localStorage.getItem('alertHistoryTimeFilter') || '7d');
  const [severityFilter, setSeverityFilter] = createSignal(localStorage.getItem('alertHistorySeverityFilter') || 'all');
  const [searchTerm, setSearchTerm] = createSignal('');
  const [alertHistory, setAlertHistory] = createSignal<Alert[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);
  
  // Ref for search input
  let searchInputRef: HTMLInputElement | undefined;
  
  // Persist filter changes to localStorage
  createEffect(() => {
    localStorage.setItem('alertHistoryTimeFilter', timeFilter());
  });
  
  createEffect(() => {
    localStorage.setItem('alertHistorySeverityFilter', severityFilter());
  });
  
  // Load alert history on mount
  onMount(async () => {
    try {
      const history = await AlertsAPI.getHistory({ limit: 1000 });
      setAlertHistory(history);
    } catch (err) {
      console.error('Failed to load alert history:', err);
    } finally {
      setLoading(false);
    }
    
    // Add keyboard event listeners
    const handleKeydown = (e: KeyboardEvent) => {
      // If already focused on an input, select, or textarea, don't interfere
      const activeElement = document.activeElement;
      if (activeElement && (
        activeElement.tagName === 'INPUT' || 
        activeElement.tagName === 'TEXTAREA' || 
        activeElement.tagName === 'SELECT'
      )) {
        // Handle Escape to clear and unfocus
        if (e.key === 'Escape' && activeElement === searchInputRef) {
          setSearchTerm('');
          searchInputRef.blur();
        }
        return;
      }
      
      // If typing a letter, number, or space, focus the search input
      if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        searchInputRef?.focus();
      }
    };
    
    document.addEventListener('keydown', handleKeydown);
    
    // Cleanup on unmount
    return () => {
      document.removeEventListener('keydown', handleKeydown);
    };
  });
  
  // Format duration for display
  const formatDuration = (startTime: string, endTime?: string) => {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : Date.now();
    const duration = end - start;
    
    // Handle negative durations (clock skew or timezone issues)
    if (duration < 0) {
      return '0m';
    }
    
    const minutes = Math.floor(duration / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    return `${minutes}m`;
  };

  // Get resource type (VM, CT, Node, Storage)
  const getResourceType = (resourceName: string) => {
    // Check VMs and containers
    const vm = state.vms?.find((v) => v.name === resourceName);
    if (vm) return 'VM';
    
    const container = state.containers?.find((c) => c.name === resourceName);
    if (container) return 'CT';
    
    // Check nodes
    const node = state.nodes?.find((n) => n.name === resourceName);
    if (node) return 'Node';
    
    // Check storage
    const storage = state.storage?.find((s) => s.name === resourceName || s.id === resourceName);
    if (storage) return 'Storage';
    
    return 'Unknown';
  };

  // Extended alert type for display
  interface ExtendedAlert extends Alert {
    status?: string;
    duration?: string;
    resourceType?: string;
  }

  // Prepare all alerts without filtering
  const allAlertsData = createMemo(() => {
    // Combine active and historical alerts
    const allAlerts: ExtendedAlert[] = [];
    
    // Add active alerts
    Object.values(activeAlerts || {}).forEach((alert) => {
      allAlerts.push({
        ...alert,
        status: 'active',
        duration: formatDuration(alert.startTime),
        resourceType: getResourceType(alert.resourceName)
      });
    });
    
    // Create a set of active alert IDs for quick lookup
    const activeAlertIds = new Set(Object.keys(activeAlerts || {}));
    
    // Add historical alerts
    alertHistory().forEach(alert => {
      // Skip if this alert is already in active alerts (avoid duplicates)
      if (activeAlertIds.has(alert.id)) {
        return;
      }
      
      allAlerts.push({
        ...alert,
        status: alert.acknowledged ? 'acknowledged' : 'resolved',
        duration: formatDuration(alert.startTime, alert.lastSeen),
        resourceType: getResourceType(alert.resourceName)
      });
    });
    
    return allAlerts;
  });

  // Apply filters to get the final alert data
  const alertData = createMemo(() => {
    let filtered = allAlertsData();
    
    // Selected bar filter (takes precedence over time filter)
    if (selectedBarIndex() !== null) {
      const trends = alertTrends();
      const index = selectedBarIndex()!;
      const bucketStart = trends.bucketTimes[index];
      const bucketEnd = bucketStart + trends.bucketSize * 60 * 60 * 1000;
      
      filtered = filtered.filter(alert => {
        const alertTime = new Date(alert.startTime).getTime();
        return alertTime >= bucketStart && alertTime < bucketEnd;
      });
    } else {
      // Time filter
      if (timeFilter() !== 'all') {
        const now = Date.now();
        const cutoff = {
          '24h': now - 24 * 60 * 60 * 1000,
          '7d': now - 7 * 24 * 60 * 60 * 1000,
          '30d': now - 30 * 24 * 60 * 60 * 1000
        }[timeFilter()];
        
        if (cutoff) {
          filtered = filtered.filter(a => new Date(a.startTime).getTime() > cutoff);
        }
      }
    }
    
    // Severity filter
    if (severityFilter() !== 'all') {
      filtered = filtered.filter(a => a.level === severityFilter());
    }
    
    // Search filter
    if (searchTerm()) {
      const term = searchTerm().toLowerCase();
      filtered = filtered.filter(alert => 
        alert.resourceName.toLowerCase().includes(term) ||
        alert.message.toLowerCase().includes(term) ||
        alert.type.toLowerCase().includes(term) ||
        alert.node.toLowerCase().includes(term)
      );
    }
    
    // Sort by start time (newest first)
    return filtered.sort((a, b) => 
      new Date(b.startTime).getTime() - new Date(a.startTime).getTime()
    );
  });

  // Group alerts by day for display
  const groupedAlerts = createMemo(() => {
    const groups = new Map();
    
    alertData().forEach(alert => {
      const date = new Date(alert.startTime);
      const dayKey = date.toLocaleDateString();
      
      if (!groups.has(dayKey)) {
        groups.set(dayKey, {
          date: date,
          alerts: []
        });
      }
      
      groups.get(dayKey).alerts.push(alert);
    });
    
    // Convert to array and sort by date (newest first)
    return Array.from(groups.values()).sort((a, b) => 
      b.date.getTime() - a.date.getTime()
    );
  });
  
  // Calculate alert trends for mini-chart
  const alertTrends = createMemo(() => {
    const now = Date.now();
    const timeRange = timeFilter() === '24h' ? 24 : timeFilter() === '7d' ? 7 * 24 : timeFilter() === '30d' ? 30 * 24 : 90 * 24; // hours
    const bucketSize = timeFilter() === '24h' ? 1 : timeFilter() === '7d' ? 6 : timeFilter() === '30d' ? 24 : 72; // hours per bucket
    const numBuckets = Math.min(Math.floor(timeRange / bucketSize), 30); // Limit to 30 buckets max
    
    // Calculate start time for the chart
    const startTime = now - timeRange * 60 * 60 * 1000;
    
    // Initialize buckets
    const buckets = new Array(numBuckets).fill(0);
    // bucketTimes represents the START of each bucket
    const bucketTimes = new Array(numBuckets).fill(0).map((_, i) => 
      startTime + i * bucketSize * 60 * 60 * 1000
    );
    
    // Filter alerts based on current time filter
    let alertsToCount = allAlertsData();
    if (timeFilter() !== 'all') {
      const cutoff = {
        '24h': now - 24 * 60 * 60 * 1000,
        '7d': now - 7 * 24 * 60 * 60 * 1000,
        '30d': now - 30 * 24 * 60 * 60 * 1000
      }[timeFilter()];
      
      if (cutoff) {
        alertsToCount = alertsToCount.filter(a => new Date(a.startTime).getTime() > cutoff);
      }
    }
    
    alertsToCount.forEach(alert => {
      const alertTime = new Date(alert.startTime).getTime();
      if (alertTime >= startTime && alertTime <= now) {
        const bucketIndex = Math.floor((alertTime - startTime) / (bucketSize * 60 * 60 * 1000));
        if (bucketIndex >= 0 && bucketIndex < numBuckets) {
          buckets[bucketIndex]++;
        }
      }
    });
    
    // Find max for scaling
    const max = Math.max(...buckets, 1);
    
    return {
      buckets,
      max,
      bucketSize,
      bucketTimes
    };
  });
  
  return (
    <div class="space-y-4">
      {/* Alert Trends Mini-Chart */}
      <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">
            Alert Frequency 
            <span class="text-xs text-gray-400 ml-2">({alertData().length} alerts)</span>
          </h3>
          <div class="flex items-center gap-2">
            <Show when={selectedBarIndex() !== null}>
              <button
                onClick={() => setSelectedBarIndex(null)}
                class="px-2 py-0.5 text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded hover:bg-blue-200 dark:hover:bg-blue-800/50 transition-colors"
              >
                Clear filter
              </button>
            </Show>
            <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="flex items-center gap-1">
                <div class="w-2 h-2 bg-yellow-500 rounded-full"></div>
                {alertData().filter(a => a.level === 'warning').length} warnings
              </span>
              <span class="flex items-center gap-1">
                <div class="w-2 h-2 bg-red-500 rounded-full"></div>
                {alertData().filter(a => a.level === 'critical').length} critical
              </span>
            </div>
          </div>
        </div>
        
        {/* Mini sparkline chart */}
        <div class="text-[10px] text-gray-400 mb-1">
          Showing {alertTrends().buckets.length} time periods - Total: {alertData().length} alerts
        </div>
        
        {/* Alert frequency chart */}
        <div class="h-12 bg-gray-100 dark:bg-gray-800 rounded p-1 flex items-end gap-1">
          {alertTrends().buckets.map((val, i) => {
            const scaledHeight = val > 0 ? Math.min(100, Math.max(20, Math.log(val + 1) * 20)) : 0;
            const pixelHeight = val > 0 ? Math.max(8, (scaledHeight / 100) * 40) : 0; // 40px is roughly the inner height
            const isSelected = selectedBarIndex() === i;
            return (
              <div 
                class="flex-1 group relative flex items-end cursor-pointer"
                onClick={() => setSelectedBarIndex(i === selectedBarIndex() ? null : i)}
              >
                {/* Background track for all slots */}
                <div class="absolute bottom-0 w-full h-1 bg-gray-300 dark:bg-gray-600 opacity-30 rounded-full"></div>
                {/* Actual bar */}
                <div 
                  class="w-full group relative rounded-sm transition-all"
                  style={{
                    height: `${pixelHeight}px`,
                    'background-color': val > 0 ? (isSelected ? '#2563eb' : '#3b82f6') : 'transparent',
                    'opacity': isSelected ? '1' : '0.8',
                    'box-shadow': isSelected ? '0 0 0 2px rgba(37, 99, 235, 0.4)' : 'none'
                  }}
                  title={`${val} alert${val !== 1 ? 's' : ''}`}
                >
                  {/* Tooltip on hover */}
                  <div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-1 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-10">
                    <div class="bg-gray-900 text-white text-xs rounded px-2 py-1 whitespace-nowrap">
                      <div class="font-semibold">{val} alert{val !== 1 ? 's' : ''}</div>
                      <div class="text-[10px] text-gray-300">
                        {timeFilter() === '24h' ? `${alertTrends().bucketSize} hour period` : 
                         timeFilter() === '7d' ? `${alertTrends().bucketSize / 24} day period` :
                         timeFilter() === '30d' ? `${alertTrends().bucketSize / 24} day period` :
                         `${alertTrends().bucketSize / 24} day period`}
                      </div>
                      <div class="text-[10px] text-gray-300">
                        {new Date(alertTrends().bucketTimes[i]).toLocaleString('en-US', {
                          month: 'short',
                          day: 'numeric',
                          hour: timeFilter() === '24h' ? 'numeric' : undefined,
                          minute: timeFilter() === '24h' ? '2-digit' : undefined
                        })}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
        
        {/* Time labels */}
        <div class="flex justify-between mt-1 text-[10px] text-gray-400 dark:text-gray-500">
          <span>{timeFilter() === '24h' ? '24h ago' : timeFilter() === '7d' ? '7d ago' : timeFilter() === '30d' ? '30d ago' : '90d ago'}</span>
          <span>Now</span>
        </div>
      </div>
      
      {/* Filters */}
      <div class="flex flex-wrap gap-2 mb-4">
        <select 
          value={timeFilter()}
          onChange={(e) => setTimeFilter(e.currentTarget.value)}
          class="px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600">
          <option value="24h">Last 24h</option>
          <option value="7d">Last 7d</option>
          <option value="30d">Last 30d</option>
          <option value="all">All Time</option>
        </select>
        
        <select 
          value={severityFilter()}
          onChange={(e) => setSeverityFilter(e.currentTarget.value)}
          class="px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600">
          <option value="all">All Levels</option>
          <option value="critical">Critical Only</option>
          <option value="warning">Warning Only</option>
        </select>
        
        <div class="flex-1 max-w-xs">
          <input
            ref={searchInputRef}
            type="text"
            placeholder="Search alerts..."
            value={searchTerm()}
            onInput={(e) => setSearchTerm(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setSearchTerm('');
                e.currentTarget.blur();
              }
            }}
            class="w-full px-3 py-2 text-sm border rounded-lg dark:bg-gray-700 dark:border-gray-600 
                   dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500"
          />
        </div>
      </div>
      
      {/* Alert History Table */}
      <Show 
        when={loading()}
        fallback={
          <Show 
            when={alertData().length > 0}
            fallback={
              <div class="text-center py-12 text-gray-500 dark:text-gray-400">
                <p class="text-sm">No alerts found</p>
                <p class="text-xs mt-1">Try adjusting your filters or check back later</p>
              </div>
            }
          >
            {/* Table */}
            <div class="mb-2 border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
              <div class="overflow-x-auto">
                <table class="w-full min-w-[900px] text-xs sm:text-sm">
                  <thead>
                    <tr class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Timestamp</th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Resource</th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Type</th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">Severity</th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Message</th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">Duration</th>
                      <th class="p-1 px-2 text-center text-[10px] sm:text-xs font-medium uppercase tracking-wider">Status</th>
                      <th class="p-1 px-2 text-left text-[10px] sm:text-xs font-medium uppercase tracking-wider">Node</th>
                    </tr>
                  </thead>
                  <tbody>
                    <For each={groupedAlerts()}>
                      {(group) => (
                        <>
                          {/* Date divider */}
                          <tr class="bg-gray-50 dark:bg-gray-800">
                            <td colspan="8" class="p-1 px-2 text-xs font-medium text-gray-600 dark:text-gray-400">
                              {group.date.toLocaleDateString('en-US', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
                            </td>
                          </tr>
                          
                          {/* Alerts for this day */}
                          <For each={group.alerts}>
                            {(alert) => (
                              <tr class={`border-b border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 ${
                                alert.status === 'active' ? 'bg-red-50 dark:bg-red-900/10' : ''
                              }`}>
                                {/* Timestamp */}
                                <td class="p-1 px-2 text-gray-600 dark:text-gray-400 font-mono">
                                  {new Date(alert.startTime).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}
                                </td>
                                
                                {/* Resource */}
                                <td class="p-1 px-2 font-medium text-gray-900 dark:text-gray-100 truncate max-w-[150px]">
                                  {alert.resourceName}
                                </td>
                                
                                {/* Type */}
                                <td class="p-1 px-2">
                                  <span class={`text-xs px-1 py-0.5 rounded ${
                                    alert.resourceType === 'VM' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' :
                                    alert.resourceType === 'CT' ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' :
                                    alert.resourceType === 'Node' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
                                    alert.resourceType === 'Storage' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' :
                                    'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                  }`}>
                                    {alert.type}
                                  </span>
                                </td>
                                
                                {/* Severity */}
                                <td class="p-1 px-2 text-center">
                                  <span class={`text-xs px-2 py-0.5 rounded font-medium ${
                                    alert.level === 'critical' 
                                      ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' 
                                      : 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
                                  }`}>
                                    {alert.level}
                                  </span>
                                </td>
                                
                                {/* Message */}
                                <td class="p-1 px-2 text-gray-700 dark:text-gray-300 truncate max-w-[300px]" title={alert.message}>
                                  {alert.message}
                                </td>
                                
                                {/* Duration */}
                                <td class="p-1 px-2 text-center text-gray-600 dark:text-gray-400">
                                  {alert.duration}
                                </td>
                                
                                {/* Status */}
                                <td class="p-1 px-2 text-center">
                                  <span class={`text-xs px-2 py-0.5 rounded ${
                                    alert.status === 'active' 
                                      ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 font-medium' 
                                      : alert.status === 'acknowledged'
                                      ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
                                      : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                  }`}>
                                    {alert.status}
                                  </span>
                                </td>
                                
                                {/* Node */}
                                <td class="p-1 px-2 text-gray-600 dark:text-gray-400 truncate">
                                  {alert.node || '—'}
                                </td>
                              </tr>
                            )}
                          </For>
                        </>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </div>
          </Show>
        }
      >
        <div class="text-center py-12 text-gray-500 dark:text-gray-400">
          <p class="text-sm">Loading alert history...</p>
        </div>
      </Show>
      
      {/* Administrative Actions - Only show if there's history to clear */}
      <Show when={alertHistory().length > 0}>
        <div class="mt-8 pt-6 border-t border-gray-200 dark:border-gray-700">
          <div class="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4">
            <div class="flex items-start justify-between">
              <div>
                <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200 mb-1">
                  Administrative Actions
                </h4>
                <p class="text-xs text-gray-600 dark:text-gray-400">
                  Permanently clear all alert history. Use with caution - this action cannot be undone.
                </p>
              </div>
              <button
                onClick={async () => {
                  if (confirm('Are you sure you want to clear all alert history?\n\nThis will permanently delete all historical alert data and cannot be undone.\n\nThis is typically only used for system maintenance or when starting fresh with a new monitoring setup.')) {
                    try {
                      await AlertsAPI.clearHistory();
                      setAlertHistory([]);
                      console.log('Alert history cleared successfully');
                    } catch (err) {
                      console.error('Error clearing alert history:', err);
                      alert('Error clearing alert history. Please check your connection and try again.');
                    }
                  }
                }}
                class="px-3 py-2 text-xs border border-red-300 dark:border-red-600 text-red-600 dark:text-red-400 
                       rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors flex-shrink-0"
              >
                Clear All History
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}