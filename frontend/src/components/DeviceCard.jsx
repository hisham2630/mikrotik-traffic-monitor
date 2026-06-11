import React from 'react';
import { Link } from 'react-router-dom';
import { Tooltip } from 'antd';
import {
  CheckCircleFilled,
  CloseCircleFilled,
  PauseCircleFilled,
} from '@ant-design/icons';
import { formatBps } from '../api';
import { sortByName } from '../utils/sort';

function shortIface(name) {
  if (!name) return '';
  const parts = name.split('/');
  const last = parts[parts.length - 1];
  return last.length > 14 ? `${last.slice(0, 12)}…` : last;
}

function ifaceBadgeClass(tx, rx, online) {
  if (!online) return 'device-card-badge device-card-badge--offline';
  if ((tx || 0) + (rx || 0) > 0) return 'device-card-badge device-card-badge--active';
  return 'device-card-badge device-card-badge--idle';
}

export default function DeviceCard({ device, live }) {
  const ifaces = sortByName(device.interfaces || []);
  const online = device.enabled && device.online;
  const offline = device.enabled && !device.online;
  const disabled = !device.enabled;

  let cardClass = 'device-card';
  if (disabled) cardClass += ' device-card--disabled';
  else if (offline) cardClass += ' device-card--offline';
  else cardClass += ' device-card--online';

  const statusIcon = disabled ? (
    <PauseCircleFilled className="device-card-status device-card-status--disabled" />
  ) : online ? (
    <CheckCircleFilled className="device-card-status device-card-status--online" />
  ) : (
    <CloseCircleFilled className="device-card-status device-card-status--offline" />
  );

  return (
    <Link to={`/devices/${device.id}`} className={cardClass}>
      <div className="device-card-header">
        {statusIcon}
        <span className="device-card-title" title={device.name}>
          {device.name}
        </span>
      </div>

      <div className="device-card-body">
        {disabled && (
          <span className="device-card-alert">Disabled</span>
        )}
        {offline && (
          <Tooltip title={device.last_error || 'Offline'}>
            <span className="device-card-alert">
              {device.last_error
                ? device.last_error.length > 22
                  ? `${device.last_error.slice(0, 20)}…`
                  : device.last_error
                : 'Offline'}
            </span>
          </Tooltip>
        )}
        {ifaces.length === 0 && !disabled && (
          <span className="device-card-badge device-card-badge--idle">No interfaces</span>
        )}
        {ifaces.map((iface) => {
          const key = `${device.id}:${iface.interface_name}`;
          const l = live[key];
          const tx = l?.tx ?? iface.tx_bps ?? 0;
          const rx = l?.rx ?? iface.rx_bps ?? 0;
          const label = shortIface(iface.interface_name);
          return (
            <Tooltip
              key={iface.interface_name}
              title={`${iface.interface_name} — TX ${formatBps(tx)} / RX ${formatBps(rx)}`}
            >
              <span className={ifaceBadgeClass(tx, rx, online && !disabled)}>
                <span className="device-card-badge-name">{label}</span>
                <span className="device-card-badge-tx">↑{formatBps(tx)}</span>
                <span className="device-card-badge-rx">↓{formatBps(rx)}</span>
              </span>
            </Tooltip>
          );
        })}
      </div>

      <div className="device-card-meta">
        <span className="device-card-poll">{device.polling_interval_sec}s</span>
      </div>

      <div className="device-card-footer">{device.host}</div>
    </Link>
  );
}
