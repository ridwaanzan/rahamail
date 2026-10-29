// ==========================================
// PURE SVG CANVAS-FREE CHARTS (Fast & Crisp)
// ==========================================

// Renders a smooth bezier curve spline area chart for live traffic
export function renderSplineTrafficChart(elementId, dataPoints) {
  const container = document.getElementById(elementId);
  if (!container) return;

  const width = 600;
  const height = 180;
  const paddingX = 20;
  const paddingY = 20;

  const maxVal = Math.max(...dataPoints, 100);
  const minVal = 0;

  const stepX = (width - paddingX * 2) / (dataPoints.length - 1);
  const points = dataPoints.map((val, idx) => {
    const x = paddingX + idx * stepX;
    const y = height - paddingY - ((val - minVal) / (maxVal - minVal)) * (height - paddingY * 2);
    return { x, y };
  });

  // Calculate cubic bezier path
  let pathD = `M ${points[0].x} ${points[0].y}`;
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i === 0 ? 0 : i - 1];
    const p1 = points[i];
    const p2 = points[i + 1];
    const p3 = points[i + 2] || p2;

    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;

    pathD += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`;
  }

  const fillD = `${pathD} L ${points[points.length - 1].x} ${height - paddingY} L ${points[0].x} ${height - paddingY} Z`;

  container.innerHTML = `
    <svg viewBox="0 0 ${width} ${height}" class="w-full h-full overflow-visible">
      <defs>
        <linearGradient id="trafficGradient" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#06b6d4" stop-opacity="0.45" />
          <stop offset="100%" stop-color="#06b6d4" stop-opacity="0.0" />
        </linearGradient>
      </defs>
      <!-- Horizontal gridlines -->
      <line x1="${paddingX}" y1="${paddingY}" x2="${width - paddingX}" y2="${paddingY}" stroke="currentColor" class="text-slate-200 dark:text-slate-800" stroke-dasharray="4" />
      <line x1="${paddingX}" y1="${height / 2}" x2="${width - paddingX}" y2="${height / 2}" stroke="currentColor" class="text-slate-200 dark:text-slate-800" stroke-dasharray="4" />
      <line x1="${paddingX}" y1="${height - paddingY}" x2="${width - paddingX}" y2="${height - paddingY}" stroke="currentColor" class="text-slate-200 dark:text-slate-800" />
      
      <!-- Gradient Fill -->
      <path d="${fillD}" fill="url(#trafficGradient)" />

      <!-- Spline Line -->
      <path d="${pathD}" fill="none" stroke="#06b6d4" stroke-width="3" stroke-linecap="round" />

      <!-- Glowing Pulsing End Point -->
      <circle cx="${points[points.length - 1].x}" cy="${points[points.length - 1].y}" r="5" fill="#06b6d4" class="filter drop-shadow" />
      <circle cx="${points[points.length - 1].x}" cy="${points[points.length - 1].y}" r="9" fill="#06b6d4" opacity="0.3" class="animate-ping" />
    </svg>
  `;
}

// Renders mini SVG sparkline for KPI cards
export function renderMiniSparkline(elementId, dataPoints, color = '#06b6d4') {
  const container = document.getElementById(elementId);
  if (!container) return;

  const width = 120;
  const height = 40;
  const maxVal = Math.max(...dataPoints, 10);
  const minVal = Math.min(...dataPoints, 0);

  const stepX = width / (dataPoints.length - 1);
  const points = dataPoints.map((val, idx) => {
    const x = idx * stepX;
    const y = height - ((val - minVal) / (maxVal - minVal || 1)) * (height - 8) - 4;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(' ');

  container.innerHTML = `
    <svg viewBox="0 0 ${width} ${height}" class="w-full h-full overflow-visible">
      <polyline points="${points}" fill="none" stroke="${color}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  `;
}

window.renderSplineTrafficChart = renderSplineTrafficChart;
window.renderMiniSparkline = renderMiniSparkline;
