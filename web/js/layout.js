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
  fonts.href = 'https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@500;600;700;800&family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap';
  fonts.rel = 'stylesheet';
  document.head.appendChild(fonts);

  const css = document.createElement('link');
  css.id = 'app-css';
  css.rel = 'stylesheet';
  css.href = '/css/app.css?v=ns9';
  document.head.appendChild(css);
};

Threads.mountSidebar = function (active) {
  const el = document.getElementById('sidebar');
  if (!el) return;
  el.classList.add('th-sidebar', 'sb');

  const ico = {
    home:     `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9.5L12 3l9 6.5V20a1 1 0 01-1 1H4a1 1 0 01-1-1z"/><path d="M9 21V12h6v9"/></svg>`,
    chart:    `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M4 19V9"/><path d="M10 19V5"/><path d="M16 19v-7"/><path d="M22 19V8" opacity=".55"/></svg>`,
    ai:       `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M12 3c4.97 0 9 4.03 9 9s-4.03 9-9 9-9-4.03-9-9 4.03-9 9-9z" opacity=".4"/><path d="M9 12h6M12 9v6"/></svg>`,
    post:     `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>`,
    pen:      `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 013 3L7 19l-4 1 1-4z"/></svg>`,
    magic:    `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M15 4V2m0 14v-2M8 9H6m10 0h-2m-5.64-4.36L7 3.27M17 17l-1.36-1.36M3.27 7l1.37 1.37M17 7l-1.36 1.36M7 17l1.36-1.36"/><circle cx="12" cy="9" r="3"/></svg>`,
    chat:     `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/><path d="M8 10h8M8 14h5" opacity=".55"/></svg>`,
    reply:    `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M9 17l-5-5 5-5"/><path d="M4 12h9a7 7 0 017 7v1"/></svg>`,
    image:    `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="8.5" cy="9" r="1.5"/><path d="M21 15l-5-5L5 20"/></svg>`,
    cal:      `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/></svg>`,
    carousel: `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="3" y="5" width="10" height="14" rx="1.5"/><rect x="14.5" y="7" width="6.5" height="10" rx="1.5" opacity=".5"/></svg>`,
    clock:    `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>`,
    ig:       `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="2" y="2" width="20" height="20" rx="5"/><circle cx="12" cy="12" r="4.5"/><circle cx="17.5" cy="6.5" r=".8" fill="currentColor" stroke="none"/></svg>`,
    user:     `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c.6-4 4-7 8-7s7.4 3 8 7"/></svg>`,
    logout:   `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>`,
  };

  const link = (page, path, iconKey, label, badge) => {
    const href = Threads.appUrl(path);
    const on = page === active ? ' is-active' : '';
    const b = badge != null && badge !== '' ? `<span class="sb-badge">${badge}</span>` : '';
    const i = ico[iconKey] ? `<span class="sb-icon" aria-hidden="true">${ico[iconKey]}</span>` : '';
    return `<a class="sb-item${on}" href="${href}">${i}<span>${label}</span>${b}</a>`;
  };

  const group = (label, html) => `
    <div class="sb-group">
      <div class="sb-group-label">${label}</div>
      ${html}
    </div>`;

  el.innerHTML = `
  <div class="sb-brand">
    <div class="sb-logo" aria-hidden="true">
      <svg width="28" height="28" viewBox="0 0 28 28"><rect width="28" height="28" rx="8" fill="url(#sg)"/><rect x="6" y="7" width="9" height="14" rx="2" fill="#fff" opacity=".95"/><rect x="17" y="9" width="5" height="10" rx="1.5" fill="#fff" opacity=".5"/><defs><linearGradient id="sg" x1="0" y1="0" x2="28" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#9A8FFF"/><stop offset="1" stop-color="#7B6CF6"/></linearGradient></defs></svg>
    </div>
    <div class="sb-brand-text">
      <div class="sb-brand-name">malesngonten</div>
    </div>
  </div>

  <div class="sb-ws" id="sb-ws">
    <button type="button" class="sb-acct" id="sb-acct" aria-expanded="false" aria-haspopup="listbox">
      <div class="sb-avatar" id="sb-avatar">··</div>
      <div class="sb-acct-text">
        <div class="sb-acct-name" id="sb-handle">Memuat…</div>
        <div class="sb-acct-sub" id="sb-acct-sub">Pilih akun</div>
      </div>
      <svg class="sb-acct-caret" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M2 4l4 4 4-4"/></svg>
    </button>
    <div class="sb-ws-menu" id="sb-ws-menu" hidden role="listbox"></div>
  </div>

  <nav class="sb-nav">
    ${group('Pantau', `
      ${link('ringkasan',  '/ringkasan',  'home',  'Ringkasan')}
      ${link('insights',   '/insights',   'chart', 'Insight')}
      ${link('ai-insight', '/ai-insight', 'ai',    'AI Insight')}
      ${link('posts',      '/posts',      'post',  'Post')}
      ${link('balasan',    '/balasan',    'reply', 'Balasan')}
    `)}
    ${group('Buat', `
      ${link('buat',      '/buat',      'pen',     'Tulis post')}
      ${link('generate',  '/generate',  'magic',   'Generate AI')}
      ${link('cover-templates', '/cover-templates', 'carousel', 'Template cover')}
      ${link('carousel-templates', '/carousel-templates', 'carousel', 'Template carousel')}
      ${link('chat',      '/chat',      'chat',    'Chat AI')}
      ${link('kalender',  '/kalender',  'cal',     'Kalender')}
      ${link('ig-carousel', '/ig-carousel', 'carousel', 'Carousel')}
    `)}
    ${group('Jadwal & Auto', `
      ${link('lazy',       '/lazy',       'clock',   'Antrean Lazy')}
      ${link('lazy-track', '/lazy-track', 'chart',   'Performa Lazy')}
      ${link('ig-profil',  '/ig-profil',  'ig',      'Instagram')}
    `)}
  </nav>

  <div class="sb-foot">
    <a class="sb-item${active === 'akun' ? ' is-active' : ''}" href="${Threads.appUrl('/akun')}">
      <span class="sb-icon" aria-hidden="true">${ico.user}</span><span>Akun & API</span>
    </a>
    <button type="button" id="btn-logout" class="sb-item sb-logout">
      ${ico.logout}<span>Keluar</span>
    </button>
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
    location.href = Threads.appUrl('/login');
  });

  (async () => {
    try {
      const data = await Threads.api('/api/repliz/accounts');
      const list = (data.accounts || []).filter((a) => a.id || a._id);
      const activeId = data.active_id || '';
      const activeAcct = list.find((a) => (a.id || a._id) === activeId) || list.find((a) => a.isConnected) || list[0];
      const handleOf = (a) => {
        const u = String(a?.username || a?.name || '').replace(/^@/, '');
        return u ? '@' + u : (a?.id || 'Akun Repliz');
      };
      const typeOf = (a) => String(a?.type || 'repliz').toLowerCase();
      document.getElementById('sb-handle').textContent = handleOf(activeAcct);
      document.getElementById('sb-acct-sub').textContent = activeAcct
        ? `${typeOf(activeAcct)} · ${list.length} akun Repliz`
        : 'Tidak ada akun Repliz';
      const av = document.getElementById('sb-avatar');
      const initials = String(activeAcct?.username || activeAcct?.name || 'R').replace(/^@/, '').slice(0, 2).toUpperCase();
      if (activeAcct?.picture) {
        av.textContent = '';
        av.classList.add('has-pic');
        av.style.backgroundImage = `url(${JSON.stringify(activeAcct.picture)})`;
      } else {
        av.textContent = initials;
        av.classList.remove('has-pic');
        av.style.backgroundImage = '';
      }

      menu.innerHTML = list.map((a) => {
        const id = a.id || a._id;
        const on = id === (activeAcct?.id || activeAcct?._id);
        const off = a.isConnected === false ? ' off' : '';
        const mark = on ? '<i class="bi bi-check2"></i>' : '';
        return `<button type="button" class="sb-ws-opt${on ? ' is-current' : ''}${off}" data-switch-acct="${Threads.escapeHtml(id)}" role="option" aria-selected="${on ? 'true' : 'false'}">
          <span class="sb-ws-opt-name">${Threads.escapeHtml(handleOf(a))}</span>
          <span class="sb-ws-opt-meta">${Threads.escapeHtml(typeOf(a))}${mark}</span>
        </button>`;
      }).join('') + `<a class="sb-ws-opt sb-ws-manage" href="${Threads.appUrl('/akun')}"><span>Kelola akun & API</span><i class="bi bi-arrow-right"></i></a>`;

      menu.querySelectorAll('[data-switch-acct]').forEach((btn) => {
        btn.addEventListener('click', async () => {
          const id = btn.getAttribute('data-switch-acct');
          if (!id || btn.classList.contains('is-current')) {
            closeMenu();
            return;
          }
          try {
            await Threads.api('/api/repliz/accounts/switch', { method: 'POST', body: JSON.stringify({ id }) });
            location.reload();
          } catch (err) {
            Threads.toast(err.message || 'Gagal ganti akun Repliz', false);
          }
        });
      });
    } catch (err) {
      document.getElementById('sb-handle').textContent = 'Repliz';
      document.getElementById('sb-acct-sub').textContent = err?.message || 'Belum tersambung';
      menu.innerHTML = `<a class="sb-ws-opt sb-ws-manage" href="${Threads.appUrl('/akun')}"><span>Kelola akun</span><i class="bi bi-arrow-right"></i></a>`;
    }
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
