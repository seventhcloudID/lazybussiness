window.Threads = window.Threads || {};

Threads.api = async function (path, opts = {}) {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
  if (res.status === 401 && data?.code === 'unauthorized') {
    if (!location.pathname.endsWith('/login.html')) {
      location.replace('/login.html?next=' + encodeURIComponent(location.pathname + location.search));
    }
    throw new Error('login required');
  }
  if (!res.ok) {
    throw new Error(Threads.apiErrorMessage(res, data, text));
  }
  return data;
};

/** Pesan error bersih — hindari dump HTML 502/504 dari Nginx. */
Threads.apiErrorMessage = function (res, data, text) {
  const status = res?.status || 0;
  const raw = typeof text === 'string' ? text : '';
  if (status === 504 || /504\s*Gateway|Gateway Time-out/i.test(raw)) {
    return 'Timeout (504): generate terlalu lama untuk Nginx. Naikkan proxy_read_timeout ke 300s+ di site config, lalu reload Nginx.';
  }
  if (status === 502 || /502\s*Bad Gateway/i.test(raw)) {
    return 'Bad Gateway (502): backend tidak merespons. Cek systemctl status lazybussiness.';
  }
  if (raw && /^\s*</.test(raw)) {
    return `Server error HTTP ${status || '?'} (respons HTML). Cek Nginx/timeout atau log service.`;
  }
  const msg =
    data?.error?.error_user_msg ||
    data?.error?.error_user_title ||
    data?.error?.message ||
    data?.error ||
    data?.message ||
    (raw && raw.length < 280 ? raw : '') ||
    res?.statusText ||
    'request gagal';
  return typeof msg === 'string' ? msg : JSON.stringify(msg);
};

Threads.fmtNum = function (n) {
  if (n == null || n === '') return '—';
  const x = Number(n);
  if (Number.isNaN(x)) return String(n);
  if (x >= 1e6) return (x / 1e6).toFixed(1).replace(/\.0$/, '') + 'jt';
  if (x >= 1e3) return (x / 1e3).toFixed(1).replace(/\.0$/, '') + 'rb';
  return x.toLocaleString('id-ID');
};

Threads.fmtDate = function (iso) {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleString('id-ID', {
      day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
    });
  } catch { return iso; }
};

Threads.escapeHtml = function (s) {
  return String(s ?? '').replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
};

Threads.toast = function (msg, ok) {
  let el = document.getElementById('toast');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast';
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.style.background = ok === false ? '#dc2626' : '#111827';
  el.style.display = 'block';
  clearTimeout(el._t);
  el._t = setTimeout(() => { el.style.display = 'none'; }, 3500);
};

Threads.insightMap = function (data, opts = {}) {
  let map = {};
  if (data?.metrics && typeof data.metrics === 'object') {
    map = { ...data.metrics };
  } else {
    (data?.data || []).forEach(m => {
      let val = m.total_value?.value ?? m.value;
      if (val == null && Array.isArray(m.values)) {
        val = m.values.length === 1
          ? m.values[0]?.value
          : m.values.reduce((s, v) => s + (Number(v?.value) || 0), 0);
      }
      if (m?.name) map[m.name] = val;
    });
  }
  const totals = data?.totals;
  if (opts.preferPostTotals && totals && typeof totals === 'object') {
    ['views', 'likes', 'replies', 'reposts', 'quotes'].forEach(k => {
      if (totals[k] != null) map[k] = totals[k];
    });
  }
  return map;
};

Threads.applyInsights = function (data, root, opts = {}) {
  if (!root) return;
  const map = Threads.insightMap(data, opts);
  root.querySelectorAll('[data-metric]').forEach(el => {
    const key = el.dataset.metric;
    if (!(key in map)) {
      el.textContent = '—';
      return;
    }
    Threads.animateMetric(el, map[key]);
  });
};

Threads.animateMetric = function (el, raw) {
  if (raw == null || raw === '') {
    el.textContent = '—';
    return;
  }
  const n = Number(raw);
  if (Number.isNaN(n) || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    el.textContent = Threads.fmtNum(raw);
    return;
  }
  const start = performance.now();
  const dur = 520;
  const from = 0;
  const tick = (t) => {
    const p = Math.min(1, (t - start) / dur);
    const eased = 1 - Math.pow(1 - p, 3);
    el.textContent = Threads.fmtNum(Math.round(from + (n - from) * eased));
    if (p < 1) requestAnimationFrame(tick);
    else el.textContent = Threads.fmtNum(raw);
  };
  requestAnimationFrame(tick);
};

Threads.requireConnected = async function () {
  try {
    const st = await Threads.api('/api/status');
    if (!st.connected) {
      Threads.toast('Hubungkan token dulu di halaman Token & Izin', false);
      return false;
    }
    return true;
  } catch {
    Threads.toast('Server Go belum jalan — jalankan: go run .', false);
    return false;
  }
};
