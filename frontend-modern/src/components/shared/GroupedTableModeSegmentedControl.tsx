import { splitProps, type Component, type JSX } from 'solid-js';
import FolderTreeIcon from 'lucide-solid/icons/folder-tree';
import ListIcon from 'lucide-solid/icons/list';
import { FilterSegmentedControl } from './FilterToolbar';

export type GroupedTableMode = 'grouped' | 'flat';

export const GROUPED_TABLE_MODE_ARIA_LABEL = 'Group by';
export const GROUPED_TABLE_MODE_GROUPED_TITLE = 'Grouped table view';
export const GROUPED_TABLE_MODE_FLAT_TITLE = 'Flat list view';

interface GroupedTableModeSegmentedControlProps extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'onChange'
> {
  value: GroupedTableMode;
  onChange: (value: GroupedTableMode) => void;
}

export const GroupedTableModeSegmentedControl: Component<GroupedTableModeSegmentedControlProps> = (
  props,
) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange']);

  return (
    <FilterSegmentedControl
      {...divProps}
      aria-label={divProps['aria-label'] ?? GROUPED_TABLE_MODE_ARIA_LABEL}
      value={local.value}
      onChange={(value) => local.onChange(value as GroupedTableMode)}
      options={[
        {
          value: 'grouped',
          title: GROUPED_TABLE_MODE_GROUPED_TITLE,
          label: (
            <>
              <FolderTreeIcon class="w-3 h-3" />
              Grouped
            </>
          ),
        },
        {
          value: 'flat',
          title: GROUPED_TABLE_MODE_FLAT_TITLE,
          label: (
            <>
              <ListIcon class="w-3 h-3" />
              List
            </>
          ),
        },
      ]}
    />
  );
};
