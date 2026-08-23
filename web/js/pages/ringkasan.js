Threads.pageShell('ringkasan');

let state = {
  range: '30D',
  metric: 'likes',
  sort: 'views',
  posts: [],
  insights: null,
  me: null,
  loading: 'idle', // idle | account | posts | done
};

const SORT_LABEL = {
  views: 'views tertinggi',
  likes: 'likes tertinggi',
  eng: 'interaksi tertinggi (likes + balasan + repost + kutipan)',
  er: 'engagement rate (interaksi ÷ views)',
};

const fmt = {
  num(n) {
    n = Number(n) || 0;
    if (n >= 1e6) return (n / 1e6).toFixed(1).replace(/\.0$/, '') + 'jt';
    if (n >= 1e3) return (n / 1e3).toFixed(n >= 1e4 ? 1 : 1).replace(/\.0$/, '') + 'rb';
    return String(Math.round(n));
  },
  full(n) {
    return (Number(n) || 0).toLocaleString('id-ID');
  },
};

function metricMap(insights) {
  const m = { ...(insights?.metrics || {}) };
  const t = insights?.totals || {};
  ['views', 'likes', 'replies', 'reposts', 'quotes', 'followers_count', 'follows_count', 'media_count', 'reach', 'saves', 'accounts_engaged', 'total_interactions'].forEach((k) => {
    if (m[k] == null && t[k] != null) m[k] = t[k];
  });
  if (m.followers_count == null && insights?.followers_count != null) {
    m.followers_count = insights.followers_count;
  }
  return m;
}

function postMetrics(p) {
  const m = p?.metrics || {};
  const likes = Number(m.likes ?? 0) || 0;
  const replies = Number(m.replies ?? 0) || 0;
  const reposts = Number(m.reposts ?? 0) || 0;
  const quotes = Number(m.quotes ?? 0) || 0;
  const views = Number(m.views ?? 0) || 0;
  const eng = likes + replies + reposts + quotes;
  const er = views > 0 ? (eng / views) * 100 : 0;
  return { likes, replies, reposts, quotes, views, eng, er };
}

function rangeQuery(range) {
  const days = range === '7D' ? 7 : range === '90D' ? 90 : 30;
  const until = Math.floor(Date.now() / 1000);
  const since = until - days * 86400;
  return `since=${since}&until=${until}`;
}

function relativeTime(iso) {
  if (!iso) return '—';
  const t = new Date(iso).getTime();
  if (!t) return '—';
  const diff = Date.now() - t;
  const d = Math.floor(diff / 86400000);
  if (d < 1) return Math.max(1, Math.floor(diff / 3600000)) + 'j';
  if (d < 7) return d + 'h';
  if (d < 30) return Math.floor(d / 7) + 'mgg';
  return new Date(iso).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
}

function chartDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (!d.getTime()) return '';
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
}

function pickTicks(n, maxTicks) {
  if (n <= 0) return [];
  if (n <= maxTicks) return Array.from({ length: n }, (_, i) => i);
  const out = [0];
  for (let i = 1; i < maxTicks - 1; i++) {
    out.push(Math.round((i / (maxTicks - 1)) * (n - 1)));
  }
  out.push(n - 1);
  return [...new Set(out)];
}

function sparkPath(values, w = 132, h = 28, pad = 2) {
  if (!values || values.length < 2) return null;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = Math.max(max - min, 1);
  const pts = values.map((v, i) => {
    const x = pad + (i / (values.length - 1)) * (w - pad * 2);
    const y = h - pad - ((v - min) / span) * (h - pad * 2);
    return [x, y];
  });
  const line = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join(' ');
  const area = line + ` L ${w - pad} ${h - pad} L ${pad} ${h - pad} Z`;
  return { line, area, w, h };
}

function setLoadState(phase, message) {
  state.loading = phase;
  const chip = document.getElementById('load-chip');
  const text = document.getElementById('load-chip-text');
  const chartLoad = document.getElementById('chart-load');
  const postsLoad = document.getElementById('posts-load');
  const busy = phase === 'account' || phase === 'posts';

  if (chip) {
    chip.classList.toggle('show', busy || phase === 'done');
    chip.classList.toggle('is-done', phase === 'done');
    if (phase === 'idle') chip.classList.remove('show', 'is-done');
  }
  if (text) {
    if (phase === 'done') text.textContent = 'Data siap';
    else if (message) text.textContent = message;
    else if (phase === 'account') text.textContent = 'Memuat metrik akun…';
    else if (phase === 'posts') text.textContent = 'Memuat tren & post…';
  }
  const spin = chip?.querySelector('.ov-spinner');
  if (spin) spin.style.display = busy ? '' : 'none';
  chartLoad?.classList.toggle('show', busy);
  postsLoad?.classList.toggle('show', busy);

  if (phase === 'done') {
    window.clearTimeout(setLoadState._doneTimer);
    setLoadState._doneTimer = window.setTimeout(() => {
      if (state.loading === 'done') chip?.classList.remove('show');
    }, 1600);
  }
}

