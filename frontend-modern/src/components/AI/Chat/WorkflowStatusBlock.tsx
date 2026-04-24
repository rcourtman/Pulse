import { type Component } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import CircleCheckIcon from 'lucide-solid/icons/circle-check';
import CircleDashedIcon from 'lucide-solid/icons/circle-dashed';
import ClipboardCheckIcon from 'lucide-solid/icons/clipboard-check';
import HelpCircleIcon from 'lucide-solid/icons/help-circle';
import PlayIcon from 'lucide-solid/icons/play';
import SearchIcon from 'lucide-solid/icons/search';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import type { WorkflowStatus } from './types';
import { formatIdentifierLabel } from '@/utils/textPresentation';

interface WorkflowStatusBlockProps {
  status: WorkflowStatus;
}

const phasePresentation = (phase: string) => {
  switch (phase) {
    case 'investigate':
      return {
        label: 'Investigating',
        classes:
          'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200',
        Icon: SearchIcon,
      };
    case 'clarify':
      return {
        label: 'Clarifying',
        classes:
          'border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-800 dark:bg-sky-950/40 dark:text-sky-200',
        Icon: HelpCircleIcon,
      };
    case 'plan':
      return {
        label: 'Planning',
        classes:
          'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200',
        Icon: ClipboardCheckIcon,
      };
    case 'approve':
      return {
        label: 'Awaiting Approval',
        classes:
          'border-orange-200 bg-orange-50 text-orange-800 dark:border-orange-800 dark:bg-orange-950/40 dark:text-orange-200',
        Icon: ShieldCheckIcon,
      };
    case 'execute':
      return {
        label: 'Executing',
        classes:
          'border-indigo-200 bg-indigo-50 text-indigo-800 dark:border-indigo-800 dark:bg-indigo-950/40 dark:text-indigo-200',
        Icon: PlayIcon,
      };
    case 'verify':
      return {
        label: 'Verifying',
        classes:
          'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200',
        Icon: CircleDashedIcon,
      };
    case 'complete':
      return {
        label: 'Complete',
        classes:
          'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200',
        Icon: CircleCheckIcon,
      };
    default:
      return {
        label: formatIdentifierLabel(phase || 'workflow'),
        classes:
          'border-border-subtle bg-surface text-base-content dark:border-border-subtle dark:bg-surface',
        Icon: CircleDashedIcon,
      };
  }
};

export const WorkflowStatusBlock: Component<WorkflowStatusBlockProps> = (props) => {
  const presentation = () => phasePresentation(props.status.phase);
  const toolLabel = () =>
    props.status.tool ? formatIdentifierLabel(props.status.tool, { stripPrefix: 'pulse_' }) : '';

  return (
    <div class={`my-2 rounded-md border px-3 py-2 text-xs ${presentation().classes}`}>
      <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
        <Dynamic component={presentation().Icon} class="h-3.5 w-3.5" />
        <span class="font-semibold uppercase">{presentation().label}</span>
        {toolLabel() && <span class="font-mono opacity-80">{toolLabel()}</span>}
        {props.status.state && <span class="font-mono opacity-75">state={props.status.state}</span>}
      </div>
      <p class="mt-1 leading-relaxed">{props.status.message}</p>
    </div>
  );
};
