import { Show, createSignal } from 'solid-js';
import { updateStore } from '@/stores/updates';

export function UpdateBanner() {
  const [isExpanded, setIsExpanded] = createSignal(false);
  
  // Get deployment type message
  const getUpdateInstructions = () => {
    const versionInfo = updateStore.versionInfo();
    const deploymentType = versionInfo?.deploymentType || 'systemd';
    
    switch (deploymentType) {
      case 'proxmoxve':
        return "Type 'update' in the ProxmoxVE console";
      case 'docker':
        return 'Pull the latest Docker image and recreate container';
      case 'source':
        return 'Pull latest changes and rebuild';
      default:
        return 'Run the install script to update';
    }
  };
  
  const getShortMessage = () => {
    const info = updateStore.updateInfo();
    if (!info) return '';
    return `Update available: ${info.latestVersion}`;
  };
  
  return (
    <Show when={updateStore.isUpdateVisible()}>
      <div class="bg-gradient-to-r from-blue-600 to-blue-700 text-white relative animate-slideDown">
        <div class="px-4 py-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              {/* Update icon */}
              <svg class="w-4 h-4 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2v6m0 0l3-3m-3 3l-3-3" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M2 17l.621 2.485A2 2 0 0 0 4.561 21h14.878a2 2 0 0 0 1.94-1.515L22 17" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium">{getShortMessage()}</span>
                {!isExpanded() && (
                  <>
                    <span class="text-white/80 text-sm hidden sm:inline">•</span>
                    <span class="text-white/80 text-sm hidden sm:inline">{getUpdateInstructions()}</span>
                  </>
                )}
              </div>
            </div>
            
            <div class="flex items-center gap-2">
              {/* Expand/Collapse button */}
              <button
                onClick={() => setIsExpanded(!isExpanded())}
                class="p-1 hover:bg-white/10 rounded transition-colors"
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
                class="p-1 hover:bg-white/10 rounded transition-colors"
                title="Dismiss this update"
              >
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18"></line>
                  <line x1="6" y1="6" x2="18" y2="18"></line>
                </svg>
              </button>
            </div>
          </div>
          
          {/* Expanded content */}
          <Show when={isExpanded()}>
            <div class="mt-3 pb-1">
              <div class="text-sm text-white/90 space-y-1">
                <p>
                  <span class="font-medium">Current:</span> {updateStore.versionInfo()?.version || 'Unknown'} → 
                  <span class="font-medium ml-1">Latest:</span> {updateStore.updateInfo()?.latestVersion}
                </p>
                <p>
                  <span class="font-medium">How to update:</span> {getUpdateInstructions()}
                </p>
                <Show when={updateStore.updateInfo()?.isPrerelease}>
                  <p class="text-yellow-200 text-xs">This is a pre-release version</p>
                </Show>
                <div class="flex gap-3 mt-2">
                  <a 
                    href={`https://github.com/rcourtman/Pulse/releases/tag/${updateStore.updateInfo()?.latestVersion}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-white/90 underline hover:text-white text-xs"
                  >
                    View release notes
                  </a>
                  <button
                    onClick={() => updateStore.dismissUpdate()}
                    class="text-white/70 hover:text-white text-xs underline"
                  >
                    Don't show again for this version
                  </button>
                </div>
              </div>
            </div>
          </Show>
        </div>
      </div>
    </Show>
  );
}