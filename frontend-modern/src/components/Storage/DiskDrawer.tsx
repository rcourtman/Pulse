import { Component, Show, createMemo, createSignal } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { ObjectDrawerHeader } from '@/components/shared/ObjectDrawerHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import {
  extractPhysicalDiskPresentationData,
  getPhysicalDiskHealthIndicator,
} from '@/features/storageBackups/diskPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { DiskDrawerOverview } from './DiskDrawerOverview';

interface DiskDrawerProps {
  disk: Resource;
  /** Alert override keys of the machine whose agent reports the disk. */
  alertResourceIds?: string[];
  /** The disk's own open alerts. */
  alerts?: Alert[];
  onClose: () => void;
}

type DiskDrawerTab = 'overview' | 'manage';

const DiskDrawerContent: Component<DiskDrawerProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<DiskDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  // Judged under the same thresholds as the disk's row, so the drawer's
  // verdict and dot match its Health cell.
  const diskData = createMemo(() =>
    extractPhysicalDiskPresentationData(
      props.disk,
      getDiskTemperatureThresholds,
      props.alertResourceIds,
    ),
  );
  const title = () => diskData().model || props.disk.name || 'Unknown Disk';
  const headingId = () => `disk-drawer-heading-${props.disk.id}`;
  const headerIndicator = createMemo(() => getPhysicalDiskHealthIndicator(diskData()));

  return (
    <section class="space-y-3" aria-labelledby={headingId()} data-testid="disk-drawer">
      <ObjectDrawerHeader collapseLabel={`Collapse ${title()} details`} onCollapse={props.onClose}>
        <DrawerSubjectHeading
          headingId={headingId()}
          title={title()}
          statusVariant={headerIndicator().variant}
          statusLabel={headerIndicator().label}
        />
      </ObjectDrawerHeader>

      <Subtabs
        class="mb-1"
        ariaLabel="Disk drawer sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as DiskDrawerTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          { value: 'manage', label: 'Manage' },
        ]}
      />

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
          overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={`[overflow-anchor:none] ${activeTab() === 'overview' ? '' : 'hidden'}`}>
        <DiskDrawerOverview
          disk={props.disk}
          diskData={diskData()}
          alerts={props.alerts}
          historyRange={historyRange()}
          onHistoryRangeChange={setHistoryRange}
          historyEnabled={activeTab() === 'overview'}
        />
      </div>

      <div class={`[overflow-anchor:none] ${activeTab() === 'manage' ? '' : 'hidden'}`}>
        <Show when={activeTab() === 'manage'}>
          <div data-testid="disk-manage-tab">
            <ResourceOperatorStateSection
              resourceId={props.disk.id}
              resourceType={props.disk.type}
              platformType={props.disk.platformType}
              capabilities={props.disk.capabilities}
            />
          </div>
        </Show>
      </div>
    </section>
  );
};

// Another disk must not inherit this disk's tab, range or operator form.
export const DiskDrawer: Component<DiskDrawerProps> = (props) => (
  <Show when={props.disk.id} keyed>
    {(_diskId) => <DiskDrawerContent {...props} />}
  </Show>
);