function renderKPISkeleton() {
  const row = document.getElementById('kpi-row');
  row.setAttribute('aria-busy', 'true');
  row.innerHTML = Array.from({ length: 6 }, () => `
    <article class="ov-kpi is-skel" aria-hidden="true">
      <span class="ov-kpi-skel-k"></span>
      <span class="ov-kpi-skel-v"></span>
      <span class="ov-kpi-skel-sub"></span>
      <span class="ov-kpi-skel-spark"></span>
    </article>`).join('');
}

function isInstagram() {
  return String(state.me?.type || state.insights?.repliz_account?.type || '').toLowerCase() === 'instagram';
}

function renderKPIs() {
  const m = metricMap(state.insights);
  const posts = state.posts;
  const seriesLikes = posts.map((p) => postMetrics(p).likes).reverse();
  const seriesViews = posts.map((p) => postMetrics(p).views).reverse();
  const ig = isInstagram();
  const followers = m.followers_count ?? 0;
  const likes = m.likes ?? 0;
  const views = m.views ?? 0;
  const replies = m.replies ?? 0;
  const eng = likes + replies + (m.reposts || 0) + (m.quotes || 0);
  const er = views > 0 ? (eng / views) * 100 : 0;
  const avgLikes = posts.length
    ? posts.reduce((s, p) => s + postMetrics(p).likes, 0) / posts.length
    : 0;
  const n = state.insights?.post_count || posts.length || 0;

  const kpis = ig
    ? [
        { k: 'Reach', v: fmt.full(m.reach || 0), spark: seriesViews.length > 1 ? seriesViews : null, lead: true },
        { k: 'Views', v: fmt.full(views), spark: seriesViews },
        { k: 'Likes', v: fmt.full(likes), spark: seriesLikes },
        { k: 'Komentar', v: fmt.full(replies), spark: posts.map((p) => postMetrics(p).replies).reverse() },
        { k: 'Shares', v: fmt.full(m.reposts || 0), spark: posts.map((p) => postMetrics(p).reposts).reverse() },
        { k: 'Simpan', v: fmt.full(m.saves || 0) },
      ]
    : [
        { k: 'Pengikut', v: followers ? fmt.full(followers) : '—', spark: seriesLikes.length > 1 ? seriesLikes : null, lead: true },
        { k: 'Views', v: fmt.full(views), spark: seriesViews.length > 1 ? seriesViews : null },
        { k: 'Likes', v: fmt.full(likes), spark: seriesLikes },
        { k: 'Balasan', v: fmt.full(replies), spark: posts.map((p) => postMetrics(p).replies).reverse() },
        { k: 'Engagement rate', v: er.toFixed(2) + '%', spark: posts.map((p) => postMetrics(p).er).reverse() },
        { k: 'Avg likes / post', v: fmt.full(avgLikes), spark: seriesLikes },
      ];

  const row = document.getElementById('kpi-row');
  row.innerHTML = kpis.map((k) => {
    const sp = sparkPath(k.spark?.length > 1 ? k.spark : null);
    return `<article class="ov-kpi ${k.lead ? 'ov-kpi-lead' : ''}">
      <div class="ov-kpi-k">${Threads.escapeHtml(k.k)}</div>
      <div class="ov-kpi-v mono">${Threads.escapeHtml(String(k.v))}</div>
      <div class="ov-kpi-sub">${n} post sampel</div>
      ${sp ? `<svg class="ov-spark" viewBox="0 0 ${sp.w} ${sp.h}" preserveAspectRatio="none">
        <path d="${sp.area}" fill="var(--accent-soft)"></path>
        <path d="${sp.line}" stroke="var(--accent)" stroke-width="1.5" fill="none"></path>
      </svg>` : ''}
    </article>`;
  }).join('');
  row.removeAttribute('aria-busy');
}

