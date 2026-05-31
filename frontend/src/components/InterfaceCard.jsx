import React from 'react';
import { Link } from 'react-router-dom';
import { Tooltip } from 'antd';
import {
  CheckCircleFilled,
  CloseCircleFilled,
  PauseCircleFilled,
} from '@ant-design/icons';
import { formatBps } from '../api';

function shortIface(name) {
  if (!name) return '';
  const parts = name.split('/');
  const last = parts[parts.length - 1];
  return last.length > 18 ? `${last.slice(0, 16)}…` : last;
}

export default function InterfaceCard({ device, iface, live }) {
  const online = device.enabled && device.online;
  const offline = device.enabled && !device.online;
  const disabled = !device.enabled;

  const key = `${device.id}:${iface.interface_name}`;
  const l = live[key];
  const tx = l?.tx ?? iface.tx_bps ?? 0;
  const rx = l?.rx ?? iface.rx_bps ?? 0;
  const active = online && !disabled && (tx || 0) + (rx || 0) > 0;

  let cardClass = 'device-card interface-card';
  if (disabled) cardClass += ' device-card--disabled';
  else if (offline) cardClass += ' device-card--offline';
  else if (active) cardClass += ' device-card--online';
  else cardClass += ' device-card--online device-card--idle';

  const statusIcon = disabled ? (
    <PauseCircleFilled className="device-card-status device-card-status--disabled" />
  ) : online ? (
    <CheckCircleFilled className="device-card-status device-card-status--online" />
  ) : (
    <CloseCircleFilled className="device-card-status device-card-status--offline" />
  );

  const ifaceLabel = shortIface(iface.interface_name);

  return (
    <Link to={`/devices/${device.id}`} className={cardClass}>
      <div className="device-card-header">
        {statusIcon}
        <span className="device-card-title" title={device.name}>
          {device.name}
        </span>
      </div>

      <div className="interface-card-body">
        <Tooltip title={iface.interface_name}>
          <span className="interface-card-name">{ifaceLabel}</span>
        </Tooltip>
        {disabled && <span className="device-card-alert">Disabled</span>}
        {offline && (
          <Tooltip title={device.last_error || 'Offline'}>
            <span className="device-card-alert">Offline</span>
          </Tooltip>
        )}
        <div className="interface-card-traffic">
          <span className="interface-card-traffic-row">
            <span className="interface-card-traffic-label">TX</span>
            <span className="interface-card-traffic-tx">{formatBps(tx)}</span>
          </span>
          <span className="interface-card-traffic-row">
            <span className="interface-card-traffic-label">RX</span>
            <span className="interface-card-traffic-rx">{formatBps(rx)}</span>
          </span>
        </div>
      </div>

      <div className="device-card-footer">{device.host}</div>
    </Link>
  );
}
