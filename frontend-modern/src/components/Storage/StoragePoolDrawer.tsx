import { Component, Show, createMemo, createSignal } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import type { PhysicalDiskAlertResourceIdResolver } from '@/features/storageBackups/diskPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import { getStoragePoolStateIndicator } from '@/features/storageBackups/rowPresentation';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { StoragePoolDetail } from './StoragePoolDetail';

interface StoragePoolDrawerProps {
  record: StorageRecord;
  physicalDisks: Resource[];
  getDiskAlertResourceIds?: PhysicalDiskAlertResourceIdResolver;
  /** The pool's own open alerts, matched the way its row matches them. */
  alerts?: Alert[];
  onClose: () => void;
}

type StoragePoolDrawerTab = 'overview' | 'manage';

const StoragePoolDrawerContent: Component<StoragePoolDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<StoragePoolDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const headingId = () => `storage-pool-drawer-heading-${props.record.id}`;
  // The same judgement that colours the row's State cell.
  const headerIndicator = createMemo(() => getStoragePoolStateIndicator(props.record));

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="storage-pool-drawer">
      <ObjectDrawerHeader
        collapseLabel={`Collapse ${props.record.name} details`}
        onCollapse={props.onClose}
      >
        <DrawerSubjectHeading
          headingId={headingId()}
          title={props.record.name}
          statusVariant={headerIndicator().variant}
          statusLabel={headerIndicator().label}
        />
      </ObjectDrawerHeader>

      <Subtabs
        class="mb-1"
        ariaLabel="Storage pool drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as StoragePoolDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'manage', label: 'Manage' },
        ]}
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={`[overflow-anchor:none] ${activeTab() === 'overview' ? '' : 'hidden'}`}>
        <StoragePoolDetail
          record={props.record}
          physicalDisks={props.physicalDisks}
          getDiskAlertResourceIds={props.getDiskAlertResourceIds}
          alerts={props.alerts}
          historyRange={historyRange()}
          onHistoryRangeChange={setHistoryRange}
          historyEnabled={activeTab() === 'overview'}
        />
      </div>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
        <Show when={activeTab() === 'manage'}>
          <div data-testid="storage-pool-manage-tab">
            {/* A pool row is one unified storage resource; its operator state
                is keyed on that resource like every other drawer's. */}
            <ResourceOperatorStateSection
              resourceId={props.record.id}
              resourceType="storage"
              platformType={props.record.source.platform}
            />
          </div>
        </Show>
      </div>
    </section>
  );
};

// Another pool must not inherit this pool's tab, range or operator form.
export const StoragePoolDrawer: Component<StoragePoolDrawerProps> = (props) => (
  <Show when={props.record.id} keyed>
    {(_recordId) => <StoragePoolDrawerContent {...props} />}
  </Show>
);
