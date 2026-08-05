window.Threads = window.Threads || {};

/** Customer dashboard base path (operator console is /core). */
Threads.APP = '/app';

Threads.appUrl = function (path) {
  const p = String(path || '');
  if (!p) return Threads.APP + '/';
  if (p.startsWith('/app/') || p.startsWith('/core/') || p.startsWith('/api/')) return p;
  if (p.startsWith('/')) return Threads.APP + p;
  return Threads.APP + '/' + p;
};

Threads.ensureAssets = function () {
  // Jangan inject ulang kalau halaman sudah punya app.css (hindari override versi lama).
  if (document.getElementById('app-css') || document.querySelector('link[href*="/css/app.css"]')) return;

  const pre1 = document.createElement('link');
  pre1.rel = 'preconnect';
  pre1.href = 'https://fonts.googleapis.com';
  document.head.appendChild(pre1);

  const pre2 = document.createElement('link');
  pre2.rel = 'preconnect';
  pre2.href = 'https://fonts.gstatic.com';
  pre2.crossOrigin = 'anonymous';
  document.head.appendChild(pre2);

  const fonts = document.createElement('link');
  fonts.href = 'https://fonts.googleapis.com/css2?family=Sora:wght@400;500;600;700;800&display=swap';
  fonts.rel = 'stylesheet';
  document.head.appendChild(fonts);

  const css = document.createElement('link');
  css.id = 'app-css';
  css.rel = 'stylesheet';
  css.href = '/css/app.css?v=mn79';
  document.head.appendChild(css);
};

