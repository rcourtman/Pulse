import { Component, JSX, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  STORAGE_BANNER_ACTION_BUTTON_CLASS,
  STORAGE_PAGE_BANNER_ROW_CLASS,
  STORAGE_PAGE_BANNER_TEXT_CLASS,
  type StoragePageBannerKind,
} from '@/features/storageBackups/storagePagePresentation';
import { useStoragePageBannerModel } from './useStoragePageBannerModel';

type StoragePageBannerProps = {
  kind: StoragePageBannerKind;
  onAction?: JSX.EventHandlerUnion<HTMLButtonElement, MouseEvent>;
};

export const StoragePageBanner: Component<StoragePageBannerProps> = (props) => {
  const model = useStoragePageBannerModel({
    kind: () => props.kind,
  });

  return (
    <Card padding="sm" tone="warning">
      <div class={STORAGE_PAGE_BANNER_ROW_CLASS}>
        <span class={STORAGE_PAGE_BANNER_TEXT_CLASS}>{model.message()}</span>
        <Show when={model.actionLabel()}>
          {(label) => (
            <button
              type="button"
              onClick={props.onAction}
              class={STORAGE_BANNER_ACTION_BUTTON_CLASS}
            >
              {label()}
            </button>
          )}
        </Show>
      </div>
    </Card>
  );
};

export default StoragePageBanner;
