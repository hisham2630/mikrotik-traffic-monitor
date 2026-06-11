import React, { useEffect, useState, useCallback } from 'react';
import { Table, Button, Space, Modal, Form, Select, Input, InputNumber, Switch, Checkbox, message, Popconfirm, Tabs } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, CopyOutlined } from '@ant-design/icons';
import { rules as rulesApi, devices as devicesApi, alerts, ensureArray, formatBps, parseBps } from '../api';
import { useAuth } from '../context/AuthContext';
import { sortByName } from '../utils/sort';

function channelLabel(wa, tg) {
  const parts = [];
  if (wa) parts.push('WA');
  if (tg) parts.push('TG');
  return parts.length ? parts.join('+') : '—';
}

function historyChannelLabel(v) {
  if (!v) return '—';
  return v
    .split(',')
    .map((ch) => {
      const c = ch.trim();
      if (c === 'telegram') return 'TG';
      if (c === 'whatsapp') return 'WA';
      return c;
    })
    .join('+');
}

export default function AlertRules() {
  const { isAdmin } = useAuth();
  const [ruleList, setRuleList] = useState([]);
  const [deviceList, setDeviceList] = useState([]);
  const [history, setHistory] = useState([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [copying, setCopying] = useState(false);
  const [ifaces, setIfaces] = useState([]);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    const [rulesRes, devicesRes, historyRes] = await Promise.allSettled([
      rulesApi.list(),
      devicesApi.list(),
      alerts.history(),
    ]);
    if (rulesRes.status === 'fulfilled') {
      setRuleList(ensureArray(rulesRes.value));
    } else {
      setRuleList([]);
      message.error(`Failed to load rules: ${rulesRes.reason?.message || 'unknown error'}`);
    }
    if (devicesRes.status === 'fulfilled') {
      setDeviceList(ensureArray(devicesRes.value));
    } else {
      setDeviceList([]);
      message.error(`Failed to load devices: ${devicesRes.reason?.message || 'unknown error'}`);
    }
    if (historyRes.status === 'fulfilled') {
      setHistory(ensureArray(historyRes.value));
    } else {
      setHistory([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const ruleChannels = (row) => {
    let wa = row.notify_whatsapp;
    let tg = row.notify_telegram;
    if (wa === undefined && row.notify_channel != null) {
      wa = row.notify_channel !== 'telegram';
      tg = row.notify_channel === 'telegram';
    }
    return { wa: wa !== false && wa !== 0, tg: !!tg };
  };

  const fillForm = (row) => {
    const { wa, tg } = ruleChannels(row);
    form.setFieldsValue({
      device_id: row.device_id,
      interface_id: row.interface_id,
      direction: row.direction,
      condition: row.condition,
      threshold_input: formatBps(row.threshold_bps),
      duration_sec: row.duration_sec,
      cooldown_sec: row.cooldown_sec,
      notify_whatsapp: wa,
      notify_telegram: tg,
      enabled: row.enabled,
    });
    devicesApi.interfaces(row.device_id).then((ifs) => setIfaces(sortByName(ensureArray(ifs))));
  };

  const ensureDevicesLoaded = async () => {
    if (deviceList.length > 0) return;
    try {
      const d = await devicesApi.list();
      setDeviceList(ensureArray(d));
    } catch (e) {
      message.error(e.message || 'Failed to load devices');
    }
  };

  const openModal = async (row) => {
    await ensureDevicesLoaded();
    setCopying(false);
    setEditing(row || null);
    form.resetFields();
    if (row) {
      fillForm(row);
    } else {
      form.setFieldsValue({
        direction: 'rx',
        condition: 'above',
        duration_sec: 30,
        cooldown_sec: 300,
        notify_whatsapp: true,
        notify_telegram: false,
        enabled: true,
      });
      setIfaces([]);
    }
    setOpen(true);
  };

  const openCopy = async (row) => {
    await ensureDevicesLoaded();
    setCopying(true);
    setEditing(null);
    form.resetFields();
    fillForm(row);
    setOpen(true);
  };

  const onDeviceChange = async (id) => {
    form.setFieldValue('interface_id', null);
    const ifs = sortByName(ensureArray(await devicesApi.interfaces(id)));
    setIfaces(ifs);
  };

  const save = async () => {
    const v = await form.validateFields();
    if (!v.notify_whatsapp && !v.notify_telegram) {
      message.error('Select at least one channel (WhatsApp and/or Telegram)');
      return;
    }
    const threshold_bps = parseBps(v.threshold_input);
    if (threshold_bps == null) {
      message.error('Invalid threshold. Use e.g. 10M, 1G, 23k');
      return;
    }
    const payload = { ...v, threshold_bps };
    delete payload.threshold_input;
    try {
      if (editing) await rulesApi.update(editing.id, payload);
      else await rulesApi.create(payload);
      message.success('Saved');
      setOpen(false);
      load();
    } catch (e) {
      message.error(e.message);
    }
  };

  const deviceName = (id) => deviceList.find((d) => d.id === id)?.name || id;
  const ifaceName = (rule) => {
    if (!rule.interface_id) return 'All';
    return `#${rule.interface_id}`;
  };

  const ruleColumns = [
    { title: 'Device', dataIndex: 'device_id', render: deviceName },
    { title: 'Iface', render: (_, r) => ifaceName(r) },
    { title: 'Dir', dataIndex: 'direction', width: 45 },
    { title: 'Cond', dataIndex: 'condition', width: 50 },
    {
      title: 'Threshold',
      dataIndex: 'threshold_bps',
      render: (v) => formatBps(v),
    },
    { title: 'Dur', dataIndex: 'duration_sec', width: 40, render: (v) => `${v}s` },
    { title: 'CD', dataIndex: 'cooldown_sec', width: 45, render: (v) => `${v}s` },
    {
      title: 'Ch',
      width: 50,
      render: (_, r) => {
        const { wa, tg } = ruleChannels(r);
        return channelLabel(wa, tg);
      },
    },
    { title: 'On', dataIndex: 'enabled', width: 35, render: (v) => (v ? 'Y' : 'N') },
    isAdmin && {
      title: '',
      width: 110,
      render: (_, r) => (
        <Space size={2}>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(r)} />
          <Button size="small" icon={<CopyOutlined />} onClick={() => openCopy(r)} />
          <Popconfirm title="Delete?" onConfirm={async () => { await rulesApi.delete(r.id); load(); }}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ].filter(Boolean);

  const histColumns = [
    { title: 'Time', dataIndex: 'fired_at', width: 140, render: (v) => new Date(v).toLocaleString() },
    { title: 'Device', dataIndex: 'device_id', render: deviceName },
    { title: 'Iface', dataIndex: 'interface_name' },
    { title: 'Msg', dataIndex: 'message', ellipsis: true },
    {
      title: 'Ch',
      dataIndex: 'notify_channel',
      width: 55,
      render: historyChannelLabel,
    },
    { title: 'OK', dataIndex: 'notified', width: 35, render: (v) => (v ? 'Y' : 'N') },
  ];

  return (
    <div>
      <Tabs
        size="small"
        items={[
          {
            key: 'rules',
            label: 'Rules',
            children: (
              <>
                {isAdmin && (
                  <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => openModal()} style={{ marginBottom: 8 }}>
                    Add Rule
                  </Button>
                )}
                <Table className="compact-table" size="small" rowKey="id" dataSource={ruleList} columns={ruleColumns} pagination={{ pageSize: 20 }} />
              </>
            ),
          },
          {
            key: 'history',
            label: 'History',
            children: <Table className="compact-table" size="small" rowKey="id" dataSource={history} columns={histColumns} pagination={{ pageSize: 30 }} />,
          },
        ]}
      />

      <Modal
        title={editing ? 'Edit Rule' : copying ? 'Copy Rule' : 'Add Rule'}
        open={open}
        onOk={save}
        onCancel={() => { setOpen(false); setCopying(false); }}
        maskClosable={false}
      >
        <Form form={form} layout="vertical" size="small">
          <Form.Item name="device_id" label="Device" rules={[{ required: true }]}>
            <Select options={deviceList.map((d) => ({ value: d.id, label: d.name }))} onChange={onDeviceChange} />
          </Form.Item>
          <Form.Item name="interface_id" label="Interface (empty = all)">
            <Select
              allowClear
              options={[{ value: null, label: 'All interfaces' }, ...ifaces.map((i) => ({ value: i.id, label: i.interface_name }))]}
            />
          </Form.Item>
          <Form.Item name="direction" label="Direction" rules={[{ required: true }]}>
            <Select options={['tx', 'rx', 'both'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="condition" label="Condition" rules={[{ required: true }]}>
            <Select options={['above', 'below'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item
            name="threshold_input"
            label="Threshold"
            rules={[
              { required: true, message: 'Required' },
              {
                validator: (_, value) =>
                  parseBps(value) != null
                    ? Promise.resolve()
                    : Promise.reject(new Error('Use e.g. 10M, 1G, 23k, or raw bps')),
              },
            ]}
          >
            <Input placeholder="e.g. 10M, 1G, 23k" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="duration_sec" label="Duration (sec)">
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cooldown_sec" label="Cooldown (sec)">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="Notify via" required>
            <Space>
              <Form.Item name="notify_whatsapp" valuePropName="checked" noStyle>
                <Checkbox>WhatsApp</Checkbox>
              </Form.Item>
              <Form.Item name="notify_telegram" valuePropName="checked" noStyle>
                <Checkbox>Telegram</Checkbox>
              </Form.Item>
            </Space>
          </Form.Item>
          <Form.Item name="enabled" valuePropName="checked">
            <Switch checkedChildren="Enabled" unCheckedChildren="Disabled" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
