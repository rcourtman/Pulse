import { CommandCopyButton } from '@/components/shared/Button';

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
      <CommandCopyButton
        onClick={handleCopy}
        class={props.buttonClass}
        title="Copy to clipboard"
        label="Copy to clipboard"
      />
    </div>
  );
}

export default CopyCommandBlock;
