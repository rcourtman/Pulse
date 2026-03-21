import { Component, Show } from 'solid-js';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import { NodeModalSetupGuideSection } from '@/components/Settings/NodeModalSetupGuideSection';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { controlClass, formField, formHelpText, labelClass } from '@/components/shared/Form';
import {
  getNodeTokenIdPlaceholder,
  getNodeUsernameHelp,
  getNodeUsernamePlaceholder,
} from '@/utils/nodeModalPresentation';

interface NodeModalAuthenticationSectionProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

export const NodeModalAuthenticationSection: Component<NodeModalAuthenticationSectionProps> = (
  props,
) => {
  const { modalProps, state } = props;

  return (
    <div>
      <SectionHeader
        title="Authentication"
        size="sm"
        class="mb-4"
        titleClass="text-base-content"
      />

      <div class="mb-4">
        <div class="flex gap-4">
          <label class="flex items-center">
            <input
              type="radio"
              name="authType"
              value="password"
              checked={state.formData().authType === 'password'}
              onChange={() => state.updateField('authType', 'password')}
              class="mr-2"
            />
            <span class="text-sm text-base-content">Username & Password</span>
          </label>
          <Show when={modalProps.nodeType !== 'pmg'}>
            <label class="flex items-center">
              <input
                type="radio"
                name="authType"
                value="token"
                checked={state.formData().authType === 'token'}
                onChange={() => state.updateField('authType', 'token')}
                class="mr-2"
              />
              <span class="text-sm text-base-content">
                API Token{' '}
                <span class="text-green-600 dark:text-green-400 text-xs ml-1">(Recommended)</span>
              </span>
            </label>
          </Show>
        </div>
        <Show when={modalProps.nodeType === 'pmg'}>
          <p class="text-xs text-muted mt-2">
            Proxmox Mail Gateway does not support API tokens. Use a service account with password
            authentication (for example <code>root@pam</code> or a dedicated{' '}
            <code>api@pmg</code> user).
          </p>
        </Show>
      </div>

      <Show when={state.formData().authType === 'password'}>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class={formField}>
            <label class={labelClass()}>
              Username <span class="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={state.formData().user}
              onInput={(event) => state.updateField('user', event.currentTarget.value)}
              placeholder={getNodeUsernamePlaceholder(modalProps.nodeType)}
              required={state.formData().authType === 'password'}
              class={controlClass()}
            />
            <Show when={getNodeUsernameHelp(modalProps.nodeType)}>
              {(help) => <p class={formHelpText}>{help()}</p>}
            </Show>
          </div>

          <div class={formField}>
            <label class={labelClass('flex items-center gap-2')}>
              Password
              <Show when={!state.isEditingExistingNode()}>
                <span class="text-red-500">*</span>
              </Show>
            </label>
            <input
              type="password"
              value={state.formData().password}
              onInput={(event) => state.updateField('password', event.currentTarget.value)}
              placeholder={state.isEditingExistingNode() ? 'Leave blank to keep existing' : 'Password'}
              required={state.formData().authType === 'password' && !state.isEditingExistingNode()}
              class={controlClass()}
            />
          </div>
        </div>
      </Show>

      <Show when={state.formData().authType === 'token'}>
        <div class="space-y-4">
          <NodeModalSetupGuideSection modalProps={modalProps} state={state} />

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class={formField}>
              <label class={labelClass()}>
                Token ID <span class="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={state.formData().tokenName}
                onInput={(event) => state.updateField('tokenName', event.currentTarget.value)}
                placeholder={getNodeTokenIdPlaceholder(modalProps.nodeType)}
                required={state.formData().authType === 'token'}
                class={controlClass('font-mono')}
              />
              <p class={formHelpText}>Full token ID from Proxmox (user@realm!tokenname).</p>
            </div>

            <div class={formField}>
              <label class={labelClass('flex items-center gap-2')}>
                Token Value
                <Show when={!state.isEditingExistingNode()}>
                  <span class="text-red-500">*</span>
                </Show>
              </label>
              <input
                type="password"
                value={state.formData().tokenValue}
                onInput={(event) => state.updateField('tokenValue', event.currentTarget.value)}
                placeholder={
                  state.isEditingExistingNode()
                    ? 'Leave blank to keep existing'
                    : 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
                }
                required={state.formData().authType === 'token' && !state.isEditingExistingNode()}
                class={controlClass('font-mono')}
              />
              <p class={formHelpText}>The secret value shown when creating the token.</p>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
