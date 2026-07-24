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
  css.href = '/css/app.css?v=saas30';
  document.head.appendChild(css);
};

Threads.mountSidebar = function (active) {
  const el = document.getElementById('sidebar');
  if (!el) return;
  el.classList.add('th-sidebar');

  const link = (page, href, icon, label) => {
    const on = page === active ? ' active' : '';
    return `<a class="th-nav-link${on}" href="${href}"><i class="bi ${icon}"></i><span>${label}</span></a>`;
  };

  el.innerHTML = `
  <div class="th-brand">
    <div class="th-brand-mark">T</div>
    <div>
      <div class="th-brand-text">Threads</div>
      <span class="th-brand-sub">Workspace</span>
    </div>
  </div>
  <div class="th-nav-label">Overview</div>
  ${link('ringkasan', '/ringkasan.html', 'bi-grid-1x2', 'Ringkasan')}
  ${link('profil', '/profil.html', 'bi-person-circle', 'Profil')}
  <div class="th-nav-label">Content</div>
  ${link('buat', '/buat.html', 'bi-pencil-square', 'Buat Post')}
  ${link('generate', '/generate.html', 'bi-magic', 'Generate AI')}
  ${link('lazy', '/lazy.html', 'bi-lightning-charge', 'Lazy Business')}
  ${link('posts', '/posts.html', 'bi-collection', 'Post Saya')}
  ${link('balasan', '/balasan.html', 'bi-chat-dots', 'Balasan')}
  <div class="th-nav-label">Analytics</div>
  ${link('insights', '/insights.html', 'bi-bar-chart', 'Insight')}
  ${link('ai-insight', '/ai-insight.html', 'bi-stars', 'AI Insight')}
  ${link('kuota', '/kuota.html', 'bi-speedometer2', 'Kuota')}
  <div class="th-nav-label">Instagram</div>
  ${link('ig-profil', '/ig-profil.html', 'bi-instagram', 'IG Profil')}
  ${link('ig-posts', '/ig-posts.html', 'bi-images', 'IG Posts')}
  ${link('ig-carousel', '/ig-carousel.html', 'bi-collection-play', 'IG Carousel')}
  ${link('ig-token', '/ig-token.html', 'bi-key-fill', 'IG Token')}
  <div class="th-nav-label">Settings</div>
  ${link('token', '/token.html', 'bi-key', 'Threads Token')}
  <button type="button" id="btn-logout" class="th-nav-link th-nav-logout">
    <i class="bi bi-box-arrow-right"></i><span>Logout</span>
  </button>
  `;

  el.querySelector('#btn-logout')?.addEventListener('click', async () => {
    try { await Threads.api('/api/auth/logout', { method: 'POST', body: '{}' }); } catch {}
    location.href = '/login.html';
  });
};

Threads.pageShell = function (active) {
  Threads.ensureAssets();
  Threads.mountSidebar(active);
  document.querySelector('main')?.classList.add('app-main');
};
