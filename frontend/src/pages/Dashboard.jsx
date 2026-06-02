import React, { useEffect, useState, useCallback, useMemo } from 'react';
import { Typography, Statistic, Row, Col, Segmented, Input } from 'antd';
import { AppstoreOutlined, UnorderedListOutlined, SearchOutlined, ClusterOutlined, ApiOutlined } from '@ant-design/icons';
import { useLocation } from 'react-router-dom';
import { dashboard as dashboardApi, formatBps, ensureArray } from '../api';
import { useAuth } from '../context/AuthContext';
import { useWebSocket } from '../hooks/useWebSocket';
import DeviceGroupSection from '../components/DeviceGroupSection';
import DashboardTableView from '../components/DashboardTableView';

const { Text } = Typography;

const STORAGE_VIEW_MODE = 'dashboard-view-mode';
const STORAGE_CARD_LAYOUT = 'dashboard-card-layout';

function readStoredChoice(key, allowed, fallback) {
  try {
    const value = localStorage.getItem(key);
    if (allowed.includes(value)) return value;
  } catch {
    /* ignore */
  }
  return fallback;
}

function writeStoredChoice(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* ignore */
  }
}

/** Infer group from device name: "Switch(101)-ONU-1" → "Switch(101)" */
export function getDeviceGroup(name) {
  if (!name) return 'General';
  const dash = name.search(/[-–/|]/);
  if (dash > 0) return name.slice(0, dash).trim();
  const paren = name.match(/^(.+\(\d+\))/);
  if (paren) return paren[1];
  return 'General';
}

function groupDevices(devices) {
  const map = new Map();
  for (const d of devices) {
    const g = getDeviceGroup(d.name);
    if (!map.has(g)) map.set(g, []);
    map.get(g).push(d);
  }
  return [...map.entries()]
    .sort(([a], [b]) => a.localeCompare(b, undefined, { numeric: true }))
    .map(([name, devs]) => ({
      name,
      devices: devs.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true })),
    }));
}

function mergeStatsFromOverview(data) {
  const next = {};
  for (const d of ensureArray(data)) {
    if (d.stats) next[d.id] = d.stats;
  }
  return next;
}

