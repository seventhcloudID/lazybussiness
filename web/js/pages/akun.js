Threads.pageShell('akun');

let expandedId = null;
let keysLoaded = false;
let replizAccounts = [];

const TAB_COPY = {
  workspace: {
    title: 'Workspace akun',
    lead: 'Satukan akun sosial satu brand, lalu pakai workspace itu di seluruh automasi.',
  },
  keys: {
    title: 'API keys workspace',
    lead: 'Status gateway AI dan REST API key untuk akses HTTP.',
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
  document.getElementById('repliz-connect')?.classList.toggle('hidden', isKeys);
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
        <div class="buf-ch-meta mono">${Threads.escapeHtml(ch.service || 'â€”')}${used ? ' Â· dipakai' : ''}</div>
      </div>
    </div>`;
  }).join('');

  const statusBlock = enabled ? `
    <div class="flex flex-wrap gap-2 items-center mb-3">
      ${pill(tokenOk, tokenOk ? 'Key OK' : 'Key bermasalah')}
      ${pill(!!st.tiktok_ok, st.tiktok_ok ? 'TikTok' : 'TikTok â€”')}
      ${pill(!!st.twitter_ok, st.twitter_ok ? 'X' : 'X â€”')}
      <span class="text-xs text-muted mono">${Threads.escapeHtml(st.key_hint || '')}</span>
    </div>
    ${st.tiktok_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.tiktok_error)}</p>` : ''}
    ${st.twitter_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.twitter_error)}</p>` : ''}
    ${st.channels_error || st.org_error ? `<p class="text-xs text-danger mb-2">${Threads.escapeHtml(st.channels_error || st.org_error)}</p>` : ''}
    <div class="buf-ch-grid mb-3">${rows || '<p class="text-sm text-muted m-0">Tidak ada channel.</p>'}</div>
  ` : `<p class="text-sm text-muted mb-3 m-0">${Threads.escapeHtml(st?.note || 'Belum ada Buffer API key untuk akun ini.')}</p>`;

  root.innerHTML = `
    <p class="text-xs text-muted mb-2 m-0">Hanya untuk akun brand ini â€” tidak dipakai akun lain.</p>
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
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuat Bufferâ€¦</p>`;
  try {
    const st = await Threads.api('/api/accounts/' + encodeURIComponent(accountId) + '/buffer');
    renderBuffer(root, accountId, st);
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
  }
}

function renderAI(st) {
  const root = document.getElementById('ai-body');
  if (!root) return;
  const enabled = !!st?.enabled;
  const thumb = st?.thumbnail || {};
  const model = [st?.provider, st?.model].filter(Boolean).join(' Â· ');
  const thumbLine = [thumb.model].filter(Boolean).join(' Â· ');
  root.innerHTML = `
    <div class="flex flex-wrap gap-2 items-center mb-3">
      ${pill(enabled, enabled ? 'Siap' : 'Belum dikonfigurasi')}
      ${pill(!!thumb.enabled, thumb.enabled ? 'Thumbnail siap' : 'Thumbnail off')}
    </div>
    <p class="text-sm m-0 mb-1"><span class="text-muted">Model</span> <code class="th-code">${Threads.escapeHtml(model || 'â€”')}</code></p>
    <p class="text-sm m-0 mb-3"><span class="text-muted">Thumbnail</span> <code class="th-code">${Threads.escapeHtml(thumbLine || 'â€”')}</code></p>
    <p class="text-xs text-muted m-0">Ubah di <code>.env</code>: <code>AI_BASE_URL</code>, <code>AI_MODEL</code>, <code>AI_API_KEY</code>. Generate, caption, Lazy, chat, dan thumbnail memakai gateway ini.</p>
  `;
}

async function loadAI() {
  const root = document.getElementById('ai-body');
  if (!root) return false;
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuatâ€¦</p>`;
  try {
    const st = await Threads.api('/api/ai/status');
    renderAI(st);
    return true;
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
    return false;
  }
}

