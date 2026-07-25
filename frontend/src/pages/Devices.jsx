import React, { useEffect, useState, useCallback } from 'react';
import {
  Table, Button, Space, Modal, Form, Input, InputNumber, Switch, message, Popconfirm, Checkbox, Collapse,
} from 'antd';
import { PlusOutlined, CopyOutlined, EditOutlined, DeleteOutlined, ApiOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { devices as devicesApi, ensureArray, decodeRebootDays, REBOOT_DAY_OPTIONS } from '../api';
import { compareNatural, sortDiscoveredGrouped } from '../utils/sort';
import { useAuth } from '../context/AuthContext';

const REBOOT_TIME_RULE = {
  // Browsers may include :ss on <input type="time">; API normalizes to HH:MM.
  pattern: /^([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$/,
  message: 'Use HH:MM',
};

export default function Devices() {
  const { isAdmin } = useAuth();
  const [list, setList] = useState([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [copyModal, setCopyModal] = useState(false);
  const [editing, setEditing] = useState(null);
  const [copyFrom, setCopyFrom] = useState(null);
  const [discovered, setDiscovered] = useState({});
  const [selectedIfaces, setSelectedIfaces] = useState([]);
  const [form] = Form.useForm();
  const [copyForm] = Form.useForm();

  const load = useCallback(async () => {
    try {
      setList(ensureArray(await devicesApi.list()));
    } catch {
      setList([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const openCreate = () => {
    setEditing(null);
    setDiscovered({});
    setSelectedIfaces([]);
    form.resetFields();
    form.setFieldsValue({
      port: 8728,
      polling_interval_sec: 5,
      enabled: true,
      reboot_schedule_enabled: false,
      reboot_days: [],
      reboot_time: '03:00',
    });
    setModalOpen(true);
  };

  const openEdit = async (row) => {
    setEditing(row);
    form.setFieldsValue({
      name: row.name,
      host: row.host,
      port: row.port,
      username: row.username,
      polling_interval_sec: row.polling_interval_sec,
      enabled: row.enabled,
      reboot_schedule_enabled: !!row.reboot_schedule_enabled,
      reboot_days: decodeRebootDays(row.reboot_days),
      reboot_time: row.reboot_time || '03:00',
    });
    const ifs = ensureArray(await devicesApi.interfaces(row.id));
    setSelectedIfaces(ifs.map((i) => i.interface_name));
    setDiscovered({});
    setModalOpen(true);
  };

  const fetchInterfaces = async () => {
    const v = form.getFieldsValue();
    if (!v.host || !v.username) {
      message.warning('Enter host and username first');
      return;
    }
    try {
      let data;
      if (editing) {
        data = await devicesApi.discover(editing.id);
      } else {
        if (!v.password) {
          message.warning('Password required to discover interfaces');
          return;
        }
        data = await devicesApi.previewDiscover(v);
      }
      setDiscovered(sortDiscoveredGrouped({ ...(data.grouped || {}) }));
      message.success('Interfaces loaded');
    } catch (e) {
      message.error(e.message);
    }
  };

  const onDiscoverExisting = async (deviceId) => {
    try {
      const data = await devicesApi.discover(deviceId);
      setDiscovered(sortDiscoveredGrouped({ ...(data.grouped || {}) }));
      message.success('Interfaces loaded');
    } catch (e) {
      message.error(e.message);
    }
  };

  const save = async () => {
    const v = await form.validateFields();
    try {
      let dev;
      if (editing) {
        dev = await devicesApi.update(editing.id, v);
      } else {
        if (!v.password) {
          message.error('Password required for new device');
          return;
        }
        dev = await devicesApi.create(v);
      }
      if (selectedIfaces.length) {
        const interfaces = [];
        Object.entries(discovered).forEach(([type, items]) => {
          (items || []).forEach((i) => {
            if (selectedIfaces.includes(i.name)) {
              interfaces.push({ name: i.name, type });
            }
          });
        });
        if (!interfaces.length) {
          selectedIfaces.forEach((name) => interfaces.push({ name, type: 'other' }));
        }
        await devicesApi.setInterfaces(dev.id, interfaces);
      }
      message.success('Saved');
      setModalOpen(false);
      load();
    } catch (e) {
      message.error(e.message);
    }
  };

  const doCopy = async () => {
    const v = await copyForm.validateFields();
    try {
      await devicesApi.copy(copyFrom.id, v);
      message.success('Device copied');
      setCopyModal(false);
      load();
    } catch (e) {
      message.error(e.message);
    }
  };

  const columns = [
    {
      title: '',
      width: 20,
      render: (_, r) => (
        <span
          className={`status-dot ${r.online ? 'status-online' : 'status-offline'}`}
          title={r.last_error || (r.online ? 'Online' : 'Offline')}
        />
      ),
    },
    { title: 'Name', dataIndex: 'name', render: (n, r) => <Link to={`/devices/${r.id}`}>{n}</Link> },
    {
      title: 'Status',
      width: 120,
      ellipsis: true,
      render: (_, r) =>
        r.online ? (
          <span style={{ color: '#52c41a', fontSize: 11 }}>online</span>
        ) : (
          <span style={{ color: '#ff4d4f', fontSize: 11 }} title={r.last_error}>
            {r.last_error || 'offline'}
          </span>
        ),
    },
    { title: 'Host', dataIndex: 'host' },
    { title: 'Port', dataIndex: 'port', width: 55 },
    { title: 'Poll', dataIndex: 'polling_interval_sec', width: 45, render: (v) => `${v}s` },
    {
      title: 'En',
      dataIndex: 'enabled',
      width: 40,
      render: (v) => (v ? 'Y' : 'N'),
    },
    isAdmin && {
      title: '',
      width: 120,
      render: (_, r) => (
        <Space size={2}>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)} />
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => {
              setCopyFrom(r);
              copyForm.setFieldsValue({ host: '', name: `${r.name} (copy)` });
              setCopyModal(true);
            }}
          />
          <Popconfirm title="Delete?" onConfirm={async () => { await devicesApi.delete(r.id); load(); }}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ].filter(Boolean);

  const ifacePanels = Object.entries(discovered)
    .sort(([a], [b]) => compareNatural(a, b))
    .map(([type, items]) => ({
    key: type,
    label: `${type} (${(items || []).length})`,
    children: (
      <Checkbox.Group
        value={selectedIfaces}
        onChange={setSelectedIfaces}
        style={{ display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        {(items || []).map((i) => (
          <Checkbox key={i.name} value={i.name} style={{ fontSize: 11 }}>
            {i.name}
          </Checkbox>
        ))}
      </Checkbox.Group>
    ),
  }));

  return (
    <div>
      {isAdmin && (
        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openCreate} style={{ marginBottom: 8 }}>
          Add Device
        </Button>
      )}
      <Table className="compact-table" size="small" rowKey="id" dataSource={list} columns={columns} pagination={{ pageSize: 30 }} />

      <Modal title={editing ? 'Edit Device' : 'Add Device'} open={modalOpen} onOk={save} onCancel={() => setModalOpen(false)} maskClosable={false} width={520}>
        <Form form={form} layout="vertical" size="small">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="host" label="Host" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="port" label="Port">
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="username" label="Username" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label="Password">
            <Input.Password placeholder={editing ? 'Leave blank to keep' : ''} />
          </Form.Item>
          <Form.Item name="polling_interval_sec" label="Poll interval (sec)">
            <InputNumber min={1} max={300} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="reboot_schedule_enabled"
            label="Scheduled reboot"
            valuePropName="checked"
          >
            <Switch
              onChange={(checked) => {
                if (!checked) {
                  form.setFields([
                    { name: 'reboot_days', errors: [] },
                    { name: 'reboot_time', errors: [] },
                  ]);
                }
              }}
            />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.reboot_schedule_enabled !== cur.reboot_schedule_enabled}
          >
            {() => {
              const on = !!form.getFieldValue('reboot_schedule_enabled');
              return (
                <>
                  <Form.Item
                    name="reboot_days"
                    label="Days"
                    style={on ? undefined : { display: 'none' }}
                    rules={
                      on
                        ? [
                            {
                              validator: (_, value) =>
                                value?.length
                                  ? Promise.resolve()
                                  : Promise.reject(new Error('Select at least one day')),
                            },
                          ]
                        : []
                    }
                  >
                    <Checkbox.Group options={REBOOT_DAY_OPTIONS} />
                  </Form.Item>
                  <Form.Item
                    name="reboot_time"
                    label="Time"
                    style={on ? undefined : { display: 'none' }}
                    rules={on ? [{ required: true, message: 'Time required' }, REBOOT_TIME_RULE] : []}
                    extra={on ? 'Server local time. Missed slots are caught up within 15 minutes.' : undefined}
                  >
                    <Input type="time" step={60} style={{ width: 140 }} />
                  </Form.Item>
                </>
              );
            }}
          </Form.Item>
          {isAdmin && (
            <Space>
              <Button size="small" icon={<ApiOutlined />} onClick={fetchInterfaces}>
                Fetch Interfaces
              </Button>
              {editing && (
                <Button size="small" onClick={() => onDiscoverExisting(editing.id)}>
                  Refresh from device
                </Button>
              )}
            </Space>
          )}
          {ifacePanels.length > 0 && <Collapse size="small" items={ifacePanels} style={{ marginTop: 8 }} />}
        </Form>
      </Modal>

      <Modal title="Copy Device" open={copyModal} onOk={doCopy} onCancel={() => setCopyModal(false)} maskClosable={false}>
        <Form form={copyForm} layout="vertical" size="small">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="host" label="New Host" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
