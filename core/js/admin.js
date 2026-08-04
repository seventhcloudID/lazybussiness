Core.mountShell('tenants');

const STATUS_OPTS = ['trial', 'active', 'past_due', 'suspended'];

function showAlert(msg, ok) {
  const el = document.getElementById('admin-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.className = 'th-alert mb-3 ' + (ok ? 'th-alert-ok' : '');
  el.classList.remove('hidden');
}

function statusChip(status) {
  const s = String(status || 'active');
  let cls = 'ok';
  if (s === 'past_due') cls = 'warn';
  if (s === 'suspended') cls = 'bad';
  return `<span class="core-chip ${cls}">${Threads.escapeHtml(s)}</span>`;
}

function statusOptions(selected) {
  return STATUS_OPTS.map((s) =>
    `<option value="${s}"${s === selected ? ' selected' : ''}>${s}</option>`
  ).join('');
}

async function ensureAdmin() {
  const me = await Threads.api('/api/auth/me');
  if (me.enabled && me.role && me.role !== 'admin') {
    showAlert('Core hanya untuk operator.', false);
    setTimeout(() => { location.href = '/app/ringkasan.html'; }, 800);
    return false;
  }
  return true;
}

function bindTenantActions(root, list) {
  root.querySelectorAll('[data-open]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      try {
        const res = await Threads.api(`/api/admin/tenants/${encodeURIComponent(btn.dataset.open)}/open`, {
          method: 'POST',
          body: '{}',
        });
        showAlert(`Context → ${res.tenant_name || res.tenant_id} (${res.accounts || 0} akun)`, true);
        await loadAll();
      } catch (err) {
        showAlert(err.message || 'Gagal buka tenant', false);
      }
    });
  });
  root.querySelectorAll('[data-edit-tenant]').forEach((btn) => {
    btn.addEventListener('click', () => editTenant(list.find((x) => x.id === btn.dataset.editTenant)));
  });
}

function svcIcon(on, icon, label) {
  return `<i class="bi ${icon} core-svc${on ? ' on' : ''}" title="${Threads.escapeHtml(label)}: ${on ? 'on' : 'off'}" aria-label="${Threads.escapeHtml(label)}"></i>`;
}

function keysCell(c) {
  const parts = [];
  if (c.gemini) {
    parts.push(`<span class="core-key" title="Gemini"><i class="bi bi-stars"></i>${c.gemini}</span>`);
  }
  if (c.openai) {
    parts.push(`<span class="core-key" title="OpenAI"><i class="bi bi-robot"></i>${c.openai}</span>`);
  }
  return parts.length ? parts.join('') : '<span class="mono">—</span>';
}

function accountsCell(t) {
  const rows = (t.workspaces || []).flatMap((w) => (w.accounts || []).map((a) => {
    const handle = a.threads_username || a.instagram_username || a.name || a.id;
    return `<div class="core-acct">
      <span class="core-acct-name">@${Threads.escapeHtml(String(handle).replace(/^@/, ''))}</span>
      <span class="core-svc-row">
        ${svcIcon(!!a.threads, 'bi-at', 'Threads')}
        ${svcIcon(!!a.instagram, 'bi-instagram', 'Instagram')}
        ${svcIcon(!!a.buffer, 'bi-broadcast', 'Buffer')}
      </span>
    </div>`;
  }));
  if (!rows.length) return '<span class="mono">—</span>';
  return `<div class="core-acct-stack">${rows.join('')}</div>`;
}

function renderTenants(data) {
  const root = document.getElementById('tenant-list');
  const list = data.tenants || [];
  const active = list.find((t) => t.active) || list[0];
  Core.setContext(data.active_tenant_id, data.active_workspace_id, active?.name);

  if (!list.length) {
    root.innerHTML = `<p class="core-empty">Belum ada tenant.</p>`;
    return;
  }

  root.innerHTML = `<table class="core-table core-table-tenants">
    <thead>
      <tr>
        <th>Tenant</th>
        <th>Status</th>
        <th>Plan</th>
        <th>Akun</th>
        <th>Keys</th>
        <th>Due</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      ${list.map((t) => {
        const b = t.billing || {};
        const c = t.connect || {};
        const wsName = (t.workspaces || []).map((w) => w.name || w.id).join(', ') || '—';
        return `<tr class="${t.active ? 'is-active' : ''}">
          <td>
            <div class="name">${Threads.escapeHtml(t.name || t.id)}</div>
            <div class="mono">${Threads.escapeHtml(t.id)}${t.active ? ' · context' : ''}</div>
            <div class="mono core-ws-meta"><i class="bi bi-folder2"></i> ${Threads.escapeHtml(wsName)}</div>
          </td>
          <td>${statusChip(b.status)}</td>
          <td class="mono">${Threads.escapeHtml(b.plan || '—')}</td>
          <td>${accountsCell(t)}</td>
          <td><div class="core-keys">${keysCell(c)}</div></td>
          <td class="mono">${Threads.escapeHtml(b.due_at || '—')}</td>
          <td>
            <div class="actions">
              <button type="button" class="th-btn th-btn-ghost" data-open="${Threads.escapeHtml(t.id)}">Buka</button>
              <button type="button" class="th-btn th-btn-ghost" data-edit-tenant="${Threads.escapeHtml(t.id)}">Edit</button>
            </div>
          </td>
        </tr>`;
      }).join('')}
    </tbody>
  </table>`;

  bindTenantActions(root, list);
}

