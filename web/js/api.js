window.Threads = window.Threads || {};

Threads.api = async function (path, opts = {}) {
  let res;
  try {
    res = await fetch(path, {
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
      ...opts,
    });
  } catch (e) {
    const name = e?.name || '';
    const msg = e?.message || String(e);
    if (name === 'AbortError') {
      throw new Error('Request dibatalkan');
    }
    if (/Failed to fetch|NetworkError|network error|Load failed/i.test(msg) || name === 'TypeError') {
      throw new Error('Koneksi putus saat request (sering karena generate gambar >30–60 dtk). Coba lagi; pastikan server & gateway localhost:20128 hidup.');
    }
    throw e instanceof Error ? e : new Error(msg);
  }
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
  if (res.status === 401 && data?.code === 'unauthorized') {
    if (!location.pathname.endsWith('/login.html')) {
      const portal = location.pathname.startsWith('/core') ? '/core/login.html' : '/app/login';
      location.replace(portal + '?next=' + encodeURIComponent(location.pathname + location.search));
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
  // App Go sering pakai HTTP 502 untuk gagal upstream (AI/Meta) + JSON {"error":"..."}.
  // Ambil pesan itu dulu — jangan samakan dengan Nginx Bad Gateway.
  const appMsg =
    data?.error?.error_user_msg ||
    data?.error?.error_user_title ||
    data?.error?.message ||
    (typeof data?.error === 'string' ? data.error : null) ||
    data?.message ||
    null;
  if (appMsg && typeof appMsg === 'string' && appMsg.trim()) {
    return appMsg.trim();
  }
  const looksHtml = raw && /^\s*</.test(raw);
  if (status === 504 || (looksHtml && /504\s*Gateway|Gateway Time-out/i.test(raw))) {
    return 'Timeout (504): generate terlalu lama untuk Nginx. Naikkan proxy_read_timeout ke 300s+ di site config, lalu reload Nginx.';
  }
  if (status === 502 || (looksHtml && /502\s*Bad Gateway/i.test(raw))) {
    const host = (typeof location !== 'undefined' && location.hostname) || '';
    const local = host === 'localhost' || host === '127.0.0.1' || host === '[::1]';
    if (local) {
      return 'Bad Gateway (502): proses Go tidak merespons. Restart `go run .` / threads.exe, buka http://localhost:8080.';
    }
    return 'Bad Gateway (502): backend tidak merespons. Di VPS: systemctl status lazybussiness.';
  }
  if (looksHtml) {
    return `Server error HTTP ${status || '?'} (respons HTML). Cek Nginx/timeout atau log service.`;
  }
  const msg =
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

/** Tutup dialog terbuka + resolve promise lama (hindari leak keydown). */
Threads._dismissDialog = null;
Threads._closeDialog = function () {
  if (typeof Threads._dismissDialog === 'function') {
    const fn = Threads._dismissDialog;
    Threads._dismissDialog = null;
    fn();
  }
};

/** Dialog konfirmasi styled (ganti window.confirm). Returns Promise<boolean>. */
Threads.confirm = function (message, opts = {}) {
  const title = opts.title || 'Konfirmasi';
  const okLabel = opts.okLabel || 'Ya, lanjut';
  const cancelLabel = opts.cancelLabel || 'Batal';
  const danger = opts.danger !== false;
  return new Promise((resolve) => {
    Threads._closeDialog();

    const root = document.createElement('div');
    root.id = 'th-dialog-root';
    root.className = 'th-dialog-root';
    root.setAttribute('role', 'presentation');
    root.innerHTML = `
      <div class="th-dialog-backdrop" data-dialog-cancel></div>
      <div class="th-dialog" role="dialog" aria-modal="true" aria-labelledby="th-dialog-title">
        <div class="th-dialog-icon${danger ? ' th-dialog-icon-danger' : ''}" aria-hidden="true">
          <i class="bi ${danger ? 'bi-exclamation-triangle' : 'bi-question-circle'}"></i>
        </div>
        <h2 class="th-dialog-title" id="th-dialog-title">${Threads.escapeHtml(title)}</h2>
        <p class="th-dialog-body">${Threads.escapeHtml(message)}</p>
        <div class="th-dialog-actions">
          <button type="button" class="th-btn th-btn-ghost" data-dialog-cancel>${Threads.escapeHtml(cancelLabel)}</button>
          <button type="button" class="th-btn ${danger ? 'th-btn-danger' : 'th-btn-primary'}" data-dialog-ok>${Threads.escapeHtml(okLabel)}</button>
        </div>
      </div>
    `;
    document.body.appendChild(root);
    document.body.classList.add('th-dialog-open');

    let settled = false;
    const finish = (ok) => {
      if (settled) return;
      settled = true;
      Threads._dismissDialog = null;
      document.removeEventListener('keydown', onKey);
      document.body.classList.remove('th-dialog-open');
      root.remove();
      resolve(ok);
    };
    Threads._dismissDialog = () => finish(false);

    const onKey = (e) => {
      if (e.key === 'Escape') finish(false);
      if (e.key === 'Enter') {
        const t = e.target;
        if (t && t.matches && t.matches('[data-dialog-cancel]')) return;
        if (t && t.matches && t.matches('button') && !t.matches('[data-dialog-ok]')) return;
        if (t && (t.tagName === 'TEXTAREA' || t.tagName === 'SELECT')) return;
        e.preventDefault();
        finish(true);
      }
    };
    root.querySelectorAll('[data-dialog-cancel]').forEach((el) => {
      el.addEventListener('click', () => finish(false));
    });
    root.querySelector('[data-dialog-ok]')?.addEventListener('click', () => finish(true));
    document.addEventListener('keydown', onKey);
    root.querySelector('[data-dialog-ok]')?.focus();
  });
};

/**
 * Dialog dengan body HTML + collect(root) sebelum close.
 * Returns Promise<any|null> — null kalau batal.
 */
Threads.formDialog = function (opts = {}) {
  const title = opts.title || 'Form';
  const okLabel = opts.okLabel || 'Simpan';
  const cancelLabel = opts.cancelLabel || 'Batal';
  const bodyHtml = opts.bodyHtml || '';
  const danger = !!opts.danger;
  return new Promise((resolve) => {
    Threads._closeDialog();

    const root = document.createElement('div');
    root.id = 'th-dialog-root';
    root.className = 'th-dialog-root';
    root.setAttribute('role', 'presentation');
    root.innerHTML = `
      <div class="th-dialog-backdrop" data-dialog-cancel></div>
      <div class="th-dialog" role="dialog" aria-modal="true" aria-labelledby="th-dialog-title">
        <h2 class="th-dialog-title" id="th-dialog-title">${Threads.escapeHtml(title)}</h2>
        <div class="th-dialog-body">${bodyHtml}</div>
        <div class="th-dialog-actions">
          <button type="button" class="th-btn th-btn-ghost" data-dialog-cancel>${Threads.escapeHtml(cancelLabel)}</button>
          <button type="button" class="th-btn ${danger ? 'th-btn-danger' : 'th-btn-primary'}" data-dialog-ok>${Threads.escapeHtml(okLabel)}</button>
        </div>
      </div>
    `;
    document.body.appendChild(root);
    document.body.classList.add('th-dialog-open');

    let settled = false;
    const finish = (ok) => {
      if (settled) return;
      settled = true;
      let payload = null;
      if (ok) {
        try {
          payload = typeof opts.collect === 'function' ? opts.collect(root) : true;
        } catch (err) {
          settled = false;
          Threads.toast?.(err.message || 'Form tidak valid', false);
          return;
        }
      }
      Threads._dismissDialog = null;
      document.removeEventListener('keydown', onKey);
      document.body.classList.remove('th-dialog-open');
      root.remove();
      resolve(ok ? payload : null);
    };
    Threads._dismissDialog = () => finish(false);

    const onKey = (e) => {
      if (e.key === 'Escape') finish(false);
      if (e.key === 'Enter') {
        const t = e.target;
        if (t && (t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.tagName === 'BUTTON')) return;
        e.preventDefault();
        finish(true);
      }
    };
    root.querySelectorAll('[data-dialog-cancel]').forEach((el) => {
      el.addEventListener('click', () => finish(false));
    });
    root.querySelector('[data-dialog-ok]')?.addEventListener('click', () => finish(true));
    document.addEventListener('keydown', onKey);
    const first = root.querySelector('input, select, textarea');
    (first || root.querySelector('[data-dialog-ok]'))?.focus();
  });
};

/** Dialog input styled (ganti window.prompt). Returns Promise<string|null>. */
Threads.prompt = function (message, opts = {}) {
  const title = opts.title || 'Input';
  const okLabel = opts.okLabel || 'Simpan';
  const cancelLabel = opts.cancelLabel || 'Batal';
  const defaultValue = opts.defaultValue ?? opts.value ?? '';
  const placeholder = opts.placeholder || '';
  const inputType = opts.type || 'text';
  return new Promise((resolve) => {
    Threads._closeDialog();

    const root = document.createElement('div');
    root.id = 'th-dialog-root';
    root.className = 'th-dialog-root';
    root.setAttribute('role', 'presentation');
    root.innerHTML = `
      <div class="th-dialog-backdrop" data-dialog-cancel></div>
      <div class="th-dialog" role="dialog" aria-modal="true" aria-labelledby="th-dialog-title">
        <div class="th-dialog-icon" aria-hidden="true">
          <i class="bi bi-person-plus"></i>
        </div>
        <h2 class="th-dialog-title" id="th-dialog-title">${Threads.escapeHtml(title)}</h2>
        <p class="th-dialog-body">${Threads.escapeHtml(message)}</p>
        <label class="th-label sr-only" for="th-dialog-input">${Threads.escapeHtml(message)}</label>
        <input id="th-dialog-input" class="th-input th-dialog-input" type="${Threads.escapeHtml(inputType)}"
          value="${Threads.escapeHtml(defaultValue)}" placeholder="${Threads.escapeHtml(placeholder)}" autocomplete="off">
        <div class="th-dialog-actions">
          <button type="button" class="th-btn th-btn-ghost" data-dialog-cancel>${Threads.escapeHtml(cancelLabel)}</button>
          <button type="button" class="th-btn th-btn-primary" data-dialog-ok>${Threads.escapeHtml(okLabel)}</button>
        </div>
      </div>
    `;
    document.body.appendChild(root);
    document.body.classList.add('th-dialog-open');

    const input = root.querySelector('#th-dialog-input');
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      Threads._dismissDialog = null;
      document.removeEventListener('keydown', onKey);
      document.body.classList.remove('th-dialog-open');
      root.remove();
      resolve(value);
    };
    Threads._dismissDialog = () => finish(null);

    const onKey = (e) => {
      if (e.key === 'Escape') finish(null);
      if (e.key === 'Enter') {
        e.preventDefault();
        finish(input?.value ?? '');
      }
    };
    root.querySelectorAll('[data-dialog-cancel]').forEach((el) => {
      el.addEventListener('click', () => finish(null));
    });
    root.querySelector('[data-dialog-ok]')?.addEventListener('click', () => finish(input?.value ?? ''));
    document.addEventListener('keydown', onKey);
    input?.focus();
    input?.select();
  });
};

/** Dialog info styled (ganti window.alert). Returns Promise<void>. */
Threads.alert = function (message, opts = {}) {
  const title = opts.title || 'Info';
  return new Promise((resolve) => {
    Threads._closeDialog();

    const root = document.createElement('div');
    root.id = 'th-dialog-root';
    root.className = 'th-dialog-root';
    root.innerHTML = `
      <div class="th-dialog-backdrop" data-dialog-ok></div>
      <div class="th-dialog" role="dialog" aria-modal="true" aria-labelledby="th-dialog-title">
        <h2 class="th-dialog-title" id="th-dialog-title">${Threads.escapeHtml(title)}</h2>
        <p class="th-dialog-body th-dialog-body-pre">${Threads.escapeHtml(message)}</p>
        <div class="th-dialog-actions">
          <button type="button" class="th-btn th-btn-primary" data-dialog-ok autofocus>OK</button>
        </div>
      </div>
    `;
    document.body.appendChild(root);
    document.body.classList.add('th-dialog-open');

    let settled = false;
    const finish = () => {
      if (settled) return;
      settled = true;
      Threads._dismissDialog = null;
      document.removeEventListener('keydown', onKey);
      document.body.classList.remove('th-dialog-open');
      root.remove();
      resolve();
    };
    Threads._dismissDialog = finish;

    const onKey = (e) => {
      if (e.key === 'Escape' || e.key === 'Enter') finish();
    };
    root.querySelectorAll('[data-dialog-ok]').forEach((el) => {
      el.addEventListener('click', finish);
    });
    document.addEventListener('keydown', onKey);
    root.querySelector('[data-dialog-ok]')?.focus();
  });
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
  return Threads.requireRepliz();
};

Threads.requireRepliz = async function () {
  try {
    const st = await Threads.api('/api/status');
    if (!st.repliz) {
      Threads.toast('Hubungkan Repliz dulu di Akun & API', false);
      return false;
    }
    if (!st.connected && !st.ig_connected) {
      Threads.toast('Sambungkan akun Threads/Instagram lewat Repliz di Akun', false);
      return false;
    }
    return true;
  } catch {
    Threads.toast('Server Go belum jalan — jalankan: go run .', false);
    return false;
  }
};
