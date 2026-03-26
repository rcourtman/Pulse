import type { PortalMutationState, PortalQueryState } from './types';

export function resetMutationState(state: PortalMutationState): void {
  state.pending = false;
  state.error = '';
}

export function beginMutationState(state: PortalMutationState): void {
  resetMutationState(state);
  state.pending = true;
}

export function succeedMutationState(state: PortalMutationState): void {
  resetMutationState(state);
}

export function failMutationState(state: PortalMutationState, message: string): void {
  state.pending = false;
  state.error = message;
}

export function beginQueryState<T>(state: PortalQueryState<T>, emptyData: T): void {
  state.status = 'loading';
  state.error = '';
  state.data = emptyData;
}

export function resolveQueryState<T>(state: PortalQueryState<T>, data: T): void {
  state.status = 'ready';
  state.error = '';
  state.data = data;
}

export function failQueryState<T>(state: PortalQueryState<T>, emptyData: T, message: string): void {
  state.status = 'error';
  state.error = message;
  state.data = emptyData;
}
