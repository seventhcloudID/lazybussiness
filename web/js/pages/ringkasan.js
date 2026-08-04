Threads.pageShell('ringkasan');

let state = {
  range: '30D',
  metric: 'likes',
  posts: [],
  insights: null,
  me: null,
  loading: 'idle', // idle | account | posts | done
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
  ['views', 'likes', 'replies', 'reposts', 'quotes', 'followers_count'].forEach((k) => {
    if (m[k] == null && t[k] != null) m[k] = t[k];
  });
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

function renderKPIs() {
  const m = metricMap(state.insights);
  const posts = state.posts;
  const seriesLikes = posts.map((p) => postMetrics(p).likes).reverse();
  const seriesViews = posts.map((p) => postMetrics(p).views).reverse();
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

  const kpis = [
    { k: 'Pengikut', v: fmt.full(followers), spark: seriesLikes.length > 1 ? seriesLikes : null, lead: true },
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
  const labels = { likes: 'likes', replies: 'balasan', reposts: 'repost', quotes: 'kutipan' };
  document.getElementById('chart-title').textContent =
    `${labels[state.metric] || state.metric} per post (terbaru)`;

  if (series.length < 2) {
    const waiting = state.loading === 'account' || state.loading === 'posts';
    svg.innerHTML = waiting
      ? ''
      : `<text x="440" y="160" text-anchor="middle" fill="var(--muted)" font-size="13">Belum ada cukup data post</text>`;
    document.getElementById('stat-peak').textContent = '—';
    document.getElementById('stat-avg').textContent = '—';
    document.getElementById('stat-low').textContent = '—';
    return;
  }

  const w = 880, h = 320, padL = 48, padR = 16, padT = 18, padB = 28;
  const innerW = w - padL - padR, innerH = h - padT - padB;
  const min = Math.min(...series), max = Math.max(...series);
  const span = Math.max(max - min, 1);
  const yPad = span * 0.08;
  const yMin = min - yPad, yMax = max + yPad, ySpan = yMax - yMin || 1;
  const xOf = (i) => padL + (i / Math.max(series.length - 1, 1)) * innerW;
  const yOf = (v) => padT + innerH - ((v - yMin) / ySpan) * innerH;
  const path = series.map((v, i) => (i ? 'L' : 'M') + xOf(i).toFixed(1) + ' ' + yOf(v).toFixed(1)).join(' ');
  const area = path + ` L ${xOf(series.length - 1)} ${padT + innerH} L ${xOf(0)} ${padT + innerH} Z`;
  const ticks = 4;
  const yTicks = Array.from({ length: ticks + 1 }, (_, i) => yMin + (i / ticks) * ySpan);
  const step = series.length > 20 ? 4 : 1;
  const xLabels = series.map((_, i) => i).filter((i) => i % step === 0 || i === series.length - 1);

  svg.innerHTML = `
    ${yTicks.map((t) => `
      <g>
        <line x1="${padL}" x2="${w - padR}" y1="${yOf(t)}" y2="${yOf(t)}" stroke="var(--line)" stroke-dasharray="2 3" />
        <text x="${padL - 8}" y="${yOf(t) + 4}" text-anchor="end" font-size="10.5" fill="var(--muted)" font-family="var(--mono)">${fmt.num(t)}</text>
      </g>`).join('')}
    ${xLabels.map((i) => `
      <text x="${xOf(i)}" y="${h - 8}" text-anchor="middle" font-size="10.5" fill="var(--muted)" font-family="var(--mono)">#${i + 1}</text>
    `).join('')}
    <path d="${area}" fill="var(--accent-soft)"></path>
    <path d="${path}" stroke="var(--accent)" stroke-width="2" fill="none"></path>
    <g id="chart-hover"></g>
  `;

  document.getElementById('stat-peak').textContent = fmt.full(Math.max(...series));
  document.getElementById('stat-avg').textContent = fmt.full(series.reduce((a, b) => a + b, 0) / series.length);
  document.getElementById('stat-low').textContent = fmt.full(Math.min(...series));

  svg.onmousemove = (e) => {
    const rect = svg.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * w;
    const ratio = (x - padL) / innerW;
    const idx = Math.max(0, Math.min(series.length - 1, Math.round(ratio * (series.length - 1))));
    const g = document.getElementById('chart-hover');
    g.innerHTML = `
      <line x1="${xOf(idx)}" x2="${xOf(idx)}" y1="${padT}" y2="${padT + innerH}" stroke="var(--ink)" stroke-opacity=".2" stroke-dasharray="2 3" />
      <circle cx="${xOf(idx)}" cy="${yOf(series[idx])}" r="4" fill="var(--accent)" stroke="white" stroke-width="2"/>
    `;
    tip.hidden = false;
    tip.style.left = Math.max(10, Math.min(85, (xOf(idx) / w) * 100)) + '%';
    tip.innerHTML = `<div class="ov-chart-tip-date">Post #${idx + 1}</div>
      ${labels[state.metric]}: <strong class="mono">${fmt.full(series[idx])}</strong>`;
  };
  svg.onmouseleave = () => {
    tip.hidden = true;
    const g = document.getElementById('chart-hover');
    if (g) g.innerHTML = '';
  };
}

function renderPulse() {
  const handle = state.me?.username ? '@' + String(state.me.username).replace(/^@/, '') : 'Belum connect';
  document.getElementById('pulse-handle').textContent = handle;
  const m = metricMap(state.insights);
  document.getElementById('pulse-meta').textContent = state.me?.username
    ? `${fmt.full(m.followers_count || 0)} pengikut · ${state.insights?.post_count || state.posts.length || 0} post sampel`
    : 'Hubungkan token di Settings';

  const items = [
    ['Views', m.views],
    ['Likes', m.likes],
    ['Balasan', m.replies],
    ['Repost', m.reposts],
  ];
  document.getElementById('pace-list').innerHTML = items.map(([k, v]) => `
    <div class="ov-pace-row">
      <span class="text-muted">${k}</span>
      <span class="mono font-semibold">${fmt.full(v || 0)}</span>
    </div>`).join('');
}

function renderPosts() {
  const rows = document.getElementById('posts-rows');
  const initials = String(state.me?.username || 'TH').replace(/^@/, '').slice(0, 2).toUpperCase();
  const sorted = [...state.posts].sort((a, b) => postMetrics(b).er - postMetrics(a).er).slice(0, 8);
  if (!sorted.length) {
    const waiting = state.loading === 'account' || state.loading === 'posts';
    rows.innerHTML = waiting
      ? ''
      : `<div class="ov-tr"><div class="ov-td text-muted" style="flex:1;padding:12px 0">Belum ada post di rentang ini — atau token belum terhubung.</div></div>`;
    return;
  }
  rows.innerHTML = sorted.map((p) => {
    const pm = postMetrics(p);
    const text = String(p.text || '(tanpa teks)').replace(/\s+/g, ' ').trim();
    const erClass = pm.er >= 5 ? 'hi' : pm.er >= 3 ? 'md' : 'lo';
    const href = p.permalink || '/posts.html';
    return `<div class="ov-tr">
      <div class="ov-td" style="flex:1">
        <div class="ov-post-wrap">
          <div class="ov-avatar">${Threads.escapeHtml(initials)}</div>
          <div class="ov-post-line">${Threads.escapeHtml(text)}</div>
        </div>
      </div>
      <div class="ov-td mono text-muted" style="width:72px">${Threads.escapeHtml(relativeTime(p.timestamp))}</div>
      <div class="ov-td ov-td-num" style="width:88px">${fmt.num(pm.likes)}</div>
      <div class="ov-td ov-td-num" style="width:88px">${fmt.num(pm.replies)}</div>
      <div class="ov-td ov-td-num" style="width:88px">${fmt.num(pm.views)}</div>
      <div class="ov-td ov-td-num" style="width:72px"><span class="ov-er ${erClass}">${pm.er.toFixed(1)}%</span></div>
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
    const st = await Threads.api('/api/status');
    if (!st.connected) {
      document.getElementById('top-title').textContent = 'Hubungkan akun Threads';
      document.getElementById('pulse-meta').textContent = 'Buka Settings → Threads Token';
      setLoadState('idle');
      document.getElementById('load-chip')?.classList.remove('show');
      renderKPIs();
      renderChart();
      renderPosts();
      return;
    }
    const range = rangeQuery(state.range);

    // Phase 1: KPI akun dulu (tanpa N insight per-post)
    const [me, fast] = await Promise.all([
      Threads.api('/api/me'),
      Threads.api('/api/insights?aggregate=1&posts=0&' + range).catch(() => null),
    ]);
    state.me = me;
    state.insights = fast;
    state.posts = [];
    updateTitle();
    renderKPIs();
    renderPulse();
    setLoadState('posts', 'Memuat tren & post…');
    renderChart();
    renderPosts();

    // Phase 2: chart + tabel dari sampel post
    const full = await Threads.api('/api/insights?aggregate=1&posts=12&' + range).catch(() => null);
    if (full) {
      state.insights = full;
      state.posts = Array.isArray(full.posts) ? full.posts : [];
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

document.getElementById('btn-refresh')?.addEventListener('click', () => load());

load();