function renderConnect(st) {
  const root = document.getElementById('connect-body');
  if (!root) return;
  const keys = Array.isArray(st?.keys) ? st.keys : [];
  const openapi = st?.openapi_url || (location.origin + '/openapi.yaml');
  const rows = keys.length
    ? `<ul class="text-sm m-0 pl-4 mb-3">${keys.map((k) => `
        <li class="mb-2 flex flex-wrap gap-2 items-center">
          <code class="th-code">${Threads.escapeHtml(k.prefix || k.id)}</code>
          <span class="text-xs text-muted">${Threads.escapeHtml(k.name || '')}</span>
          <button type="button" class="th-btn th-btn-ghost text-xs" data-connect-del="${Threads.escapeHtml(k.id)}">Hapus</button>
        </li>`).join('')}</ul>`
    : `<p class="text-xs text-muted mb-3">Belum ada API key. Buat satu untuk akses lewat curl / script / otomasi.</p>`;
  const docs = (st?.openapi_url || '').replace(/\/openapi\.yaml$/, '/docs') || (location.origin + '/docs');
  root.innerHTML = `
    <p class="text-sm text-muted mb-2 m-0">Akses dashboard lewat REST API: kirim header <code class="th-code">Authorization: Bearer mn_â€¦</code>.</p>
    <p class="text-xs mb-3 m-0">
      Docs: <a class="underline" href="${Threads.escapeHtml(docs)}" target="_blank" rel="noopener">${Threads.escapeHtml(docs)}</a>
      Â· Schema: <a class="underline" href="${Threads.escapeHtml(openapi)}" target="_blank" rel="noopener">openapi.yaml</a>
      ${st?.env_key_set ? ' Â· <span class="th-chip th-chip-ok">CONNECT_API_KEY di .env aktif</span>' : ''}
    </p>
    ${rows}
    <div class="flex gap-2 flex-wrap items-end">
      <div class="flex-1 min-w-[10rem]">
        <label class="th-label">Nama key</label>
        <input id="connect-key-name" class="th-input" placeholder="script / n8n / bot" autocomplete="off">
      </div>
      <button type="button" class="th-btn th-btn-primary text-xs" id="btn-connect-create">Buat API key</button>
      <a class="th-btn th-btn-ghost text-xs" href="${Threads.escapeHtml(docs)}" target="_blank" rel="noopener">Docs API</a>
    </div>
    <pre id="connect-key-once" class="th-code mt-3 text-xs whitespace-pre-wrap hidden"></pre>
  `;
  document.getElementById('btn-connect-create')?.addEventListener('click', createConnectKey);
  root.querySelectorAll('[data-connect-del]').forEach((btn) => {
    btn.addEventListener('click', () => deleteConnectKey(btn.getAttribute('data-connect-del')));
  });
}

