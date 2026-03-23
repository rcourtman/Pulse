import { Component, Match, Show, Switch } from 'solid-js';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import {
  getContainerUpdateBadgeTooltip,
  getContainerUpdateErrorTooltip,
  getUpdateButtonClass,
  getUpdateIconTooltip,
  hasContainerUpdate,
  hasContainerUpdateError,
  type ContainerUpdateBadgeProps,
  type UpdateButtonProps,
  type UpdateIconProps,
} from './containerUpdateBadgeModel';
import { useContainerUpdateButtonState } from './useContainerUpdateButtonState';

export type {
  ContainerUpdateBadgeProps,
  UpdateButtonProps,
  UpdateIconProps,
  UpdateState,
} from './containerUpdateBadgeModel';

const UpdateArrowIcon: Component<{ class?: string }> = (props) => (
  <svg class={props.class} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
    />
  </svg>
);

const ErrorIndicatorIcon: Component<{ class?: string }> = (props) => (
  <svg class={props.class} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
    />
  </svg>
);

const SpinnerIcon: Component<{ class?: string }> = (props) => (
  <svg class={props.class} fill="none" viewBox="0 0 24 24">
    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
    <path
      class="opacity-75"
      fill="currentColor"
      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
    />
  </svg>
);

const CheckIcon: Component<{ class?: string }> = (props) => (
  <svg class={props.class} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
  </svg>
);

const XIcon: Component<{ class?: string }> = (props) => (
  <svg class={props.class} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
  </svg>
);

/**
 * ContainerUpdateBadge displays a visual indicator when a container image has an update available.
 * Uses a blue color scheme to differentiate from health/status badges.
 */
export const ContainerUpdateBadge: Component<ContainerUpdateBadgeProps> = (props) => {
  return (
    <Show when={hasContainerUpdate(props.updateStatus) || hasContainerUpdateError(props.updateStatus)}>
      <Show
        when={hasContainerUpdate(props.updateStatus)}
        fallback={
          <Show when={hasContainerUpdateError(props.updateStatus)}>
            <span
              class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-surface-alt text-muted cursor-help"
              onMouseEnter={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                showTooltip(getContainerUpdateErrorTooltip(props.updateStatus), rect.left + rect.width / 2, rect.top, {
                  align: 'center',
                  direction: 'up',
                });
              }}
              onMouseLeave={() => hideTooltip()}
            >
              <ErrorIndicatorIcon class="w-3 h-3" />
              <Show when={!props.compact}>
                <span>Check failed</span>
              </Show>
            </span>
          </Show>
        }
      >
        <span
          class="inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-help"
          onMouseEnter={(e) => {
            const rect = e.currentTarget.getBoundingClientRect();
            showTooltip(getContainerUpdateBadgeTooltip(props.updateStatus), rect.left + rect.width / 2, rect.top, {
              align: 'center',
              direction: 'up',
            });
          }}
          onMouseLeave={() => hideTooltip()}
        >
          <UpdateArrowIcon class="w-3 h-3" />
          <Show when={!props.compact}>
            <span>Update</span>
          </Show>
        </span>
      </Show>
    </Show>
  );
};

/**
 * Compact version of UpdateBadge - just an icon with no text.
 * Use this in table cells where space is limited.
 */
export const UpdateIcon: Component<UpdateIconProps> = (props) => {
  return (
    <Show when={hasContainerUpdate(props.updateStatus)}>
      <span
        class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-help"
        onMouseEnter={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          showTooltip(getUpdateIconTooltip(props.updateStatus), rect.left + rect.width / 2, rect.top, {
            align: 'center',
            direction: 'up',
          });
        }}
        onMouseLeave={() => hideTooltip()}
      >
        <UpdateArrowIcon class="w-3 h-3" />
      </span>
    </Show>
  );
};

/**
 * UpdateButton displays a clickable button to trigger container updates.
 * Uses a persistent store to maintain state across WebSocket refreshes.
 *
 * If the server has disabled Docker update actions (via PULSE_DISABLE_DOCKER_UPDATE_ACTIONS
 * or the Settings UI), this component will render a read-only UpdateBadge instead,
 * allowing users to see that updates are available without being able to trigger them.
 *
 * While system settings are loading, the button displays in a disabled/loading state
 * to prevent premature clicks before the server configuration is known.
 */
export const UpdateButton: Component<UpdateButtonProps> = (props) => {
  const state = useContainerUpdateButtonState(props);

  return (
    <Show when={state.hasUpdate()}>
      <Show when={state.settingsLoaded() && state.shouldHideButton()}>
        <ContainerUpdateBadge updateStatus={props.updateStatus} compact={props.compact} />
      </Show>

      <Show when={!state.settingsLoaded() || !state.shouldHideButton()}>
        <div class="inline-flex items-center gap-1" data-prevent-toggle>
          <button
            type="button"
            class={getUpdateButtonClass(state.currentState())}
            onClick={state.handleClick}
            onMouseDown={(e) => {
              e.stopPropagation();
            }}
            disabled={state.isButtonDisabled()}
            data-prevent-toggle
            onMouseEnter={(e) => {
              const rect = e.currentTarget.getBoundingClientRect();
              showTooltip(state.buttonTooltip(), rect.left + rect.width / 2, rect.top, {
                align: 'center',
                direction: 'up',
              });
            }}
            onMouseLeave={() => hideTooltip()}
          >
            <Show when={!state.settingsLoaded()}>
              <UpdateArrowIcon class="w-3 h-3 animate-pulse opacity-50" />
            </Show>
            <Show when={state.settingsLoaded()}>
              <Switch>
                <Match when={state.currentState() === 'updating'}>
                  <SpinnerIcon class="w-3 h-3 animate-spin" />
                </Match>
                <Match when={state.currentState() === 'success'}>
                  <CheckIcon class="w-3 h-3" />
                </Match>
                <Match when={state.currentState() === 'error'}>
                  <XIcon class="w-3 h-3" />
                </Match>
                <Match when={state.currentState() === 'idle' || state.currentState() === 'confirming'}>
                  <UpdateArrowIcon class="w-3 h-3" />
                </Match>
              </Switch>
            </Show>
            <Show when={!props.compact}>
              <span class={!state.settingsLoaded() ? 'opacity-50' : ''}>
                {state.buttonLabel()}
              </span>
            </Show>
          </button>
          <Show when={state.settingsLoaded() && state.currentState() === 'confirming'}>
            <button
              type="button"
              class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-surface-alt text-muted hover:bg-surface-hover transition-colors"
              onClick={state.handleCancel}
              title="Cancel"
            >
              <XIcon class="w-3 h-3" />
            </button>
          </Show>
        </div>
      </Show>
    </Show>
  );
};

export default ContainerUpdateBadge;
