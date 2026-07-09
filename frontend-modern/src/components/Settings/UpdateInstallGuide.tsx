import { Component, For, Show } from 'solid-js';
import Download from 'lucide-solid/icons/download';
import AlertTriangle from 'lucide-solid/icons/alert-triangle';
import CheckCircle from 'lucide-solid/icons/check-circle';
import CircleAlert from 'lucide-solid/icons/circle-alert';
import type {
  UpdateInfo,
  UpdatePlan,
  UpdateReadiness,
  UpdateReadinessCheck,
  VersionInfo,
} from '@/api/updates';
import { CopyCommandBlock } from '@/components/Settings/CopyCommandBlock';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import {
  buildIdleDockerComposeCommand,
  buildIdleDockerUpdateCommand,
  buildUpdateInstallGuide,
  type UpdateInstallStep,
} from '@/components/Settings/updatesSettingsModel';

interface UpdateInstallGuideProps {
  versionInfo: VersionInfo | null;
  updateInfo: UpdateInfo | null;
  updatePlan: UpdatePlan | null;
  isInstalling: boolean;
  dockerImageTag: string;
  systemdDownloadCommand: string;
  onInstallUpdate: () => void;
}

function StepIndex(props: { index: number }) {
  return (
    <span class="flex h-6 w-6 items-center justify-center rounded-full bg-green-200 text-xs font-bold text-green-700 dark:bg-green-800 dark:text-green-300">
      {props.index}
    </span>
  );
}

function InstallStep(props: { step: UpdateInstallStep; index: number }) {
  return (
    <div class="space-y-3">
      <Show when={props.step.title}>
        <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
          <StepIndex index={props.index} />
          {props.step.title}
        </div>
      </Show>
      <Show when={props.step.command}>
        <CopyCommandBlock
          command={props.step.command!}
          containerClass="ml-0 sm:ml-8 relative group"
          codeClass={
            props.step.commandCodeClass ??
            'block rounded-md border border-border bg-base p-3 font-mono text-sm text-green-400'
          }
        />
      </Show>
      <Show when={props.step.note}>
        <p class="ml-0 text-xs text-green-600 dark:text-green-400 sm:ml-8">{props.step.note}</p>
      </Show>
    </div>
  );
}

function readinessTone(status: UpdateReadiness['status']) {
  switch (status) {
    case 'blocked':
      return 'border-red-200 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-950/40 dark:text-red-100';
    case 'attention':
      return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100';
    default:
      return 'border-green-200 bg-green-50 text-green-900 dark:border-green-800 dark:bg-green-950/40 dark:text-green-100';
  }
}

function readinessCheckTone(status: UpdateReadinessCheck['status']) {
  switch (status) {
    case 'blocked':
      return 'text-red-700 dark:text-red-300';
    case 'warning':
      return 'text-amber-700 dark:text-amber-300';
    default:
      return 'text-green-700 dark:text-green-300';
  }
}

function ReadinessCheckIcon(props: { status: UpdateReadinessCheck['status'] }) {
  if (props.status === 'blocked') {
    return <CircleAlert class="h-4 w-4" />;
  }
  if (props.status === 'warning') {
    return <AlertTriangle class="h-4 w-4" />;
  }
  return <CheckCircle class="h-4 w-4" />;
}

function UpdateReadinessPanel(props: { readiness: UpdateReadiness }) {
  return (
    <section class={`rounded-md border p-4 ${readinessTone(props.readiness.status)}`}>
      <div class="flex flex-col gap-1">
        <p class="text-sm font-semibold">Upgrade checks</p>
        <p class="text-xs opacity-90">{props.readiness.summary}</p>
      </div>
      <div class="mt-3 divide-y divide-black/10 dark:divide-white/10">
        <For each={props.readiness.checks}>
          {(check) => (
            <div class="py-3 first:pt-0 last:pb-0">
              <div class={`flex items-start gap-2 ${readinessCheckTone(check.status)}`}>
                <span class="mt-0.5 flex-shrink-0">
                  <ReadinessCheckIcon status={check.status} />
                </span>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium">{check.title}</p>
                  <p class="mt-0.5 text-xs opacity-90">{check.summary}</p>
                  <Show when={check.details?.length}>
                    <ul class="mt-2 list-disc space-y-1 pl-4 text-xs opacity-85">
                      <For each={check.details ?? []}>{(detail) => <li>{detail}</li>}</For>
                    </ul>
                  </Show>
                </div>
              </div>
            </div>
          )}
        </For>
      </div>
    </section>
  );
}

