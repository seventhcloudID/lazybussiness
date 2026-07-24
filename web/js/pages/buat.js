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

function syncMeta() {
  const n = parts.length;
  document.getElementById('parts-meta').textContent =
    n === 1 ? '1 bagian (post tunggal)' : `${n} bagian (utas berantai)`;
  document.getElementById('btn-publish').innerHTML = n > 1
    ? `<i class="bi bi-send"></i> Publikasikan utas (${n})`
    : `<i class="bi bi-send"></i> Publikasikan`;
}

function renderParts() {
  const root = document.getElementById('compose-parts');
  root.innerHTML = parts.map((text, i) => {
    const n = charLen(text);
    const over = n > MAX_CHARS;
    return `
      <div class="compose-part" data-i="${i}">
        <div class="compose-part-head">
          <span class="compose-part-n">${i + 1}</span>
          <span class="compose-part-label">${i === 0 ? 'Hook / starter' : `Lanjutan ${i + 1}`}</span>
          <span class="compose-part-count ${over ? 'over' : ''}">${n}/${MAX_CHARS}</span>
          ${parts.length > 1 ? `<button type="button" class="th-btn th-btn-ghost !py-0.5 !px-2 text-xs" data-remove="${i}" title="Hapus"><i class="bi bi-x-lg"></i></button>` : ''}
        </div>
        <textarea class="th-textarea compose-part-text" rows="${i === 0 ? 4 : 3}" maxlength="${MAX_CHARS}"
          placeholder="${i === 0 ? 'Bagian 1 — hook…' : `Bagian ${i + 1}…`}">${Threads.escapeHtml(text)}</textarea>
      </div>`;
  }).join('');
  syncMeta();
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
    localStorage.removeItem('threads_compose_parts');
    localStorage.removeItem('threads_compose_draft');
    if (loaded?.length) {
      setParts(loaded);
      Threads.toast(`Draf AI dimuat — ${parts.length} bagian utas`, true);
      return;
    }
  } catch {}
  renderParts();
})();
