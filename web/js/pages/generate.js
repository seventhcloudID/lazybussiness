Threads.pageShell('generate');

let lastDrafts = [];
/** @type {Record<number, string>} */
let draftThumbs = {};
let lastYoutubeThumb = '';
/** Instruksi terpisah per kategori (general vs youtube_to_utas). */
let instrByCat = { general: '', youtube_to_utas: '' };
let instrDirty = { general: false, youtube_to_utas: false };
/** Preset sama Lazy Business — diisi dari /api/ai/thumbnail/defaults */
let thumbPreset = {
  model: 'gpt-image-2',
  size: '1024x768',
  quality: 'high',
  crop_4_3: true,
};
let thumbEnabled = false;

function currentCategory() {
  return document.getElementById('content-category')?.value === 'youtube_to_utas'
    ? 'youtube_to_utas'
    : 'general';
}

function updateInstrLabels(cat) {
  const label = document.getElementById('instructions-label');
  const btnTpl = document.getElementById('btn-fill-template');
  const yt = cat === 'youtube_to_utas';
  if (label) {
    label.textContent = yt
      ? 'Instruksi YouTube → utas (style & larangan)'
      : 'Instruksi Umum (style & larangan)';
  }
  if (btnTpl) {
    btnTpl.textContent = yt ? 'Isi template YouTube' : 'Isi template edukasi';
  }
}

function showInstructionsFor(cat) {
  const el = document.getElementById('instructions');
  if (!el) return;
  el.value = instrByCat[cat] || '';
  if (instrDirty[cat]) el.dataset.dirty = '1';
  else delete el.dataset.dirty;
  updateInstrLabels(cat);
}

function stashCurrentInstructions() {
  const el = document.getElementById('instructions');
  if (!el) return;
  const cat = currentCategory();
  instrByCat[cat] = el.value;
  if (el.dataset.dirty) instrDirty[cat] = true;
}

