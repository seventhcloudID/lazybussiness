Threads.pageShell('kalender');

let monthKey = '';
let data = null;
let selectedDay = '';

function pad(n) { return String(n).padStart(2, '0'); }

function monthLabel(key) {
  const [y, m] = key.split('-').map(Number);
  const d = new Date(y, m - 1, 1);
  return d.toLocaleDateString('id-ID', { month: 'long', year: 'numeric' });
}

function shiftMonth(key, delta) {
  const [y, m] = key.split('-').map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

function fmtTime(iso) {
  try {
    return new Date(iso).toLocaleTimeString('id-ID', {
      timeZone: data?.timezone || 'Asia/Jakarta',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return '';
  }
}

function statusClass(st) {
  const s = String(st || '').toLowerCase();
  if (s === 'done' || s === 'skipped_ig') return 'is-done';
  if (s === 'failed') return 'is-fail';
  if (s === 'running') return 'is-run';
  if (s === 'cancelled') return 'is-cancel';
  return 'is-pending';
}

function statusLabel(st) {
  const map = {
    pending: 'Menunggu',
    running: 'Jalan…',
    done: 'Terpublish',
    failed: 'Gagal',
    cancelled: 'Dibatalkan',
    skipped_ig: 'Threads ok · IG skip',
  };
  return map[String(st || '').toLowerCase()] || st || '—';
}

function mediaSrc(e) {
  return e.thumb_url || e.image_url || '';
}

function previewLine(e) {
  if (e.parts && e.parts.length) return e.parts[0];
  return e.text || e.preview || e.title || '';
}

function renderPlatforms(e) {
  const chips = [];
  if (e.source === 'manual' || (e.threads_ids && e.threads_ids.length)) {
    chips.push('<span class="th-chip"><i class="bi bi-at"></i> Threads</span>');
  }
  if (e.ig_media_id) chips.push('<span class="th-chip"><i class="bi bi-instagram"></i> IG</span>');
  if (e.buffer_post_id) chips.push('<span class="th-chip"><i class="bi bi-tiktok"></i> TikTok</span>');
  if (e.buffer_x_post_id) chips.push('<span class="th-chip"><i class="bi bi-twitter-x"></i> X</span>');
  if (e.media_type === 'IMAGE' || mediaSrc(e)) chips.push('<span class="th-chip"><i class="bi bi-image"></i> Gambar</span>');
  if (e.media_type === 'VIDEO') chips.push('<span class="th-chip"><i class="bi bi-camera-video"></i> Video</span>');
  return chips.length ? `<div class="cal-event-platforms">${chips.join('')}</div>` : '';
}

function renderParts(e) {
  const parts = Array.isArray(e.parts) ? e.parts.filter((p) => String(p || '').trim()) : [];
  if (!parts.length) {
    const t = String(e.text || e.preview || '').trim();
    if (!t) return '';
    return `<div class="cal-event-post"><p class="cal-event-part">${Threads.escapeHtml(t)}</p></div>`;
  }
  return `<ol class="cal-event-post">${parts.map((p, i) =>
    `<li><span class="cal-part-n">${i + 1}</span><p class="cal-event-part">${Threads.escapeHtml(p)}</p></li>`
  ).join('')}</ol>`;
}

function renderMedia(e) {
  const src = mediaSrc(e);
  if (!src) return '';
  let imgSrc = src;
  try {
    const u = new URL(src, location.origin);
    imgSrc = u.origin === location.origin ? u.pathname : src;
  } catch {}
  return `<div class="cal-event-media"><img src="${Threads.escapeHtml(imgSrc)}" alt="Media post"></div>`;
}

function renderGrid() {
  const root = document.getElementById('cal-grid');
  const [y, m] = monthKey.split('-').map(Number);
  const first = new Date(y, m - 1, 1);
  let startPad = (first.getDay() + 6) % 7;
  const daysInMonth = new Date(y, m, 0).getDate();
  const today = data?.today || '';
  const byDay = data?.by_day || {};

  const cells = [];
  for (let i = 0; i < startPad; i++) {
    cells.push(`<div class="cal-cell is-out"></div>`);
  }
  for (let day = 1; day <= daysInMonth; day++) {
    const key = `${monthKey}-${pad(day)}`;
    const evs = byDay[key] || [];
    const isToday = key === today;
    const isSel = key === selectedDay;
    const chips = evs.slice(0, 2).map((e) => {
      const src = e.source === 'manual' ? 'manual' : 'lazy';
      const line = previewLine(e);
      const statusTxt = statusLabel(e.status);
      const partsN = e.parts_n > 1 ? ` · ${e.parts_n} bagian` : '';
      const title = `${fmtTime(e.at)} · ${statusTxt}${partsN} · ${line}`;
      return `<span class="cal-chip ${statusClass(e.status)} src-${src}" title="${Threads.escapeHtml(title)}">
        <span class="cal-chip-meta">
          <span class="cal-chip-time">${Threads.escapeHtml(fmtTime(e.at))}</span>
          <span class="cal-chip-status">${Threads.escapeHtml(statusTxt)}${partsN}</span>
        </span>
        <span class="cal-chip-text">${Threads.escapeHtml(line || (src === 'manual' ? 'Manual' : 'Lazy'))}</span>
      </span>`;
    }).join('');
    const more = evs.length > 2 ? `<span class="cal-more">+${evs.length - 2} lagi</span>` : '';
    cells.push(`<button type="button" class="cal-cell${isToday ? ' is-today' : ''}${isSel ? ' is-selected' : ''}${evs.length ? ' has-events' : ''}" data-day="${key}">
      <span class="cal-daynum">${day}</span>
      <div class="cal-chips">${chips}${more}</div>
    </button>`);
  }
  while (cells.length % 7 !== 0) {
    cells.push(`<div class="cal-cell is-out"></div>`);
  }
  root.innerHTML = cells.join('');
  root.setAttribute('aria-busy', 'false');
  root.querySelectorAll('[data-day]').forEach((btn) => {
    btn.addEventListener('click', () => {
      selectedDay = btn.getAttribute('data-day');
      renderGrid();
      renderDay();
    });
  });
}

function renderDay() {
  const title = document.getElementById('day-title');
  const count = document.getElementById('day-count');
  const list = document.getElementById('day-list');
  if (!selectedDay) {
    title.textContent = 'Pilih tanggal';
    count.textContent = '';
    list.innerHTML = `<p class="text-sm text-muted m-0">Klik tanggal di kalender untuk lihat detail post.</p>`;
    return;
  }
  const evs = (data?.by_day && data.by_day[selectedDay]) || [];
  const nice = new Date(selectedDay + 'T12:00:00').toLocaleDateString('id-ID', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
  });
  title.textContent = nice;
  count.textContent = evs.length ? `${evs.length} jadwal` : 'Kosong';
  if (!evs.length) {
    list.innerHTML = `<p class="text-sm text-muted m-0">Tidak ada jadwal di tanggal ini. <a href="/app/buat.html">Jadwalkan post</a> atau cek <a href="/app/lazy.html">Lazy</a>.</p>`;
    return;
  }
  list.innerHTML = evs.map((e) => {
    const src = e.source === 'manual' ? 'Manual' : 'Lazy';
    const canCancel = e.source === 'manual' && e.status === 'pending';
    const caption = e.caption ? `<p class="cal-event-caption"><strong>Caption IG:</strong> ${Threads.escapeHtml(e.caption)}</p>` : '';
    const ids = (e.threads_ids || []).filter(Boolean);
    const idsHtml = ids.length
      ? `<div class="cal-event-ids"><span class="text-xs text-muted">Threads ID:</span> ${ids.map((id) => `<code class="th-code">${Threads.escapeHtml(id)}</code>`).join(' ')}</div>`
      : '';
    return `<article class="cal-event ${statusClass(e.status)}">
      <div class="cal-event-top">
        <span class="cal-event-time">${Threads.escapeHtml(fmtTime(e.at))}</span>
        <span class="th-chip">${Threads.escapeHtml(src)}</span>
        <span class="th-chip ${statusClass(e.status)}">${Threads.escapeHtml(statusLabel(e.status))}</span>
        ${e.title && e.source === 'lazy' ? `<span class="text-xs text-muted">${Threads.escapeHtml(e.title)}</span>` : ''}
      </div>
      ${renderPlatforms(e)}
      ${renderMedia(e)}
      ${renderParts(e)}
      ${caption}
      ${e.parts_n > 1 ? `<p class="cal-event-meta">${e.parts_n} bagian utas</p>` : ''}
      ${idsHtml}
      ${e.error ? `<p class="cal-event-err">${Threads.escapeHtml(e.error)}</p>` : ''}
      <div class="cal-event-actions">
        ${canCancel ? `<button type="button" class="th-btn th-btn-ghost text-xs" data-cancel="${Threads.escapeHtml(e.id)}">Batalkan</button>` : ''}
        ${e.source === 'manual' && e.status === 'pending' ? `<a class="th-btn th-btn-ghost text-xs" href="/app/buat.html">Edit di Buat</a>` : ''}
        ${e.source === 'lazy' ? `<a class="th-btn th-btn-ghost text-xs" href="/app/lazy.html">Lazy</a>` : ''}
        ${ids.length ? `<a class="th-btn th-btn-ghost text-xs" href="/app/posts.html">Lihat posts</a>` : ''}
      </div>
    </article>`;
  }).join('');
  list.querySelectorAll('[data-cancel]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const id = btn.getAttribute('data-cancel');
      try {
        await Threads.api('/api/schedule/' + encodeURIComponent(id) + '/cancel', { method: 'POST', body: '{}' });
        Threads.toast('Jadwal dibatalkan', true);
        await load();
      } catch (err) {
        Threads.toast(err.message, false);
      }
    });
  });
}

