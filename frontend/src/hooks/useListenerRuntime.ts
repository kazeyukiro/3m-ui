import { useCallback, useEffect, useRef, useState } from 'react';
import { checkListenerRuntime, fetchListenerRuntime, testListenerConnection, type ListenerRuntime } from '../api/listenerRuntime';
import type { Listener } from '../api/nodes';
import { startVisiblePolling } from '../utils/visiblePolling';

export function useListenerRuntime(listeners: Listener[], detailID?: number) {
  const [statuses, setStatuses] = useState<Record<number, ListenerRuntime>>({});
  const [details, setDetails] = useState<Record<number, ListenerRuntime>>({});
  const [checking, setChecking] = useState<number[]>([]);
  const [testing, setTesting] = useState<number[]>([]);
  const tests = useRef(new Map<number, AbortController>());
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; const pending = tests.current; return () => { mounted.current = false; for (const c of pending.values()) c.abort(); pending.clear(); }; }, []);
  const merge = useCallback((updates: ListenerRuntime[]) => {
    if (!mounted.current) return;
    setStatuses(previous => {
      const next = { ...previous };
      for (const value of updates) {
        if (!next[value.id] || (Date.parse(value.checked_at) || 0) >= (Date.parse(next[value.id].checked_at) || 0)) next[value.id] = value;
      }
      return next;
    });
  }, []);

  const mergeDetails = useCallback((result: ListenerRuntime) => {
    if (!mounted.current) return;
    setDetails(previous => !previous[result.id] || (Date.parse(result.checked_at) || 0) >= (Date.parse(previous[result.id].checked_at) || 0)
      ? { ...previous, [result.id]: result } : previous);
    merge([result]);
  }, [merge]);

  const check = useCallback(async (id: number, signal?: AbortSignal) => {
    setChecking(previous => [...previous, id]);
    try {
      const result = await checkListenerRuntime(id, signal);
      if (!signal?.aborted) mergeDetails(result);
      return result;
    } catch (error) {
      if (mounted.current && !signal?.aborted) {
        const failed: ListenerRuntime = { id, state: 'unknown', reason: 'status_unavailable', checked_at: '', endpoints: [] };
        setStatuses(previous => ({ ...previous, [id]: failed }));
        setDetails(previous => ({ ...previous, [id]: failed }));
      }
      throw error;
    } finally {
      if (mounted.current) setChecking(previous => previous.filter(value => value !== id));
    }
  }, [mergeDetails]);

  const test = useCallback(async (id: number, reuse = false) => {
    if (tests.current.has(id)) return;
    const controller = new AbortController();
    tests.current.set(id, controller);
    setTesting(previous => [...previous, id]);
    try {
      const result = await testListenerConnection(id, controller.signal, reuse);
      if (!controller.signal.aborted) mergeDetails(result);
      return result;
    } catch (error) {
      if (mounted.current && !controller.signal.aborted) {
        const failed: ListenerRuntime = { id, state: 'unknown', reason: 'status_unavailable', checked_at: new Date().toISOString(), endpoints: [] };
        mergeDetails(failed);
      }
      throw error;
    } finally {
      tests.current.delete(id);
      if (mounted.current) setTesting(previous => previous.filter(value => value !== id));
    }
  }, [mergeDetails]);

  useEffect(() => startVisiblePolling(async signal => {
    try {
      const result = await fetchListenerRuntime(signal);
      if (signal.aborted) return;
      merge(result);
      // Fetch endpoints only for the open drawer, keeping its observations
      // current without downloading every listener's ranges on each poll.
      if (detailID !== undefined) {
        // check invalidates just this node if its detailed request fails.
        await check(detailID, signal).catch(() => {});
      }
    } catch {
      if (!signal.aborted) {
        setStatuses(previous => Object.fromEntries(listeners.map(l => [l.id, {
          id: l.id, state: l.enabled ? 'unknown' : 'disabled',
          reason: l.enabled ? 'status_unavailable' : 'disabled', checked_at: previous[l.id]?.checked_at || '', endpoints: [],
        }])));
        if (detailID !== undefined) setDetails(previous => ({ ...previous, [detailID]: {
          id: detailID, state: 'unknown', reason: 'status_unavailable', checked_at: previous[detailID]?.checked_at || '', endpoints: [],
        } }));
      }
    }
  }), [listeners, detailID, merge, check]);

  return { statuses, details, checking, check, testing, test };
}
