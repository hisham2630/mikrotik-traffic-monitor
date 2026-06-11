import React, { useState } from 'react';
import { Modal, message, Button } from 'antd';
import { FolderOutlined, CaretRightOutlined, CaretDownOutlined, ReloadOutlined } from '@ant-design/icons';
import { devices as devicesApi } from '../api';
import DeviceCard from './DeviceCard';
import InterfaceCard from './InterfaceCard';
import { sortByName } from '../utils/sort';

function countStatuses(devices) {
  let online = 0;
  let offline = 0;
  let disabled = 0;
  for (const d of devices) {
    if (!d.enabled) disabled++;
    else if (d.online) online++;
    else offline++;
  }
  return { online, offline, disabled };
}

export default function DeviceGroupSection({
  groupName,
  devices,
  live,
  cardLayout = 'device',
  defaultOpen = true,
  deviceStatsById = {},
  isAdmin = false,
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [rebooting, setRebooting] = useState(false);
  const counts = countStatuses(devices);
  const device = devices.length === 1 ? devices[0] : null;
  const stats = device ? deviceStatsById[device.id] : null;

  const confirmReboot = () => {
    if (!device) return;
    Modal.confirm({
      title: 'Reboot device?',
      content: (
        <span>
          Reboot <strong>{device.name}</strong>? The device will be unavailable briefly.
        </span>
      ),
      okText: 'Reboot',
      okType: 'danger',
      cancelText: 'Cancel',
      onOk: async () => {
        setRebooting(true);
        try {
          await devicesApi.reboot(device.id);
          message.success(`Reboot sent to ${device.name}`);
        } catch (e) {
          message.error(e.message || 'Reboot failed');
          throw e;
        } finally {
          setRebooting(false);
        }
      },
    });
  };

  return (
    <section className="device-group">
      <div className="device-group-header-row">
        <button
          type="button"
          className="device-group-header"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
        >
          <span className="device-group-header-left">
            {open ? <CaretDownOutlined /> : <CaretRightOutlined />}
            <FolderOutlined className="device-group-folder" />
            <span className="device-group-name">
              {groupName} ({devices.length})
            </span>
            {device && stats && (
              <span className="device-group-stats">
                <span className="device-group-stat">CPU {stats.cpu_load}%</span>
                <span className="device-group-stat">Up {stats.uptime || '—'}</span>
              </span>
            )}
          </span>
          <span className="device-group-counts">
            {counts.online > 0 && (
              <span className="device-group-count device-group-count--online" title="Online">
                {counts.online}
              </span>
            )}
            {counts.offline > 0 && (
              <span className="device-group-count device-group-count--offline" title="Offline">
                {counts.offline}
              </span>
            )}
            {counts.disabled > 0 && (
              <span className="device-group-count device-group-count--disabled" title="Disabled">
                {counts.disabled}
              </span>
            )}
          </span>
        </button>
        {isAdmin && device && (
          <Button
            type="text"
            size="small"
            className="device-group-reboot"
            icon={<ReloadOutlined />}
            loading={rebooting}
            onClick={confirmReboot}
            title={`Reboot ${device.name}`}
          >
            Reboot
          </Button>
        )}
      </div>
      {open && (
        <div className="device-card-grid">
          {cardLayout === 'interface'
            ? devices.flatMap((d) =>
                (d.interfaces || []).length > 0
                  ? sortByName(d.interfaces || []).map((iface) => (
                      <InterfaceCard
                        key={`${d.id}:${iface.interface_name}`}
                        device={d}
                        iface={iface}
                        live={live}
                      />
                    ))
                  : [
                      <DeviceCard key={d.id} device={d} live={live} />,
                    ]
              )
            : devices.map((d) => (
                <DeviceCard key={d.id} device={d} live={live} />
              ))}
        </div>
      )}
    </section>
  );
}
