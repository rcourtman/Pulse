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
  readOnly?: Accessor<boolean>;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
}

export const DockerRuntimeSettingsCard: Component<DockerRuntimeSettingsCardProps> = (props) => {
  const isReadOnly = () => Boolean(props.readOnly?.());
  return (
    <SettingsPanel title={getDockerUpdateActionsPresentation().sectionTitle} noPadding>
      <div class="space-y-2 p-2.5 sm:p-6">
        <p class="text-xs leading-relaxed text-muted">
          {getDockerUpdateActionsPresentation().sectionDescription}
        </p>
        <div class="flex min-w-0 items-center justify-between gap-3 pt-1">
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
                icon={(iconProps) => (
                  <svg
                    class={iconProps.class}
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
            onChange={(event) => {
              if (isReadOnly()) return;
              void props.handleDisableDockerUpdateActionsChange(event.currentTarget.checked);
            }}
            disabled={
              isReadOnly() ||
              props.disableDockerUpdateActionsLocked() ||
              props.savingDockerUpdateActions()
            }
            ariaLabelledBy="docker-update-actions-toggle-label"
            ariaDescribedBy="docker-update-actions-toggle-description"
            title={
              props.disableDockerUpdateActionsLocked() ? ENVIRONMENT_LOCK_BUTTON_TITLE : undefined
            }
          />
        </div>
        <p id="docker-update-actions-toggle-description" class="text-xs leading-relaxed text-muted">
          {getDockerUpdateActionsPresentation().toggleDescription}
        </p>
        <p class="min-w-0 text-xs text-muted">
          <span>{getDockerUpdateActionsPresentation().environmentHint} </span>
          <code class="mt-1 block max-w-full overflow-x-auto whitespace-nowrap rounded-sm bg-surface-hover px-1.5 py-0.5 text-base-content sm:mt-0 sm:inline sm:overflow-visible">
            {DOCKER_UPDATE_ACTIONS_ENV_VAR}=true
          </code>
        </p>
      </div>
    </SettingsPanel>
  );
};
