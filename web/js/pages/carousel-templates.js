Threads.pageShell('carousel-templates');

const PRESET_2 = `Orang bilang content harus konsisten tiap hari.

Yang lebih penting: punya sudut yang orang ingat.`;

const PRESET_3 = `Orang bilang content harus konsisten tiap hari.

Yang lebih penting: punya sudut yang orang ingat — bukan cuma rajin posting.

Kalau sudutnya jelas, audiens datang sendiri.`;

const FALLBACK_TEMPLATES = [
  { id: 'noir', name: 'Noir', desc: 'Gelap editorial + bar emas', swatch: ['#12161E', '#1C222E', '#E8A45A'] },
  { id: 'ink', name: 'Ink', desc: 'Hitam pekat, kontras tajam', swatch: ['#0A0A0A', '#141414', '#F5F5F5'] },
  { id: 'ocean', name: 'Ocean', desc: 'Teal dalam + mint', swatch: ['#0B2428', '#123A42', '#5EEAD4'] },
  { id: 'ember', name: 'Ember', desc: 'Charcoal + strip coral', swatch: ['#1A1412', '#2A1E1A', '#E85D4C'] },
  { id: 'paper', name: 'Kertas', desc: 'Editorial terang klasik', swatch: ['#F4F1EA', '#E8E2D6', '#1A1A1A'] },
  { id: 'bloom', name: 'Bloom', desc: 'Blush pink lembut', swatch: ['#FFF0F3', '#FFE0E8', '#E85A8C'], tag: 'cewe' },
  { id: 'lilac', name: 'Lilac', desc: 'Lavender dreamy', swatch: ['#F6F1FF', '#E9DEFF', '#8B5CF6'], tag: 'cewe' },
  { id: 'peach', name: 'Peach', desc: 'Peach soft + kartu putih', swatch: ['#FFF5EE', '#FFE4D1', '#F0785A'], tag: 'cewe' },
  { id: 'bold', name: 'Bold', desc: 'Header aksen + tipe besar', swatch: ['#111827', '#1F2937', '#F59E0B'] },
  { id: 'frame', name: 'Frame', desc: 'Bingkai majalah minimal', swatch: ['#FAFAF8', '#F0EDE6', '#0F172A'] },
  { id: 'meadow', name: 'Meadow', desc: 'Sage hijau + kartu', swatch: ['#E8EEE6', '#D5E0D2', '#2F4A3A'] },
  { id: 'midnight', name: 'Midnight', desc: 'Navy dalam + aksen biru', swatch: ['#0B1220', '#152238', '#60A5FA'] },
  { id: 'coral', name: 'Coral', desc: 'Header coral cerah', swatch: ['#1F1412', '#2C1C18', '#FF7A59'] },
  { id: 'mint', name: 'Mint', desc: 'Mint segar lembut', swatch: ['#ECFDF5', '#D1FAE5', '#059669'], tag: 'cewe' },
  { id: 'cherry', name: 'Cherry', desc: 'Wine soft feminine', swatch: ['#FFF1F2', '#FFE4E6', '#BE123C'], tag: 'cewe' },
  { id: 'sand', name: 'Sand', desc: 'Pasir hangat editorial', swatch: ['#FAF6F0', '#F0E6D8', '#8B6914'] },
  { id: 'neon', name: 'Neon', desc: 'Gelap cyber + hijau neon', swatch: ['#05080A', '#0D1512', '#39FF14'] },
  { id: 'slate', name: 'Slate', desc: 'Abu dingin modern', swatch: ['#F1F5F9', '#E2E8F0', '#334155'] },
  { id: 'honey', name: 'Honey', desc: 'Amber manis + quote', swatch: ['#FFFBEB', '#FEF3C7', '#D97706'], tag: 'cewe' },
  { id: 'mono', name: 'Mono', desc: 'Putih bersih + bingkai hitam', swatch: ['#FFFFFF', '#F4F4F5', '#09090B'] },
];

