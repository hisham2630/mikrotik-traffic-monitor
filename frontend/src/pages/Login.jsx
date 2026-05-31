import React, { useState } from 'react';
import { Card, Form, Input, Button, message } from 'antd';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);

  const from = location.state?.from || '/';

  const onFinish = async (v) => {
    setLoading(true);
    try {
      const res = await login(v.username, v.password);
      if (res.must_change_password) {
        nav('/change-password');
      } else {
        nav(from, { replace: true });
      }
    } catch (e) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', height: '100vh', alignItems: 'center', justifyContent: 'center' }}>
      <Card title="MikroTik Traffic Monitor" style={{ width: 320 }}>
        <Form onFinish={onFinish} layout="vertical" size="small">
          <Form.Item name="username" label="Username" rules={[{ required: true }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="Password" rules={[{ required: true }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            Login
          </Button>
        </Form>
        <p style={{ fontSize: 11, color: '#888', marginTop: 8 }}>Default: admin / admin</p>
      </Card>
    </div>
  );
}
