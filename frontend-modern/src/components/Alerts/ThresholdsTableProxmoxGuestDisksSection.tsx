import { Show } from 'solid-js';
import HardDrive from 'lucide-solid/icons/hard-drive';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxGuestDisksSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection('guestDisks')}>
      <CollapsibleSection
        id="guestDisks"
        title={state.sectionTitles.guestDisks}
        resourceCount={state.guestDisksWithOverrides().length}
        collapsed={state.isCollapsed('guestDisks')}
        onToggle={() => state.toggleSection('guestDisks')}
        icon={<HardDrive class="w-5 h-5" />}
        isGloballyDisabled={tableProps.disableAllGuests()}
        emptyMessage={state.GUEST_DISKS_EMPTY_STATE}
      >
        <div ref={state.registerSection('guestDisks')} class="scroll-mt-24">
          <ResourceTable
            title=""
            onConfigureResourceIntent={tableProps.onConfigureResourceIntent}
            groupedResources={state.guestDisksGroupedByGuest()}
            columns={['Disk %']}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage={state.GUEST_DISKS_FILTER_EMPTY_STATE}
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            onToggleDisabled={state.toggleDisabled}
            showOfflineAlertsColumn={false}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, ['Disk %'])}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={{ disk: tableProps.guestDefaults.disk }}
            setGlobalDefaults={(value) => {
              if (typeof value === 'function') {
                const nextValue = value({ disk: tableProps.guestDefaults.disk });
                tableProps.setGuestDefaults((prev) => ({ ...prev, disk: nextValue.disk }));
              } else {
                tableProps.setGuestDefaults((prev) => ({ ...prev, disk: value.disk }));
              }
            }}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}
