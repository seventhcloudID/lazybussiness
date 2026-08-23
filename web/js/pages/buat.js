Threads.pageShell('buat');

const MAX_CHARS = 500;
let parts = [''];

function charLen(s) {
  return Array.from(String(s || '')).length;
}

function mediaType() {
  const checked = document.querySelector('input[name="mtype"]:checked');
  const map = { 'mt-text': 'TEXT', 'mt-img': 'IMAGE', 'mt-vid': 'VIDEO' };
  return map[checked?.id] || 'TEXT';
}

function publishedId(data) {
  return data?.published?.id || '';
}

/** datetime-local / "YYYY-MM-DDTHH:MM" → RFC3339 dengan offset WIB (+07:00). */
function toWIBRFC3339(raw) {
  let s = String(raw || '').trim();
  if (!s) throw new Error('Pilih tanggal & jam jadwal');
  if (/[zZ]|[+-]\d{2}:\d{2}$/.test(s)) return s;
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(s)) s += ':00';
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(s)) {
    throw new Error('Format jadwal tidak valid');
  }
  return s + '+07:00';
}

function setScheduleMinNow() {
  const el = document.getElementById('schedule-at');
  if (!el) return;
  // Min = sekarang WIB dibulatkan ke menit berikutnya.
  const fmt = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Jakarta',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
  });
  const parts = Object.fromEntries(fmt.formatToParts(new Date()).map((p) => [p.type, p.value]));
  const d = new Date(`${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:00+07:00`);
  d.setMinutes(d.getMinutes() + 2);
  const p2 = Object.fromEntries(fmt.formatToParts(d).map((p) => [p.type, p.value]));
  el.min = `${p2.year}-${p2.month}-${p2.day}T${p2.hour}:${p2.minute}`;
}

let previewHandle = 'kamu';
let previewInitials = 'TH';

function syncMeta() {
  const n = parts.length;
  document.getElementById('parts-meta').textContent =
    n === 1 ? '1 bagian' : `${n} bagian utas`;
  document.getElementById('btn-publish').innerHTML = n > 1
    ? `<i class="bi bi-send"></i> Publikasikan utas (${n})`
    : `<i class="bi bi-send"></i> Publikasikan`;
}

function renderPreview() {
  const root = document.getElementById('compose-preview');
  if (!root) return;
  const filled = parts.some((p) => String(p || '').trim());
  if (!filled) {
    root.innerHTML = `<p class="buat-preview-empty">Mulai tulis di bagian 1 — preview muncul di sini.</p>`;
    return;
  }
  const mediaURL = document.getElementById('compose-media-url')?.value?.trim() || '';
  const mtype = mediaType();
  let mediaHtml = '';
  if (mediaURL && (mtype === 'IMAGE' || document.getElementById('mt-img')?.checked)) {
    let src = mediaURL;
    try {
      const u = new URL(mediaURL, location.origin);
      src = u.origin === location.origin ? u.pathname : mediaURL;
    } catch {}
    mediaHtml = `<div class="buat-pv-media"><img src="${Threads.escapeHtml(src)}" alt=""></div>`;
  }
  root.innerHTML = parts.map((text, i) => {
    const body = String(text || '').trim();
    const show = body || i === 0;
    if (!show) return '';
    return `<article class="buat-pv-item">
      <div class="buat-pv-avatar">${Threads.escapeHtml(previewInitials)}</div>
      <div class="buat-pv-body">
        <p class="buat-pv-handle">${Threads.escapeHtml(previewHandle)}</p>
        <p class="buat-pv-text ${body ? '' : 'is-placeholder'}">${Threads.escapeHtml(body || 'Hook masih kosong…')}</p>
        ${i === 0 ? mediaHtml : ''}
      </div>
    </article>`;
  }).join('');
}