function showAlert(msg) {
  const el = document.getElementById('gen-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function nichesFromMemory(mem) {
  if (Array.isArray(mem?.niches) && mem.niches.length) {
    return mem.niches.map(n => String(n || '').trim()).filter(Boolean);
  }
  const raw = String(mem?.niche || '').trim();
  if (!raw) return [];
  return raw.split(/[\n,;|]+/).map(s => s.trim()).filter(Boolean);
}

function renderMemory(mem, opts = {}) {
  const nicheEl = document.getElementById('niche');
  if (nicheEl && !nicheEl.dataset.dirty) {
    nicheEl.value = nichesFromMemory(mem).join('\n');
  }
  const brandEl = document.getElementById('brand');
  if (brandEl && !brandEl.dataset.dirty) {
    brandEl.value = mem.brand || '';
  }
  // Jangan overwrite cache lokal kalau field absen di response (hindari textarea "hilang").
  if (!instrDirty.general && typeof mem.instructions === 'string') {
    instrByCat.general = mem.instructions;
  }
  if (!instrDirty.youtube_to_utas && typeof mem.instructions_youtube === 'string') {
    instrByCat.youtube_to_utas = mem.instructions_youtube;
  }
  const catEl = document.getElementById('content-category');
  if (catEl && !catEl.dataset.dirty && !opts.keepCategory) {
    catEl.value = mem.content_category === 'youtube_to_utas' ? 'youtube_to_utas' : 'general';
  }
  if (catEl) catEl.dataset.prev = catEl.value;
  showInstructionsFor(currentCategory());
}

function renderYoutubeSource(yt) {
  const box = document.getElementById('youtube-source-box');
  const el = document.getElementById('youtube-source');
  if (!box || !el) return;
  if (!yt?.video_id && !yt?.url) {
    box.classList.add('hidden');
    el.textContent = '';
    return;
  }
  const title = yt.title || yt.video_id || 'Video';
  const url = yt.url || ('https://www.youtube.com/watch?v=' + yt.video_id);
  const bits = [
    `<a href="${Threads.escapeHtml(url)}" target="_blank" rel="noopener" class="font-semibold text-ink underline">${Threads.escapeHtml(title)}</a>`,
  ];
  if (yt.channel) bits.push(`Channel: ${Threads.escapeHtml(yt.channel)}`);
  if (yt.window) bits.push(`Window: ${Threads.escapeHtml(yt.window)}`);
  if (yt.why_hot) bits.push(Threads.escapeHtml(yt.why_hot));
  el.innerHTML = bits.join('<br>');
  box.classList.remove('hidden');
}

const EDUKASI_TEMPLATE = `FORMAT
- Output: utas berantai 4–6 bagian (bukan 1 post panjang).
- Tiap bagian maksimal 500 karakter (batas Threads) — hitung ketat.
- Tiap bagian 2–3 kalimat saja. Pisah tiap kalimat dengan enter (baris kosong) biar enak dibaca — jangan digabung jadi satu paragraf padat.
- Bagian 1 = HOOK keras (bukan soft open / definisi / latar belakang).
- Bagian tengah = daging: mekanisme, sebab-akibat, konteks nyata.
- Bagian akhir = penutup tajam atau pertanyaan terbuka (bukan jualan).

POTENSI VIRAL (wajib)
- Jangan bikin konten "aman tapi sepi". Harus ada alasan orang stop scroll / reply / share.
- Angle wajib salah satu: kontradiksi, mekanisme tersembunyi, contoh konkret mengejutkan, insight yang enak di-share.
- Kalau bisa diganti niche lain tanpa berubah artinya → terlalu generic, ganti angle.
- Harus ada 1 poin yang masih diingat 3 detik setelah baca.

HOOK (wajib kuat)
- Kalimat pertama harus nahan scroll: kontradiksi, klaim tajam, kejadian konkret, atau "yang dikira X padahal Y".
- Dilarang buka dengan: definisi, "ada yang menarik", "banyak orang", ringkasan ensiklopedia, nada laporan pasif.
- Tension dulu → penjelasan belakangan.

ENERGI & BAHASA
- Kalimat aktif, tajam, spesifik. Bukan pasif/lembek ("dapat dikatakan", "perlu dipahami", "hal ini menunjukkan").
- Kayak orang yang paham lagi cerita ke timeline — bukan guru, bukan copywriter, bukan Wikipedia.
- Hindari basabasi ("hari ini kita bahas", "penting diketahui", "mari kita").

NICHE & ISI
- Ikuti niche yang user tentukan di app (boleh lebih dari satu; pilih/kombinasi yang relevan per draf).
- Konten RAW/daging: padat informasi + ada tensi, bukan ceramah tenang.

ANTI-DOGENG / ANTI-RUJAK
- Jangan terdengar AI, guru kehidupan, atau influencer yang lagi "membuka mata orang".
- Jangan overclaim. Kalau belum pasti, tulis sebagai observasi/dugaan.
- Jangan edgy palsu, jangan kasar murahan, jangan drama kosong.
- Hindari frasa: "banyak yang belum sadar", "di era digital", "faktanya mengejutkan", "thread ini penting".
- Tiap kalimat harus nambah informasi; kalau bisa dihapus tanpa rugi, hapus.

BAHASA
- Bahasa Indonesia natural.
- Dilarang: bullet point, numbering, "lu/gue", emoji berlebihan, hashtag, "thread 🧵".
- Boleh "kamu" atau kalimat netral tanpa sapaan kasual berlebihan.

LARANGAN
- Jangan ringkas jadi list tips.
- Jangan clickbait kosong: setiap bagian harus nambah isi.
- Jangan terdengar AI/LinkedIn.
- Jangan pasif: kalau hook lemah, utas gagal.
- Jangan lewat 500 karakter per bagian.
- Jangan filler: kalau tidak berpotensi viral di niche, jangan dikeluarkan.`;

const YOUTUBE_TEMPLATE = `MODE: YouTube → utas
- Bahan utama = 1 video yang lagi rame (sudah dipilih sistem). Jangan ganti topik bebas.
- Bukan ringkasan/transcript. Ambil angle: kenapa video itu relevan ke niche + apa yang orang miss.
- Sebut sumber (judul/channel) natural di 1 bagian saja — jangan spam link.
- Jangan mengarang fakta di luar digest/judul video.

FORMAT
- Output: utas berantai 4–6 bagian.
- Tiap bagian maksimal 500 karakter. 2–3 kalimat, pisah enter.
- Bagian 1 = HOOK dari insight video (bukan "ada video menarik").
- Tengah = daging: mekanisme / kontradiksi / dampak ke niche.
- Akhir = penutup tajam atau pertanyaan terbuka (bukan jualan, bukan "subscribe").

ANGLE
- "Yang video ini tunjukin vs yang orang kira"
- "Kenapa ini penting buat niche kamu sekarang"
- "Detail kecil di video yang orang skip tapi ngegas"

ANTI
- Jangan review YouTuber / spoiler panjang / list tips dari video.
- Jangan dogeng AI. Bahasa Indonesia natural.
- Tanpa bullet, numbering, emoji berlebihan, hashtag.`;

function draftFullText(d) {
  if (Array.isArray(d?.parts) && d.parts.length) {
    return d.parts
      .map(p => String(p || '').replace(/\\n/g, '\n').trim())
      .filter(Boolean)
      .join('\n\n');
  }
  return String(d?.draft || d?.hook || '').replace(/\\n/g, '\n').trim();
}

function partBodyHtml(text) {
  const raw = String(text || '').replace(/\\n/g, '\n').trim();
  if (!raw) return '';
  // Pecah paragraf biar ada space visual, bukan dinding teks.
  const paras = raw.split(/\n\s*\n/).map(p => p.trim()).filter(Boolean);
  if (paras.length > 1) {
    return paras.map(p => `<p>${Threads.escapeHtml(p)}</p>`).join('');
  }
  return `<p>${Threads.escapeHtml(raw)}</p>`;
}

function renderParts(parts) {
  if (!Array.isArray(parts) || !parts.length) return '';
  return `<div class="gen-thread">
    ${parts.map((p, idx) => {
      const n = Array.from(String(p || '')).length;
      const over = n > 500;
      return `
      <div class="gen-thread-part">
        <div class="gen-thread-n">${idx + 1}</div>
        <div class="gen-thread-text">
          ${partBodyHtml(p)}
          <div class="gen-part-count ${over ? 'over' : ''}">${n}/500</div>
        </div>
      </div>`;
    }).join('')}
  </div>`;
}

function renderDrafts(result) {
  const box = document.getElementById('consideration-box');
  if (result.consideration) {
    box.classList.remove('hidden');
    document.getElementById('consideration').textContent = result.consideration;
  } else {
    box.classList.add('hidden');
  }
  renderYoutubeSource(result.youtube);

  lastDrafts = result.drafts || [];
  draftThumbs = {};
  lastYoutubeThumb = result.youtube?.thumb_url || '';
  const root = document.getElementById('drafts');
  if (!lastDrafts.length) {
    root.innerHTML = '<div class="th-panel"><div class="th-empty py-10"><p class="text-sm text-muted">Belum ada draf.</p></div></div>';
    return;
  }

  root.innerHTML = lastDrafts.map((n, i) => `
    <article class="ai-draft" data-idx="${i}">
      <div class="ai-draft-top">
        <div class="tags">
          <span class="th-chip">Utas ${i + 1}</span>
          <span class="th-chip">${Threads.escapeHtml(n.format || 'THREAD')}</span>
          <span class="th-chip">${(n.parts || []).length || 1} bagian</span>
          ${n.based_on ? `<span class="th-chip">${Threads.escapeHtml(n.based_on)}</span>` : ''}
        </div>
        <h3>${Threads.escapeHtml(n.title || 'Utas edukasi')}</h3>
      </div>
      <div class="ai-draft-body">
        ${renderParts(n.parts) || `<p class="draft">${Threads.escapeHtml(n.draft || '')}</p>`}
        <p class="why">${Threads.escapeHtml(n.why || '')}</p>
        ${n.risk ? `<p class="why"><strong>Risiko:</strong> ${Threads.escapeHtml(n.risk)}</p>` : ''}
        <div class="gen-draft-split">
          <div class="gen-draft-main min-w-0">
            <div class="ai-draft-actions">
              <button type="button" class="th-btn th-btn-soft text-xs" data-copy="${i}"><i class="bi bi-clipboard"></i> Salin utas</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-thumb="${i}" ${thumbEnabled ? '' : 'disabled title="Set OPENAI_API_KEY"'}><i class="bi bi-image"></i> Thumbnail</button>
              <button type="button" class="th-btn th-btn-primary text-xs" data-use="${i}"><i class="bi bi-pencil-square"></i> Ke Buat Post</button>
              <button type="button" class="th-btn th-btn-soft text-xs" data-carousel="${i}"><i class="bi bi-images"></i> Ke Carousel IG</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="good" data-i="${i}" title="Bagus"><i class="bi bi-hand-thumbs-up"></i></button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="bad" data-i="${i}" title="Jelek"><i class="bi bi-hand-thumbs-down"></i></button>
            </div>
          </div>
          <aside class="gen-thumb-aside">
            <div class="gen-thumb-box gen-thumb-box-side" data-thumb-box="${i}">
              <div class="gen-thumb-placeholder" data-thumb-ph="${i}">Belum ada thumbnail</div>
              <img class="gen-thumb-img" alt="Thumbnail utas 4:3" hidden />
              <p class="gen-thumb-cap">Threads 4:3 · preview saja, belum di-post</p>
            </div>
          </aside>
        </div>
      </div>
    </article>`).join('');
}

function draftHook(d) {
  if (Array.isArray(d?.parts) && d.parts.length) {
    return String(d.parts[0] || '').replace(/\\n/g, '\n').trim();
  }
  if (d?.hook) return String(d.hook).replace(/\\n/g, '\n').trim();
  const full = draftFullText(d);
  return full.split(/\n\s*\n/).map(s => s.trim()).filter(Boolean)[0] || full;
}

function showThumbOnDraft(i, pathOrUrl) {
  const box = document.querySelector(`[data-thumb-box="${i}"]`);
  if (!box) return;
  const img = box.querySelector('.gen-thumb-img');
  const ph = box.querySelector(`[data-thumb-ph="${i}"]`);
  if (img) {
    img.src = pathOrUrl;
    img.hidden = false;
  }
  if (ph) ph.hidden = true;
}

function setThumbLoading(i, loading, errMsg) {
  const ph = document.querySelector(`[data-thumb-ph="${i}"]`);
  if (!ph) return;
  if (loading) {
    ph.hidden = false;
    ph.textContent = 'Merender thumbnail…';
    ph.classList.add('busy');
  } else {
    ph.classList.remove('busy');
    if (errMsg) {
      ph.hidden = false;
      ph.textContent = errMsg;
    }
  }
}

async function generateThumbnailForDraft(i, { quiet = false } = {}) {
  const d = lastDrafts[i];
  if (!d) return null;
  if (!thumbEnabled) {
    if (!quiet) Threads.toast('OPENAI_API_KEY belum aktif', false);
    return null;
  }
  const hook = draftHook(d);
  if (!hook) {
    if (!quiet) Threads.toast('Hook bagian 1 kosong', false);
    return null;
  }
  const btn = document.querySelector(`[data-thumb="${i}"]`);
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Thumbnail…';
  }
  setThumbLoading(i, true);
  try {
    // Preview di Generate pakai quality medium biar lebih cepat / jarang 504.
    // Lazy Business tetap high lewat preset server.
    const quality = quiet ? 'medium' : (thumbPreset.quality || 'medium');
    const data = await Threads.api('/api/ai/thumbnail', {
      method: 'POST',
      body: JSON.stringify({
        hook,
        model: thumbPreset.model,
        size: thumbPreset.size,
        quality,
        crop_4_3: thumbPreset.crop_4_3 !== false,
      }),
    });
    const url = data.image_url || data.path;
    if (!url) throw new Error('URL thumbnail kosong');
    draftThumbs[i] = url;
    showThumbOnDraft(i, data.path || url);
    if (!quiet) Threads.toast('Thumbnail 4:3 siap', true);
    return url;
  } catch (e) {
    const msg = e.message || String(e);
    setThumbLoading(i, false, msg.slice(0, 120));
    if (!quiet) Threads.toast(msg, false);
    return null;
  } finally {
    if (btn) {
      btn.disabled = !thumbEnabled;
      btn.innerHTML = '<i class="bi bi-image"></i> Thumbnail';
    }
  }
}

