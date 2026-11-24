import { For, Show, createSignal, type Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { formControl, formField, formHelpText, formLabel } from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import {
  AvailabilityTargetsAPI,
  type AvailabilityPushScheduleType,
} from '@/api/availabilityTargets';

const PUSH_INTERVAL_PRESETS = [
  { value: '300', label: 'Every 5 minutes' },
  { value: '900', label: 'Every 15 minutes' },
  { value: '3600', label: 'Every hour' },
  { value: '21600', label: 'Every 6 hours' },
  { value: '43200', label: 'Every 12 hours' },
  { value: '86400', label: 'Daily (24 hours)' },
  { value: '604800', label: 'Weekly (7 days)' },
] as const;

const PUSH_INTERVAL_PRESET_VALUES = new Set<string>(
  PUSH_INTERVAL_PRESETS.map((item) => item.value),
);

export interface AvailabilityPushConfigSectionProps {
  activeTargetId: string;
  justCreated: boolean;
  scheduleType: AvailabilityPushScheduleType;
  intervalSeconds: string;
  cronExpression: string;
  graceSeconds: string;
  token: string;
  busy: boolean;
  onChange: (patch: {
    pushScheduleType?: AvailabilityPushScheduleType;
    pushIntervalSeconds?: string;
    pushCronExpression?: string;
    pushGraceSeconds?: string;
    pushToken?: string;
  }) => void;
  onError: (message: string | null) => void;
  onRotatingChange: (rotating: boolean) => void;
}

const AvailabilityPushConfigSection: Component<AvailabilityPushConfigSectionProps> = (props) => {
  const [pushIntervalCustom, setPushIntervalCustom] = createSignal(false);
  const [rotatingPushToken, setRotatingPushToken] = createSignal(false);
  const [confirmRotatePushToken, setConfirmRotatePushToken] = createSignal(false);
  const [copiedPushKey, setCopiedPushKey] = createSignal<'url' | 'curl' | null>(null);

  const isCustomInterval = () =>
    pushIntervalCustom() || !PUSH_INTERVAL_PRESET_VALUES.has(props.intervalSeconds);

  const pushPingUrl = () => {
    const token = props.token.trim();
    if (!token) return '';
    const origin =
      typeof window !== 'undefined' && window.location?.origin ? window.location.origin : '';
    return `${origin}/api/push/${token}`;
  };

  const pushCurlSnippet = () => {
    const url = pushPingUrl();
    return url ? `curl -fsS -m 10 --retry 3 "${url}"` : '';
  };

  const copyPushText = async (key: 'url' | 'curl', text: string) => {
    if (!text) return;
    try {
      await navigator.clipboard?.writeText(text);
      setCopiedPushKey(key);
      setTimeout(() => {
        if (copiedPushKey() === key) setCopiedPushKey(null);
      }, 2000);
    } catch {
      props.onError('Clipboard copy failed. Select the text and copy it manually.');
    }
  };

  const handleRotatePushToken = async () => {
    const targetId = props.activeTargetId.trim();
    if (!targetId) return;
    if (!confirmRotatePushToken()) {
      setConfirmRotatePushToken(true);
      return;
    }
    setRotatingPushToken(true);
    props.onRotatingChange(true);
    props.onError(null);
    try {
      const updated = await AvailabilityTargetsAPI.rotatePushToken(targetId);
      if (updated.push?.token) {
        props.onChange({ pushToken: updated.push.token });
      }
      setConfirmRotatePushToken(false);
      setCopiedPushKey(null);
    } catch (err) {
      props.onError(err instanceof Error ? err.message : 'Failed to rotate push URL.');
    } finally {
      setRotatingPushToken(false);
      props.onRotatingChange(false);
    }
  };

  return (
    <section
      class="min-w-0 space-y-4 rounded-lg border border-border p-4 sm:col-span-2"
      data-testid="availability-push-config"
    >
      <div>
        <h3 class="text-sm font-semibold text-base-content">Push heartbeat schedule</h3>
        <p class="mt-1 text-xs text-muted">
          Your script, cron job, or backup task pings Pulse when it finishes. Pulse alerts if a ping
          misses its schedule plus grace window or reports failure. To detect when Pulse itself goes
          down, use External watchdog under Alerts and Notifications.
        </p>
      </div>
      <div class="grid min-w-0 gap-4 sm:grid-cols-2">
        <FormSelect
          label="Schedule type"
          value={props.scheduleType}
          onChange={(event) =>
            props.onChange({
              pushScheduleType: event.currentTarget.value as AvailabilityPushScheduleType,
            })
          }
        >
          <option value="interval">Every interval</option>
          <option value="cron">Cron schedule (UTC)</option>
        </FormSelect>
        <label class={formField}>
          <span class={formLabel}>Grace period (seconds)</span>
          <input
            class={formControl}
            inputMode="numeric"
            value={props.graceSeconds}
            onInput={(event) => props.onChange({ pushGraceSeconds: event.currentTarget.value })}
            placeholder="300"
          />
          <span class={formHelpText}>Extra time allowed past the deadline before alerting.</span>
        </label>
        <Show when={props.scheduleType === 'interval'}>
          <FormSelect
            label="Expected interval"
            value={isCustomInterval() ? 'custom' : props.intervalSeconds}
            onChange={(event) => {
              const val = event.currentTarget.value;
              if (val === 'custom') {
                setPushIntervalCustom(true);
              } else {
                setPushIntervalCustom(false);
                props.onChange({ pushIntervalSeconds: val });
              }
            }}
            fieldClass={isCustomInterval() ? '' : 'sm:col-span-2'}
          >
            <For each={PUSH_INTERVAL_PRESETS}>
              {(preset) => <option value={preset.value}>{preset.label}</option>}
            </For>
            <option value="custom">Custom (seconds)</option>
          </FormSelect>
          <Show when={isCustomInterval()}>
            <label class={formField}>
              <span class={formLabel}>Interval (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={props.intervalSeconds}
                onInput={(event) =>
                  props.onChange({ pushIntervalSeconds: event.currentTarget.value })
                }
                placeholder="86400"
              />
              <span class={formHelpText}>Minimum 60 seconds.</span>
            </label>
          </Show>
        </Show>
        <Show when={props.scheduleType === 'cron'}>
          <label class={`${formField} sm:col-span-2`}>
            <span class={formLabel}>Cron expression (5-field UTC)</span>
            <input
              class={`${formControl} font-mono`}
              value={props.cronExpression}
              onInput={(event) => props.onChange({ pushCronExpression: event.currentTarget.value })}
              placeholder="0 3 * * *"
            />
            <span class={formHelpText}>
              Minute Hour Day Month Weekday in UTC (for example, 0 3 * * * for 03:00 UTC daily).
            </span>
          </label>
        </Show>
      </div>

      <Show
        when={pushPingUrl()}
        fallback={
          <div class="rounded-md border border-border bg-surface-alt px-3 py-2.5 text-xs text-muted">
            Save this check to generate its private Ping URL and ready-to-paste curl command.
          </div>
        }
      >
        <div class="min-w-0 space-y-3 rounded-md border border-border bg-surface-alt p-3">
          <Show when={props.justCreated}>
            <p class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
              Push check created. Copy the Ping URL or curl command into your job below.
            </p>
          </Show>
          <div class="min-w-0 space-y-1.5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class={formLabel}>Ping URL</span>
              <div class="flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="settingsAction"
                  onClick={() => void copyPushText('url', pushPingUrl())}
                >
                  {copiedPushKey() === 'url' ? 'Copied URL' : 'Copy URL'}
                </Button>
                <Button
                  variant={confirmRotatePushToken() ? 'danger' : 'outline'}
                  size="settingsAction"
                  disabled={props.busy || rotatingPushToken()}
                  onClick={() => void handleRotatePushToken()}
                >
                  {rotatingPushToken()
                    ? 'Rotating…'
                    : confirmRotatePushToken()
                      ? 'Confirm rotate URL'
                      : 'Rotate URL'}
                </Button>
              </div>
            </div>
            <input
              class={`${formControl} min-w-0 font-mono text-xs`}
              readOnly
              aria-label="Ping URL"
              value={pushPingUrl()}
              onFocus={(event) => event.currentTarget.select()}
            />
            <Show when={confirmRotatePushToken()}>
              <p class="text-xs text-warning">
                Rotating replaces the token immediately. Any job still calling the old URL will
                receive 404.
              </p>
            </Show>
          </div>
          <div class="min-w-0 space-y-1.5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class={formLabel}>Example curl command</span>
              <Button
                variant="outline"
                size="settingsAction"
                onClick={() => void copyPushText('curl', pushCurlSnippet())}
              >
                {copiedPushKey() === 'curl' ? 'Copied curl' : 'Copy curl'}
              </Button>
            </div>
            <pre class="max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded border border-border bg-surface px-3 py-2 font-mono text-xs text-base-content">
              {pushCurlSnippet()}
            </pre>
            <p class={formHelpText}>
              Optional suffixes: append /start before a job runs to measure duration, or /fail (or
              ?status=down) on error.
            </p>
          </div>
        </div>
      </Show>
    </section>
  );
};

export default AvailabilityPushConfigSection;