function renderChart() {
  const posts = [...state.posts].reverse();
  const series = posts.map((p) => postMetrics(p)[state.metric] || 0);
  const svg = document.getElementById('chart-svg');
  const tip = document.getElementById('chart-tip');
  const wrap = document.getElementById('chart-wrap');
  const empty = document.getElementById('chart-empty');
  const yEl = document.getElementById('chart-y');
  const xEl = document.getElementById('chart-x');
  const labels = {
    views: 'views',
    likes: 'likes',
    replies: 'balasan',
    reposts: 'repost',
    quotes: 'kutipan',
  };
  const label = labels[state.metric] || state.metric;
  wrap?.setAttribute('data-metric', state.metric);
  document.getElementById('chart-title').textContent =
    `${label} per post · kiri lebih lama`;

  const waiting = state.loading === 'account' || state.loading === 'posts';
  const clearHover = () => {
    tip.hidden = true;
    const g = document.getElementById('chart-hover');
    if (g) g.innerHTML = '';
  };

  if (series.length < 2) {
    svg.innerHTML = '';
    if (yEl) yEl.innerHTML = '';
    if (xEl) xEl.innerHTML = '';
    empty?.classList.toggle('show', !waiting);
    document.getElementById('stat-peak').textContent = series.length ? fmt.full(series[0]) : '—';
    document.getElementById('stat-avg').textContent = series.length ? fmt.full(series[0]) : '—';
    document.getElementById('stat-low').textContent = series.length ? fmt.full(series[0]) : '—';
    svg.onmousemove = null;
    svg.onmouseleave = null;
    clearHover();
    return;
  }

  empty?.classList.remove('show');
  const w = 880, h = 280, padL = 8, padR = 10, padT = 10, padB = 8;
  const innerW = w - padL - padR, innerH = h - padT - padB;
  const min = Math.min(...series), max = Math.max(...series);
  const span = Math.max(max - min, 1);
  const yPad = span * 0.08;
  const yMin = Math.max(0, min - yPad);
  const yMax = max + yPad;
  const ySpan = yMax - yMin || 1;
  const avg = series.reduce((a, b) => a + b, 0) / series.length;
  const xOf = (i) => padL + (i / Math.max(series.length - 1, 1)) * innerW;
  const yOf = (v) => padT + innerH - ((v - yMin) / ySpan) * innerH;
  const path = series.map((v, i) => (i ? 'L' : 'M') + xOf(i).toFixed(1) + ' ' + yOf(v).toFixed(1)).join(' ');
  const area = path + ` L ${xOf(series.length - 1)} ${padT + innerH} L ${xOf(0)} ${padT + innerH} Z`;
  const yTicks = Array.from({ length: 5 }, (_, i) => yMin + (i / 4) * ySpan);
  const xTicks = pickTicks(series.length, series.length > 18 ? 4 : 5);

  if (yEl) {
    yEl.innerHTML = yTicks.map((t) => `<span>${Threads.escapeHtml(fmt.num(t))}</span>`).join('');
  }
  if (xEl) {
    xEl.innerHTML = xTicks.map((i) => {
      const d = chartDate(posts[i]?.timestamp) || `#${i + 1}`;
      return `<span>${Threads.escapeHtml(d)}</span>`;
    }).join('');
  }

  svg.innerHTML = `
    ${yTicks.map((t) => `<line class="ov-chart-grid" x1="${padL}" x2="${w - padR}" y1="${yOf(t)}" y2="${yOf(t)}" />`).join('')}
    <path class="ov-chart-area" d="${area}"></path>
    <line class="ov-chart-avg" x1="${padL}" x2="${w - padR}" y1="${yOf(avg)}" y2="${yOf(avg)}" />
    <path class="ov-chart-line" d="${path}"></path>
    <g id="chart-hover"></g>
  `;

  document.getElementById('stat-peak').textContent = fmt.full(Math.max(...series));
  document.getElementById('stat-avg').textContent = fmt.full(avg);
  document.getElementById('stat-low').textContent = fmt.full(Math.min(...series));

  svg.onmousemove = (e) => {
    const rect = svg.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * w;
    const ratio = (x - padL) / innerW;
    const idx = Math.max(0, Math.min(series.length - 1, Math.round(ratio * (series.length - 1))));
    const g = document.getElementById('chart-hover');
    g.innerHTML = `
      <line class="ov-chart-cross" x1="${xOf(idx)}" x2="${xOf(idx)}" y1="${padT}" y2="${padT + innerH}" />
      <circle class="ov-chart-dot" cx="${xOf(idx)}" cy="${yOf(series[idx])}" r="4.5"/>
    `;
    const when = chartDate(posts[idx]?.timestamp);
    const snip = String(posts[idx]?.text || '').replace(/\s+/g, ' ').trim().slice(0, 80);
    tip.hidden = false;
    tip.style.left = Math.max(12, Math.min(88, (xOf(idx) / w) * 100)) + '%';
    tip.innerHTML = `<div class="ov-chart-tip-date">${Threads.escapeHtml(when || 'Post')} · #${idx + 1}</div>
      ${Threads.escapeHtml(label)}: <strong class="mono">${fmt.full(series[idx])}</strong>
      ${snip ? `<div class="ov-chart-tip-snip">${Threads.escapeHtml(snip)}</div>` : ''}`;
  };
  svg.onmouseleave = clearHover;
}