async function autoThumbAllDrafts() {
  if (!document.getElementById('auto-thumb')?.checked) return;
  if (!lastDrafts.length) return;

  // YouTube → utas: pakai thumb video asli (sudah di-mirror server).
  if (lastYoutubeThumb) {
    for (let i = 0; i < lastDrafts.length; i++) {
      draftThumbs[i] = lastYoutubeThumb;
      showThumbOnDraft(i, lastYoutubeThumb);
    }
    Threads.toast(`Thumbnail YouTube siap (${lastDrafts.length} draf)`, true);
    return;
  }

  if (!thumbEnabled) return;
  const n = lastDrafts.length;
  let ok = 0;
  let lastErr = '';
  for (let i = 0; i < n; i++) {
    showAlert(`Thumbnail ${i + 1}/${n} (${thumbPreset.model} · ${thumbPreset.size} · medium) — bisa 30–90 dtk…`);
    const url = await generateThumbnailForDraft(i, { quiet: true });
    if (url) ok += 1;
    else {
      const ph = document.querySelector(`[data-thumb-ph="${i}"]`);
      if (ph?.textContent) lastErr = ph.textContent;
    }
  }
  showAlert(ok === n ? '' : (lastErr || `Thumbnail sebagian gagal (${ok}/${n})`));
  Threads.toast(
    ok ? `Thumbnail siap (${ok}/${n})` : (lastErr || 'Thumbnail gagal — cek timeout Nginx / OPENAI key'),
    ok > 0,
  );
}