function renderParts() {
  const root = document.getElementById('compose-parts');
  root.innerHTML = parts.map((text, i) => {
    const n = charLen(text);
    const over = n > MAX_CHARS;
    return `
      <div class="compose-part ${i === 0 ? 'is-hook' : ''}" data-i="${i}">
        <div class="compose-part-head">
          <span class="compose-part-n">${i + 1}</span>
          <span class="compose-part-label">${i === 0 ? 'Hook' : `Bagian ${i + 1}`}</span>
          <span class="compose-part-count ${over ? 'over' : ''}">${n}/${MAX_CHARS}</span>
          ${parts.length > 1 ? `<button type="button" class="th-btn th-btn-ghost !py-0.5 !px-2 text-xs" data-remove="${i}" title="Hapus"><i class="bi bi-x-lg"></i></button>` : ''}
        </div>
        <textarea class="th-textarea compose-part-text" rows="${i === 0 ? 5 : 3}" maxlength="${MAX_CHARS}"
          placeholder="${i === 0 ? 'Tulis hook yang menarik…' : `Lanjutkan cerita di bagian ${i + 1}…`}">${Threads.escapeHtml(text)}</textarea>
      </div>`;
  }).join('');
  syncMeta();
  renderPreview();
}

function readPartsFromDOM() {
  document.querySelectorAll('.compose-part-text').forEach((el, i) => {
    if (i < parts.length) parts[i] = el.value;
  });
}

function setParts(next) {
  const cleaned = (next || []).map(p => String(p || '').trim()).filter(Boolean);
  parts = cleaned.length
    ? cleaned.map(p => Array.from(p).slice(0, MAX_CHARS).join(''))
    : [''];
  renderParts();
}

document.getElementById('compose-parts').addEventListener('input', e => {
  const ta = e.target.closest('.compose-part-text');
  if (!ta) return;
  const wrap = ta.closest('.compose-part');
  const i = Number(wrap.dataset.i);
  parts[i] = ta.value;
  const n = charLen(ta.value);
  const count = wrap.querySelector('.compose-part-count');
  count.textContent = `${n}/${MAX_CHARS}`;
  count.classList.toggle('over', n > MAX_CHARS);
  renderPreview();
});

document.getElementById('compose-parts').addEventListener('click', e => {
  const btn = e.target.closest('[data-remove]');
  if (!btn) return;
  readPartsFromDOM();
  const i = Number(btn.dataset.remove);
  if (parts.length <= 1) return;
  parts.splice(i, 1);
  renderParts();
});

document.getElementById('btn-add-part').onclick = () => {
  readPartsFromDOM();
  if (parts.length >= 12) return Threads.toast('Maksimal 12 bagian', false);
  parts.push('');
  renderParts();
  document.querySelector(`.compose-part[data-i="${parts.length - 1}"] textarea`)?.focus();
};

async function publishChain() {
  if (!(await Threads.requireConnected())) return;
  readPartsFromDOM();
  const texts = parts.map(p => p.trim()).filter(Boolean);
  if (!texts.length) return Threads.toast('Isi minimal 1 bagian', false);
  for (let i = 0; i < texts.length; i++) {
    if (charLen(texts[i]) > MAX_CHARS) {
      return Threads.toast(`Bagian ${i + 1} melebihi ${MAX_CHARS} karakter`, false);
    }
  }

  const replyControl = document.getElementById('compose-reply-control').value;
  const status = document.getElementById('container-status');
  const btn = document.getElementById('btn-publish');
  const btnC = document.getElementById('btn-container');
  btn.disabled = true;
  btnC.disabled = true;

  try {
    const mediaURL = document.getElementById('compose-media-url').value.trim();
    const data = await Threads.api('/api/publish', {
      method: 'POST',
      body: JSON.stringify({
        media_type: mediaType(),
        text: texts[0],
        parts: texts,
        publish: true,
        reply_control: replyControl || undefined,
        image_url: mediaType() === 'IMAGE' ? mediaURL : undefined,
        video_url: mediaType() === 'VIDEO' ? mediaURL : undefined,
        reply_to_id: document.getElementById('compose-reply-to').value.trim() || undefined,
      }),
    });
    const id = publishedId(data);
    status.textContent = id
      ? `Dijadwalkan lewat Repliz · ${id}`
      : 'Dijadwalkan lewat Repliz';
    Threads.toast(texts.length > 1 ? `Utas ${texts.length} bagian dijadwalkan di Repliz` : 'Post dijadwalkan di Repliz', true);
  } catch (e) {
    status.textContent = 'Error: ' + e.message;
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
    btnC.disabled = false;
    syncMeta();
  }
}

