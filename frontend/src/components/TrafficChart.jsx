import React, { useEffect, useRef } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';

export default function TrafficChart({ samples, width = 600, height = 120 }) {
  const ref = useRef(null);
  const plotRef = useRef(null);

  useEffect(() => {
    if (!ref.current) return;
    const ts = samples.map((s) => new Date(s.ts || s.sampled_at).getTime() / 1000);
    const tx = samples.map((s) => s.tx_bps || 0);
    const rx = samples.map((s) => s.rx_bps || 0);

    if (!ts.length) {
      if (plotRef.current) {
        plotRef.current.destroy();
        plotRef.current = null;
      }
      return;
    }

    const opts = {
      width,
      height,
      series: [{}, { label: 'TX', stroke: '#1890ff' }, { label: 'RX', stroke: '#52c41a' }],
      axes: [
        { stroke: '#888' },
        { stroke: '#888', values: (_, v) => formatBps(v) },
      ],
      scales: { x: { time: true } },
    };

    if (plotRef.current) {
      plotRef.current.setData([ts, tx, rx]);
      return;
    }

    plotRef.current = new uPlot(opts, [ts, tx, rx], ref.current);
    return () => {
      plotRef.current?.destroy();
      plotRef.current = null;
    };
  }, [samples, width, height]);

  return <div ref={ref} />;
}

function formatBps(v) {
  if (v >= 1e9) return (v / 1e9).toFixed(1) + 'G';
  if (v >= 1e6) return (v / 1e6).toFixed(1) + 'M';
  if (v >= 1e3) return (v / 1e3).toFixed(1) + 'K';
  return v;
}
