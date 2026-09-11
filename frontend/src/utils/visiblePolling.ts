// Poll only while visible, with at most one request in flight. A hidden or
// unmounted page aborts its request; consumers must ignore aborted results.
export function startVisiblePolling(poll: (signal: AbortSignal) => Promise<void>, interval: number | (() => number) = 10000) {
  let stopped = false;
  let running = false;
  let resumePending = false;
  let controller: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const run = async () => {
    if (stopped || document.hidden || running) return;
    running = true;
    resumePending = false;
    controller = new AbortController();
    try {
      await poll(controller.signal);
    } catch {
      // The consumer reports request failures; polling survives rejections.
    } finally {
      running = false;
      if (!stopped && !document.hidden) {
        timer = setTimeout(() => { void run(); }, resumePending ? 0 : typeof interval === 'function' ? interval() : interval);
      }
    }
  };

  const visibilityChanged = () => {
    clearTimeout(timer);
    if (document.hidden) {
      controller?.abort();
    } else if (running) {
      resumePending = true;
    } else {
      void run();
    }
  };

  document.addEventListener('visibilitychange', visibilityChanged);
  void run();
  return () => {
    stopped = true;
    clearTimeout(timer);
    controller?.abort();
    document.removeEventListener('visibilitychange', visibilityChanged);
  };
}
