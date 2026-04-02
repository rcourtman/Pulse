import type {
  SummaryScopeKind,
  SummaryScopeSource,
  SummaryScopeState,
  SummarySeriesGroupScope,
} from './summaryCardInteraction';

export interface SummaryScopePresentation {
  contextLabel: string | null;
  kind: SummaryScopeKind;
  label: string;
  mode: 'all' | 'preview' | 'pinned';
}

interface BuildSummaryScopePresentationOptions {
  allLabel: string;
  resolveEntityLabel?: (seriesId: string) => string | null | undefined;
  resolveGroupLabel?: (scope: SummarySeriesGroupScope) => string | null | undefined;
  state: SummaryScopeState;
}

const modeFromSource = (source: SummaryScopeSource): SummaryScopePresentation['mode'] => {
  switch (source) {
    case 'preview':
      return 'preview';
    case 'pinned':
      return 'pinned';
    default:
      return 'all';
  }
};

export const buildSummaryScopePresentation = (
  options: BuildSummaryScopePresentationOptions,
): SummaryScopePresentation => {
  const resolveGroupLabel =
    options.resolveGroupLabel ?? ((scope: SummarySeriesGroupScope) => scope.label ?? scope.id);
  const resolveEntityLabel =
    options.resolveEntityLabel ?? ((seriesId: string) => seriesId);

  if (options.state.kind === 'group' && options.state.groupScope) {
    return {
      contextLabel: null,
      kind: 'group',
      label: resolveGroupLabel(options.state.groupScope)?.trim() || options.allLabel,
      mode: modeFromSource(options.state.source),
    };
  }

  if (options.state.kind === 'entity' && options.state.seriesId) {
    return {
      contextLabel: options.state.groupScope
        ? (resolveGroupLabel(options.state.groupScope)?.trim() ?? null)
        : null,
      kind: 'entity',
      label: resolveEntityLabel(options.state.seriesId)?.trim() || options.state.seriesId,
      mode: modeFromSource(options.state.source),
    };
  }

  return {
    contextLabel: null,
    kind: 'page',
    label: options.allLabel,
    mode: 'all',
  };
};