function renderPulse() {
  const ig = isInstagram();
  const handle = state.me?.username ? '@' + String(state.me.username).replace(/^@/, '') : 'Belum connect';
  document.getElementById('pulse-handle').textContent = handle;
  const m = metricMap(state.insights);
  const followers = m.followers_count || 0;
  const n = state.insights?.post_count || state.posts.length || 0;
  const typeLabel = ig ? 'Instagram' : (state.me?.type ? String(state.me.type) : 'Repliz');
  const followBit = followers
    ? `${fmt.full(followers)} pengikut`
    : (ig ? `${fmt.full(m.reach || 0)} reach` : null);
  document.getElementById('pulse-meta').textContent = state.me?.username
    ? [typeLabel, followBit, `${n} post sampel`].filter(Boolean).join(' · ')
    : 'Pilih akun Repliz di sidebar';

  const items = ig
    ? [
        ['Reach', m.reach],
        ['Views', m.views],
        ['Likes', m.likes],
        ['Komentar', m.replies],
        ['Shares', m.reposts],
        ['Simpan', m.saves],
        ['Akun engaged', m.accounts_engaged],
      ]
    : [
        ['Pengikut', followers],
        ['Views', m.views],
        ['Likes', m.likes],
        ['Balasan', m.replies],
        ['Repost', m.reposts],
      ];
  document.getElementById('pace-list').innerHTML = items.map(([k, v]) => `
    <div class="ov-pace-row">
      <span class="text-muted">${k}</span>
      <span class="mono font-semibold">${v == null || v === '' ? '—' : fmt.full(v)}</span>
    </div>`).join('');
  const link = document.getElementById('pulse-profile-link');
  if (link) link.href = ig ? '/app/ig-profil' : '/app/profil';
}

function sortValue(p) {
  const pm = postMetrics(p);
  if (state.sort === 'likes') return pm.likes;
  if (state.sort === 'eng') return pm.eng;
  if (state.sort === 'er') return pm.er;
  return pm.views;
}

function setSort(sort) {
  if (!SORT_LABEL[sort]) return;
  state.sort = sort;
  document.querySelectorAll('#sort-toggle .ov-mt-btn').forEach((b) => {
    b.classList.toggle('is-on', b.dataset.sort === sort);
  });
  document.querySelectorAll('#top-posts-head .ov-td[data-sort]').forEach((el) => {
    el.classList.toggle('is-sort', el.dataset.sort === sort);
  });
  renderPosts();
}

function renderPosts() {
  const rows = document.getElementById('posts-rows');
  const hint = document.getElementById('top-posts-hint');
  const initials = String(state.me?.username || 'TH').replace(/^@/, '').slice(0, 2).toUpperCase();
  const days = state.range === '7D' ? 7 : state.range === '90D' ? 90 : 30;
  const n = state.posts.length;
  const sorted = [...state.posts].sort((a, b) => sortValue(b) - sortValue(a)).slice(0, 8);
  if (hint) {
    hint.textContent = n
      ? `${sorted.length} teratas dari ${n} post sampel (${days} hari), diurutkan ${SORT_LABEL[state.sort]}.`
      : `Sampel post ${days} hari — pilih urutan Views, Likes, Interaksi, atau ER.`;
  }
  if (!sorted.length) {
    const waiting = state.loading === 'account' || state.loading === 'posts';
    rows.innerHTML = waiting
      ? ''
      : `<div class="ov-tr"><div class="ov-td text-muted" style="flex:1;padding:12px 0">Belum ada post Repliz di rentang ini.</div></div>`;
    return;
  }
  const mark = (key) => (state.sort === key ? ' is-sort' : '');
  rows.innerHTML = sorted.map((p, i) => {
    const pm = postMetrics(p);
    const text = String(p.text || '(tanpa teks)').replace(/\s+/g, ' ').trim();
    const erClass = pm.er >= 5 ? 'hi' : pm.er >= 3 ? 'md' : 'lo';
    const href = p.permalink || '/app/posts';
    return `<div class="ov-tr">
      <div class="ov-td ov-rank" style="width:28px">${i + 1}</div>
      <div class="ov-td" style="flex:1">
        <div class="ov-post-wrap">
          <div class="ov-avatar">${Threads.escapeHtml(initials)}</div>
          <div class="ov-post-line">${Threads.escapeHtml(text)}</div>
        </div>
      </div>
      <div class="ov-td mono text-muted" style="width:72px">${Threads.escapeHtml(relativeTime(p.timestamp))}</div>
      <div class="ov-td ov-td-num${mark('views')}" style="width:88px">${fmt.num(pm.views)}</div>
      <div class="ov-td ov-td-num${mark('likes')}" style="width:88px">${fmt.num(pm.likes)}</div>
      <div class="ov-td ov-td-num${mark('eng')}" style="width:88px">${fmt.num(pm.eng)}</div>
      <div class="ov-td ov-td-num${mark('er')}" style="width:72px"><span class="ov-er ${erClass}">${pm.er.toFixed(1)}%</span></div>
      <div class="ov-td" style="width:36px">
        <a class="th-btn th-btn-ghost !p-1 text-xs" href="${Threads.escapeHtml(href)}" target="_blank" rel="noopener" title="Buka">→</a>
      </div>
    </div>`;
  }).join('');
}