export default function Dashboard() {
  const location = useLocation();
  const { isAdmin } = useAuth();
  const [overview, setOverview] = useState([]);
  const [live, setLive] = useState({});
  const [deviceStatsById, setDeviceStatsById] = useState({});
  const [viewMode, setViewMode] = useState(() =>
    readStoredChoice(STORAGE_VIEW_MODE, ['cards', 'table'], 'cards')
  );
  const [cardLayout, setCardLayout] = useState(() =>
    readStoredChoice(STORAGE_CARD_LAYOUT, ['device', 'interface'], 'device')
  );
  const [filter, setFilter] = useState('');

  useEffect(() => {
    writeStoredChoice(STORAGE_VIEW_MODE, viewMode);
  }, [viewMode]);

  useEffect(() => {
    writeStoredChoice(STORAGE_CARD_LAYOUT, cardLayout);
  }, [cardLayout]);

  const load = useCallback(async () => {
    try {
      const data = await dashboardApi.overview();
      setOverview(ensureArray(data));
      setDeviceStatsById((prev) => ({ ...prev, ...mergeStatsFromOverview(data) }));
    } catch {
      setOverview([]);
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  // Refresh when navigating back to the dashboard (e.g. after adding a device).
  useEffect(() => {
    if (location.pathname === '/') load();
  }, [location.pathname, load]);

  const deviceIds = useMemo(() => ensureArray(overview).map((d) => d.id), [overview]);

  useWebSocket(deviceIds, (msg) => {
    if (msg.type === 'status') {
      const st = msg.payload;
      setOverview((prev) =>
        ensureArray(prev).map((d) =>
          d.id === st.device_id
            ? { ...d, online: st.online, last_error: st.error || '' }
            : d
        )
      );
      return;
    }
    if (msg.type === 'device_stats') {
      const st = msg.payload;
      if (st?.device_id != null) {
        setDeviceStatsById((prev) => ({ ...prev, [st.device_id]: st }));
      }
      return;
    }
    if (!msg.device_id || !msg.interface) return;
    const key = `${msg.device_id}:${msg.interface}`;
    setLive((prev) => ({
      ...prev,
      [key]: { tx: msg.tx_bps || 0, rx: msg.rx_bps || 0 },
    }));
    setOverview((prev) =>
      ensureArray(prev).map((d) => {
        if (d.id !== msg.device_id) return d;
        const interfaces = ensureArray(d.interfaces).map((i) =>
          i.interface_name === msg.interface
            ? { ...i, tx_bps: msg.tx_bps || 0, rx_bps: msg.rx_bps || 0 }
            : i
        );
        return { ...d, interfaces };
      })
    );
  });

  const filteredOverview = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return ensureArray(overview);
    return ensureArray(overview).filter(
      (d) =>
        d.name?.toLowerCase().includes(q) ||
        d.host?.toLowerCase().includes(q) ||
        ensureArray(d.interfaces).some((i) =>
          i.interface_name?.toLowerCase().includes(q)
        )
    );
  }, [overview, filter]);

  const groups = useMemo(() => groupDevices(filteredOverview), [filteredOverview]);

  const stats = useMemo(() => {
    const devs = ensureArray(overview);
    let online = 0;
    let totalTx = 0;
    let totalRx = 0;
    for (const d of devs) {
      if (d.online) online++;
      for (const i of ensureArray(d.interfaces)) {
        const key = `${d.id}:${i.interface_name}`;
        const l = live[key];
        totalTx += l?.tx ?? i.tx_bps ?? 0;
        totalRx += l?.rx ?? i.rx_bps ?? 0;
      }
    }
    return { devices: devs.length, online, totalTx, totalRx };
  }, [overview, live]);

  return (
    <div className="dashboard-page">
      <Row gutter={12} align="middle" style={{ marginBottom: 10 }}>
        <Col flex="auto">
          <Row gutter={12}>
            <Col>
              <Statistic title="Devices" value={stats.devices} valueStyle={{ fontSize: 16 }} />
            </Col>
            <Col>
              <Statistic
                title="Online"
                value={stats.online}
                valueStyle={{ fontSize: 16, color: '#52c41a' }}
              />
            </Col>
            <Col>
              <Statistic
                title="Total TX"
                value={formatBps(stats.totalTx)}
                valueStyle={{ fontSize: 16, color: '#69b1ff' }}
              />
            </Col>
            <Col>
              <Statistic
                title="Total RX"
                value={formatBps(stats.totalRx)}
                valueStyle={{ fontSize: 16, color: '#95de64' }}
              />
            </Col>
          </Row>
        </Col>
        <Col>
          <Segmented
            size="small"
            value={viewMode}
            onChange={setViewMode}
            options={[
              { value: 'cards', icon: <AppstoreOutlined />, label: 'Cards' },
              { value: 'table', icon: <UnorderedListOutlined />, label: 'Table' },
            ]}
          />
        </Col>
      </Row>

      <div className="dashboard-toolbar">
        <Input
          size="small"
          allowClear
          prefix={<SearchOutlined style={{ color: '#666' }} />}
          placeholder="Filter devices, hosts, interfaces…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          style={{ maxWidth: 280 }}
        />
        {viewMode === 'cards' && (
          <Segmented
            size="small"
            value={cardLayout}
            onChange={setCardLayout}
            options={[
              { value: 'device', icon: <ClusterOutlined />, label: 'By device' },
              { value: 'interface', icon: <ApiOutlined />, label: 'By interface' },
            ]}
          />
        )}
        <Text type="secondary" style={{ fontSize: 11 }}>
          {viewMode === 'cards'
            ? cardLayout === 'device'
              ? 'One card per device — interfaces shown as traffic badges. Click a card for charts.'
              : 'One card per interface — each port shown separately within its device group.'
            : 'Table view — one row per interface.'}
        </Text>
      </div>

      {viewMode === 'cards' ? (
        <div className="dashboard-card-view">
          {groups.length === 0 ? (
            <Text type="secondary">No devices match your filter.</Text>
          ) : (
            groups.map((g) => (
              <DeviceGroupSection
                key={g.name}
                groupName={g.name}
                devices={g.devices}
                live={live}
                cardLayout={cardLayout}
                deviceStatsById={deviceStatsById}
                isAdmin={isAdmin}
              />
            ))
          )}
        </div>
      ) : (
        <DashboardTableView overview={filteredOverview} live={live} />
      )}
    </div>
  );
}
