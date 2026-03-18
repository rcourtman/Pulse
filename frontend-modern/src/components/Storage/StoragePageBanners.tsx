import { Component, Match, Switch } from 'solid-js';
import StoragePageBanner from '@/components/Storage/StoragePageBanner';
import { useStoragePageBannersModel } from './useStoragePageBannersModel';

type StoragePageBannersProps = {
  kind: () => 'reconnecting' | 'fetch-error' | 'disconnected' | 'waiting-for-data' | null;
  reconnect: () => void;
};

export const StoragePageBanners: Component<StoragePageBannersProps> = (props) => {
  const model = useStoragePageBannersModel({
    kind: props.kind,
  });

  return (
    <Switch>
      <Match when={model.reconnectActionKind() === 'reconnecting'}>
        <StoragePageBanner kind="reconnecting" onAction={props.reconnect} />
      </Match>
      <Match when={props.kind() === 'fetch-error'}>
        <StoragePageBanner kind="fetch-error" />
      </Match>
      <Match when={model.reconnectActionKind() === 'disconnected'}>
        <StoragePageBanner kind="disconnected" onAction={props.reconnect} />
      </Match>
      <Match when={props.kind() === 'waiting-for-data'}>
        <StoragePageBanner kind="waiting-for-data" />
      </Match>
    </Switch>
  );
};

export default StoragePageBanners;