async function createConnectKey() {
  const name = document.getElementById('connect-key-name')?.value?.trim() || 'default';
  try {
    const data = await Threads.api('/api/connect/keys', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
    Threads.toast('Connect key dibuat â€” salin sekarang', true);
    await loadConnect();
    const el = document.getElementById('connect-key-once');
    if (el && data?.key) {
      el.textContent = data.key + '\n\n' + (data.note || 'Simpan sekarang â€” tidak ditampilkan ulang.');
      el.classList.remove('hidden');
    }
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function deleteConnectKey(id) {
  if (!id) return;
  if (!(await Threads.confirm('Hapus Connect API key ini?', { title: 'Hapus key', okLabel: 'Hapus' }))) return;
  try {
    await Threads.api('/api/connect/keys', { method: 'DELETE', body: JSON.stringify({ id }) });
    Threads.toast('Key dihapus', true);
    await loadConnect();
  } catch (e) {
    Threads.toast(e.message, false);
  }
}

async function loadConnect() {
  const root = document.getElementById('connect-body');
  if (!root) return false;
  root.innerHTML = `<p class="text-sm text-muted m-0">Memuatâ€¦</p>`;
  try {
    const st = await Threads.api('/api/connect/keys');
    renderConnect(st);
    return true;
  } catch (e) {
    root.innerHTML = `<p class="text-sm text-muted m-0">${Threads.escapeHtml(e.message)}</p>`;
    return false;
  }
}

async function loadAllKeys() {
  const results = await Promise.all([loadAI(), loadConnect()]);
  return results.every(Boolean);
}

async function loadOrgBreadcrumb() {
  const el = document.getElementById('org-breadcrumb');
  if (!el) return;
  try {
    const org = await Threads.api('/api/org');
    const t = org.tenant?.name || org.tenant?.id || 'Tenant';
    const w = org.workspace?.name || org.workspace?.id || 'Workspace';
    el.textContent = `${t} Â· workspace ${w}`;
  } catch {
    el.textContent = 'Tenant Â· Workspace';
  }
}

function platformOptions(platform, selected, preferred) {
  const list = replizAccounts.filter((a) => String(a.type || '').toLowerCase() === platform);
  const wanted = String(preferred || '').replace(/^@/, '').toLowerCase();
  if (!selected && wanted) {
    const matches = list.filter((a) => String(a.username || '').replace(/^@/, '').toLowerCase() === wanted);
    if (matches.length === 1) selected = matches[0].id || matches[0]._id || matches[0].accountId || '';
  }
  const options = [`<option value="">Belum dipilih</option>`];
  list.forEach((a) => {
    const id = a.id || a._id || a.accountId || '';
    const handle = a.username ? '@' + String(a.username).replace(/^@/, '') : (a.name || id);
    options.push(`<option value="${Threads.escapeHtml(id)}"${id === selected ? ' selected' : ''}>${Threads.escapeHtml(handle)}</option>`);
  });
  return options.join('');
}

function card(a, activeId) {
  const id = a.id;
  const active = id === activeId;
  return `<section class="th-panel akun-card${active ? ' is-active-acct' : ''}" data-id="${Threads.escapeHtml(id)}">
    <div class="akun-card-summary">
      <div class="flex items-center gap-3 min-w-0">
        <div class="sb-avatar shrink-0">${Threads.escapeHtml(String(a.name || id).slice(0, 2).toUpperCase())}</div>
        <div class="min-w-0">
          <div class="font-semibold truncate">${Threads.escapeHtml(a.name || id)}</div>
          <div class="text-xs text-muted truncate">Workspace ${active ? '· aktif' : ''}</div>
        </div>
      </div>
      <div class="akun-card-meta">
        ${active
          ? `<span class="th-chip th-chip-ok">Dipakai sekarang</span>`
          : `<button type="button" class="th-btn th-btn-soft text-xs" data-switch>Pakai</button>`}
        ${active ? '' : `<button type="button" class="th-btn th-btn-ghost text-xs text-danger" data-delete>Hapus</button>`}
      </div>
    </div>
    <div class="th-panel-body grid gap-3">
      <div><label class="th-label">Nama workspace</label><input class="th-input" data-workspace-name value="${Threads.escapeHtml(a.name || '')}"></div>
      <div class="grid md:grid-cols-3 gap-3">
        <div><label class="th-label"><i class="bi bi-at"></i> Threads</label><select class="th-input" data-platform="threads">${platformOptions('threads', a.repliz_threads_id, a.threads_username || a.name)}</select></div>
        <div><label class="th-label"><i class="bi bi-instagram"></i> Instagram</label><select class="th-input" data-platform="instagram">${platformOptions('instagram', a.repliz_instagram_id, a.instagram_username || a.name)}</select></div>
        <div><label class="th-label"><i class="bi bi-tiktok"></i> TikTok</label><select class="th-input" data-platform="tiktok">${platformOptions('tiktok', a.repliz_tiktok_id, a.tiktok_username || a.name)}</select></div>
      </div>
      <div><button type="button" class="th-btn th-btn-primary text-xs" data-save-workspace><i class="bi bi-check2"></i> Simpan pasangan akun</button></div>
    </div>
  </section>`;
}

async function load() {
  const root = document.getElementById('akun-list');
  root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Memuat workspace…</p></div>`;
  loadOrgBreadcrumb();
  try {
    const data = await Threads.api('/api/accounts');
    const list = data.accounts || [];
    replizAccounts = data.repliz_accounts || [];
    const activeId = data.active_id || '';
    if (!list.length) {
      root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Belum ada workspace. Klik Workspace baru.</p></div>`;
      return;
    }
    root.innerHTML = list.map((a) => card(a, activeId)).join('');
  } catch (e) {
    root.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">${Threads.escapeHtml(e.message)}</p></div>`;
  }
}

function oauthRedirectURL(platform, accountId) {
  const u = new URL(location.href);
  const port = u.port || (u.protocol === 'https:' ? '443' : '80');
  const origin = (u.hostname === 'localhost' || u.hostname === '127.0.0.1')
    ? `${u.protocol}//127.0.0.1.sslip.io:${port}`
    : u.origin;
  let path = '/auth/repliz/' + encodeURIComponent(platform);
  if (accountId) path += '/' + encodeURIComponent(accountId);
  return origin + path;
}

async function startSocialConnect(platform, accountId) {
  platform = String(platform || '').toLowerCase();
  try {
    sessionStorage.setItem('mn_repliz_oauth', JSON.stringify({ platform, accountId: accountId || '', t: Date.now() }));
    const redirect = oauthRedirectURL(platform, accountId);
    const q = new URLSearchParams({ platform, redirect });
    const d = await Threads.api('/api/repliz/authorize?' + q.toString());
    if (!d?.url) throw new Error('Repliz tidak mengirim URL otorisasi');
    location.href = d.url;
  } catch (e) {
    sessionStorage.removeItem('mn_repliz_oauth');
    Threads.toast(e.message || 'Gagal memulai OAuth Repliz', false);
  }
}

function parseOAuthPath() {
  const m = String(location.pathname || '').match(/^\/auth\/repliz\/([a-z0-9]+)(?:\/([^/]+))?/i);
  if (!m) return null;
  return { platform: m[1].toLowerCase(), accountId: decodeURIComponent(m[2] || '') };
}

async function consumeOAuthReturn() {
  const q = new URLSearchParams(location.search);
  const h = new URLSearchParams(String(location.hash || '').replace(/^#/, ''));
  const code = q.get('code') || h.get('code') || '';
  const err = q.get('error_description') || q.get('error') || '';
  const fromPath = parseOAuthPath();
  let stored = null;
  try { stored = JSON.parse(sessionStorage.getItem('mn_repliz_oauth') || 'null'); } catch {}
  const pending = fromPath || (stored?.platform ? stored : null);
  if (!fromPath && !code && !err) return;
  sessionStorage.removeItem('mn_repliz_oauth');
  if (err) {
    showAlert(err, false);
    history.replaceState({}, '', Threads.appUrl('/akun'));
    return;
  }
  if (!pending?.platform || !code) {
    history.replaceState({}, '', Threads.appUrl('/akun'));
    return;
  }
  try {
    await Threads.api('/api/repliz/connect', {
      method: 'POST',
      body: JSON.stringify({
        platform: pending.platform,
        code,
        account_id: pending.accountId || '',
      }),
    });
    Threads.toast('Akun Repliz terhubung', true);
    showAlert('Akun terhubung lewat Repliz.', true);
  } catch (e) {
    showAlert(e.message, false);
    Threads.toast(e.message, false);
  }
  history.replaceState({}, '', Threads.appUrl('/akun'));
}

document.querySelectorAll('.akun-tab').forEach((btn) => {
  btn.addEventListener('click', () => setTab(btn.dataset.tab));
});

document.getElementById('btn-add').onclick = async () => {
  const name = prompt('Nama workspace / brand:', 'Workspace baru');
  if (!name?.trim()) return;
  try {
    const data = await Threads.api('/api/accounts', { method: 'POST', body: JSON.stringify({ name: name.trim() }) });
    if (data?.account?.id) await Threads.api('/api/accounts/switch', { method: 'POST', body: JSON.stringify({ id: data.account.id }) });
    Threads.toast('Workspace dibuat', true);
    await load();
  } catch (e) { Threads.toast(e.message, false); }
};

document.getElementById('repliz-connect')?.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-connect]');
  if (!btn) return;
  startSocialConnect(btn.getAttribute('data-connect'));
});