function updateTitle() {
  const name = state.me?.username
    ? String(state.me.username).replace(/^@/, '')
    : null;
  const label = state.range === '7D' ? '7 hari' : state.range === '90D' ? '90 hari' : '30 hari';
  document.getElementById('top-title').textContent = name
    ? `Halo ${name} — ${label} terakhir`
    : 'Ringkasan';
}

async function load() {
  setLoadState('account', 'Memuat metrik akun…');
  renderKPISkeleton();
  renderChart();
  renderPosts();
  try {
    const accs = await Threads.api('/api/repliz/accounts');
    const list = accs.accounts || [];
    const activeId = accs.active_id || '';
    const active = list.find((a) => (a.id || a._id) === activeId) || list[0];
    if (!active) {
      document.getElementById('top-title').textContent = 'Hubungkan Repliz';
      document.getElementById('pulse-meta').textContent = 'Tidak ada akun Repliz';
      setLoadState('idle');
      document.getElementById('load-chip')?.classList.remove('show');
      renderKPIs();
      renderChart();
      renderPosts();
      return;
    }
    state.me = {
      username: active.username || active.name,
      name: active.name || active.username,
      id: active.id || active._id,
      type: active.type,
    };
    updateTitle();
    const range = rangeQuery(state.range);
    const q = range + '&account_id=' + encodeURIComponent(state.me.id);

    const fast = await Threads.api('/api/insights?aggregate=1&posts=0&' + q).catch(() => null);
    state.insights = fast;
    state.posts = [];
    renderKPIs();
    renderPulse();
    setLoadState('posts', 'Memuat tren & post…');
    renderChart();
    renderPosts();

    const full = await Threads.api('/api/insights?aggregate=1&posts=40&' + q).catch(() => null);
    if (full) {
      state.insights = full;
      state.posts = Array.isArray(full.posts) ? full.posts : [];
      if (full.repliz_account?.username) {
        state.me = {
          username: full.repliz_account.username,
          name: full.repliz_account.name || full.repliz_account.username,
          id: full.repliz_account.id || state.me.id,
          type: full.repliz_account.type || state.me.type,
        };
        updateTitle();
      }
      renderKPIs();
      renderPulse();
      renderChart();
      renderPosts();
    }
    setLoadState('done');
  } catch (e) {
    setLoadState('idle');
    document.getElementById('load-chip')?.classList.remove('show');
    document.getElementById('chart-load')?.classList.remove('show');
    document.getElementById('posts-load')?.classList.remove('show');
    document.getElementById('top-title').textContent = 'Server offline';
    Threads.toast(e.message || 'Gagal muat ringkasan', false);
  }
}

document.getElementById('range-seg')?.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-range]');
  if (!btn) return;
  state.range = btn.dataset.range;
  document.querySelectorAll('#range-seg .ov-seg-btn').forEach((b) => b.classList.toggle('is-on', b === btn));
  updateTitle();
  load();
});

document.getElementById('metric-toggle')?.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-metric]');
  if (!btn) return;
  state.metric = btn.dataset.metric;
  document.querySelectorAll('#metric-toggle .ov-mt-btn').forEach((b) => b.classList.toggle('is-on', b === btn));
  renderChart();
});

document.getElementById('sort-toggle')?.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-sort]');
  if (!btn) return;
  setSort(btn.dataset.sort);
});

document.getElementById('top-posts-head')?.addEventListener('click', (e) => {
  const col = e.target.closest('[data-sort]');
  if (!col) return;
  setSort(col.dataset.sort);
});

document.getElementById('btn-refresh')?.addEventListener('click', () => load());

load();
