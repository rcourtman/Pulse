import Monitor from 'lucide-solid/icons/monitor';

import { Card } from '@/components/shared/Card';
import { TagInput } from '@/components/shared/TagInput';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxGuestFilteringSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <CollapsibleSection
      id="guest-filtering"
      title={state.sectionTitles.guestFiltering}
      collapsed={state.isCollapsed('guest-filtering')}
      onToggle={() => state.toggleSection('guest-filtering')}
      icon={<Monitor class="w-5 h-5" />}
      emptyMessage={state.GUEST_FILTERING_EMPTY_STATE}
    >
      <div class="grid grid-cols-1 gap-6 p-4 xl:grid-cols-3">
        <Card padding="md" tone="card">
          <div class="mb-2">
            <h3 class="text-sm font-semibold text-base-content">
              {state.guestFilterPresentation.ignoredPrefixes.title}
            </h3>
            <p class="text-xs text-muted">
              {state.guestFilterPresentation.ignoredPrefixes.description}
            </p>
          </div>
          <TagInput
            tags={tableProps.ignoredGuestPrefixes()}
            label={state.guestFilterPresentation.ignoredPrefixes.title}
            onChange={(tags) => {
              tableProps.setIgnoredGuestPrefixes(tags);
              tableProps.setHasUnsavedChanges(true);
            }}
            placeholder={state.guestFilterPresentation.ignoredPrefixes.placeholder}
          />
        </Card>

        <Card padding="md" tone="card">
          <div class="mb-2">
            <h3 class="text-sm font-semibold text-base-content">
              {state.guestFilterPresentation.tagWhitelist.title}
            </h3>
            <p class="text-xs text-muted">
              {state.guestFilterPresentation.tagWhitelist.description}
            </p>
          </div>
          <TagInput
            tags={tableProps.guestTagWhitelist()}
            label={state.guestFilterPresentation.tagWhitelist.title}
            onChange={(tags) => {
              tableProps.setGuestTagWhitelist(tags);
              tableProps.setHasUnsavedChanges(true);
            }}
            placeholder={state.guestFilterPresentation.tagWhitelist.placeholder}
          />
        </Card>

        <Card padding="md" tone="card">
          <div class="mb-2">
            <h3 class="text-sm font-semibold text-base-content">
              {state.guestFilterPresentation.tagBlacklist.title}
            </h3>
            <p class="text-xs text-muted">
              {state.guestFilterPresentation.tagBlacklist.description}
            </p>
          </div>
          <TagInput
            tags={tableProps.guestTagBlacklist()}
            label={state.guestFilterPresentation.tagBlacklist.title}
            onChange={(tags) => {
              tableProps.setGuestTagBlacklist(tags);
              tableProps.setHasUnsavedChanges(true);
            }}
            placeholder={state.guestFilterPresentation.tagBlacklist.placeholder}
          />
        </Card>
      </div>
    </CollapsibleSection>
  );
}