function bindUserActions(root) {
  root.querySelectorAll('[data-toggle-user]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const active = btn.dataset.active !== '1';
      try {
        await Threads.api(`/api/admin/users/${encodeURIComponent(btn.dataset.toggleUser)}`, {
          method: 'PATCH',
          body: JSON.stringify({ active }),
        });
        await loadAll();
      } catch (err) {
        showAlert(err.message || 'Gagal update user', false);
      }
    });
  });
  root.querySelectorAll('[data-pass-user]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const pass = await Threads.prompt('Password baru untuk user ini.', {
        title: 'Ganti password',
        okLabel: 'Simpan',
        type: 'password',
      });
      if (pass == null || !String(pass).trim()) return;
      try {
        await Threads.api(`/api/admin/users/${encodeURIComponent(btn.dataset.passUser)}/password`, {
          method: 'POST',
          body: JSON.stringify({ password: String(pass) }),
        });
        showAlert('Password diubah', true);
      } catch (err) {
        showAlert(err.message || 'Gagal ganti password', false);
      }
    });
  });
}

function renderUsers(data, tenants) {
  const root = document.getElementById('user-list');
  const list = data.users || [];
  if (!list.length) {
    root.innerHTML = `<p class="core-empty">Belum ada user. Seed admin via AUTH_USER/AUTH_PASSWORD atau tambah di sini.</p>`;
    return;
  }
  const tenantName = (id) => {
    const t = (tenants || []).find((x) => x.id === id);
    return t ? t.name : id || '—';
  };

  root.innerHTML = `<table class="core-table">
    <thead>
      <tr>
        <th>User</th>
        <th>Role</th>
        <th>Tenant</th>
        <th>Status</th>
        <th>Dibuat</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      ${list.map((u) => `<tr>
        <td>
          <div class="name">${Threads.escapeHtml(u.username)}</div>
          <div class="mono">${Threads.escapeHtml(u.id)}</div>
        </td>
        <td><span class="core-chip">${Threads.escapeHtml(u.role)}</span></td>
        <td>${Threads.escapeHtml(u.role === 'admin' ? '—' : tenantName(u.tenant_id))}</td>
        <td><span class="core-chip${u.active ? ' ok' : ' bad'}">${u.active ? 'aktif' : 'off'}</span></td>
        <td class="mono">${Threads.escapeHtml((u.created_at || '').slice(0, 10) || '—')}</td>
        <td>
          <div class="actions">
            <button type="button" class="th-btn th-btn-ghost" data-toggle-user="${Threads.escapeHtml(u.id)}" data-active="${u.active ? '1' : '0'}">
              ${u.active ? 'Off' : 'On'}
            </button>
            <button type="button" class="th-btn th-btn-ghost" data-pass-user="${Threads.escapeHtml(u.id)}">Pass</button>
          </div>
        </td>
      </tr>`).join('')}
    </tbody>
  </table>`;

  bindUserActions(root);
}

async function editTenant(t) {
  if (!t) return;
  const b = t.billing || {};
  const payload = await Threads.formDialog({
    title: `Edit ${t.name || t.id}`,
    okLabel: 'Simpan',
    bodyHtml: `
      <div class="space-y-3 text-left">
        <div>
          <label class="th-label" for="t-name">Nama</label>
          <input id="t-name" class="th-input" value="${Threads.escapeHtml(t.name || '')}">
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div>
            <label class="th-label" for="t-status">Status</label>
            <select id="t-status" class="th-input">${statusOptions(b.status || 'active')}</select>
          </div>
          <div>
            <label class="th-label" for="t-plan">Plan</label>
            <input id="t-plan" class="th-input" value="${Threads.escapeHtml(b.plan || '')}">
          </div>
        </div>
        <div>
          <label class="th-label" for="t-due">Due at</label>
          <input id="t-due" class="th-input" value="${Threads.escapeHtml(b.due_at || '')}" placeholder="2026-08-01">
        </div>
        <div>
          <label class="th-label" for="t-notes">Notes settlement</label>
          <textarea id="t-notes" class="th-input" rows="2">${Threads.escapeHtml(b.notes || '')}</textarea>
        </div>
      </div>`,
    collect: (root) => ({
      name: root.querySelector('#t-name')?.value?.trim(),
      billing: {
        status: root.querySelector('#t-status')?.value,
        plan: root.querySelector('#t-plan')?.value?.trim(),
        due_at: root.querySelector('#t-due')?.value?.trim(),
        notes: root.querySelector('#t-notes')?.value?.trim(),
      },
    }),
  });
  if (!payload) return;
  try {
    await Threads.api(`/api/admin/tenants/${encodeURIComponent(t.id)}`, {
      method: 'PATCH',
      body: JSON.stringify(payload),
    });
    showAlert('Tenant disimpan', true);
    await loadAll();
  } catch (err) {
    showAlert(err.message || 'Gagal simpan', false);
  }
}

