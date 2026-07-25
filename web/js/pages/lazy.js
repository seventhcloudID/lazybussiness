Threads.pageShell('lazy');

let pollTimer = null;
let watchJobId = null;
let fastPoll = false;
let detailJob = null;
let previewIdx = 0;
let previewBlobUrl = '';
let renderSeq = 0;

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

function statusBadge(st) {
  const map = {
    pending: ['Menunggu', 'pend'],
    running: ['Jalan', 'run'],
    done: ['Selesai', 'ok'],
    failed: ['Gagal', 'bad'],
    skipped_ig: ['Threads OK · IG skip', 'warn'],
  };
  const [label, cls] = map[st] || [st, ''];
  return `<span class="lazy-badge ${cls}">${Threads.escapeHtml(label)}</span>`;
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
  if (job.buffer_post_id) bits.push('Buffer TikTok (Notify Me): ' + job.buffer_post_id);
  if (job.buffer_error) bits.push('Buffer: ' + job.buffer_error);
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
  document.getElementById('enabled').checked = !!cfg.enabled;
  document.getElementById('enabled-label').textContent = cfg.enabled ? 'ON' : 'OFF';
  document.getElementById('posts-per-day').value = cfg.posts_per_day || 5;
  if (cfg.topic_hint != null && document.getElementById('topic').dataset.dirty !== '1') {
    document.getElementById('topic').value = cfg.topic_hint || '';
  }

  document.getElementById('stat-today').textContent = st.today || '—';
  const c = st.counts || {};
  document.getElementById('stat-done').textContent = String((c.done || 0) + (c.skipped_ig || 0));
  document.getElementById('stat-pending').textContent = String((c.pending || 0) + (c.running || 0));
  document.getElementById('stat-fail').textContent = String((c.failed || 0));

  const warns = st.warnings || [];
  const wEl = document.getElementById('lazy-warnings');
  if (warns.length) {
    wEl.classList.remove('hidden');
    wEl.innerHTML = warns.map(w => `<div class="lazy-warn">${Threads.escapeHtml(w)}</div>`).join('');
  } else {
    wEl.classList.add('hidden');
    wEl.innerHTML = '';
  }

  const bits = [];
  bits.push(`TZ ${st.timezone || '—'}`);
  bits.push(st.public_ok ? 'PUBLIC_BASE_URL OK' : 'PUBLIC_BASE_URL belum');
  bits.push(st.threads_ok ? 'Threads OK' : 'Threads —');
  bits.push(st.instagram_ok ? 'IG OK' : 'IG —');
  bits.push(st.buffer_ok ? 'Buffer TikTok OK' : 'Buffer —');
  bits.push(st.ai_ok ? 'AI OK' : 'AI —');
  document.getElementById('lazy-meta').textContent = bits.join(' · ');

  if (st.next_pending) {
    document.getElementById('next-hint').textContent =
      `Berikutnya post jam ${fmtTime(st.next_pending.scheduled_at)}`;
  } else {
    document.getElementById('next-hint').textContent = cfg.enabled
      ? 'Tidak ada antrian hari ini'
      : 'Otomasi OFF — centang ON + Simpan untuk jadwal 5×/hari';
  }

  const jobs = st.jobs_today || [];
  const list = document.getElementById('job-list');
  const anyActive = jobs.some(j => j.status === 'running' || j.status === 'pending');
  setFastPoll(anyActive || !!watchJobId);

  if (!jobs.length) {
    list.innerHTML = '<p class="text-sm text-muted p-4 m-0">Belum ada rencana — nyalakan otomasi lalu simpan (baru muncul jam post).</p>';
  } else {
    list.innerHTML = jobs.map(j => {
      const snip = (j.parts?.[0] || '').replace(/\s+/g, ' ').slice(0, 80);
      return `
      <button type="button" class="lazy-job" data-job="${Threads.escapeHtml(j.id)}">
        <div class="lazy-job-main">
          <strong>${fmtTime(j.scheduled_at)}</strong>
          ${statusBadge(j.status)}
          <span class="lazy-job-title">${Threads.escapeHtml(j.title || j.id)}</span>
        </div>
        ${snip ? `<div class="lazy-job-snip">${Threads.escapeHtml(snip)}${(j.parts?.[0] || '').length > 80 ? '…' : ''}</div>` : ''}
        ${j.buffer_post_id ? `<div class="lazy-job-snip">Buffer TikTok · Notify Me</div>` : ''}
        ${j.buffer_error ? `<div class="lazy-job-err">Buffer: ${Threads.escapeHtml(j.buffer_error)}</div>` : ''}
        ${j.error ? `<div class="lazy-job-err">${Threads.escapeHtml(j.error)}</div>` : ''}
      </button>`;
    }).join('');
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

document.getElementById('topic').addEventListener('input', () => {
  document.getElementById('topic').dataset.dirty = '1';
});

document.getElementById('brand').addEventListener('input', () => {
  if (detailJob?.parts?.length) renderCarouselPreview();
});

document.getElementById('job-list').addEventListener('click', async e => {
  const btn = e.target.closest('[data-job]');
  if (!btn) return;
  try {
    const job = await Threads.api('/api/lazy/jobs/' + encodeURIComponent(btn.dataset.job));
    previewIdx = 0;
    showJobDetail(job);
  } catch (err) {
    Threads.toast(err.message, false);
  }
});

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
  if (!confirm('Hapus antrian hari ini dan buat jadwal baru dari sekarang?')) return;
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
    const mem = await Threads.api('/api/ai/memory');
    if (mem?.brand) document.getElementById('brand').value = mem.brand;
  } catch {}
  await refresh();
  setFastPoll(false);
})();
