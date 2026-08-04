window.Core = window.Core || {};

Core.mountShell = function (active) {
  const host = document.getElementById('core-sidebar');
  if (!host) return;

  const item = (id, href, icon, label) => {
    const on = id === active ? ' is-active' : '';
    return `<a class="core-item${on}" href="${href}">
      <span class="core-ico"><i class="bi ${icon}"></i></span>
      <span>${label}</span>
    </a>`;
  };

  host.innerHTML = `
    <div class="core-brand">
      <div class="core-logo" aria-hidden="true">C</div>
      <div class="core-brand-text">
        <div class="core-mark">Core</div>
        <div class="core-sub">Operator console</div>
      </div>
    </div>

    <nav class="core-nav" aria-label="Core">
      <div class="core-group">
        <div class="core-group-label">Access</div>
        ${item('tenants', '/core/', 'bi-buildings', 'Tenants & Users')}
        ${item('pricing', '/core/pricing.html', 'bi-tags', 'Pricing')}
      </div>
      <div class="core-group">
        <div class="core-group-label">Tools</div>
        ${item('app', '/app/ringkasan.html', 'bi-window', 'Buka App')}
      </div>
    </nav>

    <div class="core-foot">
      <div class="core-ctx" id="core-ctx-box">
        <strong id="core-ctx-tenant">—</strong>
        <span id="core-ctx-meta">Context proses</span>
      </div>
      <div class="core-foot-actions">
        <button type="button" class="core-item core-danger" id="btn-logout">
          <span class="core-ico"><i class="bi bi-box-arrow-right"></i></span>
          <span>Logout</span>
        </button>
      </div>
    </div>
  `;

  document.getElementById('btn-logout')?.addEventListener('click', async () => {
    try { await Threads.api('/api/auth/logout', { method: 'POST', body: '{}' }); } catch {}
    location.href = '/core/login.html';
  });

  document.getElementById('core-burger')?.addEventListener('click', () => {
    document.body.classList.toggle('core-nav-open');
  });
  document.body.addEventListener('click', (e) => {
    if (!document.body.classList.contains('core-nav-open')) return;
    if (e.target.closest('.core-sidebar') || e.target.closest('#core-burger')) return;
    document.body.classList.remove('core-nav-open');
  });
};

Core.setContext = function (tenantId, workspaceId, tenantName) {
  const title = document.getElementById('core-ctx-tenant');
  const meta = document.getElementById('core-ctx-meta');
  if (title) title.textContent = tenantName || tenantId || '—';
  if (meta) meta.textContent = `${tenantId || '—'} / ${workspaceId || '—'}`;
};