Threads.mountSidebar = function (active) {
  const el = document.getElementById('sidebar');
  if (!el) return;
  el.classList.add('th-sidebar', 'sb');

  const link = (page, path, icon, label, badge) => {
    const href = Threads.appUrl(path);
    const on = page === active ? ' is-active' : '';
    const b = badge ? `<span class="sb-badge">${badge}</span>` : '<span></span>';
    return `<a class="sb-item${on}" href="${href}"><span class="sb-icon"><i class="bi ${icon}"></i></span><span>${label}</span>${b}</a>`;
  };

  const group = (label, html) => `
    <div class="sb-group">
      <div class="sb-group-label">${label}</div>
      ${html}
    </div>`;

  el.innerHTML = `
  <div class="sb-brand">
    <div class="sb-logo" aria-hidden="true"><span class="sb-logo-dot"></span></div>
    <div class="sb-brand-text">
      <div class="sb-brand-name">malesngonten</div>
      <div class="sb-brand-sub">dashboard</div>
    </div>
  </div>

  <div class="sb-ws" id="sb-ws">
    <button type="button" class="sb-acct" id="sb-acct" aria-expanded="false" aria-haspopup="listbox">
      <div class="sb-avatar" id="sb-avatar">··</div>
      <div class="sb-acct-text">
        <div class="sb-acct-name" id="sb-handle">Memuat…</div>
        <div class="sb-acct-sub" id="sb-acct-sub">Pilih akun</div>
      </div>
      <i class="bi bi-chevron-down sb-acct-caret" aria-hidden="true"></i>
    </button>
    <div class="sb-ws-menu" id="sb-ws-menu" hidden role="listbox"></div>
  </div>

  <nav class="sb-nav">
    ${group('Umum', `
      ${link('ringkasan', '/ringkasan.html', 'bi-grid-1x2', 'Ringkasan')}
      ${link('profil', '/profil.html', 'bi-person-circle', 'Profil')}
    `)}
    ${group('Konten', `
      ${link('buat', '/buat.html', 'bi-pencil-square', 'Buat post')}
      ${link('generate', '/generate.html', 'bi-magic', 'Generate')}
      ${link('lazy', '/lazy.html', 'bi-lightning-charge', 'Lazy')}
      ${link('lazy-track', '/lazy-track.html', 'bi-graph-up-arrow', 'Lazy track')}
      ${link('posts', '/posts.html', 'bi-collection', 'Posts')}
      ${link('balasan', '/balasan.html', 'bi-chat-dots', 'Balasan')}
    `)}
    ${group('Analytics', `
      ${link('insights', '/insights.html', 'bi-bar-chart', 'Insight')}
      ${link('ai-insight', '/ai-insight.html', 'bi-stars', 'AI insight')}
      ${link('kuota', '/kuota.html', 'bi-speedometer2', 'Kuota')}
    `)}
    ${group('Instagram', `
      ${link('ig-profil', '/ig-profil.html', 'bi-instagram', 'Profil')}
      ${link('ig-posts', '/ig-posts.html', 'bi-images', 'Posts')}
      ${link('ig-carousel', '/ig-carousel.html', 'bi-collection-play', 'Carousel')}
      ${link('carousel-templates', '/carousel-templates.html', 'bi-palette2', 'Template')}
    `)}
    ${group('Akun', `
      ${link('akun', '/akun.html', 'bi-key', 'API & koneksi')}
      <button type="button" id="btn-logout" class="sb-item sb-logout">
        <span class="sb-icon"><i class="bi bi-box-arrow-right"></i></span>
        <span>Keluar</span>
        <span></span>
      </button>
    `)}
  </nav>

  <div class="sb-foot">
    <div class="sb-quota" id="sb-quota" hidden>
      <div class="sb-quota-row">
        <span>Kuota API</span>
        <span class="mono" id="sb-quota-label">—</span>
      </div>
      <div class="sb-bar"><div class="sb-bar-fill" id="sb-quota-fill" style="width:0%"></div></div>
    </div>
    <a class="sb-upgrade" href="${Threads.appUrl('/buat.html')}">Buat post</a>
  </div>
  `;

  const acctBtn = el.querySelector('#sb-acct');
  const menu = el.querySelector('#sb-ws-menu');
  const closeMenu = () => {
    menu.hidden = true;
    acctBtn?.setAttribute('aria-expanded', 'false');
    el.querySelector('#sb-ws')?.classList.remove('is-open');
  };
  const openMenu = () => {
    menu.hidden = false;
    acctBtn?.setAttribute('aria-expanded', 'true');
    el.querySelector('#sb-ws')?.classList.add('is-open');
  };

  acctBtn?.addEventListener('click', (e) => {
    e.stopPropagation();
    if (menu.hidden) openMenu();
    else closeMenu();
  });
  document.addEventListener('click', (e) => {
    if (!el.contains(e.target)) closeMenu();
  });

  el.querySelector('#btn-logout')?.addEventListener('click', async () => {
    try { await Threads.api('/api/auth/logout', { method: 'POST', body: '{}' }); } catch {}
    location.href = Threads.appUrl('/login.html');
  });

  (async () => {
    try {
      const [data, org] = await Promise.all([
        Threads.api('/api/accounts'),
        Threads.api('/api/org').catch(() => null),
      ]);
      const list = data.accounts || [];
      const activeAcct = list.find(a => a.active) || list[0];
      const label = activeAcct?.threads_username
        ? '@' + String(activeAcct.threads_username).replace(/^@/, '')
        : (activeAcct?.name || 'Belum connect');
      document.getElementById('sb-handle').textContent = label;
      const wsName = org?.workspace?.name || 'Workspace';
      document.getElementById('sb-acct-sub').textContent = activeAcct
        ? `${wsName} · ${list.length} akun`
        : wsName;
      const initials = String(activeAcct?.threads_username || activeAcct?.name || 'T')
        .replace(/^@/, '').slice(0, 2).toUpperCase();
      document.getElementById('sb-avatar').textContent = initials;

      menu.innerHTML = list.map(a => {
        const name = a.threads_username
          ? '@' + String(a.threads_username).replace(/^@/, '')
          : (a.name || a.id);
        const mark = a.active ? '<i class="bi bi-check2"></i>' : '';
        return `<button type="button" class="sb-ws-opt${a.active ? ' is-current' : ''}" data-switch-acct="${Threads.escapeHtml(a.id)}" role="option" aria-selected="${a.active ? 'true' : 'false'}">
          <span class="sb-ws-opt-name">${Threads.escapeHtml(name)}</span>
          <span class="sb-ws-opt-meta">${a.lazy_enabled ? 'Lazy' : ''}${mark}</span>
        </button>`;
      }).join('') + `<a class="sb-ws-opt sb-ws-manage" href="${Threads.appUrl('/akun.html')}"><span>Kelola akun & API</span><i class="bi bi-arrow-right"></i></a>`;

      menu.querySelectorAll('[data-switch-acct]').forEach(btn => {
        btn.addEventListener('click', async () => {
          const id = btn.getAttribute('data-switch-acct');
          if (!id || btn.classList.contains('is-current')) {
            closeMenu();
            return;
          }
          try {
            await Threads.api('/api/accounts/switch', { method: 'POST', body: JSON.stringify({ id }) });
            location.reload();
          } catch (err) {
            Threads.toast(err.message || 'Gagal ganti akun', false);
          }
        });
      });
    } catch {
      document.getElementById('sb-handle').textContent = 'Belum connect';
      document.getElementById('sb-acct-sub').textContent = 'Buka kelola akun';
      menu.innerHTML = `<a class="sb-ws-opt sb-ws-manage" href="${Threads.appUrl('/akun.html')}"><span>Kelola akun</span><i class="bi bi-arrow-right"></i></a>`;
    }
    try {
      const q = await Threads.api('/api/quota');
      const row = Array.isArray(q?.data) ? q.data[0] : q;
      const used = Number(row?.quota_usage ?? 0);
      const max = Number(row?.config?.quota_total ?? 250);
      if (!Number.isFinite(used) || !Number.isFinite(max) || max <= 0) return;
      const pct = Math.max(0, Math.min(100, Math.round((used / max) * 100)));
      document.getElementById('sb-quota').hidden = false;
      document.getElementById('sb-quota-label').textContent = `${used}/${max}`;
      document.getElementById('sb-quota-fill').style.width = pct + '%';
    } catch {}
  })();
};

Threads.pageShell = function (active) {
  Threads.ensureAssets();
  document.body.classList.add('strand-body');
  Threads.mountSidebar(active);
  const main = document.querySelector('main');
  if (main) {
    main.classList.add('app-main', 'main');
  }
};
