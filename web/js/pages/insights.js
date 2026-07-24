Threads.pageShell('insights');

let lastPayload = null;

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
  const parts = [
    { key: 'likes', label: 'Likes' },
    { key: 'replies', label: 'Balasan' },
    { key: 'reposts', label: 'Repost' },
    { key: 'quotes', label: 'Kutipan' },
  ];
  const total = parts.reduce((s, p) => s + (Number(map[p.key]) || 0), 0);
  const legend = document.getElementById('mix-legend');
  if (!total) {
    legend.textContent = 'Belum ada interaksi';
    return;
  }
  legend.innerHTML = parts
    .map(p => {
      const v = Number(map[p.key]) || 0;
      if (!v) return null;
      const pct = ((v / total) * 100).toFixed(0);
      return `${p.label} ${pct}%`;
    })
    .filter(Boolean)
    .join(' · ');
}

function truncate(text, n = 90) {
  const t = (text || '').trim().replace(/\s+/g, ' ');
  if (!t) return 'Tanpa teks';
  return t.length > n ? t.slice(0, n) + '…' : t;
}

function renderPosts(posts, sortKey) {
  const tbody = document.getElementById('top-posts');
  const items = [...(posts || [])];
  if (!items.length) {
    tbody.innerHTML = '<tr><td colspan="7" class="text-center text-muted py-10">Belum ada data post.</td></tr>';
    return;
  }

  items.sort((a, b) => {
    const av = sortKey === 'score' ? (a.score || 0) : (Number(a.metrics?.[sortKey]) || 0);
    const bv = sortKey === 'score' ? (b.score || 0) : (Number(b.metrics?.[sortKey]) || 0);
    return bv - av;
  });

  tbody.innerHTML = items.slice(0, 10).map((p, i) => {
    const m = p.metrics || {};
    const eng = (m.likes || 0) + (m.replies || 0) + (m.reposts || 0) + (m.quotes || 0);
    const rate = m.views ? ((eng / m.views) * 100).toFixed(1) + '%' : '—';
    return `<tr>
      <td class="text-muted">${i + 1}</td>
      <td class="max-w-[320px]">
        <div class="text-sm leading-snug">${Threads.escapeHtml(truncate(p.text))}</div>
        <div class="text-[11px] text-muted mt-1">${Threads.escapeHtml(p.media_type || 'TEXT')} · ${Threads.fmtDate(p.timestamp)}</div>
      </td>
      <td>${Threads.fmtNum(m.views)}</td>
      <td>${Threads.fmtNum(m.likes)}</td>
      <td>${Threads.fmtNum(m.replies)}</td>
      <td>${rate}</td>
      <td class="text-right whitespace-nowrap">
        ${p.permalink ? `<a class="th-btn th-btn-ghost !py-1 !px-2 text-xs" href="${Threads.escapeHtml(p.permalink)}" target="_blank" rel="noopener">Buka</a>` : ''}
      </td>
    </tr>`;
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
    ? `ER ${engRate ? engRate.toFixed(2) : '—'}% dari ${Threads.fmtNum(map.views)} views`
    : 'Engagement rate: —';
  document.getElementById('hero-posts-hint').textContent = data.post_count
    ? `${data.post_count} post dianalisis`
    : '—';

  const src = document.getElementById('insights-source');
  if (data.source === 'account') src.textContent = 'Sumber: metrik akun';
  else if (data.source === 'posts_aggregate') src.textContent = 'Sumber: agregasi post';
  else src.textContent = 'Sumber: —';

  renderPosts(data.posts || [], document.getElementById('sort-posts').value);
}

function setDateRange(days) {
  const until = new Date();
  const since = new Date();
  const untilEl = document.getElementById('insights-until');
  const sinceEl = document.getElementById('insights-since');
  if (!days) {
    sinceEl.value = '';
    untilEl.value = '';
    return;
  }
  since.setDate(until.getDate() - days);
  const fmt = d => d.toISOString().slice(0, 10);
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
  if (since) q.set('since', Math.floor(new Date(since).getTime() / 1000));
  if (until) q.set('until', Math.floor(new Date(until + 'T23:59:59').getTime() / 1000));
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
