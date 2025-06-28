# Comprehensive Table Styling Comparison Report

This report analyzes styling differences between the main dashboard table and the other tables (Snapshots, Backups, Storage) in the Pulse application.

## 1. Main Dashboard Table (index.html + dashboard.js)

### Table Element
```html
<table id="main-table" class="w-full min-w-[1100px] table-fixed text-xs sm:text-sm" role="table" aria-label="Virtual machines and containers">
```
- **Width**: `w-full min-w-[1100px]`
- **Layout**: `table-fixed`
- **Text size**: `text-xs sm:text-sm`
- **Role**: `role="table"`
- **Aria label**: `aria-label="Virtual machines and containers"`

### Header Styling
```html
<thead class="bg-gray-100 dark:bg-gray-800">
  <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
    <th class="sticky left-0 top-0 bg-gray-100 dark:bg-gray-700 z-20 sortable p-1 px-2">Name</th>
    <th class="sticky top-0 bg-gray-100 dark:bg-gray-700 z-10 p-1 px-2">Type</th>
    <!-- etc... -->
  </tr>
</thead>
```
- **Background**: Double background classes on `tr`
- **Text**: `text-gray-600 dark:text-gray-300`
- **Sticky headers**: First column has `z-20`, others have `z-10`
- **Padding**: `p-1 px-2`

### Row Styling (from dashboard.js)
```javascript
// Base row classes
const baseClasses = 'border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700';

// Node header rows
nodeHeaderRow.className = 'node-header bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs';
```

### Cell Styling
```javascript
// Name cell (sticky)
const stickyNameCell = PulseApp.ui.common.createStickyColumn(nameContent, { title: guest.name });
// Creates: "sticky left-0 z-10 py-1 px-2 align-middle whitespace-nowrap overflow-hidden text-ellipsis max-w-0"

// Regular cells - most have default text color (inherit)
row.appendChild(PulseApp.ui.common.createTableCell(typeIcon)); // No additional classes
row.appendChild(PulseApp.ui.common.createTableCell(guest.vmid)); // No additional classes
row.appendChild(PulseApp.ui.common.createTableCell(uptimeDisplay, 'py-1 px-2 align-middle whitespace-nowrap overflow-hidden text-ellipsis'));
```

**Key observation**: Most cells use inherited text color, only special cells get explicit text color classes.

---

## 2. Snapshots Table (snapshots.js)

### Table Element
```html
<table class="w-full text-xs sm:text-sm">
```
- **Width**: `w-full` (no min-width)
- **Layout**: Not specified (defaults to auto)
- **Text size**: `text-xs sm:text-sm`
- **No role or aria-label**

### Header Styling
```html
<thead class="bg-gray-100 dark:bg-gray-800">
  <tr class="text-[10px] sm:text-xs font-medium tracking-wider text-left text-gray-600 uppercase bg-gray-100 dark:bg-gray-700 dark:text-gray-300 border-b border-gray-300 dark:border-gray-600">
    <th class="sortable p-1 px-2 whitespace-nowrap">VMID</th>
    <!-- etc... -->
  </tr>
</thead>
```
- **Background**: Same double background pattern
- **Text**: Same as main table
- **No sticky headers**
- **Padding**: `p-1 px-2`

### Row Styling
```html
<tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700">
```
- Same as main table base row

### Cell Styling
```html
<td class="p-1 px-2 align-middle">${snapshot.vmid}</td>
<td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400 whitespace-nowrap">${age}</td>
```
- **VMID cell**: No text color (inherits default)
- **Age cell**: Explicit `text-gray-500 dark:text-gray-400`
- **Description cell**: Also has `text-gray-500 dark:text-gray-400`

---

## 3. Backups Table (backups.js)

### Table Element
```html
<table class="w-full text-xs sm:text-sm">
```
- Same as snapshots table

### Header Styling
- Identical to snapshots table

### Row Styling
- Identical to snapshots table

### Cell Styling
```html
<td class="p-1 px-2 align-middle">${backup.vmid}</td>
<td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400">
  <div class="max-w-[120px] sm:max-w-[200px] lg:max-w-[300px] truncate">
    ${backup.notes || '-'}
  </div>
</td>
<td class="p-1 px-2 align-middle whitespace-nowrap">${formatBytes(backup.size)}</td>
<td class="p-1 px-2 align-middle text-gray-500 dark:text-gray-400 whitespace-nowrap">${age}</td>
```
- **VMID cell**: No text color
- **Notes cell**: `text-gray-500 dark:text-gray-400`
- **Storage cell**: `text-gray-500 dark:text-gray-400`
- **Size cell**: No text color
- **Age cell**: `text-gray-500 dark:text-gray-400`

