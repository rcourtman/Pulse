# Pulse Modern Design System 🎨

This document outlines the standard UI primitives, tokens, and components that conform to Pulse's single-source-of-truth semantic design spec.

**Goal:** All new pages, panels, toggles, buttons, and text should map exactly to these shared specifications instead of relying on manually typing raw Tailwind color configs (like `bg-gray-800` or `#1a1c23`).

## Core Principles
1. **Never hardcode hex values** or use static `gray` / `white` / `slate` labels for structural layout colors. 
2. Use the **Semantic Tokens**. These resolve dynamically inside `tailwind.config.js` via `index.css` CSS-variables to flawlessly support light/dark transitions without needing a literal `dark:` prefix in the layout classes.

## Enforcement
- `npm run lint` is a hard gate for design-system regressions in `src/components` and `src/pages`.
- CI must fail on:
  - orphaned utility prefixes (for example `dark:hover:` or `group-hover:` with no class after them)
  - `dark:bg-white` / `dark:hover:bg-white`
  - low-contrast class combinations like `bg-base` + `text-white`
- If a UI requires exceptional styling, prefer adding a semantic token or shared component rather than bypassing the rule.

---

## 🏗 Tokens

Use standard Tailwind prefixes with these dynamic keys:

| Token | Utility Example | Represents | When to Use |
|:---|:---|:---|:---|
| `base` | `bg-base` | Application background | The absolute bottom layer of the page, or inactive tracker backgrounds. |
| `surface` | `bg-surface` | Elevated structural elements | Cards, panels, dialog boxes, standard button backgrounds, table headers. |
| `surface-hover` | `hover:bg-surface-hover` | Interactive state | When a table row, button, or list item is hovered or active. |
| `surface-alt` | `bg-surface-alt` | Slightly elevated surface | Selected states, inline badges, secondary panels, form group backgrounds. |
| `border-subtle` | `border-border-subtle` | Faint divider | Very subtle dividers that don't need sharp contrast. |
| `border` | `border-border` | Standard structure borders | Outline for panels, buttons, dialog boxes, and inputs. |
| `base-content` | `text-base-content` | Primary readable text | The main text color instead of absolute black/white. |
| `muted` | `text-muted` | De-emphasized text | Secondary tags, descriptions, disabled labels, or table column headers. |

**Example of what NOT to do:**
```tsx
// ❌ BAD
<div class="bg-white dark:bg-[#1a1c23] border-gray-200 dark:border-gray-800 text-gray-900 border" />
```

**Example of the spec:**
```tsx
// ✅ GOOD
<div class="bg-surface border-border text-base-content border" />
```

---

## 🧩 CSS Component Classes

Reusable component classes defined in `index.css` using `@layer base`. Use these instead of repeating the same token combinations:

| Class | What it provides | Use for |
|:---|:---|:---|
| `.card` | `bg-surface border border-border shadow-sm rounded-md` | Content panels, dialog boxes |
| `.card-hover` | `hover:bg-surface-hover transition-colors` | Hoverable list items, clickable rows |
| `.form-input` | Full input styling (border, bg, text, focus ring, disabled state) | Text inputs, textareas |
| `.form-select` | Same as `.form-input` + dropdown arrow padding | Select dropdowns |
| `.btn-secondary` | Bordered ghost button with hover | Cancel buttons, secondary actions |
| `.table-header` | `text-muted uppercase tracking-wider` | Table column headers |
| `.table-row` | `border-b border-border hover:bg-surface-hover` | Table body rows |

```tsx
// ❌ BAD — repeating tokens manually
<input class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500" />

// ✅ GOOD — use the component class
<input class="form-input" />
```

---

## 🧩 Standard Components
Before building a native toggle or stylized button from scratch, heavily favor the standardized layout elements contained in `/src/components/shared/`.

### 1. `Card` (`/src/components/shared/Card.tsx`)
The absolute core container for content across Pulse. Automatically maps to `bg-surface` and standard borders/shadows/padding.
```tsx
import { Card } from '@/components/shared/Card';

<Card hoverable border padding="md">
  Main metric panel content!
</Card>
```

### 2. `Button` (`/src/components/shared/Button.tsx`)
A unified standard button built on semantic tokens instead of inline styling.
**Variants:** `primary` | `secondary` (default) | `danger` | `ghost` | `outline`
```tsx
import { Button } from '@/components/shared/Button';

<Button variant="primary" size="md"> Save Settings </Button>
<Button variant="secondary" isLoading> Saving... </Button>
```

### 3. `Toggle` (`/src/components/shared/Toggle.tsx`)
Dynamic switch mapped to `base`, `surface`, and `blue`.
```tsx
import { Toggle } from '@/components/shared/Toggle';

<Toggle checked={true} label="Enable Discovery" description="Scans network elements"/>
```

### 4. `FilterButtonGroup` (`/src/components/shared/FilterButtonGroup.tsx`)
Segmented radio-style navigation controls. Uses `muted` for inactive buttons, and maps the active card to a highlighted `surface`.
```tsx
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';

<FilterButtonGroup 
  value={currentView()} 
  onChange={setView} 
  options={[
    { value: 'pve', label: 'Virtual Environment' },
    { value: 'pbs', label: 'Backup Server' }
  ]} 
/>
```

### 5. `Table` primitives (`/src/components/shared/Table.tsx`)
Wraps the raw HTML table structure with strict structural tags dynamically handling their hover states, borders, font weights, and tracking properties.
```tsx
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/shared/Table';

<Table>
  <TableHeader>
    <TableRow>
      <TableHead>Status</TableHead>
    </TableRow>
  </TableHeader>
  <TableBody>
    <TableRow>
      <TableCell>Online</TableCell>
    </TableRow>
  </TableBody>
</Table>
```

### 6. Form utilities (`/src/components/shared/Form.ts`)
Pre-composed class strings for form elements. Import individual exports instead of composing tokens manually.
```tsx
import { formControl, formSelect, formLabel, formField, formCheckbox } from '@/components/shared/Form';

<div class={formField}>
  <label class={formLabel}>API Key</label>
  <input class={formControl} type="text" />
</div>
```
