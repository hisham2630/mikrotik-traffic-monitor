import React, { useState } from 'react';
import { FolderOutlined, CaretRightOutlined, CaretDownOutlined } from '@ant-design/icons';
import DeviceCard from './DeviceCard';

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

export default function DeviceGroupSection({ groupName, devices, live, defaultOpen = true }) {
  const [open, setOpen] = useState(defaultOpen);
  const counts = countStatuses(devices);

  return (
    <section className="device-group">
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
      {open && (
        <div className="device-card-grid">
          {devices.map((d) => (
            <DeviceCard key={d.id} device={d} live={live} />
          ))}
        </div>
      )}
    </section>
  );
}