document.getElementById('btn-keys-refresh')?.addEventListener('click', async () => {
  const ok = await loadAllKeys();
  Threads.toast(ok ? 'Status API diperbarui' : 'Gagal muat sebagian status API', ok);
});

document.getElementById('akun-list').addEventListener('click', async (e) => {
  const cardEl = e.target.closest('[data-id]');
  if (!cardEl) return;
  const id = cardEl.dataset.id;

  if (e.target.closest('[data-switch]')) {
    try {
      await Threads.api('/api/accounts/switch', { method: 'POST', body: JSON.stringify({ id }) });
      Threads.toast('Workspace aktif diganti', true);
      location.reload();
    } catch (err) {
      Threads.toast(err.message, false);
    }
    return;
  }

  if (e.target.closest('[data-save-workspace]')) {
    const payload = {
      name: cardEl.querySelector('[data-workspace-name]')?.value?.trim() || id,
      repliz_threads_id: cardEl.querySelector('[data-platform="threads"]')?.value || '',
      repliz_instagram_id: cardEl.querySelector('[data-platform="instagram"]')?.value || '',
      repliz_tiktok_id: cardEl.querySelector('[data-platform="tiktok"]')?.value || '',
    };
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id), { method: 'PATCH', body: JSON.stringify(payload) });
      Threads.toast('Workspace tersimpan', true);
      await load();
    } catch (err) { Threads.toast(err.message, false); }
    return;
  }

  if (e.target.closest('[data-delete]')) {
    if (!(await Threads.confirm('Hapus workspace ini? Akun Repliz tidak ikut terhapus.', {
      title: 'Hapus workspace',
      okLabel: 'Hapus',
    }))) return;
    try {
      await Threads.api('/api/accounts/' + encodeURIComponent(id), { method: 'DELETE' });
      Threads.toast('Workspace dihapus', true);
      await load();
    } catch (err) {
      Threads.toast(err.message, false);
    }
  }
});

const initialTab = new URLSearchParams(location.search).get('tab') === 'keys' ? 'keys' : 'workspace';
setTab(initialTab);
consumeOAuthReturn().then(() => load());
