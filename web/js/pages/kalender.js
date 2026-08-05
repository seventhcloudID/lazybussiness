Threads.pageShell('kalender');

let monthKey = '';
let data = null;
let selectedDay = '';

const pad = (n) => String(n).padStart(2, '0');

function monthLabel(key) {
  const [y, m] = key.split('-').map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString('id-ID', { month: 'long', year: 'numeric' });
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

const STATUS = {
  pending:   { cls: 'is-pending',   label: 'Menunggu',     icon: 'bi-hourglass-split' },
  running:   { cls: 'is-run',       label: 'Jalan…',       icon: 'bi-play-circle' },
  done:      { cls: 'is-done',      label: 'Terpublish',   icon: 'bi-check2-circle' },
  failed:    { cls: 'is-fail',      label: 'Gagal',        icon: 'bi-x-circle' },
  cancelled: { cls: 'is-cancel',    label: 'Dibatalkan',   icon: 'bi-slash-circle' },
  skipped_ig:{ cls: 'is-done',      label: 'Threads · IG skip', icon: 'bi-check2-square' },
};

function statusMeta(st) {
  const k = String(st || '').toLowerCase();
  return STATUS[k] || { cls: 'is-pending', label: st || '—', icon: 'bi-question-circle' };
}

function previewLine(e) {
  if (e.parts && e.parts.length) return String(e.parts[0]);
  return e.text || e.preview || e.title || '';
}

function partsCount(e) {
  const n = e.parts_n || (Array.isArray(e.parts) ? e.parts.filter((p) => String(p || '').trim()).length : 0);
  return n > 1 ? n : 0;
}

function mediaSrc(e) {
  return e.thumb_url || e.image_url || '';
}

function clip(s, n) {
  s = String(s || '').trim();
  if (!s) return '';
  const r = Array.from(s);
  return r.length <= n ? s : r.slice(0, n).join('') + '…';
}

function renderGrid() {
  const root = document.getElementById('cal-grid');
  const [y, m] = monthKey.split('-').map(Number);
  const first = new Date(y, m - 1, 1);
  const startPad = (first.getDay() + 6) % 7;
  const daysInMonth = new Date(y, m, 0).getDate();
  const today = data?.today || '';
  const byDay = data?.by_day || {};

  const cells = [];
  for (let i = 0; i < startPad; i++) {
    cells.push('<div class="cal-cell is-out"></div>');
  }
  for (let day = 1; day <= daysInMonth; day++) {
    const key = `${monthKey}-${pad(day)}`;
    const evs = byDay[key] || [];
    const isToday = key === today;
    const isSel = key === selectedDay;

    const chips = evs.slice(0, 2).map((e) => {
      const m = statusMeta(e.status);
      const src = e.source === 'manual' ? 'manual' : 'lazy';
      const line = previewLine(e);
      const parts = partsCount(e);
      const title = `${fmtTime(e.at)} · ${m.label}${parts ? ` · ${parts} bagian` : ''} · ${line}`;
      return `<span class="cal-chip ${m.cls} src-${src}" title="${Threads.escapeHtml(title)}">
        <span class="cal-chip-head">
          <span class="cal-chip-dot"></span>
          <span class="cal-chip-time">${Threads.escapeHtml(fmtTime(e.at))}</span>
          <span class="cal-chip-status">${Threads.escapeHtml(m.label)}${parts ? ` · ${parts}` : ''}</span>
        </span>
        <span class="cal-chip-text">${Threads.escapeHtml(clip(line, 120) || '—')}</span>
      </span>`;
    }).join('');

    cells.push(`<button type="button" class="cal-cell${isToday ? ' is-today' : ''}${isSel ? ' is-selected' : ''}${evs.length ? ' has-events' : ''}" data-day="${key}">
      <span class="cal-cell-top">
        <span class="cal-daynum">${day}</span>
        ${evs.length ? `<span class="cal-count">${evs.length}</span>` : ''}
      </span>
      <div class="cal-chips">${chips}</div>
      ${evs.length > 2 ? `<span class="cal-more">+${evs.length - 2} lagi</span>` : ''}
    </button>`);
  }
  while (cells.length % 7 !== 0) {
    cells.push('<div class="cal-cell is-out"></div>');
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
  return chips.length ? `<div class="cal-card-platforms">${chips.join('')}</div>` : '';
}

function renderDay() {
  const title = document.getElementById('day-title');
  const count = document.getElementById('day-count');
  const list = document.getElementById('day-list');
  if (!selectedDay) {
    title.textContent = 'Pilih tanggal';
    count.textContent = '';
    list.innerHTML = `<div class="cal-day-empty">
      <i class="bi bi-calendar3"></i>
      <p>Klik tanggal di kalender untuk melihat detail jadwal.</p>
    </div>`;
    return;
  }
  const evs = (data?.by_day && data.by_day[selectedDay]) || [];
  const nice = new Date(selectedDay + 'T12:00:00').toLocaleDateString('id-ID', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
  });
  title.textContent = nice;
  count.textContent = evs.length ? `${evs.length} jadwal` : 'Tidak ada jadwal';
  if (!evs.length) {
    list.innerHTML = `<div class="cal-day-empty">
      <i class="bi bi-calendar-x"></i>
      <p>Tidak ada jadwal di tanggal ini.</p>
      <a class="th-btn th-btn-soft text-xs" href="/app/buat.html"><i class="bi bi-calendar-plus"></i> Jadwalkan post</a>
    </div>`;
    return;
  }

  list.innerHTML = evs.map((e) => {
    const m = statusMeta(e.status);
    const src = e.source === 'manual' ? 'Manual' : 'Lazy';
    const canCancel = e.source === 'manual' && e.status === 'pending';
    const ids = (e.threads_ids || []).filter(Boolean);
    const line = previewLine(e);
    const parts = partsCount(e);
    const img = mediaSrc(e);
    let imgSrc = '';
    if (img) {
      try {
        const u = new URL(img, location.origin);
        imgSrc = u.origin === location.origin ? u.pathname : img;
      } catch { imgSrc = img; }
    }

    return `<article class="cal-card ${m.cls}">
      <div class="cal-card-main">
        <div class="cal-card-head">
          <span class="cal-card-time">${Threads.escapeHtml(fmtTime(e.at))}</span>
          <span class="th-chip cal-src ${e.source === 'manual' ? 'src-manual' : 'src-lazy'}">${src}</span>
          <span class="cal-badge ${m.cls}"><i class="bi ${m.icon}"></i> ${Threads.escapeHtml(m.label)}</span>
          ${e.title && e.source === 'lazy' ? `<span class="cal-card-title">${Threads.escapeHtml(e.title)}</span>` : ''}
        </div>
        <p class="cal-card-text">${Threads.escapeHtml(line || '—')}</p>
        ${renderPlatforms(e)}
        <div class="cal-card-meta">
          ${parts ? `<span><i class="bi bi-list-ol"></i> ${parts} bagian</span>` : ''}
          ${e.caption ? `<span class="cal-card-cap" title="Caption IG"><i class="bi bi-camera"></i> ${Threads.escapeHtml(clip(e.caption, 80))}</span>` : ''}
          ${ids.length ? `<span><i class="bi bi-at"></i> Post terpublish</span>` : ''}
          ${e.error ? `<span class="cal-card-err"><i class="bi bi-exclamation-triangle"></i> ${Threads.escapeHtml(clip(e.error, 90))}</span>` : ''}
        </div>
      </div>
      ${img ? `<div class="cal-card-media"><img src="${Threads.escapeHtml(imgSrc)}" alt="Media post"></div>` : ''}
      <div class="cal-card-foot">
        ${ids.length ? `<a class="th-btn th-btn-ghost text-xs" href="/app/posts.html"><i class="bi bi-box-arrow-up-right"></i> Lihat</a>` : ''}
        ${canCancel ? `<button type="button" class="th-btn th-btn-ghost text-xs" data-cancel="${Threads.escapeHtml(e.id)}"><i class="bi bi-x-lg"></i> Batalkan</button>` : ''}
        ${e.source === 'manual' && e.status === 'pending' ? `<a class="th-btn th-btn-ghost text-xs" href="/app/buat.html"><i class="bi bi-pencil"></i> Edit</a>` : ''}
        ${e.source === 'lazy' ? `<a class="th-btn th-btn-ghost text-xs" href="/app/lazy.html"><i class="bi bi-lightning-charge"></i> Lazy</a>` : ''}
        ${e.source === 'manual' ? `<button type="button" class="th-btn th-btn-danger-soft text-xs" data-delete-schedule="${Threads.escapeHtml(e.id)}"><i class="bi bi-calendar-x"></i> Hapus jadwal</button>` : ''}
        ${ids.length ? `<button type="button" class="th-btn th-btn-danger-soft text-xs" data-delete-content="${Threads.escapeHtml(e.id)}" data-ids="${Threads.escapeHtml(ids.join(','))}"><i class="bi bi-trash"></i> Hapus konten</button>` : ''}
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

  list.querySelectorAll('[data-delete-content]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const ids = btn.getAttribute('data-ids').split(',').filter(Boolean);
      if (!ids.length) return;
      const label = ids.length === 1 ? '1 post Threads' : `${ids.length} post Threads`;
      const ok = await Threads.confirm(
        `Hapus ${label} dari platform Threads? Tindakan ini tidak bisa dibatalkan.`,
        { title: 'Hapus konten', okLabel: 'Ya, hapus', danger: true }
      );
      if (!ok) return;
      try {
        for (const mid of ids) {
          await Threads.api('/api/media/' + encodeURIComponent(mid), { method: 'DELETE' });
        }
        Threads.toast('Konten dihapus', true);
        await load();
      } catch (err) {
        Threads.toast(err.message, false);
      }
    });
  });

  list.querySelectorAll('[data-delete-schedule]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const id = btn.getAttribute('data-delete-schedule');
      const ok = await Threads.confirm(
        'Hapus jadwal ini permanen dari daftar? Post yang sudah terpublish di Threads tidak ikut dihapus.',
        { title: 'Hapus jadwal', okLabel: 'Ya, hapus', danger: true }
      );
      if (!ok) return;
      try {
        await Threads.api('/api/schedule/' + encodeURIComponent(id), { method: 'DELETE' });
        Threads.toast('Jadwal dihapus', true);
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