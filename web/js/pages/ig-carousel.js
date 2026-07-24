Threads.pageShell('ig-carousel');

const MAX_SLIDES = 10;
const MAX_CHARS = 500;
let slides = [];
let previewIdx = 0;
let syncing = false;

function showAlert(msg) {
  const el = document.getElementById('ig-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function ensureSlides() {
  if (slides.length >= 2) return;
  slides = [
    { text: '', image_url: '' },
    { text: '', image_url: '' },
  ];
}

function partsFromSlides() {
  return slides.map(s => String(s.text || '').trim()).filter(Boolean);
}

function setSlidesFromParts(parts, keepImages = false) {
  const prev = keepImages ? slides : [];
  slides = (parts || []).map((t, i) => ({
    text: String(t || '').trim(),
    image_url: prev[i]?.image_url || '',
  })).filter(s => s.text);
  ensureSlides();
}

function readEditorToSlide() {
  if (!slides[previewIdx]) return;
  slides[previewIdx] = {
    text: document.getElementById('f-text').value,
    image_url: document.getElementById('f-image').value,
  };
}

function writeSlideToEditor() {
  const s = slides[previewIdx] || { text: '', image_url: '' };
  syncing = true;
  document.getElementById('f-text').value = s.text || '';
  document.getElementById('f-image').value = s.image_url || '';
  document.getElementById('text-count').textContent = `${Array.from(s.text || '').length}/${MAX_CHARS}`;
  document.getElementById('editor-title').textContent = `Edit slide ${previewIdx + 1}`;
  document.getElementById('btn-remove-slide').disabled = slides.length <= 2;
  syncing = false;
}

function renderFilmstrip() {
  const root = document.getElementById('filmstrip');
  document.getElementById('film-label').textContent = `${slides.length} slide`;
  root.innerHTML = slides.map((s, i) => {
    const active = i === previewIdx ? ' active' : '';
    const filled = s.text ? ' filled' : '';
    const hasImg = (s.image_url || '').trim() ? ' has-img' : '';
    const snip = (s.text || 'Kosong').replace(/\s+/g, ' ').slice(0, 48);
    return `
      <button type="button" class="igc-thumb${active}${filled}${hasImg}" data-jump="${i}">
        <span class="igc-thumb-n">${i + 1}</span>
        <span class="igc-thumb-title">${Threads.escapeHtml(snip)}</span>
      </button>`;
  }).join('');
  root.querySelector('.igc-thumb.active')?.scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: 'smooth' });
}

function brandHandle() {
  const raw = (document.getElementById('brand')?.value || '').trim().replace(/^@+/, '');
  return raw ? `@${raw}` : '';
}

function renderPreview() {
  if (previewIdx >= slides.length) previewIdx = slides.length - 1;
  if (previewIdx < 0) previewIdx = 0;
  const s = slides[previewIdx] || {};
  const card = document.getElementById('preview-card');
  card.classList.remove('igc-flip');
  void card.offsetWidth;
  card.classList.add('igc-flip');

  const handle = brandHandle();
  const handleEl = document.getElementById('preview-handle');
  const dividerEl = document.getElementById('preview-divider');
  if (handle) {
    handleEl.hidden = false;
    handleEl.textContent = handle;
    dividerEl.hidden = false;
  } else {
    handleEl.hidden = true;
    handleEl.textContent = '';
    dividerEl.hidden = true;
  }

  document.getElementById('preview-text').textContent = s.text || 'Isi slide (= bagian utas) muncul di sini.';
  document.getElementById('preview-meta').textContent = `${previewIdx + 1} / ${slides.length}`;
  document.getElementById('preview-dots').innerHTML = slides.map((_, i) =>
    `<button type="button" class="${i === previewIdx ? 'on' : ''}" data-dot="${i}"></button>`
  ).join('');
}

function selectSlide(i) {
  readEditorToSlide();
  previewIdx = Math.max(0, Math.min(i, slides.length - 1));
  writeSlideToEditor();
  renderFilmstrip();
  renderPreview();
}

function refreshAll() {
  ensureSlides();
  writeSlideToEditor();
  renderFilmstrip();
  renderPreview();
}

