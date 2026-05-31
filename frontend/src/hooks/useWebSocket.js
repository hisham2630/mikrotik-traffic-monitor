import { useEffect, useRef, useCallback } from 'react';

const RECONNECT_DELAY_MS = 3000;

export function useWebSocket(deviceIds, onMessage) {
  const wsRef = useRef(null);
  const onMessageRef = useRef(onMessage);
  const deviceIdsRef = useRef(deviceIds);
  const reconnectTimer = useRef(null);
  const unmounted = useRef(false);

  // Keep refs current so callbacks always use the latest values.
  onMessageRef.current = onMessage;
  deviceIdsRef.current = deviceIds;

  const sendSubscribe = useCallback((ws, ids) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ action: 'subscribe', device_ids: ids || [] }));
    }
  }, []);

  const connect = useCallback(() => {
    if (unmounted.current) return;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${window.location.host}/ws`);
    wsRef.current = ws;

    ws.onopen = () => {
      sendSubscribe(ws, deviceIdsRef.current);
    };

    ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data);
        onMessageRef.current(data);
      } catch (_) {}
    };

    ws.onclose = () => {
      if (unmounted.current) return;
      reconnectTimer.current = setTimeout(() => connect(), RECONNECT_DELAY_MS);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [sendSubscribe]);

  // Connect once on mount; reconnect loop handles the rest.
  useEffect(() => {
    unmounted.current = false;
    connect();
    return () => {
      unmounted.current = true;
      clearTimeout(reconnectTimer.current);
      if (wsRef.current) {
        wsRef.current.onclose = null; // Prevent reconnect on intentional close.
        wsRef.current.close();
      }
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Re-subscribe when the device ID list changes.
  useEffect(() => {
    sendSubscribe(wsRef.current, deviceIds);
  }, [deviceIds, sendSubscribe]);

  return {};
}
