import { Component, Accessor, Show } from 'solid-js';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import SettingsPanel from '@/components/shared/SettingsPanel';
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
  <SettingsPanel
    title={getDockerUpdateActionsPresentation().sectionTitle}
    description={getDockerUpdateActionsPresentation().sectionDescription}
    noPadding
  >
    <div class="p-2.5 sm:p-4">
      <div class="flex min-w-0 items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <span
            id="docker-update-actions-toggle-label"
            class="text-xs font-medium text-base-content sm:text-sm"
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
      <p
        id="docker-update-actions-toggle-description"
        class="mt-2 line-clamp-2 text-[11px] text-muted sm:text-xs"
      >
        {getDockerUpdateActionsPresentation().toggleDescription}
      </p>
      <p class="mt-1 min-w-0 text-[10px] text-muted sm:text-xs">
        <span class="hidden sm:inline">
          {getDockerUpdateActionsPresentation().environmentHint}{' '}
        </span>
        <code class="block max-w-full overflow-x-auto whitespace-nowrap rounded bg-surface-hover px-1 py-0.5 text-base-content sm:inline sm:overflow-visible">
          {DOCKER_UPDATE_ACTIONS_ENV_VAR}=true
        </code>
      </p>
    </div>
  </SettingsPanel>
);
