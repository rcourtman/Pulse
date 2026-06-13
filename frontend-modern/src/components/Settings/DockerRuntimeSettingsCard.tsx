import { Component, Accessor, Show } from 'solid-js';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { ENVIRONMENT_LOCK_BUTTON_TITLE } from '@/utils/environmentLockPresentation';
import {
  DOCKER_UPDATE_ACTIONS_ENV_VAR,
  getDockerUpdateActionsPresentation,
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
          {getDockerUpdateActionsPresentation().sectionTitle}
        </h3>
        <p class="text-sm text-muted">{getDockerUpdateActionsPresentation().sectionDescription}</p>
      </div>

      <div class="flex items-start justify-between gap-4 rounded-md border border-border bg-surface-hover p-4">
        <div class="flex-1 space-y-1">
          <div class="flex items-center gap-2">
            <span
              id="docker-update-actions-toggle-label"
              class="text-sm font-medium text-base-content"
            >
              {getDockerUpdateActionsPresentation().toggleLabel}
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
          <p id="docker-update-actions-toggle-description" class="text-xs text-muted">
            {getDockerUpdateActionsPresentation().toggleDescription}
          </p>
          <p class="text-xs text-muted mt-1">
            {getDockerUpdateActionsPresentation().environmentHint}{' '}
            <code class="px-1 py-0.5 rounded bg-surface-hover text-base-content">
              {DOCKER_UPDATE_ACTIONS_ENV_VAR}=true
            </code>
          </p>
        </div>
        <div class="flex-shrink-0">
          <TogglePrimitive
            checked={props.disableDockerUpdateActions()}
            onChange={(event) =>
              props.handleDisableDockerUpdateActionsChange(event.currentTarget.checked)
            }
            disabled={props.disableDockerUpdateActionsLocked() || props.savingDockerUpdateActions()}
            ariaLabelledBy="docker-update-actions-toggle-label"
            ariaDescribedBy="docker-update-actions-toggle-description"
            title={
              props.disableDockerUpdateActionsLocked() ? ENVIRONMENT_LOCK_BUTTON_TITLE : undefined
            }
          />
        </div>
      </div>
    </div>
  </div>
);
