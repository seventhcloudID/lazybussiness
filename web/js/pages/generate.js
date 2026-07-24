Threads.pageShell('generate');

let lastDrafts = [];

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
        <div class="ai-draft-actions">
          <button type="button" class="th-btn th-btn-soft text-xs" data-copy="${i}"><i class="bi bi-clipboard"></i> Salin utas</button>
          <button type="button" class="th-btn th-btn-primary text-xs" data-use="${i}"><i class="bi bi-pencil-square"></i> Ke Buat Post</button>
          <button type="button" class="th-btn th-btn-soft text-xs" data-carousel="${i}"><i class="bi bi-images"></i> Ke Carousel IG</button>
          <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="good" data-i="${i}" title="Bagus"><i class="bi bi-hand-thumbs-up"></i></button>
          <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="bad" data-i="${i}" title="Jelek"><i class="bi bi-hand-thumbs-down"></i></button>
        </div>
      </div>
    </article>`).join('');
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

document.getElementById('btn-generate').onclick = async () => {
  showAlert('');
  if (!(await Threads.requireConnected())) return;
  const btn = document.getElementById('btn-generate');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Generating…';
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
  if (use) {
    const d = lastDrafts[Number(use.dataset.use)];
    const parts = Array.isArray(d?.parts) && d.parts.length
      ? d.parts.map(p => String(p || '').replace(/\\n/g, '\n').trim()).filter(Boolean)
      : draftFullText(d).split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
    const text = parts.join('\n\n');
    try {
      localStorage.setItem('threads_compose_parts', JSON.stringify(parts));
      localStorage.setItem('threads_compose_draft', text);
      await Threads.api('/api/ai/feedback', {
        method: 'POST',
        body: JSON.stringify({
          draft_key: d.key || String(use.dataset.use),
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
})();
