import { For, Show } from 'solid-js';
import type { Alert } from '@/types/api';

interface ResourceTableProps {
  title: string;
  resources: any[];
  columns: string[];
  activeAlerts?: Record<string, Alert>;
  onEdit: (resourceId: string, thresholds: any, defaults: any) => void;
  onSaveEdit: (resourceId: string) => void;
  onCancelEdit: () => void;
  onRemoveOverride: (resourceId: string) => void;
  onToggleDisabled?: (resourceId: string) => void;
  onToggleNodeConnectivity?: (nodeId: string) => void;
  editingId: () => string | null;
  editingThresholds: () => Record<string, any>;
  setEditingThresholds: (value: Record<string, any>) => void;
  formatMetricValue: (metric: string, value: number | undefined) => string;
  hasActiveAlert: (resourceId: string, metric: string) => boolean;
}

export function ResourceTable(props: ResourceTableProps) {
  const MetricValueWithHeat = (metricProps: { 
    resourceId: string; 
    metric: string; 
    value: number; 
    isOverridden: boolean 
  }) => (
    <div class="flex items-center justify-center gap-1">
      <span class={`text-sm ${
        metricProps.isOverridden 
          ? 'text-gray-900 dark:text-gray-100 font-medium' 
          : 'text-gray-400 dark:text-gray-500'
      }`}>
        {props.formatMetricValue(metricProps.metric, metricProps.value)}
      </span>
      <Show when={props.hasActiveAlert(metricProps.resourceId, metricProps.metric)}>
        <div 
          class="w-1.5 h-1.5 rounded-full bg-red-500 animate-pulse"
          title="Active alert"
        />
      </Show>
    </div>
  );

  return (
    <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
      <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h3 class="text-sm font-medium text-gray-900 dark:text-gray-100">{props.title}</h3>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Resource
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Type
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <For each={props.columns}>
                {(column) => (
                  <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {column}
                  </th>
                )}
              </For>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Alerts
              </th>
              <th class="px-3 py-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <Show when={props.resources.length === 0} fallback={
              <For each={props.resources}>
                {(resource) => {
                  const isEditing = () => props.editingId() === resource.id;
                  const thresholds = () => isEditing() ? props.editingThresholds() : resource.thresholds;
                  const displayValue = (metric: string) => {
                    if (isEditing()) return thresholds()[metric] || resource.defaults[metric] || '';
                    return resource.thresholds[metric] || resource.defaults[metric] || 0;
                  };
                  const isOverridden = (metric: string) => {
                    return resource.thresholds[metric] !== undefined && resource.thresholds[metric] !== null;
                  };
                  
                  return (
                    <tr class={`hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors ${resource.disabled ? 'opacity-40' : ''}`}>
                      <td class="px-3 py-1.5">
                        <div class="flex items-center gap-2">
                          <span class={`text-sm font-medium ${resource.disabled ? 'text-gray-500 dark:text-gray-500' : 'text-gray-900 dark:text-gray-100'}`}>
                            {resource.name}
                          </span>
                          <Show when={'vmid' in resource && resource.vmid}>
                            <span class="text-xs text-gray-500">({resource.vmid})</span>
                          </Show>
                          <Show when={resource.type === 'storage' && 'node' in resource && resource.node}>
                            <span class="text-xs text-gray-500">on {resource.node}</span>
                          </Show>
                          <Show when={resource.type === 'guest' && 'node' in resource && resource.node}>
                            <span class="text-xs text-gray-500">on {resource.node}</span>
                          </Show>
                          <Show when={resource.hasOverride || (resource.type === 'node' && resource.disableConnectivity)}>
                            <span class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded">
                              Custom
                            </span>
                          </Show>
                        </div>
                      </td>
                      <td class="px-3 py-1.5">
                        <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                          resource.type === 'node' ? 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300' :
                          resource.type === 'storage' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' :
                          resource.resourceType === 'VM' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' :
                          'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                        }`}>
                          {resource.resourceType}
                        </span>
                      </td>
                      <td class="px-3 py-1.5">
                        <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
                          resource.status === 'online' || resource.status === 'running' ?
                            'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' :
                          resource.status === 'offline' || resource.status === 'stopped' ?
                            'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' :
                            'bg-gray-100 dark:bg-gray-900/50 text-gray-700 dark:text-gray-300'
                        }`}>
                          {resource.status}
                        </span>
                      </td>
                      
                      {/* Metric columns - dynamically rendered based on resource type */}
                      <For each={props.columns}>
                        {(column) => {
                          const metric = column.toLowerCase().replace(' %', '').replace(' mb/s', '').replace('disk r', 'diskRead').replace('disk w', 'diskWrite').replace('net in', 'networkIn').replace('net out', 'networkOut');
                          
                          // Check if this metric applies to this resource type
                          const showMetric = () => {
                            if (resource.type === 'node' && ['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
                              return false;
                            }
                            if (resource.type === 'storage') {
                              return metric === 'usage';
                            }
                            return true;
                          };
                          
                          return (
                            <td class="px-3 py-1.5 text-center">
                              <Show when={showMetric()} fallback={
                                <span class="text-sm text-gray-400 dark:text-gray-500">-</span>
                              }>
                                <Show when={isEditing()} fallback={
                                  <MetricValueWithHeat 
                                    resourceId={resource.id}
                                    metric={metric}
                                    value={displayValue(metric)}
                                    isOverridden={isOverridden(metric)}
                                  />
                                }>
                                  <input
                                    type="number"
                                    min="0"
                                    max={metric.includes('disk') || metric.includes('memory') || metric.includes('cpu') || metric === 'usage' ? 100 : 10000}
                                    value={thresholds()[metric] || ''}
                                    onInput={(e) => props.setEditingThresholds({
                                      ...props.editingThresholds(),
                                      [metric]: parseInt(e.currentTarget.value) || undefined
                                    })}
                                    class="w-14 px-1 py-0.5 text-sm text-center border border-gray-300 dark:border-gray-600 rounded
                                           bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                                  />
                                </Show>
                              </Show>
                            </td>
                          );
                        }}
                      </For>
                      
                      {/* Alerts column */}
                      <td class="px-3 py-1.5 text-center">
                        <Show when={resource.type === 'guest' && props.onToggleDisabled}>
                          <button
                            onClick={() => props.onToggleDisabled!(resource.id)}
                            class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                              resource.disabled
                                ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                            }`}
                          >
                            {resource.disabled ? 'Disabled' : 'Enabled'}
                          </button>
                        </Show>
                        <Show when={resource.type === 'node' && props.onToggleNodeConnectivity}>
                          <button
                            onClick={() => props.onToggleNodeConnectivity!(resource.id)}
                            class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                              resource.disableConnectivity
                                ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                            }`}
                            title="Toggle connectivity alerts for this node"
                          >
                            {resource.disableConnectivity ? 'No Offline' : 'Alert Offline'}
                          </button>
                        </Show>
                        <Show when={resource.type === 'storage'}>
                          <button
                            onClick={() => props.onToggleDisabled!(resource.id)}
                            class={`px-2 py-0.5 text-xs font-medium rounded transition-colors ${
                              resource.disabled
                                ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800/50'
                                : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/50'
                            }`}
                          >
                            {resource.disabled ? 'Disabled' : 'Enabled'}
                          </button>
                        </Show>
                      </td>
                      
                      {/* Actions column */}
                      <td class="px-3 py-1.5">
                        <div class="flex items-center justify-center gap-1">
                          <Show when={!isEditing()} fallback={
                            <>
                              <button
                                onClick={() => props.onSaveEdit(resource.id)}
                                class="p-1 text-green-600 hover:text-green-700 dark:text-green-400 dark:hover:text-green-300"
                                title="Save"
                              >
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                </svg>
                              </button>
                              <button
                                onClick={props.onCancelEdit}
                                class="p-1 text-gray-600 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                                title="Cancel"
                              >
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                              </button>
                            </>
                          }>
                            <button
                              onClick={() => props.onEdit(resource.id, resource.thresholds, resource.defaults)}
                              class="p-1 text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                              title="Edit thresholds"
                            >
                              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                              </svg>
                            </button>
                            <Show when={resource.hasOverride || (resource.type === 'node' && resource.disableConnectivity)}>
                              <button
                                onClick={() => props.onRemoveOverride(resource.id)}
                                class="p-1 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                                title="Remove override"
                              >
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                              </button>
                            </Show>
                          </Show>
                        </div>
                      </td>
                    </tr>
                  );
                }}
              </For>
            }>
              <tr>
                <td colspan={props.columns.length + 5} class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                  No {props.title.toLowerCase()} found
                </td>
              </tr>
            </Show>
          </tbody>
        </table>
      </div>
    </div>
  );
}