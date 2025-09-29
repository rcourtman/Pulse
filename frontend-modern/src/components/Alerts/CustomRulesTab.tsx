import { For, Show } from 'solid-js';
import type { CustomAlertRule } from '@/types/alerts';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { EmptyState } from '@/components/shared/EmptyState';

interface CustomRulesTabProps {
  rules: CustomAlertRule[];
  onUpdateRules: (rules: CustomAlertRule[]) => void;
  onHasChanges: (hasChanges: boolean) => void;
}

export function CustomRulesTab(props: CustomRulesTabProps) {
  const deleteRule = (ruleId: string) => {
    const updatedRules = props.rules.filter((r) => r.id !== ruleId);
    props.onUpdateRules(updatedRules);
    props.onHasChanges(true);
  };

  const toggleRule = (ruleId: string) => {
    const updatedRules = props.rules.map((r) =>
      r.id === ruleId ? { ...r, enabled: !r.enabled } : r,
    );
    props.onUpdateRules(updatedRules);
    props.onHasChanges(true);
  };

  const getFilterDescription = (rule: CustomAlertRule): string => {
    return rule.filterConditions.filters
      .map((filter) => {
        if (
          filter.type === 'metric' &&
          filter.field &&
          filter.operator &&
          filter.value !== undefined
        ) {
          return `${filter.field} ${filter.operator} ${filter.value}%`;
        } else if (filter.type === 'text' && filter.field && filter.value) {
          return `${filter.field}: ${filter.value}`;
        } else {
          return filter.rawText || '';
        }
      })
      .join(` ${rule.filterConditions.logicalOperator} `);
  };

  const getThresholdsSummary = (rule: CustomAlertRule): string => {
    const parts: string[] = [];
    if (rule.thresholds.cpu !== undefined) parts.push(`CPU: ${rule.thresholds.cpu}%`);
    if (rule.thresholds.memory !== undefined) parts.push(`Memory: ${rule.thresholds.memory}%`);
    if (rule.thresholds.disk !== undefined) parts.push(`Disk: ${rule.thresholds.disk}%`);
    return parts.length > 0 ? parts.join(', ') : 'Using global defaults';
  };

  return (
    <div class="space-y-4">
      {/* Header */}
      <Card padding="md">
        <SectionHeader
          title="Custom alert rules"
          description="Custom rules apply specific thresholds to guests matching filter conditions. Rules are evaluated in priority order (higher number = higher priority)."
          size="md"
          titleClass="text-gray-800 dark:text-gray-200"
          descriptionClass="text-sm text-gray-600 dark:text-gray-400"
        />
      </Card>

      {/* Priority Order Explanation */}
      <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
        <div class="flex items-start gap-3">
          <svg
            class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <div class="text-sm text-blue-800 dark:text-blue-200">
            <p class="font-medium mb-1">Alert Control Methods:</p>
            <ol class="list-decimal list-inside space-y-0.5 text-xs">
              <li>Guest-specific overrides in Settings</li>
              <li>Custom rules (evaluated by priority number)</li>
              <li>Global default thresholds</li>
            </ol>
          </div>
        </div>
      </div>

      {/* Rules List */}
      <Show
        when={props.rules.length > 0}
        fallback={
          <Card padding="lg">
            <EmptyState
              icon={
                <svg
                  class="h-12 w-12 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 6v6m0 0v6m0-6h6m-6 0H6"
                  />
                </svg>
              }
              title="No custom alert rules"
              description="Create rules from the Dashboard by applying filters and choosing Create Alert."
            />
          </Card>
        }
      >
        <div class="space-y-3">
          <For each={props.rules.sort((a, b) => b.priority - a.priority)}>
            {(rule) => (
              <Card padding="md" class="overflow-hidden">
                <div class="flex items-start justify-between mb-3">
                  <div class="flex-1">
                    <div class="flex items-center gap-2 mb-1">
                      <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200">
                        {rule.name}
                      </h4>
                      <span
                        class={`px-2 py-0.5 text-xs font-medium rounded-full ${
                          rule.enabled
                            ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
                            : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
                        }`}
                      >
                        {rule.enabled ? 'Active' : 'Disabled'}
                      </span>
                      <span class="px-2 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded-full">
                        Priority: {rule.priority}
                      </span>
                    </div>
                    <Show when={rule.description}>
                      <p class="text-xs text-gray-600 dark:text-gray-400 mb-2">
                        {rule.description}
                      </p>
                    </Show>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => toggleRule(rule.id)}
                      class="p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                      title={rule.enabled ? 'Disable rule' : 'Enable rule'}
                    >
                      <svg
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <Show
                          when={rule.enabled}
                          fallback={
                            <path d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          }
                        >
                          <path d="M18.364 5.636a9 9 0 010 12.728m0 0a9 9 0 01-12.728 0m12.728 0L5.636 5.636m12.728 0L5.636 18.364" />
                        </Show>
                      </svg>
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteRule(rule.id)}
                      class="p-1.5 text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                      title="Delete rule"
                    >
                      <svg
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="flex items-start gap-2">
                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400 w-20">
                      Filters:
                    </span>
                    <div class="flex-1">
                      <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                        {getFilterDescription(rule)}
                      </code>
                    </div>
                  </div>

                  <div class="flex items-start gap-2">
                    <span class="text-xs font-medium text-gray-600 dark:text-gray-400 w-20">
                      Thresholds:
                    </span>
                    <span class="text-xs text-gray-700 dark:text-gray-300">
                      {getThresholdsSummary(rule)}
                    </span>
                  </div>

                  <Show
                    when={rule.notifications.email?.enabled || rule.notifications.webhook?.enabled}
                  >
                    <div class="flex items-start gap-2">
                      <span class="text-xs font-medium text-gray-600 dark:text-gray-400 w-20">
                        Notify:
                      </span>
                      <div class="flex gap-2">
                        <Show when={rule.notifications.email?.enabled}>
                          <span class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded">
                            Email
                          </span>
                        </Show>
                        <Show when={rule.notifications.webhook?.enabled}>
                          <span class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded">
                            Webhook
                          </span>
                        </Show>
                      </div>
                    </div>
                  </Show>
                </div>
              </Card>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}
