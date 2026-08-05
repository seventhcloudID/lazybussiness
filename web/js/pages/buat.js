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
  let prevId = document.getElementById('compose-reply-to').value.trim();
  const status = document.getElementById('container-status');
  const btn = document.getElementById('btn-publish');
  const btnC = document.getElementById('btn-container');
  btn.disabled = true;
  btnC.disabled = true;

  const log = [];
  try {
    for (let i = 0; i < texts.length; i++) {
      status.textContent = `Mempublish bagian ${i + 1}/${texts.length}…\n` + log.join('\n');
      const isRoot = i === 0 && !prevId;
      const body = {
        media_type: isRoot ? mediaType() : 'TEXT',
        text: texts[i],
        publish: true,
      };
      if (i === 0 && replyControl) body.reply_control = replyControl;
      if (prevId) body.reply_to_id = prevId;
      if (isRoot) {
        const mediaURL = document.getElementById('compose-media-url').value.trim();
        if (body.media_type === 'IMAGE') body.image_url = mediaURL;
        if (body.media_type === 'VIDEO') body.video_url = mediaURL;
      }
      const data = await Threads.api('/api/publish', {
        method: 'POST',
        body: JSON.stringify(body),
      });
      const id = publishedId(data);
      if (!id) throw new Error(`Bagian ${i + 1}: tidak dapat ID publish`);
      log.push(`${i + 1}. ${id}`);
      prevId = id;
      if (i < texts.length - 1) await new Promise(r => setTimeout(r, 600));
    }
    status.textContent = `Utas terpublish (${texts.length} bagian):\n` + log.join('\n');
    Threads.toast(texts.length > 1 ? `Utas ${texts.length} bagian terpublish` : 'Post dipublikasikan', true);
  } catch (e) {
    status.textContent = (log.length ? `Sebagian terkirim:\n${log.join('\n')}\n\n` : '') + 'Error: ' + e.message;
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
    btnC.disabled = false;
    syncMeta();
  }
}

async function createContainerOnly() {
  if (!(await Threads.requireConnected())) return;
  readPartsFromDOM();
  const text = (parts[0] || '').trim();
  if (!text) return Threads.toast('Isi bagian 1 dulu', false);
  if (charLen(text) > MAX_CHARS) return Threads.toast(`Melebihi ${MAX_CHARS} karakter`, false);

  const body = {
    media_type: mediaType(),
    text,
    reply_control: document.getElementById('compose-reply-control').value,
    reply_to_id: document.getElementById('compose-reply-to').value.trim() || undefined,
    publish: false,
  };
  const mediaURL = document.getElementById('compose-media-url').value.trim();
  if (body.media_type === 'IMAGE') body.image_url = mediaURL;
  if (body.media_type === 'VIDEO') body.video_url = mediaURL;

  const data = await Threads.api('/api/publish', {
    method: 'POST',
    body: JSON.stringify(body),
  });
  const id = data?.container?.id || '';
  document.getElementById('container-status').textContent =
    'Container bagian 1: ' + (id || JSON.stringify(data.container));
  Threads.toast('Container dibuat', true);
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
  const body = {
    run_at: runAt.length === 16 ? runAt : runAt.slice(0, 16),
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
loadScheduleList();
setInterval(loadScheduleList, 60000);

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
  if (!url) {
    if (box) box.hidden = true;
    return;
  }
  if (input) input.value = url;
  document.getElementById('mt-img')?.click();
  if (box && img) {
    // Prefer relative path for preview if absolute same-origin
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
      body: JSON.stringify({ hook }),
    });
    const url = data.image_url || data.path;
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
    const st = await Threads.api('/api/status');
    const name = st.account_name || st.threads_username || '';
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
  renderParts();
})();
