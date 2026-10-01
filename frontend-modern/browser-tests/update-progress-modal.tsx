// Mount the production UpdateProgressModal against a scripted update stream.
// HTTP (/api/updates/status, /api/version) is scripted by the Playwright
// runner; the SSE transport is replaced here so the runner can hold the
// stream open and silent, which a fulfilled HTTP response cannot do.
import { render } from 'solid-js/web';
import { UpdateProgressModal } from '../src/components/UpdateProgressModal';
import '../src/index.css';

type Listener = ((event: MessageEvent | Event) => void) | null;

class ScriptedEventSource {
  static instances: ScriptedEventSource[] = [];
  readonly url: string;
  readyState = 0;
  onopen: Listener = null;
  onmessage: Listener = null;
  onerror: Listener = null;

  constructor(url: string) {
    this.url = url;
    ScriptedEventSource.instances.push(this);
    setTimeout(() => {
      if (this.readyState === 2) return;
      this.readyState = 1;
      this.onopen?.(new Event('open'));
    }, 0);
  }

  close() {
    this.readyState = 2;
  }

  addEventListener() {}
  removeEventListener() {}
}

const live = () =>
  ScriptedEventSource.instances.filter((instance) => instance.readyState !== 2).at(-1);

const w = window as unknown as Record<string, unknown>;
w.EventSource = ScriptedEventSource;
w.__updateStream = {
  emit(status: unknown) {
    const stream = live();
    if (!stream) return false;
    stream.onmessage?.(new MessageEvent('message', { data: JSON.stringify(status) }));
    return true;
  },
  fail() {
    const stream = live();
    if (!stream) return false;
    stream.onerror?.(new Event('error'));
    return true;
  },
  opened: () => ScriptedEventSource.instances.length,
  open: () => Boolean(live()),
};

// Everything the modal ever showed, so a transient restart claim that was
// later corrected still fails the run.
const seen = { restarting: false, completed: false, progress: [] as number[] };
w.__seen = seen;
new MutationObserver(() => {
  const text = document.body.innerText;
  if (text.includes('Pulse is restarting')) seen.restarting = true;
  if (text.includes('Update Completed Successfully')) seen.completed = true;
  const match = /Progress\s+(\d+)%/.exec(text);
  if (match) {
    const value = Number(match[1]);
    if (seen.progress.at(-1) !== value) seen.progress.push(value);
  }
}).observe(document.body, { subtree: true, childList: true, characterData: true });

let boots = 1;
try {
  boots = Number(sessionStorage.getItem('update-progress-boots') || '0') + 1;
  sessionStorage.setItem('update-progress-boots', String(boots));
} catch {
  // Storage unavailable: treat as a first boot.
}

render(
  () =>
    boots > 1 ? (
      <main class="p-6">
        <p data-testid="reloaded">Reloaded after update (boot {boots})</p>
      </main>
    ) : (
      <main class="p-6">
        <h1 class="text-lg font-semibold">Update progress harness</h1>
        <UpdateProgressModal
          isOpen={true}
          onClose={() => undefined}
          onViewHistory={() => undefined}
          connected={() => true}
          reconnecting={() => false}
        />
      </main>
    ),
  document.getElementById('root')!,
);
