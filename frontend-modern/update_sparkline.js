const fs = require('fs');
const file = './src/components/shared/InteractiveSparkline.tsx';
let code = fs.readFileSync(file, 'utf8');

// 1. Optimize findNearestMetricPoint
code = code.replace(
`const findNearestMetricPoint = (points: MetricPoint[], targetTimestamp: number): MetricPoint | null => {`,
`const findNearestMetricPoint = (points: MetricPoint[], targetTimestamp: number, hintIndex: number = -1): { point: MetricPoint, index: number } | null => {`
);

code = code.replace(
`  const candidate = points[low];
  const previous = low > 0 ? points[low - 1] : candidate;
  return Math.abs(previous.timestamp - targetTimestamp) <= Math.abs(candidate.timestamp - targetTimestamp)
    ? previous
    : candidate;
};`,
`  const candidate = points[low];
  const previous = low > 0 ? points[low - 1] : candidate;
  if (Math.abs(previous.timestamp - targetTimestamp) <= Math.abs(candidate.timestamp - targetTimestamp)) {
    return { point: previous, index: low > 0 ? low - 1 : low };
  }
  return { point: candidate, index: low };
};`
);

// Update calls to findNearestMetricPoint
code = code.replace(
`      const point = findNearestMetricPoint(series.hoverData, targetTimestamp);
      if (!point) continue;`,
`      const nearest = findNearestMetricPoint(series.hoverData, targetTimestamp);
      if (!nearest) continue;
      const point = nearest.point;`
);

// 2. Add Fill/Area Gradients & 3. Hover crosshair capping
code = code.replace(
`        const x = clamp(((point.timestamp - windowStart) / rangeMs) * vbW, 0, vbW);
                const normalized = yMode() === 'auto'
                  ? Math.max(0, point.value) / scaleMax
                  : Math.min(Math.max(point.value, 0), 100) / 100;
                const y = vbH - normalized * vbH;
                return \`\${x.toFixed(1)},\${y.toFixed(1)}\`;
              });
              return { path: \`M\${points.join('L')}\`, color: series.color, seriesIndex: index };`,
`        const x = clamp(((point.timestamp - windowStart) / rangeMs) * vbW, 0, vbW);
                const normalized = yMode() === 'auto'
                  ? Math.max(0, point.value) / scaleMax
                  : Math.min(Math.max(point.value, 0), 100) / 100;
                const y = vbH - normalized * vbH;
                return { x, y };
              });
              const pathStrings = points.map(p => \`\${p.x.toFixed(1)},\${p.y.toFixed(1)}\`);
              const path = \`M\${pathStrings.join('L')}\`;
              let areaPath = '';
              if (validSeries.length === 1 && points.length > 1) {
                areaPath = \`\${path} L\${points[points.length - 1].x.toFixed(1)},\${vbH} L\${points[0].x.toFixed(1)},\${vbH} Z\`;
              }
              return { path, areaPath, color: series.color, seriesIndex: index };`
);

// Track minY for hover line
code = code.replace(
`    let nearestSeriesIndex: number | null = null;
    let nearestDistance = Number.POSITIVE_INFINITY;
    const values: HoverSeriesValue[] = [];`,
`    let nearestSeriesIndex: number | null = null;
    let nearestDistance = Number.POSITIVE_INFINITY;
    const values: HoverSeriesValue[] = [];
    let minY = vbH;`
);

code = code.replace(
`      if (shouldTrackNearest) {
        const pointY = valueToChartY(point.value);`,
`      const pointY = valueToChartY(point.value);
      if (pointY < minY) minY = pointY;
      if (shouldTrackNearest) {`
);

code = code.replace(
`      timestamp: tooltipValues[0].timestamp,
      totalValues,`,
`      timestamp: tooltipValues[0].timestamp,
      totalValues,
      minY,`
);

// 3. De-duplicate Identical Tooltip Values
code = code.replace(
`    const tooltipValues: HoverSeriesValue[] = focusedTooltip && effectiveSeriesIndex !== null
      ? values.filter((value) => value.seriesIndex === effectiveSeriesIndex)
      : (
        props.sortTooltipByValue
          ? selectTopValuesByValue(values, maxRows())
          : values.slice(0, maxRows())
      );`,
`    let groupedValues = values;
    if (!focusedTooltip) {
      const byValue = new Map<number, HoverSeriesValue[]>();
      for (const v of values) {
        let key = Math.round(v.value * 1000) / 1000;
        if (!byValue.has(key)) byValue.set(key, []);
        byValue.get(key)!.push(v);
      }
      groupedValues = [];
      for (const arr of byValue.values()) {
        if (arr.length > 1) {
          groupedValues.push({
            name: \`\${arr.length} Series\`,
            color: 'currentColor',
            value: arr[0].value,
            timestamp: arr[0].timestamp,
            seriesIndex: -1,
          });
        } else {
          groupedValues.push(arr[0]);
        }
      }
    }

    const tooltipValues: HoverSeriesValue[] = focusedTooltip && effectiveSeriesIndex !== null
      ? values.filter((value) => value.seriesIndex === effectiveSeriesIndex)
      : (
        props.sortTooltipByValue
          ? selectTopValuesByValue(groupedValues, maxRows())
          : groupedValues.slice(0, maxRows())
      );`
);