async function createContainerOnly() {
  Threads.toast('Repliz tidak memakai container Meta. Pakai Publikasikan untuk jadwal Repliz.', false);
}

document.getElementById('btn-container').onclick = () =>
  createContainerOnly().catch(e => Threads.toast(e.message, false));
document.getElementById('btn-publish').onclick = () =>
  publishChain().catch(e => Threads.toast(e.message, false));

function schedulePayload() {
  readPartsFromDOM();
  const cleaned = parts.map((p) => String(p || '').trim()).filter(Boolean);
  if (!cleaned.length) throw new Error('Isi minimal bagian 1');
  for (const p of cleaned) {
    if (charLen(p) > MAX_CHARS) throw new Error('Ada bagian lebih dari 500 karakter');
  }
  const runAt = document.getElementById('schedule-at')?.value?.trim();
  if (!runAt) throw new Error('Pilih tanggal & jam jadwal');
  // datetime-local = jam dinding WIB (label UI). Kirim offset eksplisit agar
  // server tidak salah interpretasi sebagai UTC.
  const body = {
    run_at: toWIBRFC3339(runAt),
    media_type: mediaType(),
    parts: cleaned,
    text: cleaned[0],
    reply_control: document.getElementById('compose-reply-control').value,
    reply_to_id: document.getElementById('compose-reply-to').value.trim() || undefined,
  };
  const mediaURL = document.getElementById('compose-media-url').value.trim();
  if (body.media_type === 'IMAGE') body.image_url = mediaURL;
  if (body.media_type === 'VIDEO') body.video_url = mediaURL;
  return body;
}

function fmtRunAt(iso) {
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', dateStyle: 'medium', timeStyle: 'short' });
  } catch {
    return iso;
  }
}

async function loadScheduleList() {
  const root = document.getElementById('schedule-list');
  if (!root) return;
  try {
    const data = await Threads.api('/api/schedule?all=1');
    const posts = Array.isArray(data?.posts) ? data.posts : [];
    const pending = posts.filter((p) => p.status === 'pending' || p.status === 'running');
    const recent = posts.filter((p) => p.status !== 'pending' && p.status !== 'running').slice(-5).reverse();
    const show = [...pending, ...recent];
    if (!show.length) {
      root.innerHTML = `<p class="buat-sched-empty">Belum ada jadwal.</p>`;
      return;
    }
    root.innerHTML = show.map((p) => {
      const preview = Threads.escapeHtml((p.text || p.parts?.[0] || '').slice(0, 120));
      const st = Threads.escapeHtml(p.status || '');
      const when = Threads.escapeHtml(fmtRunAt(p.run_at));
      const canCancel = p.status === 'pending';
      return `<article class="buat-sched-item">
        <div class="buat-sched-main">
          <p class="buat-sched-when">${when}</p>
          <p class="buat-sched-text">${preview || '—'}</p>
          <div class="buat-sched-meta"><span class="th-chip">${st}</span></div>
          ${p.error ? `<p class="buat-sched-err">${Threads.escapeHtml(p.error)}</p>` : ''}
        </div>
        ${canCancel ? `<button type="button" class="th-btn th-btn-ghost text-xs" data-cancel-sched="${Threads.escapeHtml(p.id)}">Batal</button>` : ''}
      </article>`;
    }).join('');
    root.querySelectorAll('[data-cancel-sched]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const id = btn.getAttribute('data-cancel-sched');
        try {
          await Threads.api('/api/schedule/' + encodeURIComponent(id) + '/cancel', { method: 'POST', body: '{}' });
          Threads.toast('Jadwal dibatalkan', true);
          loadScheduleList();
        } catch (e) {
          Threads.toast(e.message, false);
        }
      });
    });
  } catch (e) {
    root.innerHTML = `<p class="buat-sched-empty">${Threads.escapeHtml(e.message)}</p>`;
  }
}

