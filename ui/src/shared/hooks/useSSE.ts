import { useEffect, useRef, useState } from 'react';

export type SSEStatus = 'idle' | 'connecting' | 'streaming' | 'done' | 'error';

interface SSEOptions<T> {
  onMessage?: (data: T) => void;
  onDone?: () => void;
  onError?: () => void;
}

export function useSSE<T>(
  url: string,
  enabled: boolean,
  options: SSEOptions<T> = {}
): { status: SSEStatus } {
  const [status, setStatus] = useState<SSEStatus>('idle');
  const sourceRef = useRef<EventSource | null>(null);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  useEffect(() => {
    if (!enabled || !url) {
      setStatus('idle');
      return;
    }

    setStatus('connecting');
    const source = new EventSource(url, { withCredentials: true });
    sourceRef.current = source;

    source.onopen = () => setStatus('streaming');

    source.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as T;
        optionsRef.current.onMessage?.(data);
      } catch (err) {
        console.error('[SSE] JSON parse error:', err);
      }
    };

    source.addEventListener('done', () => {
      setStatus('done');
      optionsRef.current.onDone?.();
      source.close();
    });

    source.onerror = () => {
      setStatus('error');
      optionsRef.current.onError?.();
      source.close();
    };

    return () => {
      source.close();
      sourceRef.current = null;
      setStatus('idle');
    };
  }, [url, enabled]);

  return { status };
}
