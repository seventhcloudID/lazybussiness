Threads.pageShell('lazy-track');

let report = null;
let filter = 'all';
let metric = 'views';

const fmt = {
  num(n) {
    n = Number(n) || 0;
    if (n >= 1e6) return (n / 1e6).toFixed(1).replace(/\.0$/, '') + 'jt';
    if (n >= 1e3) return (n / 1e3).toFixed(1).replace(/\.0$/, '') + 'rb';
    return String(Math.round(n));
  },
  full(n) {
    return (Number(n) || 0).toLocaleString('id-ID');
  },
};

function showAlert(msg) {
  const el = document.getElementById('ltrk-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function jobMetrics(j) {
  const m = j?.metrics || {};
  const views = Number(m.views || 0);
  const likes = Number(m.likes || 0);
  const replies = Number(m.replies || 0);
  const reposts = Number(m.reposts || 0);
  const quotes = Number(m.quotes || 0);
  const engagement = likes + replies + reposts + quotes;
  const er = views > 0 ? (engagement / views) * 100 : 0;
  return { views, likes, replies, reposts, quotes, engagement, er };
}

/** ok | deleted | unknown — 0 views = dihapus; tanpa metrics = belum diukur (jangan tampilkan 0). */
function metricState(j) {
  if (j?.deleted) return 'deleted';
  if (!j?.metrics) return 'unknown';
  if (Number(j.metrics.views || 0) <= 0) return 'deleted';
  return 'ok';
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

function filteredJobs() {
  const jobs = report?.jobs || [];
  if (filter === 'all') return jobs;
  return jobs.filter(j => j.status === filter);
}

/** Post dengan metrik valid (bukan 0 views / dihapus). */
function measuredJobs() {
  return filteredJobs().filter(j => metricState(j) === 'ok');
}

function seriesFor(key) {
  // oldest → newest for spark/chart — skip post 0 views (dihapus)
  return [...measuredJobs()].reverse().map(j => jobMetrics(j)[key] || 0);
}

function statusLabel(st) {
  return ({
    done: ['Selesai', 'ok'],
    failed: ['Gagal', 'bad'],
    skipped_ig: ['IG skip', 'warn'],
  })[st] || [st || '—', ''];
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

function channelIcons(ch) {
  const bits = [];
  if (ch?.threads) bits.push('<i class="bi bi-at ltrk-ch-ico" title="Threads"></i>');
  if (ch?.ig) bits.push('<i class="bi bi-instagram ltrk-ch-ico is-ig" title="Instagram"></i>');
  if (ch?.x) bits.push('<i class="bi bi-twitter-x ltrk-ch-ico is-x" title="X"></i>');
  if (ch?.tiktok) bits.push('<i class="bi bi-tiktok ltrk-ch-ico is-tt" title="TikTok"></i>');
  if (!bits.length) return '<span class="text-muted">—</span>';
  return `<span class="ltrk-ch-icons">${bits.join('')}</span>`;
}

function renderKPIs() {
  const s = report?.summary || {};
  const measured = measuredJobs();
  const n = measured.length || s.measured || 0;
  const views = seriesFor('views');
  const likes = seriesFor('likes');
  const replies = seriesFor('replies');
  const eng = seriesFor('engagement');
  const erSeries = seriesFor('er');
  const avgViews = n ? (views.reduce((a, b) => a + b, 0) / Math.max(views.length, 1)) : 0;
  const er = (s.views || 0) > 0 ? ((s.engagement || 0) / s.views) * 100 : 0;
  const success = (s.total || 0) > 0 ? ((s.done || 0) / s.total) * 100 : 0;
  const deletedNote = (s.deleted || 0) > 0 ? ` · ${s.deleted} dihapus` : '';

  const kpis = [
    { k: 'Post Lazy', v: fmt.full(s.total || 0), sub: `${s.done || 0} selesai${deletedNote}`, spark: views, lead: true },
    { k: 'Views', v: fmt.full(s.views || 0), sub: `avg ${fmt.num(avgViews)} / post aktif`, spark: views },
    { k: 'Likes', v: fmt.full(s.likes || 0), sub: `${fmt.num(s.replies || 0)} balasan`, spark: likes },
    { k: 'Engagement', v: fmt.full(s.engagement || 0), sub: 'exclude post 0 views', spark: eng },
    { k: 'ER Lazy', v: er.toFixed(2) + '%', sub: 'dari views Threads aktif', spark: erSeries },
    { k: 'Success rate', v: success.toFixed(0) + '%', sub: `${s.failed || 0} gagal · ${s.skipped_ig || 0} IG skip`, spark: eng },
  ];

  const row = document.getElementById('kpi-row');
  row.innerHTML = kpis.map(k => {
    const sp = sparkPath(k.spark?.length > 1 ? k.spark : null);
    return `<article class="ov-kpi ${k.lead ? 'ov-kpi-lead' : ''}">
      <div class="ov-kpi-k">${Threads.escapeHtml(k.k)}</div>
      <div class="ov-kpi-v mono">${Threads.escapeHtml(String(k.v))}</div>
      <div class="ov-kpi-sub">${Threads.escapeHtml(k.sub)}</div>
      ${sp ? `<svg class="ov-spark" viewBox="0 0 ${sp.w} ${sp.h}" preserveAspectRatio="none">
        <path d="${sp.area}" fill="var(--accent-soft)"></path>
        <path d="${sp.line}" stroke="var(--accent)" stroke-width="1.5" fill="none"></path>
      </svg>` : ''}
    </article>`;
  }).join('');
  row.removeAttribute('aria-busy');
}

function renderChart() {
  const jobsAsc = [...measuredJobs()].reverse();
  const series = jobsAsc.map(j => jobMetrics(j)[metric] || 0);
  const svg = document.getElementById('chart-svg');
  const tip = document.getElementById('chart-tip');
  const labels = { views: 'views', likes: 'likes', replies: 'balasan', engagement: 'engagement' };
  document.getElementById('chart-title').textContent =
    `${labels[metric] || metric} per post Lazy (terbaru)`;
  document.getElementById('chart-sample').textContent =
    `${jobsAsc.length} post dalam filter`;

  if (series.length < 2) {
    svg.innerHTML = `<text x="440" y="160" text-anchor="middle" fill="var(--muted)" font-size="13">Belum cukup data metrik Lazy</text>`;
    document.getElementById('stat-peak').textContent = series.length ? fmt.full(series[0]) : '—';
    document.getElementById('stat-avg').textContent = series.length ? fmt.full(series[0]) : '—';
    document.getElementById('stat-low').textContent = series.length ? fmt.full(series[0]) : '—';
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
    const title = (jobsAsc[idx]?.title || jobsAsc[idx]?.id || '').slice(0, 40);
    tip.innerHTML = `<div class="ov-chart-tip-date">${Threads.escapeHtml(title || 'Post #' + (idx + 1))}</div>
      ${labels[metric]}: <strong class="mono">${fmt.full(series[idx])}</strong>`;
  };
  svg.onmouseleave = () => {
    tip.hidden = true;
    const g = document.getElementById('chart-hover');
    if (g) g.innerHTML = '';
  };
}

function renderPulse() {
  const s = report?.summary || {};
  document.getElementById('pulse-range').textContent =
    `${report?.from || '—'} → ${report?.to || '—'}`;
  document.getElementById('pulse-meta').textContent =
    `TZ ${report?.timezone || '—'} · ${s.total || 0} hasil Lazy`;

  const items = [
    ['Threads', s.threads],
    ['Instagram', s.ig],
    ['Buffer X', s.x],
    ['TikTok', s.tiktok],
  ];
  document.getElementById('pace-list').innerHTML = items.map(([k, v]) => `
    <div class="ov-pace-row">
      <span class="text-muted">${k}</span>
      <span class="mono font-semibold">${fmt.full(v || 0)}</span>
    </div>`).join('');

  const total = Math.max(s.total || 1, 1);
  const bars = [
    ['Threads', s.threads || 0, '#2563eb'],
    ['IG', s.ig || 0, '#db2777'],
    ['X', s.x || 0, '#0f172a'],
    ['TikTok', s.tiktok || 0, '#0d9488'],
  ];
  document.getElementById('ch-bars').innerHTML = bars.map(([k, v, c]) => {
    const pct = Math.min(100, Math.round((v / total) * 100));
    return `<div class="ltrk-bar">
      <div class="ltrk-bar-meta"><span>${k}</span><span class="mono">${v}</span></div>
      <div class="ltrk-bar-track"><div class="ltrk-bar-fill" style="width:${pct}%;background:${c}"></div></div>
    </div>`;
  }).join('');
}

function renderPosts() {
  const rows = document.getElementById('posts-rows');
  const jobs = filteredJobs();
  document.getElementById('ltrk-count').textContent = `${jobs.length} post`;
  if (!jobs.length) {
    rows.innerHTML = `<div class="ov-tr"><div class="ov-td text-muted" style="flex:1;padding:12px 0">Belum ada hasil Lazy di filter ini.</div></div>`;
    return;
  }
  rows.innerHTML = jobs.map(j => {
    const pm = jobMetrics(j);
    const state = metricState(j);
    const deleted = state === 'deleted';
    const showNums = state === 'ok';
    const [label, cls] = deleted ? ['Dihapus', 'warn'] : statusLabel(j.status);
    const text = String(j.snippet || j.title || j.id || '(tanpa teks)').replace(/\s+/g, ' ').trim();
    const erClass = pm.er >= 5 ? 'hi' : pm.er >= 3 ? 'md' : 'lo';
    const thumb = j.thumb_url || j.image_urls?.[0] || '';
    const dash = '—';
    return `<div class="ov-tr${deleted ? ' ltrk-row-deleted' : ''}">
      <div class="ov-td" style="flex:1">
        <div class="ov-post-wrap">
          ${thumb
            ? `<img class="ltrk-table-thumb" src="${Threads.escapeHtml(thumb)}" alt="">`
            : `<span class="ov-avatar">${Threads.escapeHtml((j.title || 'LB').slice(0, 2).toUpperCase())}</span>`}
          <div class="min-w-0">
            <div class="ov-post-text">${Threads.escapeHtml(text)}</div>
            <div class="text-[11px] text-muted mt-0.5">${Threads.escapeHtml(j.date || '')} · ${relativeTime(j.finished_at || j.scheduled_at)}${deleted ? ' · dihapus' : ''}</div>
          </div>
        </div>
      </div>
      <div class="ov-td" style="width:88px"><span class="lazy-badge ${cls}">${Threads.escapeHtml(label)}</span></div>
      <div class="ov-td" style="width:120px">${channelIcons(j.channels)}</div>
      <div class="ov-td ov-td-num mono" style="width:72px">${showNums ? fmt.num(pm.views) : dash}</div>
      <div class="ov-td ov-td-num mono" style="width:64px">${showNums ? fmt.num(pm.likes) : dash}</div>
      <div class="ov-td ov-td-num mono" style="width:64px">${showNums ? fmt.num(pm.replies) : dash}</div>
      <div class="ov-td ov-td-num" style="width:56px">${showNums ? `<span class="ov-er ${erClass}">${pm.er.toFixed(1)}%</span>` : dash}</div>
    </div>`;
  }).join('');
}

function renderAll() {
  renderKPIs();
  renderChart();
  renderPulse();
  renderPosts();
}

async function load() {
  showAlert('');
  document.getElementById('kpi-row').setAttribute('aria-busy', 'true');
  try {
    report = await Threads.api('/api/lazy/track?metrics=1');
    renderAll();
  } catch (e) {
    showAlert(e.message);
    document.getElementById('kpi-row').innerHTML = '';
    document.getElementById('posts-rows').innerHTML =
      `<div class="ov-tr"><div class="ov-td text-muted" style="flex:1;padding:12px 0">${Threads.escapeHtml(e.message)}</div></div>`;
  }
}

document.querySelectorAll('#filter-seg [data-filter]').forEach(btn => {
  btn.onclick = () => {
    filter = btn.dataset.filter;
    document.querySelectorAll('#filter-seg [data-filter]').forEach(b => b.classList.toggle('is-on', b === btn));
    renderAll();
  };
});

document.querySelectorAll('#metric-toggle [data-metric]').forEach(btn => {
  btn.onclick = () => {
    metric = btn.dataset.metric;
    document.querySelectorAll('#metric-toggle [data-metric]').forEach(b => b.classList.toggle('is-on', b === btn));
    renderChart();
  };
});

document.getElementById('btn-refresh').onclick = () => load();
document.getElementById('btn-refresh-foot').onclick = () => load();

load();
