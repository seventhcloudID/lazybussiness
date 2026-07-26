Threads.pageShell('lazy');

let pollTimer = null;
let watchJobId = null;
let fastPoll = false;
let detailJob = null;
let previewIdx = 0;
let previewBlobUrl = '';
let renderSeq = 0;
let activeTemplate = 'noir';
const TEMPLATE_NAMES = {
  noir: 'Noir', ink: 'Ink', ocean: 'Ocean', ember: 'Ember', paper: 'Kertas',
  bloom: 'Bloom', lilac: 'Lilac', peach: 'Peach', bold: 'Bold', frame: 'Frame',
  meadow: 'Meadow', midnight: 'Midnight', coral: 'Coral', mint: 'Mint', cherry: 'Cherry',
  sand: 'Sand', neon: 'Neon', slate: 'Slate', honey: 'Honey', mono: 'Mono',
};

function showAlert(msg) {
  const el = document.getElementById('lazy-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function fmtTime(iso) {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  } catch {
    return iso;
  }
}

function fmtDay(ymd) {
  if (!ymd) return '';
  try {
    const d = new Date(ymd + 'T12:00:00');
    return d.toLocaleDateString('id-ID', { weekday: 'short', day: 'numeric', month: 'short' });
  } catch {
    return ymd;
  }
}

function statusClass(st) {
  if (st === 'pending') return 'pend';
  if (st === 'running') return 'run';
  if (st === 'done' || st === 'skipped_ig') return 'ok';
  if (st === 'failed') return 'bad';
  return '';
}

function jobRowHTML(j, compact) {
  const snip = (j.parts?.[0] || '').replace(/\s+/g, ' ').slice(0, 90);
  const st = j.status || 'pending';
  const cls = statusClass(st);
  const tags = [];
  if (j.buffer_x_post_id) tags.push('X');
  if (j.buffer_post_id) tags.push('TikTok');
  if (j.thumb_url) tags.push('Thumb');
  if (j.threads_ids?.length) tags.push('Threads');
  if (j.ig_container) tags.push('IG');
  const title = j.title || (compact ? 'Slot terjadwal' : j.id);
  return `
    <button type="button" class="lazy-job is-${cls || 'pend'}${st === 'done' || st === 'skipped_ig' ? ' is-done' : ''}" data-job="${Threads.escapeHtml(j.id)}">
      <div class="lazy-job-time"><strong>${fmtTime(j.scheduled_at)}</strong></div>
      <div class="lazy-job-track" aria-hidden="true">
        <span class="lazy-job-dot"></span>
        <span class="lazy-job-rail"></span>
      </div>
      <div class="lazy-job-body">
        <div class="lazy-job-main">
          ${statusBadge(st)}
          <span class="lazy-job-title">${Threads.escapeHtml(title)}</span>
        </div>
        ${!compact && snip ? `<div class="lazy-job-snip">${Threads.escapeHtml(snip)}${(j.parts?.[0] || '').length > 90 ? '…' : ''}</div>` : ''}
        ${tags.length ? `<div class="lazy-job-tags">${tags.map(t => `<span class="lazy-tag">${t}</span>`).join('')}</div>` : ''}
        ${j.buffer_x_error ? `<div class="lazy-job-err">Buffer X: ${Threads.escapeHtml(j.buffer_x_error)}</div>` : ''}
        ${j.buffer_error ? `<div class="lazy-job-err">Buffer TikTok: ${Threads.escapeHtml(j.buffer_error)}</div>` : ''}
        ${j.error ? `<div class="lazy-job-err">${Threads.escapeHtml(j.error)}</div>` : ''}
      </div>
    </button>`;
}

function statusBadge(st) {
  const map = {
    pending: ['Menunggu', 'pend'],
    running: ['Jalan', 'run'],
    done: ['Selesai', 'ok'],
    failed: ['Gagal', 'bad'],
    skipped_ig: ['IG skip', 'warn'],
  };
  const [label, cls] = map[st] || [st, ''];
  return `<span class="lazy-badge ${cls}">${Threads.escapeHtml(label)}</span>`;
}

function channelPill(ok, icon, label) {
  return `<span class="lazy-ch ${ok ? 'is-ok' : 'is-off'}"><i class="bi ${icon}"></i>${Threads.escapeHtml(label)}</span>`;
}

