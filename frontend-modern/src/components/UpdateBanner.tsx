import { Show, createSignal, createEffect, createMemo, For } from 'solid-js';
import { updateStore } from '@/stores/updates';
import { UpdatesAPI, type UpdatePlan } from '@/api/updates';
import { UpdateConfirmationModal } from './UpdateConfirmationModal';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import { buildReleaseNotesUrl } from '@/components/updateVersion';

export function UpdateBanner() {
  const [isExpanded, setIsExpanded] = createSignal(false);
  const [updatePlan, setUpdatePlan] = createSignal<UpdatePlan | null>(null);
  const [showConfirmModal, setShowConfirmModal] = createSignal(false);
  const [isApplying, setIsApplying] = createSignal(false);
  const [copiedIndex, setCopiedIndex] = createSignal<number | null>(null);
  let latestPlanRequestID = 0;

  // Fetch update plan when update info is available
  createEffect(() => {
    const info = updateStore.updateInfo();
    const version = info?.available ? info.latestVersion?.trim() : '';
    const requestID = ++latestPlanRequestID;
    setUpdatePlan(null);

    if (!version) {
      return;
    }

    void UpdatesAPI.getUpdatePlan(version)
      .then((plan) => {
        if (requestID === latestPlanRequestID) {
          setUpdatePlan(plan);
        }
      })
      .catch((error) => {
        if (requestID === latestPlanRequestID) {
          setUpdatePlan(null);
        }
        logger.error('Failed to fetch update plan', error);
      });
  });

  const releaseNotesUrl = createMemo(() => buildReleaseNotesUrl(updateStore.updateInfo()?.latestVersion));

  const handleApplyUpdate = () => {
    setShowConfirmModal(true);
  };

  const handleConfirmUpdate = async () => {
    const info = updateStore.updateInfo();
    if (!info?.downloadUrl) return;

    setIsApplying(true);
    try {
      await UpdatesAPI.applyUpdate(info.downloadUrl);
      // Close confirmation - GlobalUpdateProgressWatcher will auto-open the progress modal
      setShowConfirmModal(false);
    } catch (error) {
      logger.error('Failed to start update', error);
      alert('Failed to start update. Please try again.');
    } finally {
      setIsApplying(false);
    }
  };

  const handleCopy = async (text: string, index: number) => {
    const success = await copyToClipboard(text);
    if (!success) {
      logger.error('Failed to copy update instruction to clipboard', new Error('clipboard copy failed'));
      return;
    }
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  // Get deployment type message
  const getUpdateInstructions = () => {
    const versionInfo = updateStore.versionInfo();
    const deploymentType = versionInfo?.deploymentType || 'systemd';

    switch (deploymentType) {
      case 'proxmoxve':
        return "ProxmoxVE users: type 'update' in console";
      case 'docker':
        return 'Docker: pull latest image';
      case 'source':
        return 'Source: pull and rebuild';
      case 'mock':
        return 'Mock environment: updates run automatically for integration tests';
      default:
        return ''; // No message, just the version info
    }
  };

  const getShortMessage = () => {
    const info = updateStore.updateInfo();
    if (!info) return '';
    return `New version available: ${info.latestVersion}`;
  };

  return (
    <Show when={updateStore.isUpdateVisible()}>
      <div class="bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800 text-blue-800 dark:text-blue-200 relative animate-slideDown">
        <div class="px-4 py-1.5">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              {/* Update icon */}
              <svg
                class="w-4 h-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  d="M12 2v6m0 0l3-3m-3 3l-3-3"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M2 17l.621 2.485A2 2 0 0 0 4.561 21h14.878a2 2 0 0 0 1.94-1.515L22 17"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>

              <div class="flex items-center gap-3 flex-wrap">
                <span class="text-sm font-medium">{getShortMessage()}</span>

                {/* Apply Update Button (automated deployments) */}
                <Show when={updatePlan()?.canAutoUpdate && !isExpanded()}>
                  <button
                    onClick={handleApplyUpdate}
                    class="px-3 py-1 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded transition-colors"
                  >
                    Apply Update
                  </button>
                </Show>

                {/* Manual Steps Badge (non-automated deployments) */}
                <Show when={updatePlan() && !updatePlan()?.canAutoUpdate && !isExpanded()}>
                  <span class="px-2 py-0.5 text-xs font-medium bg-orange-100 dark:bg-orange-900 text-orange-800 dark:text-orange-200 rounded">
                    Manual steps required
                  </span>
                </Show>

                {!isExpanded() && getUpdateInstructions() && (
                  <>
                    <span class="text-blue-600 dark:text-blue-400 text-sm hidden sm:inline">•</span>
                    <span class="text-blue-600 dark:text-blue-400 text-sm hidden sm:inline">
                      {getUpdateInstructions()}
                    </span>
                  </>
                )}
                {!isExpanded() && (
                  <a
                    href={releaseNotesUrl()}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-blue-600 dark:text-blue-400 underline text-sm hidden sm:inline hover:text-blue-700 dark:hover:text-blue-300"
                  >
                    View details →
                  </a>
                )}
              </div>
            </div>

            <div class="flex items-center gap-2">
              {/* Expand/Collapse button */}
              <button
                onClick={() => setIsExpanded(!isExpanded())}
                class="p-1 hover:bg-blue-100 dark:hover:bg-blue-800 rounded transition-colors"
                title={isExpanded() ? 'Show less' : 'Show more'}
              >
                <svg
                  class={`w-4 h-4 transform transition-transform ${isExpanded() ? 'rotate-180' : ''}`}
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <polyline points="6 9 12 15 18 9"></polyline>
                </svg>
              </button>

              {/* Dismiss button */}
              <button
                onClick={() => updateStore.dismissUpdate()}
                class="p-1 hover:bg-blue-100 dark:hover:bg-blue-800 rounded transition-colors"
                title="Dismiss this update"
              >
                <svg
                  class="w-4 h-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <line x1="18" y1="6" x2="6" y2="18"></line>
                  <line x1="6" y1="6" x2="18" y2="18"></line>
                </svg>
              </button>
            </div>
          </div>

          {/* Expanded content */}
          <Show when={isExpanded()}>
            <div class="mt-2 pb-1">
              <div class="text-sm text-blue-700 dark:text-blue-300 space-y-1">
                <p>
                  <span class="font-medium">Current:</span>{' '}
                  {updateStore.versionInfo()?.version || 'Unknown'} →
                  <span class="font-medium ml-1">Latest:</span>{' '}
                  {updateStore.updateInfo()?.latestVersion}
                </p>
                {getUpdateInstructions() && (
                  <p>
                    <span class="font-medium">Quick upgrade:</span> {getUpdateInstructions()}
                  </p>
                )}
                <Show when={updateStore.updateInfo()?.isPrerelease}>
                  <p class="text-orange-600 dark:text-orange-400 text-xs">
                    This is a pre-release version
                  </p>
                </Show>

                {/* Manual Update Instructions */}
                <Show when={updatePlan()?.instructions && (updatePlan()?.instructions?.length ?? 0) > 0}>
                  <div class="mt-3 pt-3 border-t border-blue-200 dark:border-blue-800">
                    <div class="font-medium mb-2">Update Instructions:</div>
                    <div class="space-y-2">
                      <For each={updatePlan()?.instructions || []}>
                        {(instruction, index) => (
                          <div class="bg-surface-alt rounded border border-blue-200 dark:border-blue-700 p-2">
                            <div class="flex items-start justify-between gap-2">
                              <code class="text-xs text-base-content font-mono flex-1 break-all">
                                {instruction}
                              </code>
                              <button
                                onClick={() => handleCopy(instruction, index())}
                                class="flex-shrink-0 p-1 hover:bg-blue-100 dark:hover:bg-blue-800 rounded transition-colors"
                                title="Copy to clipboard"
                              >
                                <Show when={copiedIndex() === index()} fallback={
                                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                                  </svg>
                                }>
                                  <svg class="w-4 h-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                                  </svg>
                                </Show>
                              </button>
                            </div>
                          </div>
                        )}
                      </For>
                    </div>
                  </div>
                </Show>

                {/* Apply Update Button (expanded view for automated deployments) */}
                <Show when={updatePlan()?.canAutoUpdate}>
                  <div class="mt-3 pt-3 border-t border-blue-200 dark:border-blue-800">
                    <button
                      onClick={handleApplyUpdate}
                      class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded transition-colors"
                    >
                      Apply Update Automatically
                    </button>
                  </div>
                </Show>

                <div class="flex gap-3 mt-2">
                  <a
                    href={releaseNotesUrl()}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-blue-600 dark:text-blue-400 underline hover:text-blue-700 dark:hover:text-blue-300 text-xs"
                  >
                    View release notes
                  </a>
                  <button
                    onClick={() => updateStore.dismissUpdate()}
                    class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-xs underline"
                  >
                    Don't show again for this version
                  </button>
                </div>
              </div>
            </div>
          </Show>
        </div>
      </div>

      {/* Update Confirmation Modal */}
      <UpdateConfirmationModal
        isOpen={showConfirmModal()}
        onClose={() => setShowConfirmModal(false)}
        onConfirm={handleConfirmUpdate}
        currentVersion={updateStore.versionInfo()?.version || 'Unknown'}
        latestVersion={updateStore.updateInfo()?.latestVersion || ''}
        plan={updatePlan() || {
          canAutoUpdate: false,
          requiresRoot: false,
          rollbackSupport: false,
        }}
        isApplying={isApplying()}
      />
    </Show>
  );
}
