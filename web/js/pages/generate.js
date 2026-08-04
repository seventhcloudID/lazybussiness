Threads.pageShell('generate');

let lastDrafts = [];
/** @type {Record<number, string>} */
let draftThumbs = {};
/** Preset sama Lazy Business — diisi dari /api/ai/thumbnail/defaults */
let thumbPreset = {
  model: 'gpt-image-2',
  size: '1024x768',
  quality: 'high',
  crop_4_3: true,
};
let thumbEnabled = false;

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

function renderMemory(mem) {
  const nicheEl = document.getElementById('niche');
  if (nicheEl && !nicheEl.dataset.dirty) {
    nicheEl.value = nichesFromMemory(mem).join('\n');
  }
  const brandEl = document.getElementById('brand');
  if (brandEl && !brandEl.dataset.dirty) {
    brandEl.value = mem.brand || '';
  }
  if (typeof mem.instructions === 'string' && !document.getElementById('instructions').dataset.dirty) {
    document.getElementById('instructions').value = mem.instructions;
  }
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

  lastDrafts = result.drafts || [];
  draftThumbs = {};
  const root = document.getElementById('drafts');
  if (!lastDrafts.length) {
    root.innerHTML = `<div class="gen-empty th-panel"><div class="th-empty py-12"><p class="text-sm text-muted m-0">Belum ada draf.</p></div></div>`;
    return;
  }

  root.innerHTML = lastDrafts.map((n, i) => `
    <article class="ai-draft" data-idx="${i}">
      <div class="ai-draft-top">
        <div class="tags">
          <span class="th-chip th-chip-ok">Utas ${i + 1}</span>
          <span class="th-chip">${(n.parts || []).length || 1} bagian</span>
          ${n.based_on ? `<span class="th-chip">${Threads.escapeHtml(n.based_on)}</span>` : ''}
        </div>
        <h3>${Threads.escapeHtml(n.title || 'Utas edukasi')}</h3>
        ${n.why ? `<p class="ai-draft-why">${Threads.escapeHtml(n.why)}</p>` : ''}
      </div>
      <div class="ai-draft-body">
        <div class="gen-draft-split">
          <div class="gen-draft-main min-w-0">
            ${renderParts(n.parts) || `<p class="draft">${Threads.escapeHtml(n.draft || '')}</p>`}
            ${n.risk ? `<p class="why"><strong>Risiko:</strong> ${Threads.escapeHtml(n.risk)}</p>` : ''}
            <div class="ai-draft-actions">
              <button type="button" class="th-btn th-btn-primary text-xs" data-use="${i}"><i class="bi bi-pencil-square"></i> Ke Buat Post</button>
              <button type="button" class="th-btn th-btn-soft text-xs" data-carousel="${i}"><i class="bi bi-images"></i> Carousel IG</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-copy="${i}"><i class="bi bi-clipboard"></i> Salin</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-thumb="${i}" ${thumbEnabled ? '' : 'disabled title="Set OpenAI key di API keys workspace"'}><i class="bi bi-image"></i> Thumbnail</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="good" data-i="${i}" title="Bagus"><i class="bi bi-hand-thumbs-up"></i></button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="bad" data-i="${i}" title="Jelek"><i class="bi bi-hand-thumbs-down"></i></button>
            </div>
          </div>
          <aside class="gen-thumb-aside">
            <div class="gen-thumb-box gen-thumb-box-side" data-thumb-box="${i}">
              <div class="gen-thumb-placeholder" data-thumb-ph="${i}">Belum ada thumbnail</div>
              <img class="gen-thumb-img" alt="Thumbnail utas 4:3" hidden />
              <p class="gen-thumb-cap">Threads 4:3 · preview</p>
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
  if (!thumbEnabled || !document.getElementById('auto-thumb')?.checked) return;
  if (!lastDrafts.length) return;
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

document.getElementById('instructions').addEventListener('input', () => {
  document.getElementById('instructions').dataset.dirty = '1';
});

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
      method: 'PUT',
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
      method: 'PUT',
      body: JSON.stringify({ niches, niche: niches.join('\n') }),
    });
    delete document.getElementById('niche').dataset.dirty;
    renderMemory(data.memory);
    Threads.toast(niches.length ? `${niches.length} niche disimpan` : 'Niche dikosongkan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-save-instructions').onclick = async () => {
  const instructions = document.getElementById('instructions').value;
  try {
    const data = await Threads.api('/api/ai/instructions', {
      method: 'PUT',
      body: JSON.stringify({ instructions }),
    });
    delete document.getElementById('instructions').dataset.dirty;
    document.getElementById('instructions-status').textContent = 'Tersimpan';
    renderMemory(data.memory);
    Threads.toast('Instruksi disimpan', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-fill-template').onclick = () => {
  const el = document.getElementById('instructions');
  el.value = EDUKASI_TEMPLATE;
  el.dataset.dirty = '1';
  document.getElementById('instructions-status').textContent = 'Template terisi — klik Simpan';
};

function setGenerateBusy(busy) {
  ['btn-generate', 'btn-generate-foot'].forEach((id) => {
    const btn = document.getElementById(id);
    if (!btn) return;
    btn.disabled = busy;
    btn.innerHTML = busy
      ? '<i class="bi bi-hourglass-split"></i> Generating…'
      : '<i class="bi bi-magic"></i> ' + (id === 'btn-generate-foot' ? 'Generate sekarang' : 'Generate');
  });
}

document.getElementById('btn-generate-foot')?.addEventListener('click', () => {
  document.getElementById('btn-generate')?.click();
});

document.getElementById('btn-generate').onclick = async () => {
  showAlert('');
  if (!(await Threads.requireConnected())) return;
  setGenerateBusy(true);
  showAlert('Generate draf…');
  try {
    // autosave instructions / niche if dirty
    if (document.getElementById('instructions').dataset.dirty) {
      await Threads.api('/api/ai/instructions', {
        method: 'PUT',
        body: JSON.stringify({ instructions: document.getElementById('instructions').value }),
      });
      delete document.getElementById('instructions').dataset.dirty;
    }
    if (document.getElementById('niche').dataset.dirty) {
      const niches = document.getElementById('niche').value
        .split(/[\n,;|]+/)
        .map(s => s.trim())
        .filter(Boolean);
      await Threads.api('/api/ai/niche', {
        method: 'PUT',
        body: JSON.stringify({ niches, niche: niches.join('\n') }),
      });
      delete document.getElementById('niche').dataset.dirty;
    }
    if (document.getElementById('brand')?.dataset.dirty) {
      await Threads.api('/api/ai/brand', {
        method: 'PUT',
        body: JSON.stringify({ brand: document.getElementById('brand').value.trim() }),
      });
      delete document.getElementById('brand').dataset.dirty;
    }
    if (!document.getElementById('niche').value.trim()) {
      Threads.toast('Isi niche dulu biar AI tidak menebak dari data', false);
    }
    const result = await Threads.api('/api/ai/generate', {
      method: 'POST',
      body: JSON.stringify({
        topic: document.getElementById('topic').value.trim(),
        count: 2,
      }),
    });
    showAlert('');
    renderDrafts(result);
    Threads.toast('Draf siap', true);
    await autoThumbAllDrafts();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    setGenerateBusy(false);
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
    location.href = '/app/buat.html?from=ai';
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
    location.href = '/app/ig-carousel.html?from=utas';
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