// 4. Update SVG & Canvas to use area and minY
code = code.replace(
`                      x1={hover().x}
                      y1="0"
                      x2={hover().x}
                      y2={vbH}`,
`                      x1={hover().x}
                      y1={Math.max(0, hover().minY - 4)}
                      x2={hover().x}
                      y2={vbH}`
);

// Add Defs for single series area gradient and area path logic
code = code.replace(
`                <For each={gridLineY()}>`,
`                <Show when={chartData().validSeries.length === 1}>
                  <defs>
                    <linearGradient id="single-series-area" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color={chartData().validSeries[0].color} stop-opacity="0.25" />
                      <stop offset="100%" stop-color={chartData().validSeries[0].color} stop-opacity="0" />
                    </linearGradient>
                  </defs>
                </Show>
                <For each={gridLineY()}>`
);

code = code.replace(
`                <For each={chartData().paths}>
                  {(pathData) => (
                    <path
                      d={pathData.path}`,
`                <For each={chartData().paths}>
                  {(pathData) => (
                    <g>
                      <Show when={pathData.areaPath}>
                        <path
                          d={pathData.areaPath}
                          fill="url(#single-series-area)"
                          stroke="none"
                        />
                      </Show>
                      <path
                        d={pathData.path}`
);

code = code.replace(
`                    />
                  )}
                </For>`,
`                      />
                    </g>
                  )}
                </For>`
);

code = code.replace(
`      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, height);
      ctx.stroke();`,
`      const grad = ctx.createLinearGradient(0, Math.max(0, hover.minY - 4), 0, height);
      grad.addColorStop(0, 'rgba(255, 255, 255, 0)');
      grad.addColorStop(0.1, hoverLineColor);
      grad.addColorStop(1, hoverLineColor);
      ctx.strokeStyle = grad;
      ctx.beginPath();
      ctx.moveTo(x, Math.max(0, hover.minY - 4));
      ctx.lineTo(x, height);
      ctx.stroke();`
);

// Update canvas for Area Fill
code = code.replace(
`        if (segment.length === 0) continue;
        ctx.beginPath();
        for (let i = 0; i < segment.length; i++) {
          const point = segment[i];
          const x = clamp(((point.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
          const y = valueToY(point.value);
          if (i === 0) {
            ctx.moveTo(x, y);
          } else {
            ctx.lineTo(x, y);
          }
        }
        ctx.stroke();`,
`        if (segment.length === 0) continue;
        
        // Single series area fill
        if (computed.validSeries.length === 1) {
          ctx.beginPath();
          for (let i = 0; i < segment.length; i++) {
            const point = segment[i];
            const x = clamp(((point.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
            const y = valueToY(point.value);
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
          }
          const lastPoint = segment[segment.length - 1];
          const firstPoint = segment[0];
          const lastX = clamp(((lastPoint.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
          const firstX = clamp(((firstPoint.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
          
          ctx.lineTo(lastX, height);
          ctx.lineTo(firstX, height);
          ctx.closePath();
          
          const areaGrad = ctx.createLinearGradient(0, 0, 0, height);
          areaGrad.addColorStop(0, series.color.replace('rgb', 'rgba').replace(')', ', 0.25)'));
          areaGrad.addColorStop(1, series.color.replace('rgb', 'rgba').replace(')', ', 0)'));
          
          ctx.fillStyle = areaGrad;
          ctx.fill();
        }

        ctx.beginPath();
        for (let i = 0; i < segment.length; i++) {
          const point = segment[i];
          const x = clamp(((point.timestamp - computed.windowStart) / computed.rangeMs) * width, 0, width);
          const y = valueToY(point.value);
          if (i === 0) {
            ctx.moveTo(x, y);
          } else {
            ctx.lineTo(x, y);
          }
        }
        ctx.stroke();`
);

// 5. Add Transitions to axis labels
code = code.replace(
`class="absolute left-0 text-[8px] leading-none text-slate-400 dark:text-slate-500"`,
`class="absolute left-0 text-[8px] leading-none text-slate-400 dark:text-slate-500 transition-all duration-300 ease-out"`
);

code = code.replace(
`class="absolute top-[2px] text-[9px] font-medium leading-none text-slate-500 dark:text-slate-400"`,
`class="absolute top-[2px] text-[9px] font-medium leading-none text-slate-500 dark:text-slate-400 transition-all duration-300 ease-out"`
);

// Add AreaPath to InteractiveSparklineProps
code = code.replace(
`paths: [] as { path: string; color: string; seriesIndex: number }[],`,
`paths: [] as { path: string; areaPath?: string; color: string; seriesIndex: number }[],`
);

// Type alias update where HoverSeriesValue exists inside tooltip values output

fs.writeFileSync(file, code);