---

## 4. Storage Table (storage.js)

### Table Element
```javascript
table.className = 'w-full text-sm border-collapse table-auto min-w-full';
```
- **Width**: `w-full min-w-full`
- **Layout**: `table-auto`
- **Text size**: `text-sm` (not responsive)
- **Border**: `border-collapse`

### Header Styling
```html
<tr class="border-b border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 text-xs font-medium tracking-wider text-left text-gray-600 uppercase dark:text-gray-300">
  <th class="sticky left-0 top-0 bg-gray-50 dark:bg-gray-700 z-20 p-1 px-2">Storage</th>
  <th class="sticky top-0 bg-gray-50 dark:bg-gray-700 z-10 p-1 px-2">Content</th>
  <!-- etc... -->
</tr>
```
- **Background**: Single background (cleaner)
- **Border**: Different border color (`gray-300/600` vs `gray-300/600`)
- **Text**: Same as other tables
- **Sticky headers**: Same pattern as main table

### Row Styling
```javascript
// Regular rows
const newRow = PulseApp.ui.common.createTableRow({
    classes: additionalClasses,
    isSpecialRow: !!(specialBgClass),
    specialBgClass: specialBgClass
});

// Node header rows  
nodeHeaderRow = PulseApp.ui.common.createTableRow({
    classes: 'bg-gray-100 dark:bg-gray-700/80 font-semibold text-gray-700 dark:text-gray-300 text-xs node-storage-header',
    baseClasses: '' // Override base classes
});
```

### Cell Styling
```javascript
// Sticky storage name column
const stickyStorageCell = PulseApp.ui.common.createStickyColumn(storageNameContent, {
    additionalClasses: 'text-gray-900 dark:text-gray-100'
});

// Regular cells - ALL have explicit text colors
row.appendChild(PulseApp.ui.common.createTableCell(contentBadges, 'p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs'));
row.appendChild(PulseApp.ui.common.createTableCell(store.type || 'N/A', 'p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300 text-xs'));
row.appendChild(PulseApp.ui.common.createTableCell(sharedText, 'p-1 px-2 whitespace-nowrap text-center'));
row.appendChild(PulseApp.ui.common.createTableCell(usageBarHTML, 'p-1 px-2 text-gray-600 dark:text-gray-300 min-w-[250px]'));
row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.avail), 'p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300'));
row.appendChild(PulseApp.ui.common.createTableCell(PulseApp.utils.formatBytes(store.total), 'p-1 px-2 whitespace-nowrap text-gray-600 dark:text-gray-300'));
```

**Key observation**: Storage table explicitly sets `text-gray-600 dark:text-gray-300` on EVERY cell except the shared text cell.

---

## Summary of Key Differences

### 1. Table Layout
- **Main**: `table-fixed` with `min-w-[1100px]`
- **Snapshots/Backups**: `table-auto` (default) with no min-width
- **Storage**: `table-auto` with `min-w-full`

### 2. Text Sizing
- **Main/Snapshots/Backups**: Responsive `text-xs sm:text-sm`
- **Storage**: Fixed `text-sm`

### 3. Header Background Pattern
- **Main/Snapshots/Backups**: Double background classes on `tr` (redundant)
- **Storage**: Single background (cleaner)

### 4. Text Color Philosophy (BIGGEST DIFFERENCE)
- **Main Dashboard**: Most cells inherit text color (no explicit classes)
- **Snapshots**: VMID/Name inherit, others get `text-gray-500`
- **Backups**: VMID/Size inherit, others get `text-gray-500`
- **Storage**: EVERY cell gets explicit `text-gray-600 dark:text-gray-300`

### 5. Sticky Columns
- **Main/Storage**: First column is sticky with `z-20`
- **Snapshots/Backups**: No sticky columns

### 6. Cell Text Sizes
- **Storage**: Additional `text-xs` on cells (making text smaller than header)
- **Others**: No additional text size classes on cells

### 7. Special Row Handling
- **Main**: Simple class-based approach
- **Storage**: Complex options object with special background handling

### 8. Node/Group Headers
- **Main**: Simple class string
- **Storage**: Uses options object with baseClasses override

The most significant difference is the text color approach - the storage table explicitly sets text colors on every cell, while other tables rely more on inheritance. This creates inconsistency in how text colors are managed across the application.