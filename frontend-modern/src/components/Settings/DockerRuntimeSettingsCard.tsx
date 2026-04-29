import { Component, Accessor, Show } from 'solid-js';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import { ENVIRONMENT_LOCK_BUTTON_TITLE } from '@/utils/environmentLockPresentation';
import {
  DOCKER_UPDATE_ACTIONS_ENV_VAR,
  DOCKER_UPDATE_ACTIONS_SECTION_DESCRIPTION,
  DOCKER_UPDATE_ACTIONS_SECTION_TITLE,
  DOCKER_UPDATE_ACTIONS_TOGGLE_DESCRIPTION,
  DOCKER_UPDATE_ACTIONS_TOGGLE_LABEL,
} from '@/utils/systemSettingsPresentation';

interface DockerRuntimeSettingsCardProps {
  disableDockerUpdateActions: Accessor<boolean>;
  disableDockerUpdateActionsLocked: Accessor<boolean>;
  savingDockerUpdateActions: Accessor<boolean>;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
}

export const DockerRuntimeSettingsCard: Component<DockerRuntimeSettingsCardProps> = (props) => (
  <div class="rounded-xl border border-border bg-surface p-5 shadow-sm">
    <div class="space-y-4">
      <div class="space-y-1">
        <h3 class="text-base font-semibold text-base-content">
          {DOCKER_UPDATE_ACTIONS_SECTION_TITLE}
        </h3>
        <p class="text-sm text-muted">{DOCKER_UPDATE_ACTIONS_SECTION_DESCRIPTION}</p>
      </div>

      <div class="flex items-start justify-between gap-4 rounded-md border border-border bg-surface-hover p-4">
        <div class="flex-1 space-y-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-base-content">
              {DOCKER_UPDATE_ACTIONS_TOGGLE_LABEL}
            </span>
            <Show when={props.disableDockerUpdateActionsLocked()}>
              <EnvironmentLockBadge
                envVar={DOCKER_UPDATE_ACTIONS_ENV_VAR}
                icon={(props) => (
                  <svg
                    class={props.class}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                    />
                  </svg>
                )}
              />
            </Show>
          </div>
          <p class="text-xs text-muted">{DOCKER_UPDATE_ACTIONS_TOGGLE_DESCRIPTION}</p>
          <p class="text-xs text-muted mt-1">
            Can also be set via environment variable:{' '}
            <code class="px-1 py-0.5 rounded bg-surface-hover text-base-content">
              {DOCKER_UPDATE_ACTIONS_ENV_VAR}=true
            </code>
          </p>
        </div>
        <div class="flex-shrink-0">
          <button
            type="button"
            onClick={() =>
              props.handleDisableDockerUpdateActionsChange(!props.disableDockerUpdateActions())
            }
            disabled={props.disableDockerUpdateActionsLocked() || props.savingDockerUpdateActions()}
            class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
              props.disableDockerUpdateActions() ? 'bg-blue-600' : 'bg-surface-alt'
            } ${props.disableDockerUpdateActionsLocked() ? 'opacity-50 cursor-not-allowed' : ''}`}
            role="switch"
            aria-checked={props.disableDockerUpdateActions()}
            title={
              props.disableDockerUpdateActionsLocked() ? ENVIRONMENT_LOCK_BUTTON_TITLE : undefined
            }
          >
            <span
              class={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform ${
                props.disableDockerUpdateActions() ? 'translate-x-6' : 'translate-x-1'
              }`}
            />
          </button>
        </div>
      </div>
    </div>
  </div>
);
