import { Show } from 'solid-js';
import { StatusBadge } from '@/components/shared/StatusBadge';
import {
  getAlertResourceTableMetricInputTitle,
  getAlertResourceTableMetricOffToggleProps,
  getAlertResourceTableMetricPlaceholder,
} from '@/utils/alertResourceTablePresentation';
import {
  ALERT_RESOURCE_METRIC_OFF_VALUE,
  getAlertResourceMetricBounds,
  getAlertResourceMetricStep,
  isAlertResourceMetricOff,
  resolveAlertResourceGlobalDefaultCell,
  resolveAlertResourceMetricEnableValue,
  type AlertResourceGlobalDefaultFallback,
} from './alertResourceTableModel';

interface AlertResourceMetricEditorProps {
  metric: string;
  column: string;
  scope: 'default' | 'override';
  layout: 'table' | 'card';
  value: number | undefined;
  inheritedDefault?: number;
  fallback?: AlertResourceGlobalDefaultFallback;
  onChange: (value: number | undefined) => void;
  autoFocus?: boolean;
  onFocus?: () => void;
}

/** Numeric threshold editing owns one Off/inherit contract across both layouts. */
export function AlertResourceMetricEditor(props: AlertResourceMetricEditorProps) {
  const bounds = () => getAlertResourceMetricBounds(props.metric);
  const defaultCell = () =>
    resolveAlertResourceGlobalDefaultCell(props.metric, props.value, props.fallback);
  const isDefault = () => props.scope === 'default';
  const isOff = () =>
    isDefault()
      ? defaultCell().isOff
      : isAlertResourceMetricOff(props.value, props.inheritedDefault);
  const follows = () => isDefault() && defaultCell().follows;
  const enableValue = () =>
    isDefault()
      ? defaultCell().enableValue
      : resolveAlertResourceMetricEnableValue(props.inheritedDefault, props.metric);

  const inputClass = () => {
    if (props.layout === 'card') {
      return isDefault()
        ? `min-h-11 w-full rounded-sm border p-1 text-center text-sm sm:min-h-0 ${isOff() ? 'bg-surface-hover' : 'border-border'}`
        : 'min-h-11 w-16 text-right text-xs p-1 rounded-sm border border-border bg-surface';
    }
    return `${follows() && !isOff() ? 'w-20' : 'w-16'} px-2 py-0.5 text-sm text-center border rounded ${
      isOff()
        ? 'border-border bg-surface-alt text-muted'
        : 'border-border text-base-content focus:border-blue-500 focus:ring-1 focus:ring-blue-500'
    }`;
  };

  return (
    <div
      class={`flex items-center gap-1.5 ${props.layout === 'card' ? '[&_button]:min-h-11' : 'justify-center'} ${isDefault() ? 'w-full' : ''}`}
    >
      <div class={`relative ${props.layout === 'card' && isDefault() ? 'flex-1 min-w-0' : ''}`}>
        <input
          ref={(input) => {
            queueMicrotask(() => {
              if (!props.autoFocus || !input.isConnected) return;
              input.focus();
              input.select();
            });
          }}
          type="number"
          onFocus={props.onFocus}
          min={bounds().min}
          max={bounds().max}
          step={getAlertResourceMetricStep(props.metric)}
          value={isOff() || follows() ? '' : (props.value ?? '')}
          placeholder={
            follows() && !isOff()
              ? props.fallback?.label
              : getAlertResourceTableMetricPlaceholder(isOff())
          }
          title={follows() ? props.fallback?.title : getAlertResourceTableMetricInputTitle(isOff())}
          disabled={isDefault() && isOff()}
          class={inputClass()}
          onInput={(event) => {
            const raw = event.currentTarget.value;
            // Clearing an override inherits. A blank global default is mid-edit.
            if (raw === '') {
              if (!isDefault()) props.onChange(undefined);
              return;
            }
            const value = parseFloat(raw);
            if (Number.isFinite(value) && value >= bounds().min && value <= bounds().max) {
              props.onChange(value);
            }
          }}
        />
        <Show when={isDefault() && isOff()}>
          <button
            type="button"
            class="absolute inset-0 w-full rounded-sm cursor-pointer focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-400"
            onClick={() => props.onChange(enableValue())}
            title={getAlertResourceTableMetricInputTitle(true)}
            aria-label={`Enable ${props.column} default`}
          />
        </Show>
      </div>
      <div onMouseDown={(event) => event.preventDefault()}>
        <StatusBadge
          isEnabled={!isOff()}
          onToggle={() => props.onChange(isOff() ? enableValue() : ALERT_RESOURCE_METRIC_OFF_VALUE)}
          {...getAlertResourceTableMetricOffToggleProps()}
        />
      </div>
    </div>
  );
}
