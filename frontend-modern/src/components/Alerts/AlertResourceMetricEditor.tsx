import { createEffect, createSignal, createUniqueId, on, Show } from 'solid-js';
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
import { getAlertThresholdInputError } from './alertThresholdValidation';

interface AlertResourceMetricEditorProps {
  metric: string;
  column: string;
  scope: 'default' | 'override';
  layout: 'table' | 'card';
  value: number | undefined;
  inheritedDefault?: number;
  fallback?: AlertResourceGlobalDefaultFallback;
  onChange: (value: number | undefined) => void;
  /** The resource an override row edits, so its controls say whose they are. */
  resourceName?: string;
  autoFocus?: boolean;
  onFocus?: () => void;
}

/** Numeric threshold editing owns one Off/inherit contract across both layouts. */
export function AlertResourceMetricEditor(props: AlertResourceMetricEditorProps) {
  const [inputError, setInputError] = createSignal('');
  const errorId = createUniqueId();
  let inputElement: HTMLInputElement | undefined;
  const bounds = () => getAlertResourceMetricBounds(props.metric);
  const defaultCell = () =>
    resolveAlertResourceGlobalDefaultCell(props.metric, props.value, props.fallback);
  const isDefault = () => props.scope === 'default';
  const isOff = () =>
    isDefault()
      ? defaultCell().isOff
      : isAlertResourceMetricOff(props.value, props.inheritedDefault);
  const follows = () => isDefault() && defaultCell().follows;
  // A thresholds page holds dozens of alike inputs and On/Off badges. Each
  // names its metric, and an override its resource, so assistive tech and
  // voice control can tell them apart; the column header alone is not read
  // when moving between form controls.
  const controlName = () =>
    isDefault()
      ? `${props.column} default`
      : `${props.column} for ${props.resourceName?.trim() || 'this resource'}`;
  const enableValue = () =>
    isDefault()
      ? defaultCell().enableValue
      : resolveAlertResourceMetricEnableValue(props.inheritedDefault, props.metric);

  // Off, inherit, reset and a refreshed saved policy replace the numeric draft.
  // An ignored invalid keystroke does not change props.value, so keeps its error.
  createEffect(
    on([() => props.value, isOff, follows], () => {
      inputElement?.setCustomValidity('');
      setInputError('');
    }),
  );

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
    <div class="flex min-w-0 flex-col gap-1">
      <div
        class={`flex items-center gap-1.5 ${props.layout === 'card' ? '[&_button]:min-h-11' : 'justify-center'} ${isDefault() ? 'w-full' : ''}`}
      >
        <div class={`relative ${props.layout === 'card' && isDefault() ? 'flex-1 min-w-0' : ''}`}>
          <input
            ref={(input) => {
              inputElement = input;
              queueMicrotask(() => {
                if (!props.autoFocus || !input.isConnected) return;
                input.focus();
                input.select();
              });
            }}
            type="number"
            aria-label={controlName()}
            data-alert-threshold-editor
            aria-invalid={inputError() ? 'true' : undefined}
            aria-describedby={inputError() ? errorId : undefined}
            required={isDefault() && !follows() && !isOff()}
            onInvalid={() => setInputError(inputElement?.validationMessage ?? '')}
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
            title={
              follows() ? props.fallback?.title : getAlertResourceTableMetricInputTitle(isOff())
            }
            disabled={isDefault() && isOff()}
            class={inputClass()}
            onInput={(event) => {
              const input = event.currentTarget;
              const message = getAlertThresholdInputError(input);
              input.setCustomValidity(message);
              setInputError(message);
              if (message) return;
              const raw = input.value;
              // A truly empty override inherits; a browser's bad numeric input
              // also looks empty, but was rejected above and must not inherit.
              if (raw === '') {
                if (!isDefault()) props.onChange(undefined);
                return;
              }
              props.onChange(input.valueAsNumber);
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
            onToggle={() =>
              props.onChange(isOff() ? enableValue() : ALERT_RESOURCE_METRIC_OFF_VALUE)
            }
            {...getAlertResourceTableMetricOffToggleProps()}
            ariaLabel={`${controlName()} alerts`}
          />
        </div>
      </div>
      <Show when={inputError()}>
        <p
          id={errorId}
          role="alert"
          class="max-w-40 whitespace-normal text-xs text-red-700 dark:text-red-300"
        >
          {inputError()}
        </p>
      </Show>
    </div>
  );
}
