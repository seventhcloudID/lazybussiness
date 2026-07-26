Threads.pageShell('lazy-track');

let report = null;
let filter = 'all';

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

function statusLabel(st) {
  return ({
    done: ['Selesai', 'ok'],
    failed: ['Gagal', 'bad'],
    skipped_ig: ['IG skip', 'warn'],
  })[st] || [st || '—', ''];
}

function fmtWhen(iso) {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleString('id-ID', {
      day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
    });
  } catch {
    return iso;
  }
}

function channelPills(ch) {
  const bits = [];
  if (ch?.threads) bits.push('<span class="ltrk-ch is-on">Threads</span>');
  if (ch?.ig) bits.push('<span class="ltrk-ch is-on">IG</span>');
  if (ch?.x) bits.push('<span class="ltrk-ch is-on">X</span>');
  if (ch?.tiktok) bits.push('<span class="ltrk-ch is-on">TikTok</span>');
  if (!bits.length) bits.push('<span class="ltrk-ch">—</span>');
  return bits.join('');
}

function metricChips(m) {
  if (!m) return '<span class="text-xs text-muted">Metrik Threads belum tersedia</span>';
  const views = Number(m.views || 0);
  const likes = Number(m.likes || 0);
  const replies = Number(m.replies || 0);
  const reposts = Number(m.reposts || 0);
  return `
    <span class="ltrk-metric" title="Views"><i class="bi bi-eye"></i> ${fmt.num(views)}</span>
    <span class="ltrk-metric" title="Likes"><i class="bi bi-heart"></i> ${fmt.num(likes)}</span>
    <span class="ltrk-metric" title="Replies"><i class="bi bi-chat"></i> ${fmt.num(replies)}</span>
    <span class="ltrk-metric" title="Reposts"><i class="bi bi-repeat"></i> ${fmt.num(reposts)}</span>`;
}

function filteredJobs() {
  const jobs = report?.jobs || [];
  if (filter === 'all') return jobs;
  return jobs.filter(j => j.status === filter);
}

function renderKPIs() {
  const s = report?.summary || {};
  document.getElementById('k-total').textContent = fmt.full(s.total || 0);
  document.getElementById('k-done').textContent = fmt.full(s.done || 0);
  document.getElementById('k-views').textContent = fmt.num(s.views || 0);
  document.getElementById('k-likes').textContent = fmt.num(s.likes || 0);
  document.getElementById('k-eng').textContent = fmt.num(s.engagement || 0);
  document.getElementById('k-ch').innerHTML =
    `<span title="Threads">T ${s.threads || 0}</span>` +
    `<span title="IG"> · IG ${s.ig || 0}</span>` +
    `<span title="X"> · X ${s.x || 0}</span>` +
    `<span title="TikTok"> · TT ${s.tiktok || 0}</span>`;
  document.getElementById('ltrk-range').textContent =
    `Periode ${report?.from || '—'} → ${report?.to || '—'} · TZ ${report?.timezone || '—'}`;
}

function renderList() {
  const jobs = filteredJobs();
  document.getElementById('ltrk-count').textContent = `${jobs.length} post`;
  const root = document.getElementById('ltrk-list');
  if (!jobs.length) {
    root.innerHTML = '<p class="lazy-empty">Belum ada hasil Lazy di filter ini. Nyalakan otomasi atau jalankan Run 1.</p>';
    return;
  }
  root.innerHTML = jobs.map(j => {
    const [label, cls] = statusLabel(j.status);
    const title = j.title || j.id;
    const thumb = j.thumb_url
      ? `<img class="ltrk-thumb" src="${Threads.escapeHtml(j.thumb_url)}" alt="">`
      : (j.image_urls?.[0]
        ? `<img class="ltrk-thumb" src="${Threads.escapeHtml(j.image_urls[0])}" alt="">`
        : `<span class="ltrk-thumb is-empty"><i class="bi bi-image"></i></span>`);
    const err = j.error
      ? `<div class="ltrk-err">${Threads.escapeHtml(j.error)}</div>`
      : '';
    const bufErr = [
      j.buffer_x_error ? `X: ${j.buffer_x_error}` : '',
      j.buffer_error ? `TikTok: ${j.buffer_error}` : '',
    ].filter(Boolean).join(' · ');
    return `
      <article class="ltrk-row">
        ${thumb}
        <div class="ltrk-body">
          <div class="ltrk-top">
            <div class="min-w-0">
              <strong class="ltrk-title">${Threads.escapeHtml(title)}</strong>
              <div class="ltrk-sub">
                <span>${Threads.escapeHtml(j.date || '')}</span>
                <span>·</span>
                <span>${fmtWhen(j.finished_at || j.scheduled_at)}</span>
              </div>
            </div>
            <span class="lazy-badge ${cls}">${Threads.escapeHtml(label)}</span>
          </div>
          ${j.snippet ? `<p class="ltrk-snip">${Threads.escapeHtml(j.snippet)}</p>` : ''}
          <div class="ltrk-foot">
            <div class="ltrk-chs">${channelPills(j.channels)}</div>
            <div class="ltrk-metrics">${metricChips(j.metrics)}</div>
          </div>
          ${err}
          ${bufErr ? `<div class="ltrk-err">${Threads.escapeHtml(bufErr)}</div>` : ''}
        </div>
      </article>`;
  }).join('');
}

async function load() {
  showAlert('');
  try {
    report = await Threads.api('/api/lazy/track?metrics=1');
    renderKPIs();
    renderList();
  } catch (e) {
    showAlert(e.message);
    document.getElementById('ltrk-list').innerHTML =
      `<p class="lazy-empty">${Threads.escapeHtml(e.message)}</p>`;
  }
}

document.querySelectorAll('.ltrk-filter').forEach(btn => {
  btn.onclick = () => {
    filter = btn.dataset.filter;
    document.querySelectorAll('.ltrk-filter').forEach(b => b.classList.toggle('is-on', b === btn));
    renderList();
  };
});

document.getElementById('btn-refresh').onclick = () => load();

load();
