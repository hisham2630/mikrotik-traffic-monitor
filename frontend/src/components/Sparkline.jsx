import React, { useEffect, useRef } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';

export default function Sparkline({ data, width = 120, height = 28, color = '#1890ff' }) {
  const ref = useRef(null);
  const plotRef = useRef(null);

  useEffect(() => {
    if (!ref.current || !data?.length) return;

    const xs = data.map((_, i) => i);
    const ys = data.map((d) => d || 0);

    if (plotRef.current) {
      plotRef.current.setData([xs, ys]);
      return;
    }

    plotRef.current = new uPlot(
      {
        width,
        height,
        pxAlign: 0,
        cursor: { show: false },
        legend: { show: false },
        axes: [{ show: false }, { show: false }],
        series: [{}, { stroke: color, width: 1 }],
        padding: [2, 2, 0, 0],
      },
      [xs, ys],
      ref.current
    );

    return () => {
      plotRef.current?.destroy();
      plotRef.current = null;
    };
  }, [data, width, height, color]);

  return <div className="sparkline" ref={ref} style={{ width, height }} />;
}