async function addTenant() {
  const payload = await Threads.formDialog({
    title: 'Tenant + login',
    okLabel: 'Buat',
    bodyHtml: `
      <div class="space-y-3 text-left">
        <p class="text-xs text-muted m-0">Tenant tanpa user login tidak bisa masuk /app — isi keduanya.</p>
        <div>
          <label class="th-label" for="nt-name">Nama customer</label>
          <input id="nt-name" class="th-input" placeholder="Studio Contoh">
        </div>
        <div>
          <label class="th-label" for="nt-id">ID tenant (opsional)</label>
          <input id="nt-id" class="th-input" placeholder="studio-contoh">
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div>
            <label class="th-label" for="nt-status">Status</label>
            <select id="nt-status" class="th-input">${statusOptions('trial')}</select>
          </div>
          <div>
            <label class="th-label" for="nt-plan">Plan</label>
            <input id="nt-plan" class="th-input" value="starter">
          </div>
        </div>
        <hr class="border-[var(--border)] my-1">
        <div>
          <label class="th-label" for="nt-user">Username / email login</label>
          <input id="nt-user" class="th-input" autocomplete="off" placeholder="customer@email.com">
        </div>
        <div>
          <label class="th-label" for="nt-pass">Password login</label>
          <input id="nt-pass" type="password" class="th-input" autocomplete="new-password">
        </div>
      </div>`,
    collect: (root) => {
      const name = root.querySelector('#nt-name')?.value?.trim();
      const username = root.querySelector('#nt-user')?.value?.trim();
      const password = root.querySelector('#nt-pass')?.value;
      if (!name) throw new Error('Nama wajib');
      if (!username || !password) throw new Error('Username + password login wajib');
      return {
        name,
        id: root.querySelector('#nt-id')?.value?.trim() || undefined,
        username,
        password,
        billing: {
          status: root.querySelector('#nt-status')?.value || 'trial',
          plan: root.querySelector('#nt-plan')?.value?.trim() || 'starter',
        },
      };
    },
  });
  if (!payload) return;
  try {
    const res = await Threads.api('/api/admin/tenants', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    const u = res.user?.username || payload.username;
    showAlert(`Tenant + login dibuat (${u} → /app)`, true);
    await loadAll();
  } catch (err) {
    showAlert(err.message || 'Gagal buat tenant', false);
  }
}

async function addUser(tenants) {
  const opts = (tenants || []).map((t) =>
    `<option value="${Threads.escapeHtml(t.id)}">${Threads.escapeHtml(t.name)} (${Threads.escapeHtml(t.id)})</option>`
  ).join('');
  const payload = await Threads.formDialog({
    title: 'User baru',
    okLabel: 'Buat',
    bodyHtml: `
      <div class="space-y-3 text-left">
        <div>
          <label class="th-label" for="nu-user">Username</label>
          <input id="nu-user" class="th-input" autocomplete="off">
        </div>
        <div>
          <label class="th-label" for="nu-pass">Password</label>
          <input id="nu-pass" type="password" class="th-input" autocomplete="new-password">
        </div>
        <div>
          <label class="th-label" for="nu-role">Role</label>
          <select id="nu-role" class="th-input">
            <option value="tenant" selected>tenant</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div>
          <label class="th-label" for="nu-tenant">Tenant (role tenant)</label>
          <select id="nu-tenant" class="th-input">${opts || '<option value="">— buat tenant dulu —</option>'}</select>
        </div>
      </div>`,
    collect: (root) => {
      const username = root.querySelector('#nu-user')?.value?.trim();
      const password = root.querySelector('#nu-pass')?.value;
      if (!username || !password) throw new Error('Username dan password wajib');
      return {
        username,
        password,
        role: root.querySelector('#nu-role')?.value,
        tenant_id: root.querySelector('#nu-tenant')?.value,
      };
    },
  });
  if (!payload) return;
  try {
    await Threads.api('/api/admin/users', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
    showAlert('User dibuat', true);
    await loadAll();
  } catch (err) {
    showAlert(err.message || 'Gagal buat user', false);
  }
}

let tenantCache = [];

async function loadAll() {
  showAlert('');
  const [tenants, users] = await Promise.all([
    Threads.api('/api/admin/tenants'),
    Threads.api('/api/admin/users'),
  ]);
  tenantCache = tenants.tenants || [];
  renderTenants(tenants);
  renderUsers(users, tenantCache);
}

document.getElementById('btn-refresh')?.addEventListener('click', () => loadAll().catch((e) => showAlert(e.message, false)));
document.getElementById('btn-add-tenant')?.addEventListener('click', () => addTenant());
document.getElementById('btn-add-user')?.addEventListener('click', () => addUser(tenantCache));

(async () => {
  try {
    if (!(await ensureAdmin())) return;
    await loadAll();
  } catch (err) {
    showAlert(err.message || 'Gagal muat admin', false);
  }
})();