function showJobDetail(job) {
  if (!job) return;
  detailJob = job;
  const box = document.getElementById('lazy-last');
  box.classList.remove('hidden');
  const st = job.status || '';
  document.getElementById('lazy-title').textContent =
    `${job.title || job.id} · ${st} · ${fmtTime(job.scheduled_at)}`;
  const parts = job.parts || [];
  document.getElementById('lazy-parts').innerHTML = parts.length
    ? parts.map((p, i) => `
      <button type="button" class="gen-thread-part lazy-part-btn${i === previewIdx ? ' on' : ''}" data-part="${i}">
        <div class="gen-thread-n">${i + 1}</div>
        <div class="gen-thread-text"><p>${Threads.escapeHtml(p)}</p></div>
      </button>`).join('')
    : `<p class="text-muted text-sm m-0">${
        st === 'running' || st === 'pending'
          ? 'Masih generate/publish… preview muncul setelah ada teks.'
          : 'Belum ada teks (gagal sebelum generate?).'
      }</p>`;
  const bits = [];
  if (job.caption) bits.push('Caption IG:\n' + job.caption);
  if (job.threads_ids?.length) bits.push('Threads IDs: ' + job.threads_ids.join(', '));
  if (job.ig_container) bits.push('IG container: ' + job.ig_container);
  if (job.thumb_url) bits.push('Thumbnail Threads: ' + job.thumb_url);
  if (job.image_urls?.length) bits.push('Slide images: ' + job.image_urls.length);
  const thumbBox = document.getElementById('lazy-thumb-box');
  const thumbImg = document.getElementById('lazy-thumb-img');
  if (thumbBox && thumbImg) {
    if (job.thumb_url) {
      try {
        const u = new URL(job.thumb_url, location.origin);
        thumbImg.src = u.origin === location.origin ? u.pathname : job.thumb_url;
      } catch {
        thumbImg.src = job.thumb_url;
      }
      thumbBox.hidden = false;
    } else {
      thumbBox.hidden = true;
      thumbImg.removeAttribute('src');
    }
  }
  if (job.buffer_x_post_id) bits.push('Buffer X (shareNow): ' + job.buffer_x_post_id);
  if (job.buffer_x_error) bits.push('Buffer X: ' + job.buffer_x_error);
  if (job.buffer_post_id) bits.push('Buffer TikTok (Notify Me): ' + job.buffer_post_id);
  if (job.buffer_error) bits.push('Buffer TikTok: ' + job.buffer_error);
  if (job.error) bits.push('⚠️ ' + job.error);
  document.getElementById('lazy-caption').textContent = bits.join('\n\n');

  if (parts.length) {
    if (previewIdx >= parts.length) previewIdx = 0;
    renderCarouselPreview();
  } else {
    clearCarouselPreview();
  }
}

function clearCarouselPreview() {
  const img = document.getElementById('lazy-preview-png');
  const loading = document.getElementById('lazy-preview-loading');
  const dots = document.getElementById('lazy-preview-dots');
  const meta = document.getElementById('lazy-preview-meta');
  if (previewBlobUrl) {
    URL.revokeObjectURL(previewBlobUrl);
    previewBlobUrl = '';
  }
  if (img) img.removeAttribute('src');
  if (loading) loading.classList.remove('show');
  if (dots) dots.innerHTML = '';
  if (meta) meta.textContent = '0 / 0';
}

function renderCarouselPreview() {
  const parts = detailJob?.parts || [];
  if (!parts.length) {
    clearCarouselPreview();
    return;
  }
  if (previewIdx < 0) previewIdx = parts.length - 1;
  if (previewIdx >= parts.length) previewIdx = 0;

  const brand = (document.getElementById('brand')?.value || '').trim().replace(/^@+/, '');
  const handle = document.getElementById('lazy-preview-handle');
  const avatar = document.getElementById('lazy-preview-avatar');
  if (handle) handle.textContent = brand || 'brand';
  if (avatar) avatar.textContent = (brand || '?').charAt(0).toUpperCase();

  document.getElementById('lazy-preview-meta').textContent =
    `${String(previewIdx + 1).padStart(2, '0')} / ${String(parts.length).padStart(2, '0')}`;
  document.getElementById('lazy-preview-dots').innerHTML = parts.map((_, i) =>
    `<button type="button" class="${i === previewIdx ? 'on' : ''}" data-dot="${i}" aria-label="Slide ${i + 1}"></button>`
  ).join('');

  document.querySelectorAll('.lazy-part-btn').forEach((el, i) => {
    el.classList.toggle('on', i === previewIdx);
  });

  schedulePng(parts[previewIdx], brand, previewIdx, parts.length);
}

