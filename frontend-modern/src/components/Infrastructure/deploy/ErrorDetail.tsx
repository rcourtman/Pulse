import { Component, Show, createSignal } from 'solid-js';

/** Maps common backend error substrings to user-friendly hints. */
function friendlyErrorHint(msg: string): string | undefined {
  const lower = msg.toLowerCase();
  if (lower.includes('connection refused'))
    return 'The target node refused the SSH connection. Verify SSH is running and the port is open.';
  if (lower.includes('permission denied'))
    return 'SSH authentication failed. Check that the source agent has key-based access to this node.';
  if (lower.includes('timed out') || lower.includes('timeout'))
    return 'The connection timed out. Check network connectivity and firewall rules.';
  if (lower.includes('no route to host'))
    return 'No network route to the target. Verify the IP address and that both nodes are on the same network.';
  if (lower.includes('host key verification'))
    return 'SSH host key verification failed. The target may need to be added to known_hosts.';
  return undefined;
}

const INLINE_MAX = 60;

interface ErrorDetailProps {
  message: string | undefined;
}

export const ErrorDetail: Component<ErrorDetailProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  const isLong = () => (props.message?.length ?? 0) > INLINE_MAX;
  const hint = () => (props.message ? friendlyErrorHint(props.message) : undefined);

  return (
    <Show when={props.message}>
      <div class="text-xs text-red-600 dark:text-red-400">
        <Show
          when={isLong()}
          fallback={
            <>
              <span>{props.message}</span>
              <Show when={hint()}>
                <p class="text-[11px] text-muted mt-0.5">{hint()}</p>
              </Show>
            </>
          }
        >
          <Show
            when={expanded()}
            fallback={
              <span>
                {props.message!.slice(0, INLINE_MAX)}…{' '}
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setExpanded(true);
                  }}
                  class="text-blue-600 dark:text-blue-400 hover:underline"
                >
                  more
                </button>
              </span>
            }
          >
            <span>{props.message}</span>{' '}
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setExpanded(false);
              }}
              class="text-blue-600 dark:text-blue-400 hover:underline"
            >
              less
            </button>
            <Show when={hint()}>
              <p class="text-[11px] text-muted mt-0.5">{hint()}</p>
            </Show>
          </Show>
        </Show>
      </div>
    </Show>
  );
};
