import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { ArrowRight, CircleCheck, Info } from 'lucide-solid';
import {
  AvailabilityTargetsAPI,
  type DockerPortCheckSuggestion,
  type DockerPortChecksResult,
} from '@/api/availabilityTargets';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { buildStandalonePath } from '@/routing/resourceLinks';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import {
  dockerPortCheckKey,
  offeredPortSuggestions,
  portLinesByHost,
  portOfferMessage,
  readDismissedPortChecks,
  skippedPortLines,
  summarizePortChecksResult,
  writeDismissedPortChecks,
} from './dockerPortChecksModel';

type DockerPortChecksNoticeProps = {
  // Changes when the set of containers on the page changes, so a container
  // started later is offered without a reload.
  containerSignature: string;
  // Called after checks are added, so the page can show them on their rows.
  onChecksAdded?: () => void;
};

// Phones get a full-height tap target; wider screens keep the line compact.
const NOTICE_LINK_CLASS =
  'inline-flex min-h-11 items-center underline underline-offset-2 hover:no-underline disabled:cursor-default disabled:no-underline disabled:opacity-60 focus-visible:rounded-xs focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 sm:min-h-0';
const NOTICE_BUTTON_CLASS = `font-semibold ${NOTICE_LINK_CLASS}`;
const NOTICE_TOGGLE_CLASS = `font-medium ${NOTICE_LINK_CLASS}`;

// One quiet line on the Docker page: published container ports that nothing
// checks, with a single action that adds a check for each one that answers.
// It stays out of the way: nothing renders when every port is covered, when
// the viewer cannot change settings, or once the offer is dismissed.
export function DockerPortChecksNotice(props: DockerPortChecksNoticeProps) {
  const [suggestions, { refetch }] = createResource(
    () => (presentationPolicyIsReadOnly() ? undefined : props.containerSignature),
    async (): Promise<DockerPortCheckSuggestion[]> => {
      try {
        return await AvailabilityTargetsAPI.dockerPortSuggestions();
      } catch {
        // Viewers without settings access get a refusal; they see no offer.
        return [];
      }
    },
  );
  const [dismissed, setDismissed] = createSignal(readDismissedPortChecks());
  const [showPorts, setShowPorts] = createSignal(false);
  const [adding, setAdding] = createSignal(false);
  const [result, setResult] = createSignal<DockerPortChecksResult | null>(null);
  const [failure, setFailure] = createSignal('');

  const all = createMemo(() => suggestions.latest ?? []);
  const offered = createMemo(() => offeredPortSuggestions(all(), dismissed()));
  const skippedLines = createMemo(() => skippedPortLines(all()));
  const summary = createMemo(() => {
    const current = result();
    return current ? summarizePortChecksResult(current) : null;
  });

  const rememberDismissed = (ports: DockerPortCheckSuggestion[]) => {
    const next = new Set(dismissed());
    for (const port of ports) next.add(dockerPortCheckKey(port));
    setDismissed(next);
    writeDismissedPortChecks(next, [...all(), ...ports]);
  };

  const addChecks = async () => {
    const ports = offered();
    if (adding() || ports.length === 0) return;
    setAdding(true);
    setFailure('');
    try {
      // Only the ports on offer: one dismissed earlier is not added now.
      const outcome = await AvailabilityTargetsAPI.addDockerPortChecks(ports);
      // A port that did not answer stays unchecked. Do not offer it again in
      // this browser; a port published later still gets its own offer.
      rememberDismissed(outcome.unanswered);
      setResult(outcome);
      setShowPorts(false);
      if (outcome.created.length > 0) props.onChecksAdded?.();
      await refetch();
    } catch (error) {
      setFailure(error instanceof Error && error.message ? error.message : 'The request failed.');
    } finally {
      setAdding(false);
    }
  };

  return (
    <Show when={!presentationPolicyIsReadOnly()}>
      <Show
        when={summary()}
        fallback={
          <Show when={offered().length > 0}>
            <InlineNotice
              role="status"
              data-testid="docker-port-checks-offer"
              tone="info"
              icon={<Info aria-hidden="true" />}
              onDismiss={() => rememberDismissed(offered())}
              dismissLabel="Dismiss the availability check offer"
              dismissTitle="Dismiss"
            >
              <span>{portOfferMessage(offered().length)}</span>{' '}
              <span class="inline-flex flex-wrap items-baseline gap-x-3">
                <button
                  type="button"
                  class={NOTICE_BUTTON_CLASS}
                  disabled={adding()}
                  onClick={() => void addChecks()}
                >
                  {adding() ? 'Checking ports…' : 'Add checks'}
                </button>
                <button
                  type="button"
                  class={NOTICE_TOGGLE_CLASS}
                  aria-expanded={showPorts()}
                  onClick={() => setShowPorts((current) => !current)}
                >
                  {showPorts() ? 'Hide ports' : 'Show ports'}
                </button>
              </span>
              <Show when={failure()}>
                <p class="mt-1 wrap-break-word">Could not add checks: {failure()}</p>
              </Show>
              <Show when={showPorts()}>
                <ul class="mt-1 space-y-0.5 wrap-break-word" data-testid="docker-port-checks-list">
                  <For each={portLinesByHost(offered())}>{(line) => <li>{line}</li>}</For>
                  <For each={skippedLines()}>{(line) => <li class="opacity-80">{line}</li>}</For>
                </ul>
                <p class="mt-1 opacity-80">
                  Pulse adds a check for each port that answers over HTTP now, and alerts you when
                  it stops answering.
                </p>
              </Show>
            </InlineNotice>
          </Show>
        }
      >
        {(current) => (
          <InlineNotice
            role="status"
            data-testid="docker-port-checks-result"
            tone={current().tone}
            icon={
              current().tone === 'success' ? (
                <CircleCheck aria-hidden="true" />
              ) : (
                <Info aria-hidden="true" />
              )
            }
            onDismiss={() => setResult(null)}
            dismissLabel="Dismiss"
            dismissTitle="Dismiss"
          >
            <span>{current().message}</span>
            <Show when={(result()?.created.length ?? 0) > 0}>
              {' '}
              <a
                href={buildStandalonePath('availability')}
                class="inline-flex items-center gap-1 whitespace-nowrap font-semibold underline-offset-2 hover:underline"
              >
                View checks
                <ArrowRight aria-hidden="true" class="h-3.5 w-3.5" />
              </a>
            </Show>
            <Show when={current().unansweredLine}>
              {(line) => <p class="mt-1 wrap-break-word">{line()}</p>}
            </Show>
          </InlineNotice>
        )}
      </Show>
    </Show>
  );
}

export default DockerPortChecksNotice;