async function loadMemory() {
  const mem = await Threads.api('/api/ai/memory');
  renderMemory(mem);
  return mem;
}

document.getElementById('niche').addEventListener('input', () => {
  document.getElementById('niche').dataset.dirty = '1';
});

document.getElementById('brand').addEventListener('input', () => {
  document.getElementById('brand').dataset.dirty = '1';
});

document.getElementById('btn-save-brand').onclick = async () => {
  const brand = document.getElementById('brand').value.trim();
  try {
    const data = await Threads.api('/api/ai/brand', {
      method: 'POST',
      body: JSON.stringify({ brand }),
    });
    delete document.getElementById('brand').dataset.dirty;
    renderMemory(data.memory);
    Threads.toast(brand ? 'Brand disimpan' : 'Brand dikosongkan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-save-niche').onclick = async () => {
  const niches = document.getElementById('niche').value
    .split(/[\n,;|]+/)
    .map(s => s.trim())
    .filter(Boolean);
  try {
    const data = await Threads.api('/api/ai/niche', {
      method: 'POST',
      body: JSON.stringify({ niches, niche: niches.join('\n') }),
    });
    delete document.getElementById('niche').dataset.dirty;
    renderMemory(data.memory);
    Threads.toast(niches.length ? `${niches.length} niche disimpan` : 'Niche dikosongkan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('instructions')?.addEventListener('input', () => {
  const cat = currentCategory();
  const el = document.getElementById('instructions');
  instrByCat[cat] = el.value;
  instrDirty[cat] = true;
  el.dataset.dirty = '1';
});

document.getElementById('btn-save-instructions').onclick = async () => {
  const cat = currentCategory();
  const instructions = document.getElementById('instructions').value;
  instrByCat[cat] = instructions;
  try {
    // Pastikan kategori ikut tersimpan dulu supaya field youtube tidak salah slot.
    await Threads.api('/api/ai/category', {
      method: 'POST',
      body: JSON.stringify({ category: cat }),
    });
    const data = await Threads.api('/api/ai/instructions', {
      method: 'POST',
      body: JSON.stringify({ instructions, category: cat }),
    });
    instrDirty[cat] = false;
    delete document.getElementById('instructions').dataset.dirty;
    // Pakai teks yang baru disimpan; jangan biarkan response kosong men-wipe UI.
    if (cat === 'youtube_to_utas') {
      instrByCat.youtube_to_utas = instructions;
      if (data.memory) data.memory.instructions_youtube = instructions;
    } else {
      instrByCat.general = instructions;
      if (data.memory) data.memory.instructions = instructions;
    }
    document.getElementById('instructions-status').textContent =
      cat === 'youtube_to_utas' ? 'Tersimpan (YouTube → utas)' : 'Tersimpan (Umum)';
    renderMemory(data.memory || {}, { keepCategory: true });
    showInstructionsFor(cat);
    Threads.toast(cat === 'youtube_to_utas' ? 'Instruksi YouTube disimpan' : 'Instruksi Umum disimpan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('content-category')?.addEventListener('change', async (ev) => {
  const prev = ev.target.dataset.prev || 'general';
  const el = document.getElementById('instructions');
  if (el) {
    instrByCat[prev] = el.value;
    if (el.dataset.dirty) instrDirty[prev] = true;
  }
  const cat = currentCategory();
  ev.target.dataset.prev = cat;
  ev.target.dataset.dirty = '1';
  showInstructionsFor(cat);
  document.getElementById('instructions-status').textContent =
    cat === 'youtube_to_utas' ? 'Mode YouTube — instruksi terpisah' : 'Mode Umum — instruksi terpisah';
  try {
    await Threads.api('/api/ai/category', {
      method: 'POST',
      body: JSON.stringify({ category: cat }),
    });
    delete ev.target.dataset.dirty;
  } catch (e) {
    Threads.toast(e.message, false);
  }
});

document.getElementById('btn-fill-template').onclick = () => {
  const cat = currentCategory();
  const el = document.getElementById('instructions');
  el.value = cat === 'youtube_to_utas' ? YOUTUBE_TEMPLATE : EDUKASI_TEMPLATE;
  instrByCat[cat] = el.value;
  instrDirty[cat] = true;
  el.dataset.dirty = '1';
  document.getElementById('instructions-status').textContent = 'Template terisi — klik Simpan';
};

document.getElementById('btn-generate').onclick = async () => {
  showAlert('');
  if (!(await Threads.requireConnected())) return;
  const btn = document.getElementById('btn-generate');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Generating…';
  showAlert('Generate draf…');
  try {
    // autosave instructions / niche if dirty (per kategori)
    stashCurrentInstructions();
    for (const cat of ['general', 'youtube_to_utas']) {
      if (!instrDirty[cat]) continue;
      await Threads.api('/api/ai/instructions', {
        method: 'POST',
        body: JSON.stringify({ instructions: instrByCat[cat], category: cat }),
      });
      instrDirty[cat] = false;
    }
    delete document.getElementById('instructions')?.dataset.dirty;
    if (document.getElementById('niche').dataset.dirty) {
      const niches = document.getElementById('niche').value
        .split(/[\n,;|]+/)
        .map(s => s.trim())
        .filter(Boolean);
      await Threads.api('/api/ai/niche', {
        method: 'POST',
        body: JSON.stringify({ niches, niche: niches.join('\n') }),
      });
      delete document.getElementById('niche').dataset.dirty;
    }
    if (document.getElementById('brand')?.dataset.dirty) {
      await Threads.api('/api/ai/brand', {
        method: 'POST',
        body: JSON.stringify({ brand: document.getElementById('brand').value.trim() }),
      });
      delete document.getElementById('brand').dataset.dirty;
    }
    if (!document.getElementById('niche').value.trim()) {
      Threads.toast('Isi niche dulu biar AI tidak menebak dari data', false);
    }
    const cat = document.getElementById('content-category')?.value || 'general';
    await Threads.api('/api/ai/category', {
      method: 'POST',
      body: JSON.stringify({ category: cat }),
    });
    delete document.getElementById('content-category')?.dataset.dirty;
    showAlert(cat === 'youtube_to_utas'
      ? 'Cari video YouTube yang rame + generate utas…'
      : 'Generate draf…');
    const result = await Threads.api('/api/ai/generate', {
      method: 'POST',
      body: JSON.stringify({
        topic: document.getElementById('topic').value.trim(),
        count: 2,
        category: cat,
      }),
    });
    showAlert('');
    renderDrafts(result);
    Threads.toast(cat === 'youtube_to_utas' ? 'Draf dari YouTube siap' : 'Draf siap', true);
    await autoThumbAllDrafts();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-magic"></i> Generate';
  }
};

document.getElementById('drafts').addEventListener('click', async e => {
  const copy = e.target.closest('[data-copy]');
  const use = e.target.closest('[data-use]');
  const thumb = e.target.closest('[data-thumb]');
  const car = e.target.closest('[data-carousel]');
  const fb = e.target.closest('[data-fb]');
  if (copy) {
    const d = lastDrafts[Number(copy.dataset.copy)];
    const text = draftFullText(d);
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      Threads.toast('Utas disalin (pisah bagian dengan baris kosong)', true);
    } catch {
      Threads.toast('Gagal salin', false);
    }
    return;
  }
  if (thumb) {
    await generateThumbnailForDraft(Number(thumb.dataset.thumb));
    return;
  }
  if (use) {
    const idx = Number(use.dataset.use);
    const d = lastDrafts[idx];
    const parts = Array.isArray(d?.parts) && d.parts.length
      ? d.parts.map(p => String(p || '').replace(/\\n/g, '\n').trim()).filter(Boolean)
      : draftFullText(d).split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
    const text = parts.join('\n\n');
    try {
      localStorage.setItem('threads_compose_parts', JSON.stringify(parts));
      localStorage.setItem('threads_compose_draft', text);
      if (draftThumbs[idx]) {
        localStorage.setItem('threads_compose_image_url', draftThumbs[idx]);
      } else {
        localStorage.removeItem('threads_compose_image_url');
      }
      await Threads.api('/api/ai/feedback', {
        method: 'POST',
        body: JSON.stringify({
          draft_key: d.key || String(idx),
          verdict: 'used',
          text,
        }),
      });
    } catch {}
    location.href = '/buat.html?from=ai';
    return;
  }
  if (car) {
    const d = lastDrafts[Number(car.dataset.carousel)];
    const parts = Array.isArray(d?.parts) && d.parts.length
      ? d.parts.map(p => String(p || '').replace(/\\n/g, '\n').trim()).filter(Boolean)
      : draftFullText(d).split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
    if (parts.length < 2) return Threads.toast('Utas minimal 2 bagian untuk carousel', false);
    localStorage.setItem('threads_carousel_parts', JSON.stringify(parts));
    localStorage.setItem('threads_compose_parts', JSON.stringify(parts));
    location.href = '/ig-carousel.html?from=utas';
    return;
  }
  if (fb) {
    const i = Number(fb.dataset.i);
    const d = lastDrafts[i];
    if (!d) return;
    try {
      const data = await Threads.api('/api/ai/feedback', {
        method: 'POST',
        body: JSON.stringify({
          draft_key: d.key || String(i),
          verdict: fb.dataset.fb,
          text: draftFullText(d),
        }),
      });
      renderMemory(data.memory);
      Threads.toast(fb.dataset.fb === 'good' ? 'Ditandai bagus' : 'Ditandai jelek', true);
    } catch (err) {
      Threads.toast(err.message, false);
    }
  }
});

(async () => {
  try {
    await loadMemory();
  } catch {
    /* ignore */
  }
  try {
    const def = await Threads.api('/api/ai/thumbnail/defaults');
    thumbEnabled = !!def.enabled;
    if (def.preset) {
      thumbPreset = {
        model: def.preset.model || thumbPreset.model,
        size: def.preset.size || thumbPreset.size,
        quality: def.preset.quality || thumbPreset.quality,
        crop_4_3: def.preset.crop_4_3 !== false,
      };
    }
    const chip = document.getElementById('thumb-preset');
    if (chip) {
      chip.textContent = thumbEnabled
        ? `${thumbPreset.model} · ${thumbPreset.size} · ${thumbPreset.quality}`
        : 'Thumbnail off — set OPENAI_API_KEY';
      chip.classList.toggle('ok', thumbEnabled);
      chip.classList.toggle('bad', !thumbEnabled);
    }
    const auto = document.getElementById('auto-thumb');
    if (auto && !thumbEnabled) {
      auto.checked = false;
      auto.disabled = true;
    }
  } catch {
    /* ignore */
  }
})();
