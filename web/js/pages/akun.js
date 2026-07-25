Threads.pageShell('akun');

let expandedId = null;
let keysLoaded = false;

const TAB_COPY = {
  workspace: {
    title: 'Workspace',
    lead: 'Kelola akun Threads/IG dan ganti workspace aktif.',
  },
  keys: {
    title: 'API keys',
    lead: 'Gemini & OpenAI global. Buffer key per workspace (di Kelola akun).',
  },
};

function showAlert(msg, ok) {
  const el = document.getElementById('akun-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.className = 'th-alert mb-4 ' + (ok ? 'th-alert-ok' : '');
  el.classList.remove('hidden');
}

function pill(ok, label) {
  return `<span class="th-chip${ok ? ' th-chip-ok' : ''}">${Threads.escapeHtml(label)}</span>`;
}

function serviceIcon(svc) {
  const s = String(svc || '').toLowerCase();
  if (s === 'tiktok') return 'bi-tiktok';
  if (s === 'twitter' || s === 'x') return 'bi-twitter-x';
  if (s === 'instagram') return 'bi-instagram';
  if (s === 'facebook') return 'bi-facebook';
  if (s === 'linkedin') return 'bi-linkedin';
  if (s === 'youtube') return 'bi-youtube';
  if (s === 'threads') return 'bi-at';
  return 'bi-broadcast';
}

function setTab(tab) {
  const next = tab === 'keys' ? 'keys' : 'workspace';
  document.querySelectorAll('.akun-tab').forEach((btn) => {
    const on = btn.dataset.tab === next;
    btn.classList.toggle('is-active', on);
    btn.setAttribute('aria-selected', on ? 'true' : 'false');
  });
  const panelWs = document.getElementById('panel-workspace');
  const panelKeys = document.getElementById('panel-keys');
  const isKeys = next === 'keys';
  panelWs.classList.toggle('hidden', isKeys);
  panelWs.hidden = isKeys;
  panelKeys.classList.toggle('hidden', !isKeys);
  panelKeys.hidden = !isKeys;

  const copy = TAB_COPY[next];
  document.getElementById('akun-title').textContent = copy.title;
  document.getElementById('akun-lead').textContent = copy.lead;
  document.getElementById('btn-add')?.classList.toggle('hidden', isKeys);
  document.getElementById('btn-keys-refresh')?.classList.toggle('hidden', !isKeys);

  const url = new URL(location.href);
  if (next === 'keys') url.searchParams.set('tab', 'keys');
  else url.searchParams.delete('tab');
  history.replaceState(null, '', url.pathname + url.search + url.hash);

  if (isKeys && !keysLoaded) {
    loadAllKeys().then((ok) => { keysLoaded = !!ok; });
  }
}

function renderBuffer(root, accountId, st) {
  if (!root) return;
  const enabled = !!st?.enabled;
  const tokenOk = enabled && st.token_ok === true && !st.channels_error && !st.org_error;
  const rows = (st?.channels || []).map((ch) => {
    const name = ch.display_name || ch.name || ch.id;
    const used =
      (st.tiktok_channel_id && ch.id === st.tiktok_channel_id) ||
      (st.twitter_channel_id && ch.id === st.twitter_channel_id);
    return `<div class="buf-ch${ch.locked ? ' is-locked' : ''}${used ? ' is-used' : ''}">
      <span class="buf-ch-ico"><i class="bi ${serviceIcon(ch.service)}"></i></span>
      <div class="min-w-0">
        <div class="buf-ch-name">${Threads.escapeHtml(name)}</div>
        <div class="buf-ch-meta mono">${Threads.escapeHtml(ch.service || '—')}${used ? ' · dipakai' : ''}</div>
      </div>
    </div>`;
  }).join('');

  const statusBlock = enabled ? `
    <div class="flex flex-wrap gap-2 items-center mb-3">
      ${pill(tokenOk, tokenOk ? 'Key OK' : 'Key bermasalah')}
      ${pill(!!st.tiktok_ok, st.tiktok_ok ? 'TikTok' : 'TikTok —')}
      ${pill(!!st.twitter_ok, st.twitter_ok ? 'X' : 'X —')}
      <span class="text-xs text-muted mono">${Threads.escapeHtml(st.key_hint || '')}</span>
    </div>
    ${st.tiktok_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.tiktok_error)}</p>` : ''}
    ${st.twitter_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.twitter_error)}</p>` : ''}
    ${st.channels_error || st.org_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.channels_error || st.org_error)}</p>` : ''}
    <div class="buf-ch-grid mb-3">${rows || '<p class="text-sm text-muted m-0">Tidak ada channel.</p>'}</div>
  ` : `<p class="text-sm text-muted mb-3 m-0">${Threads.escapeHtml(st?.note || 'Belum ada Buffer API key untuk akun ini.')}</p>`;

  root.innerHTML = `
    <p class="text-xs text-muted mb-2 m-0">Hanya untuk workspace ini — tidak dipakai akun lain.</p>
    ${statusBlock}
    <label class="th-label">Buffer API key</label>
    <input class="th-input" type="password" data-buffer-key placeholder="Dari publish.buffer.com/settings/api" autocomplete="off">
    <div class="flex gap-2 flex-wrap mt-2">
      <button type="button" class="th-btn th-btn-primary text-xs" data-buffer-save>Simpan Buffer</button>
      <button type="button" class="th-btn th-btn-ghost text-xs" data-buffer-clear ${enabled ? '' : 'disabled'}>Hapus</button>
    </div>
  `;

  root.querySelector('[data-buffer-save]')?.addEventListener('click', () => saveBufferKey(accountId, root));
  root.querySelector('[data-buffer-clear]')?.addEventListener('click', () => clearBufferKey(accountId));
}

async function saveBufferKey(accountId, root) {
  const key = root?.querySelector('[data-buffer-key]')?.value?.trim();
  if (!key) return Threads.toast('Isi Buffer API key', false);
  try {
    await Threads.api('/api/accounts/' + encodeURIComponent(accountId) + '/buffer', {
      method: 'PUT',
      body: JSON.stringify({ api_key: key }),
    });
    Threads.toast('Buffer key tersimpan untuk akun ini', true);
    expandedId = accountId;
    await load();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function clearBufferKey(accountId) {
  if (!(await Threads.confirm('Hapus Buffer API key akun ini?', {
    title: 'Hapus Buffer key',
    okLabel: 'Hapus key',
  }))) return;
  try {
    await Threads.api('/api/accounts/' + encodeURIComponent(accountId) + '/buffer', { method: 'DELETE' });
    Threads.toast('Buffer key dihapus', true);
    expandedId = accountId;
    await load();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function loadBufferFor(accountId) {
  const root = document.querySelector(`[data-id="${CSS.escape(accountId)}"] [data-buffer-body]`);
  if (!root) return;
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuat Buffer…</p>`;
  try {
    const st = await Threads.api('/api/accounts/' + encodeURIComponent(accountId) + '/buffer');
    renderBuffer(root, accountId, st);
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
  }
}

