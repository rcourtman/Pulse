import { createSignal, onCleanup } from 'solid-js';
import { CommandCopyButton } from '@/components/shared/Button';
import { copyToClipboard } from '@/utils/clipboard';

interface CopyCommandBlockProps {
  command: string;
  onCopy?: (command: string) => void | Promise<void>;
  containerClass?: string;
  codeClass?: string;
  buttonClass?: string;
  buttonLayout?: 'overlay' | 'inline';
}

const DEFAULT_CONTAINER_CLASS = 'relative group';
const DEFAULT_CODE_CLASS =
  'block rounded-md border border-border bg-page p-3 font-mono text-sm text-base-content';

export function CopyCommandBlock(props: CopyCommandBlockProps) {
  const [recentlyCopied, setRecentlyCopied] = createSignal(false);
  let resetTimer: ReturnType<typeof setTimeout> | undefined;

  onCleanup(() => {
    if (resetTimer) clearTimeout(resetTimer);
  });

  const handleCopy = () => {
    void (async () => {
      const copied = await copyToClipboard(props.command);
      if (!copied) return;
      setRecentlyCopied(true);
      if (resetTimer) clearTimeout(resetTimer);
      resetTimer = setTimeout(() => setRecentlyCopied(false), 2000);
      await props.onCopy?.(props.command);
    })();
  };

  return (
    <div class={props.containerClass ?? DEFAULT_CONTAINER_CLASS}>
      <code class={props.codeClass ?? DEFAULT_CODE_CLASS}>{props.command}</code>
      <CommandCopyButton
        onClick={handleCopy}
        layout={props.buttonLayout ?? 'overlay'}
        class={[
          props.buttonClass,
          recentlyCopied() ? 'text-emerald-600 dark:text-emerald-400' : undefined,
        ]
          .filter(Boolean)
          .join(' ')}
        title={recentlyCopied() ? 'Copied to clipboard' : 'Copy to clipboard'}
        label={recentlyCopied() ? 'Copied to clipboard' : 'Copy to clipboard'}
      />
    </div>
  );
}

export default CopyCommandBlock;