let renderTimer = null;
function schedulePng(text, brand, index, total) {
  clearTimeout(renderTimer);
  const loading = document.getElementById('lazy-preview-loading');
  if (loading) loading.classList.add('show');
  renderTimer = setTimeout(() => renderPng(text, brand, index, total), 200);
}

async function renderPng(text, brand, index, total) {
  const seq = ++renderSeq;
  const img = document.getElementById('lazy-preview-png');
  const loading = document.getElementById('lazy-preview-loading');
  try {
    const res = await fetch('/api/ig/carousel/render', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        text: text || '…',
        brand,
        template: activeTemplate,
        index: Number(index) || 0,
        total: Number(total) || 0,
      }),
    });
    if (seq !== renderSeq) return;
    if (res.status === 401) {
      location.replace('/login.html?next=' + encodeURIComponent(location.pathname));
      return;
    }
    if (!res.ok) throw new Error(await res.text());
    const blob = await res.blob();
    if (seq !== renderSeq) return;
    if (previewBlobUrl) URL.revokeObjectURL(previewBlobUrl);
    previewBlobUrl = URL.createObjectURL(blob);
    img.classList.remove('igc-png-in');
    img.src = previewBlobUrl;
    img.onload = () => {
      loading?.classList.remove('show');
      void img.offsetWidth;
      img.classList.add('igc-png-in');
    };
  } catch (e) {
    if (seq !== renderSeq) return;
    loading?.classList.remove('show');
    Threads.toast('Preview gagal: ' + (e.message || e), false);
  }
}

