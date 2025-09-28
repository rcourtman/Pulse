# Frontend UI Style Guide

This project now ships a handful of shared primitives to keep typography and form layouts consistent. The snippets below show the preferred usage.

## Section headers

Use `SectionHeader` for any inline card titles, modal headings, or sub-section titles instead of ad-hoc `<h2>`/`<h3>` elements.

```tsx
import { SectionHeader } from '@/components/shared/SectionHeader';

<SectionHeader
  label="Overview"
  title="Cluster health"
  description="Key metrics across every node"
  size="sm"       // sm | md | lg (defaults to md)
  align="left"    // left | center (defaults to left)
/>
```

Pass `titleClass`/`descriptionClass` when you need to tweak color or emphasis without rebuilding the layout.

## Empty states

Whenever a panel needs to show a loading, error, or "no data" treatment, render `EmptyState` inside a `Card`.

```tsx
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';

<Card padding="lg" tone="info">
  <EmptyState
    align="center"              // center | left (defaults to center)
    tone="info"                 // default | info | success | warning | danger
    icon={<MyIcon class="h-12 w-12 text-blue-400" />}
    title="No backups yet"
    description="Run your first job or adjust the filters to see activity."
    actions={(
      <Button onClick={openScheduler}>Open Scheduler</Button>
    )}
  />
</Card>
```

Icons and actions are optional; omit them when not needed.

## Form helpers

Shared form styles live in `@/components/shared/Form`. Import the helpers and apply them to each field container, label, and control for a uniform look.

```tsx
import { formField, labelClass, controlClass, formHelpText, formCheckbox } from '@/components/shared/Form';

<div class={formField}>
  <label class={labelClass('flex items-center gap-2')}>
    Host URL <span class="text-red-500">*</span>
  </label>
  <input
    type="url"
    placeholder="https://cluster.example.com:8006"
    class={controlClass('px-2 py-1.5 font-mono')}
  />
  <p class={`${formHelpText} mt-1`}>
    Use HTTPS on port 8006 for Proxmox VE and 8007 for PBS.
  </p>
</div>

<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
  <input type="checkbox" class={formCheckbox} />
  Enable this integration
</label>
```

Helper summary:

- `formField`: wraps a label + control stack.
- `labelClass(extra?)`: base typography for labels, with optional extra classes.
- `controlClass(extra?)`: base input styling; append sizing tweaks (`px-2 py-1.5`) as needed.
- `formHelpText`: small secondary text (validation notes, hints).
- `formCheckbox`: shared checkbox styling for toggles inside copy-heavy forms.

Stick to these helpers when building new settings panels, modals, or detail cards. If a component needs a variant that the helpers do not cover, extend them in `Form.ts` so the convention remains centralized.
