import React, { useEffect, useState, useMemo, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Tabs, Typography, Row, Col, Tag } from 'antd';
import { devices as devicesApi, formatBps, ensureArray } from '../api';
import { useWebSocket } from '../hooks/useWebSocket';
import TrafficChart from '../components/TrafficChart';

const { Text } = Typography;
const MAX_SAMPLES = 180; // keep up to 3 minutes of live samples at 1-s poll

export default function DeviceDetail() {
  const { id } = useParams();
  const deviceId = parseInt(id, 10);
  const [device, setDevice] = useState(null);
  const [ifaces, setIfaces] = useState([]);
  // history: { [ifaceName]: TrafficSample[] } — loaded once from REST API
  const [history, setHistory] = useState({});
  // liveTail: { [ifaceName]: TrafficSample[] } — appended from WS
  const [liveTail, setLiveTail] = useState({});
  // liveStats: { [ifaceName]: {tx, rx} } — latest values for header display
  const [liveStats, setLiveStats] = useState({});
  // Track the latest sample timestamp per interface to avoid duplicates.
  const latestTs = useRef({});

  const load = useCallback(async () => {
    const [d, ifs] = await Promise.all([
      devicesApi.get(deviceId),
      devicesApi.interfaces(deviceId),
    ]);
    setDevice(d);
    setIfaces(ensureArray(ifs));
  }, [deviceId]);

  useEffect(() => {
    load();
  }, [load]);

  useWebSocket([deviceId], (msg) => {
    if (msg.type === 'status') {
      const st = msg.payload;
      setDevice((d) => (d ? { ...d, online: st.online, last_error: st.error || '' } : d));
      return;
    }
    if (msg.device_id !== deviceId) return;

    const key = msg.interface;
    const tsMs = msg.ts ? new Date(msg.ts).getTime() : Date.now();

    // Skip if we've already seen a sample at or after this timestamp.
    if (latestTs.current[key] && tsMs <= latestTs.current[key]) return;
    latestTs.current[key] = tsMs;

    const sample = { tx_bps: msg.tx_bps || 0, rx_bps: msg.rx_bps || 0, ts: msg.ts };

    setLiveStats((prev) => ({ ...prev, [key]: { tx: msg.tx_bps || 0, rx: msg.rx_bps || 0 } }));
    setLiveTail((prev) => ({
      ...prev,
      [key]: [...(prev[key] || []), sample].slice(-MAX_SAMPLES),
    }));
  });

  const grouped = useMemo(() => {
    const g = {};
    for (const i of ensureArray(ifaces)) {
      const t = i.interface_type || 'other';
      if (!g[t]) g[t] = [];
      g[t].push(i);
    }
    return g;
  }, [ifaces]);

  const loadHistory = useCallback(
    async (iface) => {
      try {
        const samples = await devicesApi.history(deviceId, iface, 1);
        const arr = ensureArray(samples);
        setHistory((prev) => ({ ...prev, [iface]: arr }));
        // Seed latestTs so WS samples that overlap history are dropped.
        if (arr.length > 0) {
          const last = arr[arr.length - 1];
          const tsMs = last.ts ? new Date(last.ts).getTime() : 0;
          if (!latestTs.current[iface] || tsMs > latestTs.current[iface]) {
            latestTs.current[iface] = tsMs;
          }
        }
      } catch (_) {}
    },
    [deviceId],
  );

  useEffect(() => {
    ensureArray(ifaces).forEach((i) => loadHistory(i.interface_name));
  }, [ifaces, loadHistory]);

  if (!device) return null;

  const tabItems = Object.keys(grouped)
    .sort()
    .map((type) => ({
      key: type,
      label: `${type} (${grouped[type].length})`,
      children: (
        <Row gutter={[8, 8]}>
          {(grouped[type] || []).map((iface) => {
            const name = iface.interface_name;
            const stats = liveStats[name];
            const base = history[name] || [];
            const tail = liveTail[name] || [];
            // Merge: historical samples + new live tail (already deduplicated).
            const allSamples =
              tail.length > 0
                ? [...base, ...tail].slice(-MAX_SAMPLES)
                : base;

            return (
              <Col span={12} key={iface.id}>
                <div style={{ background: '#1f1f1f', padding: 8, borderRadius: 4 }}>
                  <div
                    style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}
                  >
                    <Text strong style={{ fontSize: 12 }}>
                      {name}
                    </Text>
                    <span style={{ fontSize: 10 }}>
                      TX {stats ? formatBps(stats.tx) : '—'} / RX{' '}
                      {stats ? formatBps(stats.rx) : '—'}
                    </span>
                  </div>
                  <TrafficChart samples={allSamples} width={480} height={100} />
                </div>
              </Col>
            );
          })}
        </Row>
      ),
    }));

  return (
    <div>
      <Link to="/">← Dashboard</Link>
      <div style={{ margin: '4px 0 8px' }}>
        <Text strong>{device.name}</Text> <Text type="secondary">{device.host}</Text>{' '}
        <Tag color={device.online ? 'green' : 'red'}>
          {device.online ? 'Online' : 'Offline'}
        </Tag>
        {!device.online && device.last_error && (
          <Text type="danger" style={{ fontSize: 11, marginLeft: 8 }}>
            {device.last_error}
          </Text>
        )}
      </div>
      <Tabs size="small" items={tabItems} />
    </div>
  );
}
