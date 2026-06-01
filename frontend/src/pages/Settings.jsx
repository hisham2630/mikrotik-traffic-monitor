import React, { useEffect, useState, useCallback } from 'react';
import { Card, Form, Input, InputNumber, Switch, Button, Table, message, Popconfirm, Select, Tabs, Space } from 'antd';
import { settings as settingsApi, users as usersApi, ensureArray } from '../api';

export default function Settings() {
  const [notifForm] = Form.useForm();
  const [appForm] = Form.useForm();
  const [userForm] = Form.useForm();
  const [userList, setUserList] = useState([]);

  const load = useCallback(async () => {
    const [n, a, u] = await Promise.all([
      settingsApi.notification(),
      settingsApi.app(),
      usersApi.list(),
    ]);
    notifForm.setFieldsValue({
      ...n,
      whatsapp_enabled: n.whatsapp_enabled ?? n.enabled ?? false,
    });
    appForm.setFieldsValue(a);
    setUserList(ensureArray(u));
  }, [notifForm, appForm]);

  useEffect(() => {
    load();
  }, [load]);

  const saveNotif = async () => {
    try {
      await settingsApi.updateNotification(await notifForm.validateFields());
      message.success('Notification settings saved');
    } catch (e) {
      message.error(e.message);
    }
  };

  const testWhatsApp = async () => {
    try {
      await settingsApi.testNotificationWhatsApp();
      message.success('WhatsApp test sent');
    } catch (e) {
      message.error(e.message);
    }
  };

  const testTelegram = async () => {
    try {
      await settingsApi.testNotificationTelegram();
      message.success('Telegram test sent');
    } catch (e) {
      message.error(e.message);
    }
  };

  const saveApp = async () => {
    try {
      await settingsApi.updateApp(await appForm.validateFields());
      message.success('App settings saved');
    } catch (e) {
      message.error(e.message);
    }
  };

  const addUser = async () => {
    try {
      const v = await userForm.validateFields();
      await usersApi.create(v);
      userForm.resetFields();
      message.success('User created');
      load();
    } catch (e) {
      message.error(e.message);
    }
  };

  const userColumns = [
    { title: 'User', dataIndex: 'username' },
    { title: 'Role', dataIndex: 'role' },
    {
      title: '',
      render: (_, r) => (
        <Popconfirm title="Delete user?" onConfirm={async () => { await usersApi.delete(r.id); load(); }}>
          <Button size="small" danger>
            Del
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Tabs
      size="small"
      items={[
        {
          key: 'notif',
          label: 'Notifications',
          children: (
            <Form form={notifForm} layout="vertical" size="small" style={{ maxWidth: 640 }}>
              <Form.Item
                name="message_template"
                label="Message template"
                extra="Use {message} for the alert text. Applies to WhatsApp and Telegram."
              >
                <Input placeholder="{message}" />
              </Form.Item>

              <Card size="small" title="WhatsApp" style={{ marginBottom: 12 }}>
                <Form.Item name="api_url_template" label="API URL" extra="Use {phone} and {message}">
                  <Input placeholder="http://host:3000/api/sendText?phone={phone}&text={message}&session=casher" />
                </Form.Item>
                <Form.Item name="phone_numbers" label="Phone numbers (comma-separated)">
                  <Input />
                </Form.Item>
                <Form.Item name="whatsapp_enabled" valuePropName="checked">
                  <Switch checkedChildren="Enabled" unCheckedChildren="Disabled" />
                </Form.Item>
                <Button size="small" onClick={testWhatsApp}>
                  Test WhatsApp
                </Button>
              </Card>

              <Card size="small" title="Telegram" style={{ marginBottom: 12 }}>
                <Form.Item
                  name="telegram_bot_token"
                  label="Bot token"
                  extra="Paste the full token from @BotFather (e.g. 123456789:AAH…). Must include the digits and colon."
                  rules={[
                    {
                      pattern: /^\d+:[A-Za-z0-9_-]+$/,
                      message: 'Enter the complete token (bot id, colon, secret)',
                    },
                  ]}
                >
                  <Input placeholder="123456789:AAHxxxxxxxxxxxxxxxx" autoComplete="off" spellCheck={false} />
                </Form.Item>
                <Form.Item
                  name="telegram_chat_ids"
                  label="Chat IDs (comma-separated)"
                  extra="Groups/supergroups: use the negative id (e.g. -1002405693501). Message @userinfobot in the group or use getUpdates after adding the bot."
                >
                  <Input placeholder="-1002405693501" />
                </Form.Item>
                <Form.Item name="telegram_enabled" valuePropName="checked">
                  <Switch checkedChildren="Enabled" unCheckedChildren="Disabled" />
                </Form.Item>
                <Button size="small" onClick={testTelegram}>
                  Test Telegram
                </Button>
              </Card>

              <Space>
                <Button type="primary" size="small" onClick={saveNotif}>
                  Save notifications
                </Button>
              </Space>
            </Form>
          ),
        },
        {
          key: 'app',
          label: 'App',
          children: (
            <Card size="small" style={{ maxWidth: 400 }}>
              <Form form={appForm} layout="vertical" size="small">
                <Form.Item name="retention_days" label="History retention (days)">
                  <InputNumber min={1} max={90} style={{ width: '100%' }} />
                </Form.Item>
                <Button type="primary" size="small" onClick={saveApp}>
                  Save
                </Button>
              </Form>
            </Card>
          ),
        },
        {
          key: 'users',
          label: 'Users',
          children: (
            <>
              <Card size="small" title="Add user" style={{ maxWidth: 400, marginBottom: 12 }}>
                <Form form={userForm} layout="inline" size="small">
                  <Form.Item name="username" rules={[{ required: true }]}>
                    <Input placeholder="Username" />
                  </Form.Item>
                  <Form.Item name="password" rules={[{ required: true }]}>
                    <Input.Password placeholder="Password" />
                  </Form.Item>
                  <Form.Item name="role" initialValue="viewer">
                    <Select style={{ width: 100 }} options={[{ value: 'admin', label: 'admin' }, { value: 'viewer', label: 'viewer' }]} />
                  </Form.Item>
                  <Button type="primary" size="small" onClick={addUser}>
                    Add
                  </Button>
                </Form>
              </Card>
              <Table className="compact-table" size="small" rowKey="id" dataSource={userList} columns={userColumns} pagination={false} />
            </>
          ),
        },
      ]}
    />
  );
}
