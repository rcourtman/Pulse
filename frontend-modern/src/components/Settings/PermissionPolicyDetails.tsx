import { Component, For, Show, createMemo } from 'solid-js';
import type { Permission } from '@/types/rbac';

// Describe saved rules, not a client-side authorization verdict. Unknown
// metadata must stay unknown instead of looking like an unconditional allow.
export const PermissionPolicyDetails: Component<{
  permission: Permission;
  id?: string;
  showTarget?: boolean;
}> = (props) => {
  const effect = () => {
    const value: unknown = props.permission.effect;
    if (value === undefined || value === null || value === '' || value === 'allow') return 'Allow';
    if (value === 'deny') return 'Deny';
    return typeof value === 'string' ? `Unknown effect: ${value}` : 'Effect unavailable';
  };
  const conditions = createMemo(() => {
    const value: unknown = props.permission.conditions;
    if (value === undefined || value === null) return [];
    if (typeof value !== 'object' || Array.isArray(value)) return null;
    const entries = Object.entries(value).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return entries.every(([, expected]) => typeof expected === 'string')
      ? (entries as [string, string][])
      : null;
  });
  return (
    <span
      id={props.id}
      class="block text-xs font-normal text-base-content whitespace-normal [overflow-wrap:anywhere]"
    >
      <span class="block">
        <span class="font-semibold">{effect()}</span>
        <Show when={props.showTarget}>
          {' '}
          <span>
            {props.permission.action}:{props.permission.resource}
          </span>
        </Show>
      </span>
      <Show when={conditions()} fallback={<span class="block">Conditions unavailable</span>}>
        {(entries) => (
          <Show when={entries().length > 0}>
            <span class="block">Conditions (all):</span>
            <For each={entries()}>
              {([key, value]) => (
                <span class="block">
                  <bdi class="font-mono whitespace-pre-wrap">{key}</bdi>
                  {' = '}
                  <bdi class="font-mono whitespace-pre-wrap">{JSON.stringify(value)}</bdi>
                </span>
              )}
            </For>
          </Show>
        )}
      </Show>
    </span>
  );
};