function renderOpenAI(st) {
  const root = document.getElementById('openai-body');
  const enabled = !!st?.enabled;
  root.innerHTML = `
    <div class="flex flex-wrap gap-2 items-center mb-3">
      ${pill(enabled, enabled ? 'Siap' : 'Belum ada key')}
      ${st?.model ? `<span class="text-xs text-muted mono">${Threads.escapeHtml(st.model)}</span>` : ''}
      ${st?.key_hint ? `<span class="text-xs text-muted mono">${Threads.escapeHtml(st.key_hint)}</span>` : ''}
    </div>
    <p class="text-xs text-muted mb-2 m-0">Thumbnail utas (Generate / Lazy).</p>
    <label class="th-label">API key</label>
    <input id="openai-key-input" class="th-input" type="password" placeholder="sk-…" autocomplete="off">
    <div class="flex gap-2 flex-wrap mt-2">
      <button type="button" class="th-btn th-btn-primary text-xs" id="btn-openai-save">Simpan</button>
      <button type="button" class="th-btn th-btn-ghost text-xs" id="btn-openai-clear" ${(st?.store_count || 0) > 0 ? '' : 'disabled'}>Hapus</button>
    </div>
  `;
  document.getElementById('btn-openai-save')?.addEventListener('click', saveOpenAIKey);
  document.getElementById('btn-openai-clear')?.addEventListener('click', clearOpenAIKey);
}

