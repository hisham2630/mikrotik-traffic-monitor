import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider, theme } from 'antd';
import App from './App';
import './index.css';

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <ConfigProvider
      componentSize="small"
      theme={{
        algorithm: theme.darkAlgorithm,
        token: { fontSize: 12, sizeUnit: 3, sizeStep: 3 },
      }}
    >
      <App />
    </ConfigProvider>
  </React.StrictMode>
);
