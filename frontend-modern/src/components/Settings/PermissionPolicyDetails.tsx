import { Component, For, Show, createMemo } from 'solid-js';
import type { Permission } from '@/types/rbac';
import { savedRuleConditions, savedRuleEffect } from '@/utils/rbacPresentation';

// Describe saved rules, not a client-side authorization verdict. Unknown
// metadata must stay unknown instead of looking like an unconditional allow.
export const PermissionPolicyDetails: Component<{
  permission: Permission;
  id?: string;
  showTarget?: boolean;
}> = (props) => {
  const effect = () => {
    const value: unknown = props.permission.effect;
    return (
      savedRuleEffect(value ?? undefined) ??
      (typeof value === 'string' ? `Unknown effect: ${value}` : 'Effect unavailable')
    );
  };
  const conditions = createMemo(() =>
    savedRuleConditions(props.permission.conditions ?? undefined)?.sort(([a], [b]) =>
      a < b ? -1 : a > b ? 1 : 0,
    ),
  );
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