document.getElementById('btn-schedule')?.addEventListener('click', async () => {
  const btn = document.getElementById('btn-schedule');
  btn.disabled = true;
  try {
    const body = schedulePayload();
    const data = await Threads.api('/api/schedule', { method: 'POST', body: JSON.stringify(body) });
    Threads.toast('Terjadwal: ' + fmtRunAt(data?.post?.run_at), true);
    document.getElementById('container-status').textContent =
      'Terjadwal ' + fmtRunAt(data?.post?.run_at) + ' · id ' + (data?.post?.id || '');
    loadScheduleList();
  } catch (e) {
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
  }
});
document.getElementById('btn-schedule-refresh')?.addEventListener('click', () => loadScheduleList());
setScheduleMinNow();
loadScheduleList();
setInterval(loadScheduleList, 60000);
setInterval(setScheduleMinNow, 60000);

document.getElementById('btn-repost').onclick = async () => {
  const media_id = document.getElementById('repost-id').value.trim();
  if (!media_id) return Threads.toast('Isi ID post', false);
  try {
    await Threads.api('/api/repost', { method: 'POST', body: JSON.stringify({ media_id }) });
    Threads.toast('Repost berhasil', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

function setComposeImageURL(url) {
  const input = document.getElementById('compose-media-url');
  const box = document.getElementById('compose-thumb-box');
  const img = document.getElementById('compose-thumb-img');
  const hint = document.getElementById('compose-upload-hint');
  if (!url) {
    if (box) box.hidden = true;
    if (input) input.value = '';
    if (hint) {
      hint.hidden = true;
      hint.textContent = '';
    }
    renderPreview();
    return;
  }
  if (input) input.value = url;
  document.getElementById('mt-img')?.click();
  if (box && img) {
    try {
      const u = new URL(url, location.origin);
      img.src = u.origin === location.origin ? u.pathname : url;
    } catch {
      img.src = url;
    }
    box.hidden = false;
  }
  renderPreview();
}

async function uploadComposeImage(file) {
  if (!file) return;
  if (!file.type.startsWith('image/')) {
    Threads.toast('Pilih file gambar (jpg/png/webp/gif)', false);
    return;
  }
  const hint = document.getElementById('compose-upload-hint');
  const btnLabel = document.querySelector('.buat-upload-btn');
  if (hint) {
    hint.hidden = false;
    hint.textContent = 'Mengupload ' + file.name + '…';
  }
  if (btnLabel) btnLabel.classList.add('is-busy');
  try {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch('/api/upload/image', {
      method: 'POST',
      body: fd,
      credentials: 'same-origin',
    });
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
    if (!res.ok) {
      throw new Error(data?.error || text || ('upload gagal ' + res.status));
    }
    const url = data.image_url || data.path;
    if (!url) throw new Error('URL upload kosong');
    setComposeImageURL(url);
    if (hint) {
      hint.hidden = false;
      hint.textContent = 'Terupload — mode Gambar aktif. Siap publish/jadwalkan.';
    }
    Threads.toast('Gambar terupload', true);
  } catch (e) {
    if (hint) {
      hint.hidden = false;
      hint.textContent = e.message || 'Upload gagal';
    }
    Threads.toast(e.message || e, false);
  } finally {
    if (btnLabel) btnLabel.classList.remove('is-busy');
    const input = document.getElementById('compose-upload');
    if (input) input.value = '';
  }
}

document.getElementById('compose-upload')?.addEventListener('change', (e) => {
  const file = e.target?.files?.[0];
  uploadComposeImage(file);
});

document.getElementById('btn-clear-media')?.addEventListener('click', () => {
  setComposeImageURL('');
  document.getElementById('mt-text')?.click();
});

// Drag & drop ke blok media
(() => {
  const block = document.getElementById('buat-media-block');
  if (!block) return;
  ['dragenter', 'dragover'].forEach((ev) => {
    block.addEventListener(ev, (e) => {
      e.preventDefault();
      block.classList.add('is-drop');
    });
  });
  ['dragleave', 'drop'].forEach((ev) => {
    block.addEventListener(ev, (e) => {
      e.preventDefault();
      block.classList.remove('is-drop');
    });
  });
  block.addEventListener('drop', (e) => {
    const file = e.dataTransfer?.files?.[0];
    if (file) uploadComposeImage(file);
  });
})();

document.getElementById('btn-gen-thumb').onclick = async () => {
  readPartsFromDOM();
  const hook = (parts[0] || '').trim();
  if (!hook) return Threads.toast('Isi bagian 1 (hook) dulu', false);
  const btn = document.getElementById('btn-gen-thumb');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Generating…';
  try {
    const data = await Threads.api('/api/ai/thumbnail', {
      method: 'POST',
      body: JSON.stringify({ hook, size: '1024x768', quality: 'high', crop_4_3: true }),
    });
    const url = data.path || data.local_path || data.image_url;
    if (!url) throw new Error('URL thumbnail kosong');
    setComposeImageURL(url);
    Threads.toast('Thumbnail 4:3 siap — mode Gambar aktif', true);
  } catch (e) {
    Threads.toast(e.message || e, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-image"></i> Thumbnail dari hook';
  }
};

document.getElementById('compose-media-url').addEventListener('input', () => {
  const url = document.getElementById('compose-media-url').value.trim();
  if (url) setComposeImageURL(url);
  else {
    const box = document.getElementById('compose-thumb-box');
    if (box) box.hidden = true;
  }
  renderPreview();
});

document.querySelectorAll('input[name="mtype"]').forEach((el) => {
  el.addEventListener('change', () => renderPreview());
});

(async function loadAccountChip() {
  try {
    const accs = await Threads.api('/api/repliz/accounts');
    const list = accs.accounts || [];
    const id = accs.active_id || '';
    const a = list.find((x) => (x.id || x._id) === id) || list[0];
    const name = a?.username || a?.name || '';
    if (name) {
      previewHandle = '@' + String(name).replace(/^@/, '');
      previewInitials = previewHandle.replace(/^@/, '').slice(0, 2).toUpperCase();
      const el = document.getElementById('preview-account');
      if (el) el.textContent = previewHandle;
      renderPreview();
    }
  } catch {}
})();

(function loadAIDraft() {
  try {
    let loaded = null;
    const rawParts = localStorage.getItem('threads_compose_parts');
    if (rawParts) {
      const parsed = JSON.parse(rawParts);
      if (Array.isArray(parsed) && parsed.length) loaded = parsed;
    }
    if (!loaded) {
      const draft = localStorage.getItem('threads_compose_draft');
      if (draft) {
        loaded = draft.split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
        if (!loaded.length) loaded = [draft.slice(0, MAX_CHARS)];
      }
    }
    const imageURL = localStorage.getItem('threads_compose_image_url') || '';
    localStorage.removeItem('threads_compose_parts');
    localStorage.removeItem('threads_compose_draft');
    localStorage.removeItem('threads_compose_image_url');
    if (loaded?.length) {
      setParts(loaded);
      if (imageURL) setComposeImageURL(imageURL);
      Threads.toast(
        imageURL
          ? `Draf AI + thumbnail dimuat — ${parts.length} bagian`
          : `Draf AI dimuat — ${parts.length} bagian utas`,
        true,
      );
      return;
    }
  } catch {}
  try {
    const hook = new URLSearchParams(location.search).get('hook');
    if (hook && !(parts[0] || '').trim()) {
      parts[0] = hook.slice(0, 500);
    }
  } catch {}
  renderParts();
})();
