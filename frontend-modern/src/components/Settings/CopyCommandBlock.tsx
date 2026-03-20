import Copy from 'lucide-solid/icons/copy';

interface CopyCommandBlockProps {
  command: string;
  onCopy?: (command: string) => void | Promise<void>;
  containerClass?: string;
  codeClass?: string;
  buttonClass?: string;
}

const DEFAULT_CONTAINER_CLASS = 'relative group';
const DEFAULT_CODE_CLASS =
  'block rounded-md border border-border bg-base p-3 font-mono text-sm text-base-content';
const DEFAULT_BUTTON_CLASS =
  'absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-surface-hover p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100';

export function CopyCommandBlock(props: CopyCommandBlockProps) {
  const handleCopy = () => {
    if (props.onCopy) {
      void props.onCopy(props.command);
      return;
    }
    void navigator.clipboard.writeText(props.command);
  };

  return (
    <div class={props.containerClass ?? DEFAULT_CONTAINER_CLASS}>
      <code class={props.codeClass ?? DEFAULT_CODE_CLASS}>{props.command}</code>
      <button
        type="button"
        onClick={handleCopy}
        class={props.buttonClass ?? DEFAULT_BUTTON_CLASS}
        title="Copy to clipboard"
        aria-label="Copy to clipboard"
      >
        <Copy class="h-4 w-4" />
      </button>
    </div>
  );
}

export default CopyCommandBlock;
