import { createSignal } from 'solid-js';
import type { APITokenRecord } from '@/api/security';

export interface TokenRevealPayload {
  token: string;
  record: APITokenRecord;
  source?: string;
  note?: string;
}

interface TokenRevealState extends TokenRevealPayload {
  issuedAt: number;
}

const [state, setState] = createSignal<TokenRevealState | null>(null);

export const tokenRevealStore = {
  state,
  show(payload: TokenRevealPayload) {
    setState({ ...payload, issuedAt: Date.now() });
  },
  dismiss() {
    setState(null);
  },
};

export const useTokenRevealState = () => state;
export const showTokenReveal = (payload: TokenRevealPayload) => tokenRevealStore.show(payload);
export const dismissTokenReveal = () => tokenRevealStore.dismiss();
