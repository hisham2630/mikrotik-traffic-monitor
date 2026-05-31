import React, { useEffect, useState, useCallback } from 'react';
import { Card, Form, Input, InputNumber, Switch, Button, Table, message, Popconfirm, Select, Tabs } from 'antd';
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
    notifForm.setFieldsValue(n);
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

  const testNotif = async () => {
    try {
      await settingsApi.testNotification();
      message.success('Test sent');
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
            <Card size="small" style={{ maxWidth: 600 }}>
              <Form form={notifForm} layout="vertical" size="small">
                <Form.Item name="api_url_template" label="API URL" extra="Use {phone} and {message}">
                  <Input placeholder="http://host:3000/api/sendText?phone={phone}&text={message}&session=casher" />
                </Form.Item>
                <Form.Item name="phone_numbers" label="Phone numbers (comma-separated)">
                  <Input />
                </Form.Item>
                <Form.Item name="message_template" label="Message template">
                  <Input />
                </Form.Item>
                <Form.Item name="enabled" valuePropName="checked">
                  <Switch checkedChildren="Enabled" />
                </Form.Item>
                <Button type="primary" size="small" onClick={saveNotif} style={{ marginRight: 8 }}>
                  Save
                </Button>
                <Button size="small" onClick={testNotif}>
                  Test
                </Button>
              </Form>
            </Card>
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
