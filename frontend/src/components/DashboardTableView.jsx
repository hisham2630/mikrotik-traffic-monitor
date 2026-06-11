import React, { useMemo } from 'react';
import { Table, Tag, Typography, Tooltip, Space } from 'antd';
import { Link } from 'react-router-dom';
import { formatBps, ensureArray } from '../api';
import { sortByName } from '../utils/sort';

const { Text } = Typography;

function flattenRows(overview) {
  const rows = [];
  for (const d of ensureArray(overview)) {
    const ifaces = sortByName(ensureArray(d.interfaces));
    if (ifaces.length === 0) {
      rows.push({
        key: `${d.id}-none`,
        deviceId: d.id,
        deviceName: d.name,
        host: d.host,
        online: d.online,
        lastError: d.last_error,
        enabled: d.enabled,
        polling: d.polling_interval_sec,
        iface: null,
        ifaceType: null,
        tx: 0,
        rx: 0,
      });
      continue;
    }
    for (const i of ifaces) {
      rows.push({
        key: `${d.id}-${i.interface_name}`,
        deviceId: d.id,
        deviceName: d.name,
        host: d.host,
        online: d.online,
        lastError: d.last_error,
        enabled: d.enabled,
        polling: d.polling_interval_sec,
        iface: i.interface_name,
        ifaceType: i.interface_type,
        tx: i.tx_bps || 0,
        rx: i.rx_bps || 0,
      });
    }
  }
  return rows;
}

export default function DashboardTableView({ overview, live }) {
  const tableRows = useMemo(() => {
    const base = flattenRows(overview);
    return base.map((r) => {
      if (!r.iface) return r;
      const l = live[`${r.deviceId}:${r.iface}`];
      if (!l) return r;
      return { ...r, tx: l.tx, rx: l.rx };
    });
  }, [overview, live]);

  const columns = [
    {
      title: '',
      width: 22,
      render: (_, r) => (
        <span
          className={`status-dot ${r.online ? 'status-online' : 'status-offline'}`}
          title={r.online ? 'Online' : r.lastError || 'Offline'}
        />
      ),
    },
    {
      title: 'Device',
      width: 140,
      render: (_, r) => (
        <Link to={`/devices/${r.deviceId}`}>{r.deviceName}</Link>
      ),
    },
    { title: 'Host', dataIndex: 'host', width: 110, ellipsis: true },
    {
      title: 'Interface',
      dataIndex: 'iface',
      width: 160,
      ellipsis: true,
      render: (v, r) =>
        v ? (
          <Space size={4}>
            <Text style={{ fontSize: 11 }}>{v}</Text>
            {r.ifaceType && (
              <Tag style={{ margin: 0, fontSize: 9, lineHeight: '14px' }}>{r.ifaceType}</Tag>
            )}
          </Space>
        ) : (
          <Text type="secondary" style={{ fontSize: 11 }}>
            No interfaces selected
          </Text>
        ),
    },
    {
      title: 'TX',
      width: 72,
      align: 'right',
      render: (_, r) => (
        <Text style={{ fontSize: 11, color: '#69b1ff' }}>{formatBps(r.tx)}</Text>
      ),
    },
    {
      title: 'RX',
      width: 72,
      align: 'right',
      render: (_, r) => (
        <Text style={{ fontSize: 11, color: '#95de64' }}>{formatBps(r.rx)}</Text>
      ),
    },
    {
      title: 'Poll',
      width: 40,
      render: (_, r) => `${r.polling}s`,
    },
    {
      title: 'Status',
      width: 90,
      render: (_, r) => {
        if (!r.enabled) return <Tag style={{ margin: 0, fontSize: 10 }}>disabled</Tag>;
        if (r.online) return <Tag color="success" style={{ margin: 0, fontSize: 10 }}>online</Tag>;
        const err = r.lastError || 'offline';
        return (
          <Tooltip title={err}>
            <Tag color="error" style={{ margin: 0, fontSize: 10, maxWidth: 80 }} ellipsis>
              {err.length > 12 ? `${err.slice(0, 12)}…` : err}
            </Tag>
          </Tooltip>
        );
      },
    },
  ];

  return (
    <Table
      className="compact-table"
      size="small"
      rowKey="key"
      dataSource={tableRows}
      columns={columns}
      pagination={{ pageSize: 100, size: 'small', showSizeChanger: true }}
      scroll={{ y: 'calc(100vh - 220px)' }}
    />
  );
}
