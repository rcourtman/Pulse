import { For, Show, onMount } from 'solid-js';
import Save from 'lucide-solid/icons/save';
import X from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { formControl } from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import type { ReportingFormat } from './reportingCatalogModel';
import { reportScheduleKindLabel, type ReportScheduleKind } from './reportingSchedulesModel';
import type { useReportingPanelState } from './useReportingPanelState';
import { ReportingFormField as FormField } from './ReportingFormField';
import { ResourcePicker } from './ResourcePicker';

type EditorState = Pick<
  ReturnType<typeof useReportingPanelState>,
  | 'closeScheduleForm'
  | 'patrolDigestAvailable'
  | 'reportingCatalog'
  | 'saveReportSchedule'
  | 'savingSchedule'
  | 'scheduleForm'
  | 'scheduleResources'
  | 'setScheduleKind'
  | 'setScheduleResources'
  | 'updateScheduleForm'
>;

const WEEKDAY_OPTIONS = [
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
];

export default function ReportScheduleEditor(props: { state: EditorState }) {
  let nameInput: HTMLInputElement | undefined;
  onMount(() => nameInput?.focus());
  const {
    closeScheduleForm,
    patrolDigestAvailable,
    reportingCatalog,
    saveReportSchedule,
    savingSchedule,
    scheduleForm,
    scheduleResources,
    setScheduleKind,
    setScheduleResources,
    updateScheduleForm,
  } = props.state;
  const performanceReport = () => reportingCatalog()?.performanceReport ?? null;
  const isDigestForm = () => scheduleForm().kind === 'patrol_digest';
  const isInventoryForm = () => scheduleForm().kind === 'vm_inventory';
  // Existing digest schedules stay editable after Patrol is switched off.
  const inventoryAvailable = () => !!reportingCatalog()?.vmInventoryExport || isInventoryForm();
  const showReportTypeSelect = () =>
    inventoryAvailable() || patrolDigestAvailable() || isDigestForm();
  return (
    <section class="space-y-4 border-y border-base-300 py-4">
      <Show when={showReportTypeSelect()}>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <FormSelect
            label="Report type"
            value={scheduleForm().kind}
            onChange={(e) => setScheduleKind(e.currentTarget.value as ReportScheduleKind)}
            help={
              isDigestForm()
                ? 'Emails what Patrol checked, found, fixed, and spent over the last seven days. Nothing is saved to disk.'
                : isInventoryForm()
                  ? 'Exports the current inventory of all VMs in this organization as CSV each time it runs, including newly discovered VMs.'
                  : 'A performance report for the resources you pick, as a PDF or CSV.'
            }
          >
            <option value="resources">{reportScheduleKindLabel('resources')}</option>
            <Show when={inventoryAvailable()}>
              <option value="vm_inventory">{reportScheduleKindLabel('vm_inventory')}</option>
            </Show>
            <Show when={patrolDigestAvailable() || isDigestForm()}>
              <option value="patrol_digest">{reportScheduleKindLabel('patrol_digest')}</option>
            </Show>
          </FormSelect>
        </div>
      </Show>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <FormField label="Schedule name">
          <input
            ref={nameInput}
            aria-label="Schedule name"
            type="text"
            class={formControl}
            value={scheduleForm().name}
            onInput={(e) => updateScheduleForm({ name: e.currentTarget.value })}
            placeholder={
              isDigestForm()
                ? 'Patrol weekly summary'
                : isInventoryForm()
                  ? 'Monthly VM inventory'
                  : 'Monthly client report'
            }
          />
        </FormField>
        <FormField label="Timezone">
          <input
            aria-label="Timezone"
            type="text"
            class={formControl}
            value={scheduleForm().timezone}
            onInput={(e) => updateScheduleForm({ timezone: e.currentTarget.value })}
            placeholder="Europe/London"
          />
        </FormField>
      </div>

      <div
        class={
          isDigestForm()
            ? 'grid grid-cols-1 gap-4 md:grid-cols-2'
            : isInventoryForm()
              ? 'grid grid-cols-1 gap-4 md:grid-cols-3'
              : 'grid grid-cols-1 gap-4 md:grid-cols-4'
        }
      >
        <Show when={!isDigestForm()}>
          <FormSelect
            label="Cadence"
            value={scheduleForm().cadenceType}
            onChange={(e) =>
              updateScheduleForm({
                cadenceType: e.currentTarget.value as 'monthly' | 'weekly',
              })
            }
          >
            <option value="monthly">Monthly</option>
            <option value="weekly">Weekly</option>
          </FormSelect>
        </Show>
        <Show
          when={scheduleForm().cadenceType === 'monthly' && !isDigestForm()}
          fallback={
            <FormSelect
              label="Weekday"
              value={scheduleForm().weekday}
              onChange={(e) => updateScheduleForm({ weekday: e.currentTarget.value })}
            >
              <For each={WEEKDAY_OPTIONS}>
                {(day) => <option value={day}>{day[0].toUpperCase() + day.slice(1)}</option>}
              </For>
            </FormSelect>
          }
        >
          <FormField label="Day of month">
            <input
              aria-label="Day of month"
              type="number"
              min="1"
              max="28"
              class={formControl}
              value={scheduleForm().dayOfMonth}
              onInput={(e) => updateScheduleForm({ dayOfMonth: Number(e.currentTarget.value) })}
            />
          </FormField>
        </Show>
        <FormField label="Time">
          <input
            aria-label="Time"
            type="time"
            class={formControl}
            value={scheduleForm().time}
            onInput={(e) => updateScheduleForm({ time: e.currentTarget.value })}
          />
        </FormField>
        <Show when={scheduleForm().kind === 'resources'}>
          <FormSelect
            label="Format"
            value={scheduleForm().format}
            onChange={(e) =>
              updateScheduleForm({ format: e.currentTarget.value as ReportingFormat })
            }
          >
            <option value="pdf">PDF</option>
            <option value="csv">CSV</option>
          </FormSelect>
        </Show>
      </div>

      <Show when={scheduleForm().kind === 'resources'}>
        <FormField
          label="Resources"
          helpText="Use explicit resources, tags, or both. Scheduled reports use the previous reporting boundary."
        >
          <ResourcePicker
            maxSelection={performanceReport()?.multiResourceMax}
            selected={scheduleResources}
            onSelectionChange={setScheduleResources}
          />
        </FormField>
      </Show>

      <Show when={!isDigestForm()}>
        <div
          class={`grid grid-cols-1 gap-4 ${isInventoryForm() ? 'md:grid-cols-2' : 'md:grid-cols-3'}`}
        >
          <Show when={!isInventoryForm()}>
            <FormField label="Tag filter" helpText="Comma-separated tags">
              <input
                aria-label="Tag filter"
                type="text"
                class={formControl}
                value={scheduleForm().tagFilter}
                onInput={(e) => updateScheduleForm({ tagFilter: e.currentTarget.value })}
                placeholder="production, customer-facing"
              />
            </FormField>
          </Show>
          <FormSelect
            label="Delivery"
            value={scheduleForm().deliveryMethod}
            onChange={(e) =>
              updateScheduleForm({
                deliveryMethod: e.currentTarget.value as 'email' | 'disk',
              })
            }
          >
            <option value="email">Email recipients</option>
            <option value="disk">Save to disk</option>
          </FormSelect>
          <FormField label="Retention" helpText="Generated files kept on this server">
            <input
              aria-label="Retention"
              type="number"
              min="1"
              max="120"
              class={formControl}
              value={scheduleForm().retentionCount}
              onInput={(e) => updateScheduleForm({ retentionCount: Number(e.currentTarget.value) })}
            />
          </FormField>
        </div>
      </Show>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <FormField
          label="Email recipients"
          helpText="Blank uses the existing email notification recipients"
        >
          <input
            aria-label="Email recipients"
            type="text"
            class={formControl}
            value={scheduleForm().recipients}
            onInput={(e) => updateScheduleForm({ recipients: e.currentTarget.value })}
            placeholder="client@example.com, ops@example.com"
            disabled={scheduleForm().deliveryMethod !== 'email'}
          />
        </FormField>
        <div class="grid grid-cols-1 gap-3 pt-6 sm:grid-cols-2">
          <label class="inline-flex items-center gap-2 text-sm text-muted">
            <input
              type="checkbox"
              checked={scheduleForm().enabled}
              onChange={(e) => updateScheduleForm({ enabled: e.currentTarget.checked })}
            />
            Enabled
          </label>
          <Show when={!isDigestForm()}>
            <label class="inline-flex items-center gap-2 text-sm text-muted">
              <input
                type="checkbox"
                checked={scheduleForm().attach}
                onChange={(e) => updateScheduleForm({ attach: e.currentTarget.checked })}
                disabled={scheduleForm().deliveryMethod !== 'email'}
              />
              Attach report
            </label>
          </Show>
        </div>
      </div>

      <Show when={!isDigestForm()}>
        <p class="text-xs text-muted">
          Generated reports are saved on this Pulse server. With Attach report off, email sends a
          notice containing the saved path.
        </p>
      </Show>

      <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" size="md" class="gap-2" onClick={closeScheduleForm}>
          <X size={16} />
          Cancel
        </Button>
        <Button
          variant="primary"
          size="md"
          class="gap-2"
          isLoading={savingSchedule()}
          disabled={savingSchedule()}
          onClick={saveReportSchedule}
        >
          <Save size={16} />
          Save schedule
        </Button>
      </div>
    </section>
  );
}