async function load() {
  const grid = document.getElementById('cal-grid');
  grid.setAttribute('aria-busy', 'true');
  document.getElementById('month-label').textContent = monthLabel(monthKey);
  try {
    data = await Threads.api('/api/calendar?month=' + encodeURIComponent(monthKey));
    if (!selectedDay || !selectedDay.startsWith(monthKey)) {
      selectedDay = data.today && data.today.startsWith(monthKey) ? data.today : `${monthKey}-01`;
    }
    renderGrid();
    renderDay();
  } catch (e) {
    grid.innerHTML = `<p class="text-sm text-muted p-4 m-0">${Threads.escapeHtml(e.message)}</p>`;
    Threads.toast(e.message, false);
  }
}

(function init() {
  const now = new Date();
  monthKey = `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
  document.getElementById('btn-prev').onclick = () => { monthKey = shiftMonth(monthKey, -1); load(); };
  document.getElementById('btn-next').onclick = () => { monthKey = shiftMonth(monthKey, 1); load(); };
  document.getElementById('btn-today').onclick = () => {
    const n = new Date();
    monthKey = `${n.getFullYear()}-${pad(n.getMonth() + 1)}`;
    selectedDay = `${monthKey}-${pad(n.getDate())}`;
    load();
  };
  document.getElementById('btn-refresh').onclick = () => load();
  load();
})();
