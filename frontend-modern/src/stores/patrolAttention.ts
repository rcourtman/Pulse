import { createSignal } from 'solid-js';
import {
  getPatrolAttention,
  getPatrolAttentionDetail,
  getPatrolAttentionSummary,
  type AttentionFilter,
  type AttentionItem,
  type AttentionItemDetail,
  type AttentionSummary,
} from '@/api/patrolAttention';

const [summary, setSummary] = createSignal<AttentionSummary | null>(null);
const [items, setItems] = createSignal<AttentionItem[]>([]);
const [selectedDetail, setSelectedDetail] = createSignal<AttentionItemDetail | null>(null);
const [loading, setLoading] = createSignal(false);
const [detailLoading, setDetailLoading] = createSignal(false);
const [error, setError] = createSignal<string | null>(null);
const [filter, setFilter] = createSignal<AttentionFilter>('active');

let listRequest = 0;
let detailRequest = 0;

export const patrolAttentionStore = {
  summary,
  items,
  selectedDetail,
  loading,
  detailLoading,
  error,
  filter,

  setFilter,

  async loadSummary() {
    try {
      const value = await getPatrolAttentionSummary();
      setSummary(value);
      return value;
    } catch (cause) {
      setSummary(null);
      throw cause;
    }
  },

  async load(nextFilter: AttentionFilter = filter()) {
    const request = ++listRequest;
    setFilter(nextFilter);
    setLoading(true);
    setError(null);
    try {
      const response = await getPatrolAttention(nextFilter);
      if (request !== listRequest) return;
      setItems(response.data);
      setSummary(response.summary);
    } catch (cause) {
      if (request !== listRequest) return;
      setItems([]);
      setSummary(null);
      setError(cause instanceof Error ? cause.message : 'Patrol attention is unavailable.');
    } finally {
      if (request === listRequest) {
        setLoading(false);
      }
    }
  },

  async select(itemId: string | null) {
    const request = ++detailRequest;
    if (!itemId) {
      setSelectedDetail(null);
      setDetailLoading(false);
      return;
    }
    setDetailLoading(true);
    try {
      const detail = await getPatrolAttentionDetail(itemId);
      if (request === detailRequest) {
        setSelectedDetail(detail);
      }
    } catch {
      if (request === detailRequest) {
        setSelectedDetail(null);
      }
    } finally {
      if (request === detailRequest) {
        setDetailLoading(false);
      }
    }
  },

  clear() {
    listRequest++;
    detailRequest++;
    setSummary(null);
    setItems([]);
    setSelectedDetail(null);
    setLoading(false);
    setDetailLoading(false);
    setError(null);
    setFilter('active');
  },
};