async function saveOpenAIKey() {
  const key = document.getElementById('openai-key-input')?.value?.trim();
  if (!key) return Threads.toast('Isi OpenAI API key', false);
  try {
    await Threads.api('/api/openai/keys', {
      method: 'PUT',
      body: JSON.stringify({ api_key: key }),
    });
    Threads.toast('OpenAI key tersimpan', true);
    await loadOpenAI();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function clearOpenAIKey() {
  if (!(await Threads.confirm('Hapus OpenAI key dari UI? Key di .env tidak terhapus.', {
    title: 'Hapus OpenAI key',
    okLabel: 'Hapus key',
  }))) return;
  try {
    await Threads.api('/api/openai/keys', { method: 'DELETE' });
    Threads.toast('Key UI dihapus', true);
    await loadOpenAI();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function loadOpenAI() {
  const root = document.getElementById('openai-body');
  if (!root) return false;
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuat…</p>`;
  try {
    const st = await Threads.api('/api/openai/keys');
    renderOpenAI(st);
    return true;
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
    return false;
  }
}

function renderGemini(st) {
  const root = document.getElementById('gemini-body');
  if (!root) return;
  const enabled = !!st?.enabled;
  const storeN = st?.from_store ?? 0;
  const masked = Array.isArray(st?.stored_masked) ? st.stored_masked : [];
  const maskedList = masked.length
    ? `<ul class="text-xs text-muted mono mt-2 mb-0 pl-4">${masked.map((m) => `<li>${Threads.escapeHtml(m)}</li>`).join('')}</ul>`
    : '';
  root.innerHTML = `
    <div class="flex flex-wrap gap-2 items-center mb-3">
      ${pill(enabled, enabled ? 'Siap' : 'Belum ada key')}
      ${st?.model ? `<span class="text-xs text-muted mono">${Threads.escapeHtml(st.model)}</span>` : ''}
      <span class="text-xs text-muted">UI ${storeN} · .env ${st?.from_env ?? 0}</span>
    </div>
    <p class="text-xs text-muted mb-2 m-0">Generate, Lazy, Chat, balasan — satu key per baris.</p>
    <label class="th-label">API keys</label>
    <textarea id="gemini-keys-input" class="th-textarea" rows="3" placeholder="AIza…&#10;AIza…" autocomplete="off"></textarea>
    ${maskedList}
    <div class="flex gap-2 flex-wrap mt-2">
      <button type="button" class="th-btn th-btn-primary text-xs" id="btn-gemini-save">Simpan</button>
      <button type="button" class="th-btn th-btn-ghost text-xs" id="btn-gemini-clear" ${storeN > 0 ? '' : 'disabled'}>Hapus</button>
    </div>
  `;
  document.getElementById('btn-gemini-save')?.addEventListener('click', saveGeminiKey);
  document.getElementById('btn-gemini-clear')?.addEventListener('click', clearGeminiKey);
}

async function saveGeminiKey() {
  const text = document.getElementById('gemini-keys-input')?.value?.trim();
  if (!text) return Threads.toast('Tempel minimal 1 Gemini API key', false);
  try {
    await Threads.api('/api/ai/keys', {
      method: 'PUT',
      body: JSON.stringify({ keys_text: text }),
    });
    Threads.toast('Gemini keys tersimpan', true);
    await loadGemini();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function clearGeminiKey() {
  if (!(await Threads.confirm('Hapus Gemini key dari UI? Key di .env tidak terhapus.', {
    title: 'Hapus Gemini keys',
    okLabel: 'Hapus keys',
  }))) return;
  try {
    await Threads.api('/api/ai/keys', { method: 'DELETE' });
    Threads.toast('Key UI dihapus', true);
    await loadGemini();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function loadGemini() {
  const root = document.getElementById('gemini-body');
  if (!root) return false;
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuat…</p>`;
  try {
    const st = await Threads.api('/api/ai/keys');
    renderGemini(st);
    return true;
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
    return false;
  }
}

async function loadAllKeys() {
  const results = await Promise.all([loadGemini(), loadOpenAI()]);
  return results.every(Boolean);
}

function card(a) {
  const active = a.active ? ' is-active-acct' : '';
  const open = expandedId === a.id;
  const handle = a.threads_username ? '@' + a.threads_username.replace(/^@/, '') : a.name || a.id;
  return `<section class="th-panel akun-card${active}${open ? ' is-open' : ''}" data-id="${Threads.escapeHtml(a.id)}">
    <div class="akun-card-summary">
      <div class="min-w-0">
        <div class="font-semibold truncate">${Threads.escapeHtml(handle)}</div>
        <div class="text-xs text-muted truncate">${Threads.escapeHtml(a.name || a.id)}${a.active ? ' · aktif' : ''}</div>
      </div>
      <div class="akun-card-meta">
        ${pill(a.threads_connected, a.threads_connected ? 'Threads' : 'Threads —')}
        ${pill(a.instagram_connected, a.instagram_connected ? 'IG' : 'IG —')}
        ${pill(!!a.buffer_enabled, a.buffer_enabled ? 'Buffer' : 'Buffer —')}
        ${pill(a.lazy_enabled, a.lazy_enabled ? 'Lazy' : 'Lazy off')}
        ${a.active
          ? `<span class="th-chip th-chip-ok">Aktif</span>`
          : `<button type="button" class="th-btn th-btn-soft text-xs" data-switch>Buka</button>`}
        <button type="button" class="th-btn th-btn-ghost text-xs" data-toggle-detail aria-expanded="${open ? 'true' : 'false'}">
          ${open ? 'Tutup' : 'Kelola'} <i class="bi bi-chevron-${open ? 'up' : 'down'}"></i>
        </button>
      </div>
    </div>
    ${open ? `
    <div class="th-panel-body akun-card-detail border-t border-line">
      <div class="grid gap-4 md:grid-cols-2">
        <div class="md:col-span-2">
          <label class="th-label">Nama tampilan</label>
          <div class="flex gap-2">
            <input class="th-input" data-name value="${Threads.escapeHtml(a.name || '')}" placeholder="bimosept">
            <button type="button" class="th-btn th-btn-ghost" data-rename>Simpan</button>
          </div>
        </div>
        <div>
          <label class="th-label">Threads access token</label>
          <input class="th-input" type="password" data-threads-token placeholder="Long-lived token Threads" autocomplete="off">
          <div class="flex gap-2 mt-2 flex-wrap">
            <button type="button" class="th-btn th-btn-primary text-xs" data-save-threads>Simpan Threads</button>
            <button type="button" class="th-btn th-btn-ghost text-xs" data-clear-threads>Hapus</button>
          </div>
        </div>
        <div>
          <label class="th-label">Instagram access token</label>
          <input class="th-input" type="password" data-ig-token placeholder="Long-lived token Instagram" autocomplete="off">
          <div class="flex gap-2 mt-2 flex-wrap">
            <button type="button" class="th-btn th-btn-primary text-xs" data-save-ig>Simpan IG</button>
            <button type="button" class="th-btn th-btn-ghost text-xs" data-clear-ig>Hapus</button>
          </div>
        </div>
        <div class="md:col-span-2 border-t border-line pt-4" data-buffer-body>
          <p class="text-sm text-muted m-0">Memuat Buffer…</p>
        </div>
        <div class="md:col-span-2 flex justify-between items-center gap-2 flex-wrap">
          <p class="text-xs text-muted m-0">Lazy, AI memory, dan Buffer tersimpan per akun.</p>
          ${a.active ? '' : `<button type="button" class="th-btn th-btn-ghost text-xs text-danger" data-delete>Hapus akun</button>`}
        </div>
      </div>
    </div>` : ''}
  </section>`;
}

async function load() {
  const root = document.getElementById('akun-list');
  root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Memuat…</p></div>`;
  try {
    const data = await Threads.api('/api/accounts');
    const list = data.accounts || [];
    if (!list.length) {
      root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Belum ada akun. Klik Tambah akun.</p></div>`;
      return;
    }
    if (expandedId && !list.some((a) => a.id === expandedId)) expandedId = null;
    root.innerHTML = list.map(card).join('');
    if (expandedId) await loadBufferFor(expandedId);
  } catch (e) {
    root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">${Threads.escapeHtml(e.message)}</p></div>`;
  }
}

document.querySelectorAll('.akun-tab').forEach((btn) => {
  btn.addEventListener('click', () => setTab(btn.dataset.tab));
});

document.getElementById('btn-add').onclick = async () => {
  const name = await Threads.prompt('Isi nama tampilan atau handle (tanpa @).', {
    title: 'Tambah akun',
    placeholder: 'contoh: bimosept',
    okLabel: 'Tambah akun',
  });
  if (name == null) return;
  const trimmed = name.trim();
  if (!trimmed) return Threads.toast('Isi nama akun', false);
  try {
    const created = await Threads.api('/api/accounts', { method: 'POST', body: JSON.stringify({ name: trimmed }) });
    Threads.toast('Akun ditambah', true);
    if (created?.account?.id) expandedId = created.account.id;
    await load();
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-keys-refresh')?.addEventListener('click', async () => {
  const ok = await loadAllKeys();
  Threads.toast(ok ? 'Status API diperbarui' : 'Gagal muat sebagian status API', ok);
});

document.getElementById('akun-list').addEventListener('click', async (e) => {
  const cardEl = e.target.closest('[data-id]');
  if (!cardEl) return;
  const id = cardEl.dataset.id;

  if (e.target.closest('[data-toggle-detail]')) {
    expandedId = expandedId === id ? null : id;
    await load();
    return;
  }

  if (e.target.closest('[data-switch]')) {
    try {
      await Threads.api('/api/accounts/switch', { method: 'POST', body: JSON.stringify({ id }) });
      Threads.toast('Workspace diganti', true);
      location.reload();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-rename]')) {
    const name = cardEl.querySelector('[data-name]')?.value?.trim();
    if (!name) return Threads.toast('Isi nama tampilan', false);
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id), {
        method: 'PATCH',
        body: JSON.stringify({ name }),
      });
      Threads.toast('Nama disimpan', true);
      expandedId = id;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-save-threads]')) {
    const token = cardEl.querySelector('[data-threads-token]')?.value?.trim();
    if (!token) return Threads.toast('Isi token Threads', false);
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id) + '/threads-token', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      Threads.toast('Token Threads tersimpan', true);
      showAlert('Threads terhubung untuk akun ini.', true);
      expandedId = id;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-clear-threads]')) {
    if (!(await Threads.confirm('Hapus token Threads akun ini?', {
      title: 'Hapus token Threads',
      okLabel: 'Hapus token',
    }))) return;
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id) + '/threads-token', { method: 'DELETE' });
      Threads.toast('Token Threads dihapus', true);
      expandedId = id;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-save-ig]')) {
    const token = cardEl.querySelector('[data-ig-token]')?.value?.trim();
    if (!token) return Threads.toast('Isi token Instagram', false);
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id) + '/ig-token', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      Threads.toast('Token IG tersimpan', true);
      expandedId = id;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-clear-ig]')) {
    if (!(await Threads.confirm('Hapus token Instagram akun ini?', {
      title: 'Hapus token Instagram',
      okLabel: 'Hapus token',
    }))) return;
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id) + '/ig-token', { method: 'DELETE' });
      Threads.toast('Token IG dihapus', true);
      expandedId = id;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-delete]')) {
    if (!(await Threads.confirm('Hapus akun dari daftar? File data di disk tetap ada.', {
      title: 'Hapus akun',
      okLabel: 'Hapus akun',
    }))) return;
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id), { method: 'DELETE' });
      Threads.toast('Akun dihapus dari daftar', true);
      if (expandedId === id) expandedId = null;
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
  }
});

const initialTab = new URLSearchParams(location.search).get('tab') === 'keys' ? 'keys' : 'workspace';
setTab(initialTab);
load();
