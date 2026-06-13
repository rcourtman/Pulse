import { Show } from 'solid-js';
import X from 'lucide-solid/icons/x';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import type { Alert } from '@/types/api';
import {
  ALERT_BULK_EDIT_CLEAR_LABEL,
  getAlertBulkEditOpenLabel,
} from '@/utils/alertBulkEditPresentation';
import { ActionIconButton } from '@/components/shared/Button';
import { useAlertResourceTableState } from './useAlertResourceTableState';
import type { GroupHeaderMeta, Resource } from '@/features/alerts/thresholds/tableTypes';
import { AlertResourceTableDesktop } from './AlertResourceTableDesktop';
import { AlertResourceTableMobile } from './AlertResourceTableMobile';

export type { GroupHeaderMeta, Resource } from '@/features/alerts/thresholds/tableTypes';

export interface ResourceTableProps {
  title: string;
  resources?: Resource[];
  groupedResources?: Record<string, Resource[]>;
  columns: string[];
  activeAlerts?: Record<string, Alert>;
  emptyMessage?: string;
  onEdit: (
    resourceId: string,
    thresholds: Record<string, number | undefined>,
    defaults: Record<string, number | undefined>,
    note: string | undefined,
  ) => void;
  onSaveEdit: (resourceId: string) => void;
  onCancelEdit: () => void;
  onRemoveOverride: (resourceId: string) => void;
  onToggleDisabled?: (resourceId: string) => void;
  onToggleNodeConnectivity?: (nodeId: string) => void;
  showOfflineAlertsColumn?: boolean; // Show separate column for offline/connectivity alerts
  globalOfflineSeverity?: 'warning' | 'critical';
  onSetGlobalOfflineState?: (state: OfflineState) => void;
  onSetOfflineState?: (resourceId: string, state: OfflineState) => void;
  onToggleBackup?: (resourceId: string, forceState?: boolean) => void;
  onToggleSnapshot?: (resourceId: string, forceState?: boolean) => void;
  showDelayColumn?: boolean;
  globalDelaySeconds?: number;
  editingId: () => string | null;
  editingThresholds: () => Record<string, number | undefined>;
  setEditingThresholds: (value: Record<string, number | undefined>) => void;
  formatMetricValue: (metric: string, value: number | undefined) => string;
  hasActiveAlert: (resourceId: string, metric: string) => boolean;
  globalDefaults?: Record<string, number | undefined>;
  setGlobalDefaults?: (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => void;
  setHasUnsavedChanges?: (value: boolean) => void;
  globalDisableFlag?: () => boolean;
  onToggleGlobalDisable?: () => void;
  globalDisableOfflineFlag?: () => boolean;
  onToggleGlobalDisableOffline?: () => void;
  metricDelaySeconds?: Record<string, number>;
  onMetricDelayChange?: (metricKey: string, value: number | null) => void;
  groupHeaderMeta?: Record<string, GroupHeaderMeta>;
  factoryDefaults?: Record<string, number | undefined>;
  onResetDefaults?: () => void;
  editingNote: () => string;
  setEditingNote: (value: string) => void;
  onBulkEdit?: (resourceIds: string[]) => void;
}

export type OfflineState = 'off' | 'warning' | 'critical';

export function ResourceTable(props: ResourceTableProps) {
  const { isMobile } = useBreakpoint();
  const {
    activeMetricInput,
    setActiveMetricInput,
    showDelayRow,
    setShowDelayRow,
    selectedIds,
    hasRows,
    hasCustomGlobalDefaults,
    toggleSelection,
    toggleAll,
    allSelected,
    someSelected,
    clearSelectedIds,
  } = useAlertResourceTableState({
    resources: props.resources,
    groupedResources: props.groupedResources,
    globalDefaults: props.globalDefaults,
    factoryDefaults: props.factoryDefaults,
  });

  return (
    <>
      <Show
        when={!isMobile()}
        fallback={
          <AlertResourceTableMobile
            table={props}
            hasRows={hasRows}
            hasCustomGlobalDefaults={hasCustomGlobalDefaults}
            setActiveMetricInput={setActiveMetricInput}
          />
        }
      >
        <AlertResourceTableDesktop
          table={props}
          hasRows={hasRows}
          hasCustomGlobalDefaults={hasCustomGlobalDefaults}
          activeMetricInput={activeMetricInput}
          setActiveMetricInput={setActiveMetricInput}
          showDelayRow={showDelayRow}
          setShowDelayRow={setShowDelayRow}
          selectedIds={selectedIds}
          toggleSelection={toggleSelection}
          toggleAll={toggleAll}
          allSelected={allSelected}
          someSelected={someSelected}
        />
      </Show>

      <Show when={selectedIds().size > 0 && props.onBulkEdit}>
        <div class="fixed bottom-8 left-1/2 -translate-x-1/2 bg-base border border-border shadow-2xl rounded-full px-5 py-3 flex items-center gap-6 z-[100] animate-in slide-in-from-bottom-5">
          <span class="text-sm font-medium text-white">
            {selectedIds().size} <span class="text-slate-400">selected</span>
          </span>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="bg-blue-600 hover:bg-blue-500 text-white rounded-full px-5 py-1.5 text-sm font-medium transition-colors shadow-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
              onClick={() => {
                if (props.onBulkEdit) {
                  props.onBulkEdit(Array.from(selectedIds()));
                  clearSelectedIds();
                }
              }}
            >
              {getAlertBulkEditOpenLabel()}
            </button>
            <ActionIconButton
              class="rounded-full bg-surface text-slate-400 hover:bg-slate-700 hover:text-white focus-visible:ring-offset-0"
              onClick={clearSelectedIds}
              label={ALERT_BULK_EDIT_CLEAR_LABEL}
              size="sm"
              tone="neutral"
            >
              <X class="w-4 h-4" aria-hidden="true" />
            </ActionIconButton>
          </div>
        </div>
      </Show>
    </>
  );
}