function renderStatus(st) {
  const cfg = st.config || {};
  const on = !!cfg.enabled;
  document.getElementById('enabled').checked = on;
  document.getElementById('enabled-label').textContent = on ? 'ON' : 'OFF';
  const thumbOn = cfg.thumbnail_enabled !== false;
  document.getElementById('thumb-enabled').checked = thumbOn;
  document.getElementById('thumb-label').textContent = thumbOn ? 'ON' : 'OFF';
  if (cfg.carousel_template) {
    activeTemplate = cfg.carousel_template;
    const nameEl = document.getElementById('tpl-current-name');
    if (nameEl) nameEl.textContent = TEMPLATE_NAMES[activeTemplate] || activeTemplate;
  }
  document.getElementById('posts-per-day').value = cfg.posts_per_day || 5;
  if (cfg.topic_hint != null && document.getElementById('topic').dataset.dirty !== '1') {
    document.getElementById('topic').value = cfg.topic_hint || '';
  }
  document.getElementById('stat-today').textContent = st.today || '—';
  const c = st.counts || {};
  document.getElementById('stat-done').textContent = String((c.done || 0) + (c.skipped_ig || 0));
  document.getElementById('stat-pending').textContent = String((c.pending || 0) + (c.running || 0));
  document.getElementById('stat-fail').textContent = String((c.failed || 0));

  const hero = document.getElementById('lazy-hero');
  if (hero) hero.dataset.on = on ? '1' : '0';
  const heroTitle = document.getElementById('lazy-hero-title');
  if (heroTitle) heroTitle.textContent = on ? 'ON' : 'OFF';
  let heroNext = 'Nyalakan otomasi lalu simpan untuk mulai jadwal.';
  if (on && st.next_pending) {
    heroNext = `Post berikutnya hari ini jam ${fmtTime(st.next_pending.scheduled_at)}.`;
  } else if (on && st.next_tomorrow) {
    heroNext = `Hari ini selesai. Besok mulai jam ${fmtTime(st.next_tomorrow.scheduled_at)}.`;
  } else if (on) {
    heroNext = 'Otomasi aktif — menunggu slot terjadwal.';
  }
  document.getElementById('lazy-hero-next').textContent = heroNext;

  const ch = document.getElementById('lazy-channels');
  if (ch) {
    ch.innerHTML = [
      channelPill(!!st.threads_ok, 'bi-at', 'Threads'),
      channelPill(!!st.instagram_ok, 'bi-instagram', 'Instagram'),
      channelPill(!!st.buffer_ok, 'bi-broadcast', 'Buffer'),
      channelPill(!!st.ai_ok, 'bi-stars', 'Gemini'),
      channelPill(thumbOn && !!st.thumb_ok, 'bi-image', thumbOn ? 'Thumbnail' : 'Thumb OFF'),
      channelPill(!!st.public_ok, 'bi-link-45deg', 'Public URL'),
    ].join('');
  }

  const warns = st.warnings || [];
  const wEl = document.getElementById('lazy-warnings');
  if (warns.length) {
    wEl.classList.remove('hidden');
    wEl.innerHTML = warns.map(w => `<div class="lazy-warn">${Threads.escapeHtml(w)}</div>`).join('');
  } else {
    wEl.classList.add('hidden');
    wEl.innerHTML = '';
  }

  document.getElementById('lazy-meta').textContent = `TZ ${st.timezone || '—'} · ${cfg.posts_per_day || 5}x/hari`;

  if (st.next_pending) {
    document.getElementById('next-hint').textContent =
      `Berikutnya ${fmtTime(st.next_pending.scheduled_at)}`;
  } else if (st.next_tomorrow) {
    document.getElementById('next-hint').textContent =
      `Selesai · besok ${fmtTime(st.next_tomorrow.scheduled_at)}`;
  } else {
    document.getElementById('next-hint').textContent = on ? 'Tidak ada antrian' : 'Otomasi OFF';
  }

  const todayTitle = document.getElementById('schedule-today-title');
  if (todayTitle) todayTitle.textContent = st.today ? fmtDay(st.today) : 'Antrian';

  const jobs = st.jobs_today || [];
  const list = document.getElementById('job-list');
  const anyActive = jobs.some(j => j.status === 'running' || j.status === 'pending');
  setFastPoll(anyActive || !!watchJobId);

  if (!jobs.length) {
    list.innerHTML = '<p class="lazy-empty">Belum ada rencana — nyalakan otomasi lalu simpan.</p>';
  } else {
    list.innerHTML = jobs.map(j => jobRowHTML(j, false)).join('');
  }

  const tomEl = document.getElementById('tomorrow-date');
  if (tomEl) tomEl.textContent = st.tomorrow ? fmtDay(st.tomorrow) : 'Jadwal';
  const tomJobs = st.jobs_tomorrow || [];
  const tomList = document.getElementById('tomorrow-list');
  const tomHint = document.getElementById('tomorrow-hint');
  if (st.next_tomorrow) {
    tomHint.textContent = `${tomJobs.length} slot · mulai ${fmtTime(st.next_tomorrow.scheduled_at)}`;
  } else if (on) {
    tomHint.textContent = tomJobs.length ? `${tomJobs.length} slot` : 'Belum terbuat';
  } else {
    tomHint.textContent = 'Otomasi OFF';
  }
  if (!tomJobs.length) {
    tomList.innerHTML = '<p class="lazy-empty">Belum ada jadwal besok. Simpan saat otomasi ON.</p>';
  } else {
    tomList.innerHTML = tomJobs.map(j => jobRowHTML(j, true)).join('');
  }

  if (watchJobId) {
    const w = jobs.find(j => j.id === watchJobId);
    if (w) {
      const hadParts = !!(detailJob?.parts?.length);
      showJobDetail(w);
      if (w.status !== 'pending' && w.status !== 'running') {
        watchJobId = null;
        const btn = document.getElementById('btn-run-now');
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-play-fill"></i> Run 1 sekarang';
        Threads.toast(
          w.status === 'failed' ? 'Gagal — buka detail di antrian' : `Selesai: ${w.status}`,
          w.status !== 'failed'
        );
        if (!hadParts && w.parts?.length) previewIdx = 0;
      }
    }
  }
}

function setFastPoll(on) {
  if (fastPoll === on) return;
  fastPoll = on;
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = setInterval(refresh, on ? 3000 : 20000);
}

async function refresh() {
  try {
    const st = await Threads.api('/api/lazy/status');
    renderStatus(st);
    showAlert('');
  } catch (e) {
    showAlert(e.message);
  }
}

document.getElementById('enabled').addEventListener('change', () => {
  document.getElementById('enabled-label').textContent =
    document.getElementById('enabled').checked ? 'ON' : 'OFF';
});

document.getElementById('thumb-enabled').addEventListener('change', () => {
  document.getElementById('thumb-label').textContent =
    document.getElementById('thumb-enabled').checked ? 'ON' : 'OFF';
});

