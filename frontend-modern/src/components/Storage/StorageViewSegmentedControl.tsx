import { splitProps, type Component, type JSX } from 'solid-js';
import DatabaseIcon from 'lucide-solid/icons/database';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import type { StorageView } from './storagePageState';

interface StorageViewSegmentedControlProps extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'onChange'
> {
  value: StorageView;
  onChange: (value: StorageView) => void;
}

export const StorageViewSegmentedControl: Component<StorageViewSegmentedControlProps> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange']);

  return (
    <FilterSegmentedControl
      {...divProps}
      role={divProps.role ?? 'group'}
      aria-label={divProps['aria-label'] ?? 'Storage table view'}
      value={local.value}
      onChange={(value) => local.onChange(value as StorageView)}
      options={[
        {
          value: 'pools',
          title: 'Show storage pools and backup targets',
          label: (
            <>
              <DatabaseIcon aria-hidden="true" class="h-3 w-3" />
              Storage
            </>
          ),
        },
        {
          value: 'disks',
          title: 'Show physical disks',
          label: (
            <>
              <HardDriveIcon aria-hidden="true" class="h-3 w-3" />
              Physical Disks
            </>
          ),
        },
      ]}
    />
  );
};

export default StorageViewSegmentedControl;