['f-text', 'f-image'].forEach(id => {
  document.getElementById(id).addEventListener('input', () => {
    if (syncing) return;
    if (id === 'f-text') {
      document.getElementById('text-count').textContent =
        `${Array.from(document.getElementById('f-text').value).length}/${MAX_CHARS}`;
    }
    readEditorToSlide();
    renderFilmstrip();
    renderPreview();
  });
});

document.getElementById('brand').addEventListener('input', () => renderPreview());

document.getElementById('btn-save-brand').onclick = async () => {
  const brand = document.getElementById('brand').value.trim();
  try {
    await Threads.api('/api/ai/brand', {
      method: 'PUT',
      body: JSON.stringify({ brand }),
    });
    renderPreview();
    Threads.toast(brand ? 'Brand disimpan' : 'Brand dikosongkan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('filmstrip').addEventListener('click', e => {
  const btn = e.target.closest('[data-jump]');
  if (btn) selectSlide(Number(btn.dataset.jump));
});
document.getElementById('preview-dots').addEventListener('click', e => {
  const btn = e.target.closest('[data-dot]');
  if (btn) selectSlide(Number(btn.dataset.dot));
});
document.getElementById('btn-prev').onclick = () =>
  selectSlide((previewIdx - 1 + slides.length) % slides.length);
document.getElementById('btn-next').onclick = () =>
  selectSlide((previewIdx + 1) % slides.length);

document.getElementById('btn-add-slide').onclick = () => {
  readEditorToSlide();
  if (slides.length >= MAX_SLIDES) return Threads.toast('Maksimal 10 slide', false);
  slides.push({ text: '', image_url: '' });
  selectSlide(slides.length - 1);
};

document.getElementById('btn-remove-slide').onclick = () => {
  if (slides.length <= 2) return;
  readEditorToSlide();
  slides.splice(previewIdx, 1);
  selectSlide(Math.min(previewIdx, slides.length - 1));
};

document.getElementById('caption').addEventListener('input', e => {
  document.getElementById('caption-count').textContent = `${e.target.value.length}/2200`;
});

document.getElementById('btn-copy-caption').onclick = async () => {
  const t = document.getElementById('caption').value.trim();
  if (!t) return Threads.toast('Caption kosong', false);
  try {
    await navigator.clipboard.writeText(t);
    Threads.toast('Caption disalin', true);
  } catch {
    Threads.toast('Gagal salin', false);
  }
};

document.getElementById('btn-copy-slides').onclick = async () => {
  readEditorToSlide();
  const text = partsFromSlides().map((t, i) => `[${i + 1}]\n${t}`).join('\n\n');
  try {
    await navigator.clipboard.writeText(text);
    Threads.toast('Teks slide disalin', true);
  } catch {
    Threads.toast('Gagal salin', false);
  }
};

document.getElementById('btn-to-threads').onclick = () => {
  readEditorToSlide();
  const parts = partsFromSlides();
  if (parts.length < 1) return Threads.toast('Isi slide dulu', false);
  localStorage.setItem('threads_compose_parts', JSON.stringify(parts));
  localStorage.setItem('threads_compose_draft', parts.join('\n\n'));
  location.href = '/buat.html?from=carousel';
};

document.getElementById('btn-from-utas').onclick = () => {
  try {
    const raw = localStorage.getItem('threads_carousel_parts') || localStorage.getItem('threads_compose_parts');
    if (!raw) return Threads.toast('Belum ada utas tersimpan — generate dulu', false);
    const parts = JSON.parse(raw);
    if (!Array.isArray(parts) || !parts.length) return Threads.toast('Utas kosong', false);
    setSlidesFromParts(parts);
    previewIdx = 0;
    refreshAll();
    Threads.toast(`Utas dimuat — ${slides.length} slide`, true);
  } catch {
    Threads.toast('Gagal baca utas', false);
  }
};

document.getElementById('btn-gen').onclick = async () => {
  showAlert('');
  const btn = document.getElementById('btn-gen');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> …';
  try {
    readEditorToSlide();
    const existing = partsFromSlides();
    const brand = document.getElementById('brand').value.trim();
    const body = {
      brand,
      topic: document.getElementById('topic').value.trim(),
      count: 6,
    };
    // Kalau sudah ada utas di editor, pakai itu; kalau tidak, generate utas baru.
    if (existing.length >= 2) body.parts = existing;

    if (brand) {
      await Threads.api('/api/ai/brand', {
        method: 'PUT',
        body: JSON.stringify({ brand }),
      });
    }

    const result = await Threads.api('/api/ai/carousel', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const parts = result.parts?.length
      ? result.parts
      : (result.slides || []).map(s => s.text || [s.headline, s.body].filter(Boolean).join('\n\n')).filter(Boolean);
    setSlidesFromParts(parts, true);
    document.getElementById('caption').value = result.caption || '';
    document.getElementById('caption-count').textContent = `${document.getElementById('caption').value.length}/2200`;
    document.getElementById('consideration').textContent =
      [result.title, result.consideration].filter(Boolean).join(' · ');
    if (result.brand) document.getElementById('brand').value = result.brand;
    localStorage.setItem('threads_carousel_parts', JSON.stringify(partsFromSlides()));
    previewIdx = 0;
    refreshAll();
    Threads.toast(`Siap — ${slides.length} slide dari utas`, true);
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-magic"></i> Generate utas → carousel';
  }
};

document.getElementById('btn-publish').onclick = async () => {
  showAlert('');
  readEditorToSlide();
  try {
    const st = await Threads.api('/api/ig/status');
    if (!st.connected) {
      Threads.toast('Hubungkan token IG dulu', false);
      location.href = '/ig-token.html';
      return;
    }
  } catch {
    return Threads.toast('Server belum jalan', false);
  }

  const urls = slides.map(s => (s.image_url || '').trim()).filter(Boolean);
  if (urls.length < 2) return Threads.toast('Isi minimal 2 URL gambar publik', false);
  if (urls.length !== slides.length) {
    if (!confirm(`Hanya ${urls.length}/${slides.length} slide punya URL. Publish ${urls.length}?`)) return;
  }

  const btn = document.getElementById('btn-publish');
  const status = document.getElementById('publish-status');
  btn.disabled = true;
  status.textContent = 'Upload ke Instagram…';
  try {
    const data = await Threads.api('/api/ig/carousel/publish', {
      method: 'POST',
      body: JSON.stringify({
        image_urls: urls,
        caption: document.getElementById('caption').value.trim(),
      }),
    });
    status.textContent = `Published · ${data.container || 'ok'}`;
    Threads.toast('Carousel terpublish', true);
  } catch (e) {
    status.textContent = 'Error: ' + e.message;
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
  }
};

document.addEventListener('keydown', e => {
  if (e.target.matches('input, textarea, select')) return;
  if (e.key === 'ArrowLeft') selectSlide((previewIdx - 1 + slides.length) % slides.length);
  if (e.key === 'ArrowRight') selectSlide((previewIdx + 1) % slides.length);
});

(async function init() {
  try {
    const mem = await Threads.api('/api/ai/memory');
    if (mem?.brand) document.getElementById('brand').value = mem.brand;
  } catch {}

  // Prefer package from Generate / Lazy
  try {
    const raw = localStorage.getItem('threads_carousel_parts');
    if (raw) {
      const parts = JSON.parse(raw);
      if (Array.isArray(parts) && parts.length) {
        setSlidesFromParts(parts);
        localStorage.removeItem('threads_carousel_parts');
        const cap = localStorage.getItem('threads_carousel_caption');
        if (cap) {
          document.getElementById('caption').value = cap;
          document.getElementById('caption-count').textContent = `${cap.length}/2200`;
          localStorage.removeItem('threads_carousel_caption');
        }
        Threads.toast(`Utas dimuat — ${slides.length} slide`, true);
      }
    }
  } catch {}

  try {
    const st = await Threads.api('/api/ig/status');
    if (!st.connected) {
      showAlert('Token IG belum terhubung — edit/teks tetap bisa, publish butuh IG Token.');
    }
  } catch {}

  refreshAll();
})();