export const UpdateInstallGuide: Component<UpdateInstallGuideProps> = (props) => {
  const guide = () =>
    buildUpdateInstallGuide(
      props.versionInfo,
      props.updateInfo,
      props.updatePlan,
      props.dockerImageTag,
      props.systemdDownloadCommand,
    );
  const readinessBlocked = () => props.updatePlan?.readiness?.status === 'blocked';
  const introText = () =>
    readinessBlocked()
      ? 'Resolve blocked upgrade checks before installing. Manual steps are listed for reference:'
      : guide()!.introText;

  return (
    <>
      <Show when={props.versionInfo?.isDocker && !props.updateInfo?.available}>
        <div class="space-y-3 rounded-md border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900">
          <div class="flex items-center gap-2">
            <svg
              class="h-5 w-5 flex-shrink-0 text-blue-600 dark:text-blue-400"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.186.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.888c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.186.186 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 00.185-.185V9.006a.185.185 0 00-.185-.186h-2.119a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-.001 2.716h2.118a.185.185 0 00.185-.185v-1.888a.185.185 0 00-.185-.185H2.136a.185.185 0 00-.186.185v1.888c0 .102.084.185.186.185m23.063-3.167a.509.509 0 00-.376-.25.431.431 0 00-.116-.01.431.431 0 00-.114.01 3.6 3.6 0 00-1.618.877c-.186.166-.356.36-.509.577a6.6 6.6 0 00-1.117-1.474 6.6 6.6 0 00-9.336 0 6.6 6.6 0 00-1.938 4.684 6.6 6.6 0 001.938 4.684 6.6 6.6 0 004.668 1.938 6.6 6.6 0 004.668-1.938 6.6 6.6 0 001.938-4.684 6.6 6.6 0 00-.185-1.41 3.6 3.6 0 001.587-.904.509.509 0 00.134-.459" />
            </svg>
            <p class="text-sm font-medium text-blue-800 dark:text-blue-200">Docker Installation</p>
          </div>
          <p class="text-xs text-blue-700 dark:text-blue-300">
            Updates are managed through Docker. Use these commands to check for and apply updates:
          </p>
          <div class="space-y-2">
            <CopyCommandBlock
              command={buildIdleDockerComposeCommand()}
              codeClass="block rounded-md border border-border bg-base p-2.5 font-mono text-xs text-blue-400"
            />
            <p class="text-[10px] text-blue-600 dark:text-blue-400">
              Not using Compose?{' '}
              <code class="rounded bg-blue-100 px-1 py-0.5 text-[10px] dark:bg-blue-800">
                {buildIdleDockerUpdateCommand()}
              </code>{' '}
              then re-run your original docker run command. A plain docker restart keeps the old
              image running.
            </p>
          </div>
        </div>
      </Show>

      <Show when={props.versionInfo?.isSourceBuild}>
        <div class="rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900">
          <p class="text-xs text-blue-800 dark:text-blue-200">
            <strong>Built from source:</strong> Pull the latest code from git and rebuild to
            update.
          </p>
        </div>
      </Show>

      <Show when={props.updateInfo?.warning}>
        <div class="rounded-md border border-amber-200 bg-amber-50 p-3 dark:border-amber-700 dark:bg-amber-900">
          <p class="text-xs text-amber-800 dark:text-amber-200">{props.updateInfo?.warning}</p>
        </div>
      </Show>

      <Show when={props.updateInfo?.available && guide()}>
        <div class="overflow-hidden rounded-md border border-green-200 bg-green-50 dark:border-green-700 dark:bg-green-900">
          <div class="border-b border-green-200 bg-green-100 px-5 py-4 dark:border-green-800 dark:bg-green-800">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-3">
                <div class="rounded-md bg-green-100 p-2 dark:bg-green-900">
                  <svg
                    class="h-5 w-5 text-green-700 dark:text-green-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                    />
                  </svg>
                </div>
                <div>
                  <h4 class="text-base font-semibold text-green-900 dark:text-green-100">
                    {guide()!.headerTitle}
                  </h4>
                  <p class="text-xs text-green-700 dark:text-green-300">{guide()!.headerSummary}</p>
                </div>
              </div>

              <Show when={props.updatePlan?.canAutoUpdate}>
                <button
                  type="button"
                  onClick={props.onInstallUpdate}
                  disabled={props.isInstalling || readinessBlocked()}
                  class={`flex w-full items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all sm:w-auto ${
                    props.isInstalling
                      ? 'cursor-not-allowed bg-green-400 text-white dark:bg-green-600'
                      : readinessBlocked()
                        ? 'cursor-not-allowed bg-red-200 text-red-800 dark:bg-red-900 dark:text-red-200'
                      : 'bg-green-600 text-white hover:bg-green-700'
                  }`}
                >
                  <Show
                    when={props.isInstalling}
                    fallback={
                      <Show
                        when={readinessBlocked()}
                        fallback={
                          <>
                            <Download class="h-4 w-4" />
                            Install Update
                          </>
                        }
                      >
                        <CircleAlert class="h-4 w-4" />
                        Install blocked
                      </Show>
                    }
                  >
                    <LoadingSpinner size="md" tone="inverse" />
                    Installing...
                  </Show>
                </button>
              </Show>
            </div>
          </div>

          <div class="space-y-4 p-5">
            <Show when={props.updatePlan?.readiness}>
              {(readiness) => <UpdateReadinessPanel readiness={readiness()} />}
            </Show>

            <div class="mb-3 text-sm text-green-700 dark:text-green-300">{introText()}</div>

            <For each={guide()!.steps}>
              {(step, index) => <InstallStep step={step} index={index() + 1} />}
            </For>
          </div>

          <Show when={props.updateInfo?.releaseNotes}>
            <div class="border-t border-green-200 bg-surface px-5 py-3 dark:border-green-800">
              <details class="group">
                <summary class="flex cursor-pointer items-center gap-2 text-sm font-medium text-green-700 transition-colors hover:text-green-800 dark:text-green-300 dark:hover:text-green-200">
                  <svg
                    class="h-4 w-4 transition-transform group-open:rotate-90"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                  </svg>
                  View Release Notes
                </summary>
                <pre class="mt-3 max-h-64 overflow-y-auto rounded-md border border-border bg-surface-alt p-4 font-mono text-xs whitespace-pre-wrap text-base-content">
                  {props.updateInfo?.releaseNotes}
                </pre>
              </details>
            </div>
          </Show>
        </div>
      </Show>
    </>
  );
};

export default UpdateInstallGuide;
