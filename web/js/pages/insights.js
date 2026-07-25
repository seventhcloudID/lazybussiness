Threads.pageShell('insights');

let lastPayload = null;

const MIX_PARTS = [
  { key: 'likes', label: 'Likes' },
  { key: 'replies', label: 'Balasan' },
  { key: 'reposts', label: 'Repost' },
  { key: 'quotes', label: 'Kutipan' },
];

function showAlert(msg) {
  const el = document.getElementById('insights-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function metricMap(data) {
  if (data?.metrics && typeof data.metrics === 'object') return { ...data.metrics };
  const map = {};
  (data?.data || []).forEach(m => {
    let val = m.total_value?.value ?? m.value;
    if (val == null && Array.isArray(m.values)) {
      val = m.values.length === 1
        ? m.values[0]?.value
        : m.values.reduce((s, v) => s + (Number(v?.value) || 0), 0);
    }
    map[m.name] = Number(val) || 0;
  });
  return map;
}

function renderMix(map) {
  const parts = MIX_PARTS.map(p => ({ ...p, v: Number(map[p.key]) || 0 }));
  const total = parts.reduce((s, p) => s + p.v, 0);
  const bar = document.getElementById('mix-bar');
  const legend = document.getElementById('mix-legend');
  const totalLabel = document.getElementById('mix-total-label');

  if (totalLabel) {
    totalLabel.textContent = total ? `${Threads.fmtNum(total)} interaksi` : 'Belum ada interaksi';
  }

  if (!total) {
    bar.innerHTML = '';
    legend.innerHTML = '<li class="text-muted font-medium">Belum ada interaksi di rentang ini</li>';
    return;
  }

  bar.innerHTML = parts
    .filter(p => p.v > 0)
    .map((p, i) => {
      const pct = Math.max(2, (p.v / total) * 100);
      return `<span data-tone="${p.key}" style="width:${pct}%;animation-delay:${i * 0.06}s" title="${p.label}: ${Threads.fmtNum(p.v)}"></span>`;
    })
    .join('');

  legend.innerHTML = parts
    .filter(p => p.v > 0)
    .map(p => {
      const pct = ((p.v / total) * 100).toFixed(0);
      return `<li><i data-tone="${p.key}"></i>${p.label}<em>${pct}% · ${Threads.fmtNum(p.v)}</em></li>`;
    })
    .join('');
}

function truncate(text, n = 110) {
  const t = (text || '').trim().replace(/\s+/g, ' ');
  if (!t) return 'Tanpa teks';
  return t.length > n ? t.slice(0, n) + '…' : t;
}

function sortValue(p, sortKey) {
  if (sortKey === 'score') return p.score || 0;
  return Number(p.metrics?.[sortKey]) || 0;
}

function renderPosts(posts, sortKey) {
  const root = document.getElementById('top-posts');
  const items = [...(posts || [])];
  if (!items.length) {
    root.innerHTML = '<div class="ins-posts-empty">Belum ada data post di rentang ini.</div>';
    return;
  }

  items.sort((a, b) => sortValue(b, sortKey) - sortValue(a, sortKey));
  const top = items.slice(0, 10);
  const maxVal = Math.max(...top.map(p => sortValue(p, sortKey)), 1);

  root.innerHTML = top.map((p, i) => {
    const m = p.metrics || {};
    const eng = (m.likes || 0) + (m.replies || 0) + (m.reposts || 0) + (m.quotes || 0);
    const rate = m.views ? ((eng / m.views) * 100).toFixed(1) + '%' : '—';
    const barPct = Math.max(4, Math.round((sortValue(p, sortKey) / maxVal) * 100));
    const open = p.permalink
      ? `<a class="th-btn th-btn-ghost !py-1 !px-2 text-xs" href="${Threads.escapeHtml(p.permalink)}" target="_blank" rel="noopener">Buka</a>`
      : '';
    return `
      <article class="ins-post">
        <div class="ins-rank">${i + 1}</div>
        <div class="ins-post-body">
          <p class="ins-post-text">${Threads.escapeHtml(truncate(p.text))}</p>
          <p class="ins-post-meta">${Threads.escapeHtml(p.media_type || 'TEXT')} · ${Threads.fmtDate(p.timestamp)} · ER ${rate}</p>
          <div class="ins-post-bar" title="Relatif ke #1"><i style="width:${barPct}%;animation-delay:${i * 0.04}s"></i></div>
        </div>
        <div class="ins-post-stats">
          <span class="ins-chip"><i class="bi bi-eye"></i>${Threads.fmtNum(m.views)}</span>
          <span class="ins-chip"><i class="bi bi-heart"></i>${Threads.fmtNum(m.likes)}</span>
          <span class="ins-chip"><i class="bi bi-chat"></i>${Threads.fmtNum(m.replies)}</span>
        </div>
        <div class="ins-post-actions">${open}</div>
      </article>`;
  }).join('');
}

function applyPayload(data) {
  lastPayload = data;
  const map = metricMap(data);
  Threads.applyInsights(
    { data: Object.entries(map).map(([name, value]) => ({ name, total_value: { value } })) },
    document.getElementById('insights-stats')
  );
  renderMix(map);

  const engRate = Number(data.engagement_rate) || 0;
  const eng = Number(data.engagement) || ((map.likes || 0) + (map.replies || 0) + (map.reposts || 0) + (map.quotes || 0));
  document.getElementById('hero-eng-rate').textContent = engRate ? engRate.toFixed(2) + '%' : '—';
  document.getElementById('hero-eng').textContent = Threads.fmtNum(eng);
  document.getElementById('hero-eng-hint').textContent = map.views
    ? `Dari ${Threads.fmtNum(map.views)} views di rentang ini`
    : 'Belum ada views di rentang terpilih';
  document.getElementById('hero-posts-hint').textContent = data.post_count
    ? `${data.post_count} post dianalisis`
    : 'Belum ada post';

  const src = document.getElementById('insights-source');
  if (data.source === 'account') src.textContent = 'Sumber: metrik akun';
  else if (data.source === 'posts_aggregate') src.textContent = 'Sumber: agregasi post';
  else src.textContent = 'Sumber: —';

  const rangeEl = document.getElementById('insights-range');
  if (rangeEl) {
    const fmt = unix => {
      const n = Number(unix);
      if (!n) return '—';
      return new Date(n * 1000).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
    };
    if (data.since || data.until) {
      rangeEl.textContent = `Rentang: ${fmt(data.since)} – ${fmt(data.until)}`;
    } else {
      rangeEl.textContent = 'Rentang: —';
    }
  }

  renderPosts(data.posts || [], document.getElementById('sort-posts').value);
}

const INSIGHTS_EARLIEST = '2024-04-13';

function setDateRange(days) {
  const until = new Date();
  const since = new Date();
  const untilEl = document.getElementById('insights-until');
  const sinceEl = document.getElementById('insights-since');
  const fmt = d => {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  };
  if (!days) {
    sinceEl.value = INSIGHTS_EARLIEST;
    untilEl.value = fmt(until);
    return;
  }
  since.setDate(until.getDate() - days);
  const earliest = new Date(INSIGHTS_EARLIEST + 'T00:00:00');
  if (since < earliest) since.setTime(earliest.getTime());
  sinceEl.value = fmt(since);
  untilEl.value = fmt(until);
}

async function loadInsights() {
  showAlert('');
  const body = document.getElementById('insights-body');
  body.style.opacity = '0.55';
  const since = document.getElementById('insights-since').value;
  const until = document.getElementById('insights-until').value;
  const q = new URLSearchParams();
  const nowSec = Math.floor(Date.now() / 1000);
  if (since) {
    let s = Math.floor(new Date(since + 'T00:00:00').getTime() / 1000);
    if (s < 1712991600) s = 1712991600;
    q.set('since', String(s));
  }
  if (until) {
    let u = Math.floor(new Date(until + 'T23:59:59').getTime() / 1000);
    if (u > nowSec) u = nowSec;
    q.set('until', String(u));
  }
  try {
    const data = await Threads.api('/api/insights?aggregate=1&' + q.toString());
    applyPayload(data);
    if (data?.warning) showAlert(data.warning);
    else if (!(data?.data || []).length && !(data?.posts || []).length) {
      showAlert('Tidak ada data insight untuk rentang ini.');
    }
  } finally {
    body.style.opacity = '1';
  }
}

document.getElementById('btn-insights').onclick = () =>
  loadInsights().catch(e => {
    showAlert(e.message);
    Threads.toast(e.message, false);
  });

document.getElementById('sort-posts').onchange = () => {
  if (lastPayload) renderPosts(lastPayload.posts || [], document.getElementById('sort-posts').value);
};

document.getElementById('insights-presets').addEventListener('click', e => {
  const btn = e.target.closest('button[data-days]');
  if (!btn) return;
  document.querySelectorAll('#insights-presets button').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  setDateRange(Number(btn.dataset.days));
  loadInsights().catch(err => {
    showAlert(err.message);
    Threads.toast(err.message, false);
  });
});

(async () => {
  if (!(await Threads.requireConnected())) return;
  setDateRange(30);
  try {
    await loadInsights();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  }
})();
