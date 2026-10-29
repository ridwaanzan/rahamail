import { initTheme, toggleTheme } from './theme.js';
import { renderSplineTrafficChart, renderMiniSparkline } from './charts.js';
import { initDialogs, showToast, showModalAlert, showModalConfirm, showDkimModal } from './dialog.js';

// Global state
let sseSource = null;
let trafficHistory = [40, 65, 80, 50, 95, 120, 160, 110, 145, 185, 210, 240];

document.addEventListener('DOMContentLoaded', () => {
  initTheme();
  initDialogs();

  // Initial Charts
  renderSplineTrafficChart('sseTrafficChart', trafficHistory);
  renderMiniSparkline('sparklineDomains', [12, 19, 25, 32, 40, 46], '#06b6d4');
  renderMiniSparkline('sparklineUsers', [45, 80, 120, 165, 200, 239], '#3b82f6');

  // Initialize SSE if on dashboard
  if (document.getElementById('sseConnectionStatus')) {
    initDashboardSSE();
  }
});

function initDashboardSSE() {
  if (sseSource) sseSource.close();

  sseSource = new EventSource('/api/v1/sse');

  sseSource.onopen = () => {
    updateSSEUI(true);
  };

  sseSource.onerror = () => {
    updateSSEUI(false);
  };

  sseSource.addEventListener('stats', (e) => {
    try {
      const stats = JSON.parse(e.data);
      if (stats.messages !== undefined) {
        const el = document.getElementById('statEmails');
        if (el) el.innerText = stats.messages;
      }
      if (stats.domains !== undefined) {
        const el = document.getElementById('statDomains');
        if (el) el.innerText = stats.domains;
      }
      if (stats.users !== undefined) {
        const el = document.getElementById('statUsers');
        if (el) el.innerText = stats.users;
      }

      // Append new traffic point
      const nextVal = Math.floor(Math.random() * 80) + 120;
      trafficHistory.shift();
      trafficHistory.push(nextVal);
      renderSplineTrafficChart('sseTrafficChart', trafficHistory);

    } catch (err) {
      console.error('Failed to parse SSE stats:', err);
    }
  });

  sseSource.addEventListener('new_email', (e) => {
    try {
      const email = JSON.parse(e.data);
      appendLiveLogRow(email);
    } catch (err) {
      console.error('Failed to parse new_email event:', err);
    }
  });
}

function updateSSEUI(isConnected) {
  const pulseBadge = document.getElementById('ssePulseIndicator');
  const statusText = document.getElementById('sseConnectionStatus');
  const statusPill = document.getElementById('sseStatusPill');

  if (pulseBadge && statusText) {
    if (isConnected) {
      pulseBadge.className = 'w-3 h-3 rounded-full bg-emerald-500 animate-ping';
      statusText.innerText = 'Connected';
      statusText.className = 'text-xs font-semibold text-emerald-500';
      if (statusPill) statusPill.className = 'inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-500 border border-emerald-500/20';
    } else {
      pulseBadge.className = 'w-3 h-3 rounded-full bg-rose-500';
      statusText.innerText = 'Disconnected';
      statusText.className = 'text-xs font-semibold text-rose-500';
      if (statusPill) statusPill.className = 'inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-rose-500/10 text-rose-500 border border-rose-500/20';
    }
  }
}

function appendLiveLogRow(email) {
  const tbody = document.getElementById('liveLogsTableBody');
  if (!tbody) return;

  const timeStr = new Date().toLocaleTimeString();
  const tr = document.createElement('tr');
  tr.className = 'border-b border-slate-200 dark:border-dark-border hover:bg-slate-50 dark:hover:bg-dark-cardHover transition-colors';
  tr.innerHTML = `
    <td class="py-3 px-4 text-xs text-slate-500 dark:text-dark-muted font-mono">${timeStr}</td>
    <td class="py-3 px-4 font-medium text-slate-800 dark:text-dark-text">${escapeHtml(email.from)}</td>
    <td class="py-3 px-4 text-xs text-slate-500 dark:text-dark-muted">Just now</td>
    <td class="py-3 px-4">
      <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">Received</span>
    </td>
    <td class="py-3 px-4 text-right">
      <button class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"><i class="bi bi-three-dots-vertical"></i></button>
    </td>
  `;
  tbody.insertBefore(tr, tbody.firstChild);
}

function escapeHtml(text) {
  if (!text) return '';
  return String(text)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

window.escapeHtml = escapeHtml;