let templates = FALLBACK_TEMPLATES.slice();
let savedTemplate = 'noir';
let selectedTemplate = 'noir';
let brand = 'brandmu';
let testText = PRESET_2;
const previewUrls = {};
let heroUrl = '';
let debounceTimer = null;
let heroSeq = 0;
let gallerySeq = 0;

function showAlert(msg) {
  const el = document.getElementById('tpl-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function metaOf(id) {
  return templates.find(t => t.id === id) || FALLBACK_TEMPLATES.find(t => t.id === id) || { id, name: id, desc: '' };
}

function currentBrand() {
  return (document.getElementById('tpl-brand')?.value || brand || 'brandmu').trim().replace(/^@+/, '') || 'brandmu';
}

function currentText() {
  const t = (document.getElementById('tpl-text')?.value || '').trim();
  return t || 'Isi slide muncul di sini.';
}

function updateTextCount() {
  const el = document.getElementById('tpl-text');
  const n = Array.from(el.value || '').length;
  document.getElementById('tpl-text-count').textContent = `${n}/500`;
}

function updateActiveBar() {
  const m = metaOf(selectedTemplate);
  document.getElementById('tpl-active-name').textContent = m.name || selectedTemplate;
  document.getElementById('tpl-active-desc').textContent = m.desc || '';
  const dirty = selectedTemplate !== savedTemplate;
  const btn = document.getElementById('btn-apply');
  btn.disabled = !dirty;
  document.getElementById('tpl-save-hint').textContent = dirty
    ? `Belum disimpan — ganti dari ${metaOf(savedTemplate).name}`
    : 'Sudah aktif di workspace ini';
  document.getElementById('tpl-hero-cap').textContent =
    `${m.name || selectedTemplate} · PNG 1080×1350 · @${currentBrand()}`;
}

function renderGallery() {
  const root = document.getElementById('tpl-gallery');
  root.innerHTML = templates.map(t => {
    const on = t.id === selectedTemplate ? ' is-on' : '';
    const src = previewUrls[t.id] || '';
    const tag = t.tag ? `<span class="tpl-card-tag">${Threads.escapeHtml(t.tag)}</span>` : '';
    return `
      <button type="button" class="tpl-card${on}" data-tpl="${Threads.escapeHtml(t.id)}">
        <span class="tpl-card-frame">
          ${src
            ? `<img class="tpl-card-img" src="${src}" alt="Preview ${Threads.escapeHtml(t.name)}" loading="lazy">`
            : `<span class="tpl-card-skel" style="--c1:${t.swatch?.[0] || '#222'};--c2:${t.swatch?.[1] || '#333'};--ca:${t.swatch?.[2] || '#999'}"></span>`}
          ${t.id === savedTemplate ? '<span class="tpl-card-badge">Aktif</span>' : ''}
          ${tag}
        </span>
        <span class="tpl-card-meta">
          <strong>${Threads.escapeHtml(t.name || t.id)}</strong>
          <small>${Threads.escapeHtml(t.desc || '')}</small>
        </span>
      </button>`;
  }).join('');
  updateActiveBar();
}

async function fetchSlidePng(templateId, text, brandName) {
  const res = await fetch('/api/ig/carousel/render', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      text,
      brand: brandName,
      template: templateId,
      index: 0,
      total: 5,
    }),
  });
  if (res.status === 401) {
    location.replace('/login.html?next=' + encodeURIComponent(location.pathname));
    return null;
  }
  if (!res.ok) throw new Error(await res.text());
  return res.blob();
}

async function refreshHero() {
  const seq = ++heroSeq;
  const loading = document.getElementById('tpl-hero-loading');
  const img = document.getElementById('tpl-hero-img');
  loading.classList.add('show');
  try {
    const blob = await fetchSlidePng(selectedTemplate, currentText(), currentBrand());
    if (seq !== heroSeq || !blob) return;
    if (heroUrl) URL.revokeObjectURL(heroUrl);
    heroUrl = URL.createObjectURL(blob);
    img.src = heroUrl;
    img.onload = () => {
      if (seq === heroSeq) loading.classList.remove('show');
    };
  } catch (e) {
    if (seq !== heroSeq) return;
    loading.classList.remove('show');
    Threads.toast('Preview gagal: ' + (e.message || e), false);
  }
  updateActiveBar();
}

