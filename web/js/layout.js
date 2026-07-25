window.Threads = window.Threads || {};

Threads.ensureAssets = function () {
  if (document.getElementById('app-css')) return;

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
  fonts.href = 'https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:ital,wght@0,400;0,500;0,600;0,700;0,800;1,400&display=swap';
  fonts.rel = 'stylesheet';
  document.head.appendChild(fonts);

  const css = document.createElement('link');
  css.id = 'app-css';
  css.rel = 'stylesheet';
  css.href = '/css/app.css?v=strand10';
  document.head.appendChild(css);
};

Threads.mountSidebar = function (active) {
  const el = document.getElementById('sidebar');
  if (!el) return;
  el.classList.add('th-sidebar', 'sb');

  const link = (page, href, icon, label, badge) => {
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
    <div class="sb-logo" aria-hidden="true">
      <svg viewBox="0 0 24 24" width="22" height="22">
        <circle cx="12" cy="12" r="11" fill="var(--accent)"/>
        <path d="M6 16 C 9 8, 15 8, 18 16" stroke="#fff" stroke-width="1.8" fill="none" stroke-linecap="round"/>
        <path d="M6 12 C 9 6, 15 6, 18 12" stroke="#fff" stroke-width="1.4" fill="none" stroke-linecap="round" opacity=".55"/>
      </svg>
    </div>
    <div class="sb-brand-text">
      <div class="sb-brand-name">Threads</div>
      <div class="sb-brand-sub">Workspace</div>
    </div>
  </div>

  <div class="sb-acct" id="sb-acct">
    <div class="sb-avatar" id="sb-avatar">··</div>
    <div class="sb-acct-text">
      <div class="sb-acct-name" id="sb-handle">Belum connect</div>
      <div class="sb-acct-sub">Personal workspace</div>
    </div>
  </div>

  <nav class="sb-nav">
    ${group('Overview', `
      ${link('ringkasan', '/ringkasan.html', 'bi-grid-1x2', 'Ringkasan')}
      ${link('profil', '/profil.html', 'bi-person-circle', 'Profil')}
    `)}
    ${group('Content', `
      ${link('buat', '/buat.html', 'bi-pencil-square', 'Buat Post')}
      ${link('generate', '/generate.html', 'bi-magic', 'Generate AI')}
      ${link('thumbnail', '/thumbnail.html', 'bi-image', 'Lab Thumbnail')}
      ${link('lazy', '/lazy.html', 'bi-lightning-charge', 'Lazy Business')}
      ${link('posts', '/posts.html', 'bi-collection', 'Post Saya')}
      ${link('balasan', '/balasan.html', 'bi-chat-dots', 'Balasan')}
    `)}
    ${group('Analytics', `
      ${link('insights', '/insights.html', 'bi-bar-chart', 'Insight')}
      ${link('ai-insight', '/ai-insight.html', 'bi-stars', 'AI Insight')}
      ${link('kuota', '/kuota.html', 'bi-speedometer2', 'Kuota')}
    `)}
    ${group('Instagram', `
      ${link('ig-profil', '/ig-profil.html', 'bi-instagram', 'IG Profil')}
      ${link('ig-posts', '/ig-posts.html', 'bi-images', 'IG Posts')}
      ${link('ig-carousel', '/ig-carousel.html', 'bi-collection-play', 'IG Carousel')}
      ${link('ig-token', '/ig-token.html', 'bi-key-fill', 'IG Token')}
    `)}
    ${group('Settings', `
      ${link('token', '/token.html', 'bi-key', 'Threads Token')}
      <button type="button" id="btn-logout" class="sb-item sb-logout">
        <span class="sb-icon"><i class="bi bi-box-arrow-right"></i></span>
        <span>Logout</span>
        <span></span>
      </button>
    `)}
  </nav>

  <div class="sb-foot">
    <div class="sb-quota" id="sb-quota" hidden>
      <div class="sb-quota-row">
        <span>API quota</span>
        <span class="mono" id="sb-quota-label">—</span>
      </div>
      <div class="sb-bar"><div class="sb-bar-fill" id="sb-quota-fill" style="width:0%"></div></div>
    </div>
    <a class="sb-upgrade" href="/buat.html">+ Buat post baru</a>
  </div>
  `;

  el.querySelector('#btn-logout')?.addEventListener('click', async () => {
    try { await Threads.api('/api/auth/logout', { method: 'POST', body: '{}' }); } catch {}
    location.href = '/login.html';
  });

  // Soft-fill account chip (non-blocking)
  (async () => {
    try {
      const me = await Threads.api('/api/me');
      const handle = me?.username ? '@' + String(me.username).replace(/^@/, '') : '';
      if (handle) {
        document.getElementById('sb-handle').textContent = handle;
        const initials = String(me.username || 'T').replace(/^@/, '').slice(0, 2).toUpperCase();
        document.getElementById('sb-avatar').textContent = initials;
      }
    } catch {}
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
