Threads.pageShell('thumbnail');

const PREF_KEY = 'threads_thumb_lab_prefs';
const HIST_KEY = 'threads_thumb_lab_hist';
const MAX_HIST = 12;

let defaults = null;
let lastResult = null;

function showAlert(msg) {
  const el = document.getElementById('thumb-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function fillSelect(sel, options, value) {
  const list = [...options];
  if (value && !list.includes(value)) list.unshift(value);
  sel.innerHTML = list.map(o =>
    `<option value="${Threads.escapeHtml(o)}">${Threads.escapeHtml(o)}</option>`
  ).join('');
  if (value) sel.value = value;
}

function loadPrefs() {
  try {
    return JSON.parse(localStorage.getItem(PREF_KEY) || '{}') || {};
  } catch {
    return {};
  }
}

function savePrefs() {
  const prefs = {
    model: document.getElementById('model').value,
    size: document.getElementById('size').value,
    sizeCustom: document.getElementById('size-custom').value.trim(),
    quality: document.getElementById('quality').value,
    crop43: document.getElementById('crop43').checked,
    extra: document.getElementById('extra').value,
    customOnly: document.getElementById('custom-only').checked,
  };
  localStorage.setItem(PREF_KEY, JSON.stringify(prefs));
  Threads.toast('Preferensi lab disimpan di browser', true);
}

function loadHist() {
  try {
    const arr = JSON.parse(localStorage.getItem(HIST_KEY) || '[]');
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

function saveHist(arr) {
  localStorage.setItem(HIST_KEY, JSON.stringify(arr.slice(0, MAX_HIST)));
}

function renderHist() {
  const hist = loadHist();
  document.getElementById('hist-count').textContent = String(hist.length);
  const root = document.getElementById('history');
  if (!hist.length) {
    root.innerHTML = '<p class="text-sm text-muted m-0">Riwayat kosong. Tiap generate berhasil masuk sini biar bisa dibandingin.</p>';
    return;
  }
  root.innerHTML = hist.map((h, i) => `
    <button type="button" class="thumb-lab-card" data-hist="${i}">
      <img src="${Threads.escapeHtml(h.path || h.image_url)}" alt="">
      <div class="thumb-lab-card-meta">
        <strong>${Threads.escapeHtml(h.model || '?')}</strong>
        <span>${Threads.escapeHtml(h.req_size || '')} → ${Threads.escapeHtml(h.size || '')}</span>
        <span>${Threads.escapeHtml(h.quality || '')}${h.crop_4_3 === false ? ' · no crop' : ' · 4:3'}</span>
      </div>
    </button>
  `).join('');
}

function showResult(data, reqMeta) {
  lastResult = { ...data, ...reqMeta };
  document.getElementById('preview-empty').hidden = true;
  const box = document.getElementById('preview-box');
  box.hidden = false;
  const img = document.getElementById('preview-img');
  img.src = data.path || data.image_url;
  document.getElementById('preview-actions').hidden = false;
  document.getElementById('btn-open').href = data.path || data.image_url;
  document.getElementById('last-meta').textContent =
    `${data.model} · ${reqMeta.req_size || ''} → ${data.width}×${data.height} · ${reqMeta.quality || ''}`;
  document.getElementById('prompt-out').textContent = data.prompt || '';
}

document.getElementById('btn-gen').onclick = async () => {
  showAlert('');
  if (!(await Threads.requireConnected())) return;
  const hook = document.getElementById('hook').value.trim();
  const customOnly = document.getElementById('custom-only').checked;
  const extra = document.getElementById('extra').value.trim();
  if (!customOnly && !hook) return Threads.toast('Isi hook dulu', false);
  if (customOnly && !extra) return Threads.toast('Isi prompt custom di Extra', false);

  const sizeCustom = document.getElementById('size-custom').value.trim();
  const reqSize = sizeCustom || document.getElementById('size').value;
  const body = {
    hook,
    model: document.getElementById('model').value,
    size: reqSize,
    quality: document.getElementById('quality').value,
    crop_4_3: document.getElementById('crop43').checked,
    extra,
    custom_only: customOnly,
  };

  const btn = document.getElementById('btn-gen');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Generating…';
  showAlert('Memanggil OpenAI Images… ini bisa 10–40 detik.');
  try {
    const data = await Threads.api('/api/ai/thumbnail', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const meta = {
      req_size: reqSize,
      quality: body.quality,
      crop_4_3: body.crop_4_3,
    };
    showResult(data, meta);
    const hist = loadHist();
    hist.unshift({
      path: data.path,
      image_url: data.image_url,
      model: data.model,
      size: data.size,
      width: data.width,
      height: data.height,
      req_size: reqSize,
      quality: body.quality,
      crop_4_3: body.crop_4_3,
      hook: hook.slice(0, 120),
      at: Date.now(),
    });
    saveHist(hist);
    renderHist();
    showAlert('');
    Threads.toast('Thumbnail siap', true);
  } catch (e) {
    showAlert(e.message || String(e));
    Threads.toast(e.message || e, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-image"></i> Generate';
  }
};

document.getElementById('btn-save-prefs').onclick = () => savePrefs();

document.getElementById('btn-clear-hist').onclick = () => {
  localStorage.removeItem(HIST_KEY);
  renderHist();
  Threads.toast('Riwayat dibersihkan', true);
};

document.getElementById('btn-copy-url').onclick = async () => {
  const url = lastResult?.image_url || lastResult?.path;
  if (!url) return;
  try {
    await navigator.clipboard.writeText(url);
    Threads.toast('URL disalin', true);
  } catch {
    Threads.toast('Gagal salin', false);
  }
};

document.getElementById('btn-to-buat').onclick = () => {
  const url = lastResult?.image_url || lastResult?.path;
  if (!url) return Threads.toast('Belum ada hasil', false);
  localStorage.setItem('threads_compose_image_url', url);
  location.href = '/buat.html?from=thumb';
};

document.getElementById('history').addEventListener('click', e => {
  const btn = e.target.closest('[data-hist]');
  if (!btn) return;
  const h = loadHist()[Number(btn.dataset.hist)];
  if (!h) return;
  showResult(h, {
    req_size: h.req_size,
    quality: h.quality,
    crop_4_3: h.crop_4_3,
  });
});

(async () => {
  try {
    defaults = await Threads.api('/api/ai/thumbnail/defaults');
  } catch {
    defaults = {
      enabled: false,
      model: 'gpt-image-1',
      size: '1536x1024',
      quality: 'medium',
      models: ['gpt-image-1', 'gpt-image-1-mini', 'gpt-image-1.5', 'gpt-image-2', 'dall-e-3'],
      sizes: ['1024x1024', '1536x1024', '1024x1536', '1536x1152', '1792x1024'],
      qualities: ['low', 'medium', 'high', 'auto'],
    };
  }

  const prefs = loadPrefs();
  fillSelect(document.getElementById('model'), defaults.models || [], prefs.model || defaults.model);
  fillSelect(document.getElementById('size'), defaults.sizes || [], prefs.size || defaults.size);
  fillSelect(document.getElementById('quality'), defaults.qualities || [], prefs.quality || defaults.quality);
  if (prefs.sizeCustom) document.getElementById('size-custom').value = prefs.sizeCustom;
  if (typeof prefs.crop43 === 'boolean') document.getElementById('crop43').checked = prefs.crop43;
  if (prefs.extra) document.getElementById('extra').value = prefs.extra;
  if (prefs.customOnly) document.getElementById('custom-only').checked = true;

  const st = document.getElementById('thumb-status');
  if (defaults.enabled) {
    st.textContent = `OpenAI OK · default ${defaults.model}`;
    st.classList.add('ok');
  } else {
    st.textContent = 'OPENAI_API_KEY belum aktif';
    st.classList.add('bad');
  }

  renderHist();
})();