async function refreshGallery() {
  const seq = ++gallerySeq;
  const text = currentText();
  const b = currentBrand();
  renderGallery();
  await Promise.all(templates.map(async t => {
    try {
      const blob = await fetchSlidePng(t.id, text, b);
      if (seq !== gallerySeq || !blob) return;
      if (previewUrls[t.id]) URL.revokeObjectURL(previewUrls[t.id]);
      previewUrls[t.id] = URL.createObjectURL(blob);
    } catch (e) {
      console.warn('preview', t.id, e);
    }
  }));
  if (seq === gallerySeq) renderGallery();
}

function scheduleRefresh(all) {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(async () => {
    await refreshHero();
    if (all) await refreshGallery();
  }, 320);
}

async function applyTemplate() {
  showAlert('');
  const btn = document.getElementById('btn-apply');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Menyimpan…';
  try {
    const cur = await Threads.api('/api/lazy/config');
    await Threads.api('/api/lazy/config', {
      method: 'PUT',
      body: JSON.stringify({
        enabled: !!cur.enabled,
        posts_per_day: cur.posts_per_day || 5,
        timezone: cur.timezone || 'Asia/Jakarta',
        topic_hint: cur.topic_hint || '',
        thumbnail_enabled: cur.thumbnail_enabled !== false,
        carousel_template: selectedTemplate,
      }),
    });
    const b = currentBrand();
    if (b && b !== 'brandmu') {
      try {
        await Threads.api('/api/ai/brand', { method: 'PUT', body: JSON.stringify({ brand: b }) });
      } catch {}
    }
    savedTemplate = selectedTemplate;
    renderGallery();
    Threads.toast(`Template ${metaOf(savedTemplate).name} diterapkan`, true);
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    btn.innerHTML = '<i class="bi bi-check2"></i> Terapkan';
    updateActiveBar();
  }
}

function setText(value) {
  document.getElementById('tpl-text').value = value;
  updateTextCount();
  scheduleRefresh(true);
}

document.getElementById('tpl-gallery').addEventListener('click', e => {
  const btn = e.target.closest('[data-tpl]');
  if (!btn) return;
  selectedTemplate = btn.dataset.tpl;
  renderGallery();
  refreshHero();
});

document.getElementById('btn-apply').onclick = () => applyTemplate();
document.getElementById('btn-refresh-all').onclick = () => {
  refreshHero();
  refreshGallery();
};

document.querySelectorAll('[data-preset]').forEach(btn => {
  btn.onclick = () => setText(btn.dataset.preset === '3' ? PRESET_3 : PRESET_2);
});

document.getElementById('tpl-text').addEventListener('input', () => {
  updateTextCount();
  // Hero cepat; galeri di-debounce lebih jarang
  scheduleRefresh(false);
  clearTimeout(document.getElementById('tpl-text')._galTimer);
  document.getElementById('tpl-text')._galTimer = setTimeout(() => refreshGallery(), 900);
});

document.getElementById('tpl-brand').addEventListener('input', () => {
  scheduleRefresh(false);
  clearTimeout(document.getElementById('tpl-brand')._galTimer);
  document.getElementById('tpl-brand')._galTimer = setTimeout(() => refreshGallery(), 900);
});

(async () => {
  document.getElementById('tpl-text').value = testText;
  updateTextCount();

  try {
    const mem = await Threads.api('/api/ai/memory');
    brand = (mem?.brand || '').replace(/^@+/, '') || 'brandmu';
  } catch {
    brand = 'brandmu';
  }
  document.getElementById('tpl-brand').value = brand;

  try {
    const tpl = await Threads.api('/api/ig/carousel/templates');
    if (tpl?.templates?.length) templates = tpl.templates;
    savedTemplate = tpl.active || 'noir';
    selectedTemplate = savedTemplate;
  } catch {
    templates = FALLBACK_TEMPLATES.slice();
  }

  renderGallery();
  await refreshHero();
  await refreshGallery();
})();
