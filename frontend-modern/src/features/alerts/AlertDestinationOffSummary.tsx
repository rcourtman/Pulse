interface AlertDestinationOffSummaryProps {
  message: string;
  onShowSettings: () => void;
}

// A switched-off destination states what turning it on does in one line
// instead of rendering its whole form greyed out. The settings stay one click
// away without enabling the destination, so a saved but disabled setup can be
// inspected without marking the page as changed.
export function AlertDestinationOffSummary(props: AlertDestinationOffSummaryProps) {
  return (
    <div
      class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm text-muted"
      data-alert-destination-off
    >
      <span>{props.message}</span>
      <button
        type="button"
        class="min-h-11 text-sm font-medium text-blue-600 hover:underline sm:min-h-0 dark:text-blue-300"
        onClick={() => props.onShowSettings()}
      >
        {ALERT_DESTINATION_SHOW_SETTINGS_LABEL}
      </button>
    </div>
  );
}

export const ALERT_DESTINATION_SHOW_SETTINGS_LABEL = 'Show settings';
