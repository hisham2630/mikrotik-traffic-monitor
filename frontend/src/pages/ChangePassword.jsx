import React, { useState } from 'react';
import { Card, Form, Input, Button, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { auth } from '../api';
import { useAuth } from '../context/AuthContext';

export default function ChangePassword() {
  const { refresh } = useAuth();
  const nav = useNavigate();
  const [loading, setLoading] = useState(false);

  const onFinish = async (v) => {
    if (v.password !== v.confirm) {
      message.error('Passwords do not match');
      return;
    }
    setLoading(true);
    try {
      await auth.changePassword(v.password);
      await refresh();
      message.success('Password updated');
      nav('/', { replace: true });
    } catch (e) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', height: '100vh', alignItems: 'center', justifyContent: 'center' }}>
      <Card title="Change Password" style={{ width: 320 }}>
        <Form onFinish={onFinish} layout="vertical" size="small">
          <Form.Item name="password" label="New password" rules={[{ required: true, min: 4 }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="confirm" label="Confirm" rules={[{ required: true }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            Save
          </Button>
        </Form>
      </Card>
    </div>
  );
}