document.getElementById('topic').addEventListener('input', () => {
  document.getElementById('topic').dataset.dirty = '1';
});

document.getElementById('brand').addEventListener('input', () => {
  if (detailJob?.parts?.length) renderCarouselPreview();
});

async function openJobFromClick(e) {
  const btn = e.target.closest('[data-job]');
  if (!btn) return;
  try {
    const job = await Threads.api('/api/lazy/jobs/' + encodeURIComponent(btn.dataset.job));
    previewIdx = 0;
    showJobDetail(job);
  } catch (err) {
    Threads.toast(err.message, false);
  }
}

document.getElementById('job-list').addEventListener('click', openJobFromClick);
document.getElementById('tomorrow-list').addEventListener('click', openJobFromClick);

document.getElementById('lazy-parts').addEventListener('click', e => {
  const btn = e.target.closest('[data-part]');
  if (!btn) return;
  previewIdx = Number(btn.dataset.part) || 0;
  renderCarouselPreview();
});

document.getElementById('lazy-preview-dots').addEventListener('click', e => {
  const btn = e.target.closest('[data-dot]');
  if (!btn) return;
  previewIdx = Number(btn.dataset.dot) || 0;
  renderCarouselPreview();
});

document.getElementById('lazy-prev').onclick = () => {
  const n = detailJob?.parts?.length || 0;
  if (!n) return;
  previewIdx = (previewIdx - 1 + n) % n;
  renderCarouselPreview();
};

document.getElementById('lazy-next').onclick = () => {
  const n = detailJob?.parts?.length || 0;
  if (!n) return;
  previewIdx = (previewIdx + 1) % n;
  renderCarouselPreview();
};

document.getElementById('btn-save').onclick = async () => {
  showAlert('');
  const posts = Math.max(5, Math.min(12, Number(document.getElementById('posts-per-day').value) || 5));
  document.getElementById('posts-per-day').value = posts;
  try {
    const brand = document.getElementById('brand').value.trim();
    if (brand) {
      await Threads.api('/api/ai/brand', { method: 'PUT', body: JSON.stringify({ brand }) });
    }
    await Threads.api('/api/lazy/config', {
      method: 'PUT',
      body: JSON.stringify({
        enabled: document.getElementById('enabled').checked,
        posts_per_day: posts,
        topic_hint: document.getElementById('topic').value.trim(),
        thumbnail_enabled: document.getElementById('thumb-enabled').checked,
        carousel_template: activeTemplate,
      }),
    });
    delete document.getElementById('topic').dataset.dirty;
    Threads.toast('Pengaturan disimpan', true);
    await refresh();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-run-now').onclick = async () => {
  showAlert('');
  const btn = document.getElementById('btn-run-now');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Background…';
  try {
    const brand = document.getElementById('brand').value.trim();
    if (brand) {
      await Threads.api('/api/ai/brand', { method: 'PUT', body: JSON.stringify({ brand }) });
    }
    const data = await Threads.api('/api/lazy/run-now', { method: 'POST', body: '{}' });
    const job = data.job || data;
    watchJobId = job.id;
    previewIdx = 0;
    showJobDetail(job);
    Threads.toast('Job mulai di background — pantau antrian + preview', true);
    setFastPoll(true);
    await refresh();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-play-fill"></i> Run 1 sekarang';
  }
};

document.getElementById('btn-refresh').onclick = () => refresh();

document.getElementById('btn-replan').onclick = async () => {
  if (!(await Threads.confirm('Hapus antrian hari ini dan buat jadwal baru dari sekarang?', {
    title: 'Reset jadwal hari ini',
    okLabel: 'Reset jadwal',
  }))) return;
  try {
    await Threads.api('/api/lazy/replan', { method: 'POST', body: '{}' });
    Threads.toast('Jadwal hari ini di-reset', true);
    await refresh();
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

(async () => {
  try {
    const tpl = await Threads.api('/api/ig/carousel/templates');
    activeTemplate = tpl.active || 'noir';
    const nameEl = document.getElementById('tpl-current-name');
    if (nameEl) nameEl.textContent = TEMPLATE_NAMES[activeTemplate] || activeTemplate;
  } catch {}
  try {
    const mem = await Threads.api('/api/ai/memory');
    if (mem?.brand) document.getElementById('brand').value = mem.brand;
  } catch {}
  await refresh();
  setFastPoll(false);
})();
