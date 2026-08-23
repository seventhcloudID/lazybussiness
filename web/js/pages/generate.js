Threads.pageShell('generate');

let lastDrafts = [];
/** @type {Record<number, string>} */
let draftThumbs = {};
/** Preset sama Lazy Business — diisi dari /api/ai/thumbnail/defaults */
let thumbPreset = {
  model: 'cx/gpt-5.5-image',
  size: '1080x1350',
  quality: 'high',
  crop_4_3: true,
};
let thumbEnabled = false;

const COVER_TEMPLATE_KEY = 'threads_cover_template_v1';
const COVER_TEMPLATE_NAMES = {
  'edge-clean': 'Edge Clean',
  'split-roomy': 'Split Roomy',
  'inset-editorial': 'Inset Editorial',
  'left-cut': 'Left Cut',
  'right-cut': 'Right Cut',
  'low-editorial': 'Low Editorial',
};

function activeCoverTemplate() {
  try {
    const id = localStorage.getItem(COVER_TEMPLATE_KEY) || 'edge-clean';
    return COVER_TEMPLATE_NAMES[id] ? id : 'edge-clean';
  } catch {
    return 'edge-clean';
  }
}

const THUMB_EXTRA_KEY = 'threads_generate_thumb_extra_v20';
const THUMB_COLOR_GUARD = `
ATURAN FINAL — WAJIB MENGALAHKAN ARAHAN LAYOUT LAIN
- Bright editorial daylight, exposure terang dan bersih, white balance netral.
- Warna foto natural dan hidup; skin tone sehat.
- Hasil model harus berupa FOTO LATAR SAJA. Jangan menggambar panel, shape, kartu, kotak putih, border, shadow, atau overlay grafis.
- Gunakan SATU figur publik terkenal hanya bila nama, perusahaan, karya, atau produknya benar-benar menjadi subjek utama hook: Sam Altman untuk OpenAI/ChatGPT, Mark Zuckerberg untuk Meta/Instagram/Threads, Elon Musk untuk X/xAI/Tesla/SpaceX, Jennie BLACKPINK untuk BLACKPINK/K-pop/fashion/beauty, atau figur lain dengan hubungan langsung yang tertulis di hook. Jangan memakai selebritas hanya karena bidangnya terasa berdekatan. Topik UMKM, toko, penjualan, produktivitas, atau teknologi umum tanpa figur/perusahaan spesifik wajib memakai model manusia ekspresif yang sesuai konteks.
- Jika tidak ada figur terkenal yang aman dan relevan, gunakan satu model manusia dengan wajah, ekspresi, gestur, dan situasi yang sangat kuat. Jangan hanya memakai meja, laptop, atau benda mati generik.
- Wajib tampilkan MOMEN AKTIF yang terbaca tanpa headline: keputusan, reaksi, kekacauan, perbandingan, konsekuensi, atau interaksi fisik. Wajah harus punya emosi spesifik; tangan/tubuh sedang melakukan sesuatu; satu properti foreground memperjelas konflik.
- DILARANG membuat foto stock pasif: orang duduk tenang menatap laptop/ponsel, tersenyum generik ke layar, mengetik biasa, meja kerja rapi tanpa kejadian, atau pose corporate tepat di tengah frame.
- Jika ide adegan dari Editorial Director pasif, ubah menjadi versi aktif yang tetap faktual. Gunakan kontras berantakan-versus-rapi, reaksi saat menemukan masalah, keputusan yang tertunda, atau pekerjaan yang hampir gagal sesuai konteks hook.
- Gunakan framing editorial dekat, asimetris, berlapis, dengan foreground kuat. Jangan selalu membuat medium shot datar dari depan.
- Lokasi, pakaian, aktivitas, dan properti harus langsung menunjukkan SATU mikro-segmen dari utas, misalnya warung makan, salon, bengkel, supplier kemasan, atau jenis usaha spesifik lain. Hindari kantor/meja generik yang bisa dipakai untuk semua bisnis.
- Jangan merender headline, label, handle, CTA, angka, watermark, huruf, logo, wordmark, nama aplikasi, domain, atau tulisan dekoratif. Visual produk soft-selling harus tetap anonim.
- Jika adegan memakai perangkat, pilih maksimal SATU perangkat utama: satu HP ATAU satu laptop. Layar harus menjadi satu bidang datar yang sepenuhnya berada DI DALAM bezel/frame perangkat, mengikuti sudut perspektif dan arah bodinya.
- Bodi perangkat wajib utuh dan opaque dengan occlusion yang benar. Dilarang membuat layar terpisah, layar melayang, layar duplikat, layar transparan, panel UI di belakang HP, display menembus bodi, bezel patah, engsel laptop melengkung, atau keyboard tidak tersambung.
- Isi layar dibuat gelap/kosong atau berupa pantulan lembut tanpa UI dan tanpa tulisan. Kalau geometri perangkat tidak bisa dibuat meyakinkan, hilangkan perangkat dan gunakan properti fisik lain yang relevan.
- Letakkan focal point di area atas/tengah. Area bawah 45% boleh tetap berupa latar foto, tetapi jangan menaruh wajah, tangan, atau objek penting di sana karena akan ditutup panel aplikasi.
- Pertahankan detail shadow dan kontras tegas tanpa black crush.
- Jangan sepia, vintage, retro, brown/olive color grading, murky, gloomy, underexposed, faded, washed-out, desaturated, atau vignette gelap.`;
const DEFAULT_THUMB_EXTRA = `Buat FOTO LATAR editorial portrait 4:5, 1080×1350 untuk cover konten. Aplikasi akan menambahkan panel putih dan teks sesudah gambar dibuat, jadi model hanya bertugas menghasilkan foto.

INPUT
Hook/topik: {{hook}}
Lokasi: {{lokasi}}
Ide adegan dari hasil utas: {{cover_brief}}

KONTEN VISUAL
- Satu foto editorial realistis yang benar-benar relevan dengan hook.
- Tampilkan satu jenis usaha/pekerjaan yang spesifik dari utas lewat lokasi, pakaian, aktivitas, dan properti yang khas. Jangan mengganti target sempit menjadi "pemilik bisnis" generik.
- Foto harus terasa seperti satu frame dari kejadian yang sedang berlangsung, bukan orang yang sengaja berpose. Pilih satu titik ketegangan visual yang membuat orang bertanya "sedang terjadi apa?" tetapi jawabannya tetap sesuai hook.
- Tentukan dengan spesifik: aksi fisik subjek, emosi wajah, objek yang sedang disentuh/diubah, konflik visual, foreground, dan sudut kamera.
- Gunakan SATU figur publik terkenal hanya ketika nama, perusahaan, karya, atau produknya menjadi subjek utama hook: Sam Altman untuk OpenAI/ChatGPT; Mark Zuckerberg untuk Meta/Instagram/Threads; Elon Musk untuk X/xAI/Tesla/SpaceX; Jennie BLACKPINK untuk BLACKPINK/K-pop/fashion/beauty; atau figur besar lain dengan hubungan langsung yang tertulis di hook.
- Figur publik hanya menjadi visual hook editorial. Jangan menyiratkan endorsement, testimoni, skandal, penggunaan produk, atau kejadian faktual yang tidak ada di hook. Topik UMKM, toko, penjualan, produktivitas, atau teknologi umum tanpa figur/perusahaan spesifik wajib memakai model manusia ekspresif yang sesuai konteks, bukan selebritas acak.
- Jika produk, aplikasi, atau layanan adalah inti bahasan, visualisasikan mekanisme atau manfaatnya tanpa identitas merek. Jangan menampilkan logo, wordmark, nama aplikasi, domain, atau UI bermerek.
- PERANGKAT ELEKTRONIK: maksimal satu perangkat utama. Jika memakai HP, layar wajib terkunci di dalam bezel HP dan satu perspektif dengan bodi; jangan ada panel/layar kedua di belakangnya. Jika memakai laptop, layar wajib tersambung wajar pada engsel dan keyboard; jangan ada display melayang atau menembus bodi. Perangkat harus opaque dan memiliki occlusion yang benar terhadap tangan serta meja.
- Buat layar gelap, kosong, atau hanya berisi pantulan cahaya lembut. Jangan membuat screenshot, dashboard, spreadsheet, chat, UI, teks, atau kartu apa pun pada layar.
- Tempatkan focal point di area atas/tengah dan jaga area bawah 45% bebas dari wajah, tangan, serta objek penting.
- Foto tetap full-frame sampai bawah; jangan membuat ruang putih atau panel sendiri.
- Jangan membuat kolase, screenshot aplikasi, kartu, UI palsu, atau tempelan gambar kecil.
- Jangan membuat orang hanya duduk, mengetik, membaca, atau memegang ponsel dengan ekspresi netral. Jangan memakai komposisi stock-photo corporate yang bersih tetapi tanpa cerita.
- Jangan ada tulisan terbaca di foto, layar, papan, pakaian, kemasan, atau properti.

LARANGAN
- Rasio selain 4:5
- Teks, huruf, angka, logo, nama aplikasi, domain, panel, shape, kartu, border, atau shadow apa pun
- Background kusam, terlalu gelap, atau color cast cokelat/kuning
- Kolase, UI aplikasi, watermark, atau label sponsored
- Visual generik yang tidak membuktikan topik
- Layar di belakang HP/laptop, layar lepas dari bezel, perangkat transparan, layar ganda, geometri engsel/keyboard yang cacat
- Fakta, harga, jarak, durasi, atau kondisi yang tidak diberikan

${THUMB_COLOR_GUARD}`;

function thumbExtraEl() {
  return document.getElementById('thumb-prompt');
}

function loadThumbExtra() {
  const el = thumbExtraEl();
  if (!el) return;
  try {
    const saved = localStorage.getItem(THUMB_EXTRA_KEY);
    el.value = saved != null && String(saved).trim() ? saved : DEFAULT_THUMB_EXTRA;
  } catch {
    el.value = DEFAULT_THUMB_EXTRA;
  }
}

function saveThumbExtra() {
  const el = thumbExtraEl();
  if (!el) return;
  try {
    localStorage.setItem(THUMB_EXTRA_KEY, el.value);
  } catch { /* ignore */ }
}

function fillThumbTemplate(tpl, hook, coverBrief) {
  const brand = String(document.getElementById('brand')?.value || '').replace(/^@/, '').trim();
  const topic = String(document.getElementById('topic')?.value || '').trim();
  let h = String(hook || '').trim();
  if (h.length > 2000) h = h.slice(0, 2000) + '…';
  const lokasi = topic || 'ambil dari hook — jangan mengarang destinasi yang tidak disebut';
  return String(tpl || '')
    .split('{{brand}}').join(brand)
    .split('{{hook}}').join(h || '(hook kosong)')
    .split('{{lokasi}}').join(lokasi)
    .split('{{cover_brief}}').join(String(coverBrief || '').trim() || '(belum tersedia; tentukan adegan dari hook)');
}

function buildThumbExtra(hook, coverBrief) {
  const extra = String(thumbExtraEl()?.value || '').trim() || DEFAULT_THUMB_EXTRA;
  const brief = String(coverBrief || lastPipelineResult?.cover_brief || '').trim();
  return fillThumbTemplate(extra, hook, brief);
}

function updateThumbPromptPreview() {
  const preview = document.getElementById('thumb-prompt-preview');
  if (!preview) return;
  const draft = lastDrafts[0];
  const hook = draft ? draftHook(draft) : String(document.getElementById('topic')?.value || '').trim();
  preview.value = buildThumbExtra(hook, lastPipelineResult?.cover_brief);
}

function showAlert(msg, ok) {
  const el = document.getElementById('gen-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.classList.remove('th-alert-ok', 'th-alert-err');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden', 'th-alert-ok', 'th-alert-err');
  el.classList.add(ok === true ? 'th-alert-ok' : 'th-alert-err');
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
  const product = mem.product || {};
  const legacyProduct = [
    product.name && `Nama produk: ${product.name}`,
    product.audience && `Target pengguna: ${product.audience}`,
    product.description && `Produk membantu: ${product.description}`,
    product.proof && `Fitur/bukti: ${product.proof}`,
    product.cta && `CTA lembut: ${product.cta}`,
  ].filter(Boolean).join('\n');
  const productFields = { 'product-knowledge': product.knowledge || legacyProduct };
  Object.entries(productFields).forEach(([id, value]) => {
    const el = document.getElementById(id);
    if (el && !el.dataset.dirty) el.value = value || '';
  });
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
  return `<div class="gen-thread gen-thread-cols">
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
  updateThumbPromptPreview();
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
              <button type="button" class="th-btn th-btn-soft text-xs" data-lazy="${i}"><i class="bi bi-lightning-charge"></i> Antre Lazy</button>
              <button type="button" class="th-btn th-btn-soft text-xs" data-carousel="${i}"><i class="bi bi-images"></i> Carousel IG</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-copy="${i}"><i class="bi bi-clipboard"></i> Salin</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-thumb="${i}" ${thumbEnabled ? '' : 'disabled title="Set AI_API_KEY di .env"'}><i class="bi bi-image"></i> Thumbnail</button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="good" data-i="${i}" title="Bagus"><i class="bi bi-hand-thumbs-up"></i></button>
              <button type="button" class="th-btn th-btn-ghost text-xs" data-fb="bad" data-i="${i}" title="Jelek"><i class="bi bi-hand-thumbs-down"></i></button>
            </div>
          </div>
          <aside class="gen-thumb-aside">
            <div class="gen-thumb-box gen-thumb-box-side" data-thumb-box="${i}">
              <div class="gen-thumb-placeholder" data-thumb-ph="${i}">Belum ada thumbnail</div>
              <img class="gen-thumb-img" alt="Thumbnail 4:5" hidden />
              <p class="gen-thumb-cap">IG 4:5 · preview</p>
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

function compactCoverTitle(value) {
  let text = String(value || '').replace(/\s+/g, ' ').trim();
  text = text.replace(/^(?:(?:slide|bagian|utas)\s*)?\d+\s*(?:\/|dari)\s*\d+\s*[:.)-]*\s*/i, '').trim();
  const words = text.split(/\s+/).filter(Boolean);
  if (words.length > 24) text = `${words.slice(0, 24).join(' ').replace(/[,;:-]+$/, '')}…`;
  const runes = Array.from(text);
  if (runes.length > 150) text = `${runes.slice(0, 149).join('').replace(/[\s,;:-]+$/, '')}…`;
  return text || 'Untitled';
}

function draftCoverTitle(d) {
  const slides = lastPipelineResult?.package?.story?.slides || lastPipelineResult?.strategy?.slides || [];
  const coverSlide = slides.find((slide) => String(slide?.role || '').toLowerCase() === 'cover')
    || slides.find((slide) => Number(slide?.index) === 1);
  return compactCoverTitle(
    lastPipelineResult?.cover_title
      || coverSlide?.headline
      || d?.title
      || d?.hook
      || draftHook(d),
  );
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
    if (!quiet) Threads.toast('Thumbnail belum aktif', false);
    return null;
  }
  const hook = draftHook(d);
  const coverTitle = draftCoverTitle(d);
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
    const data = await Threads.api('/api/ai/thumbnail', {
      method: 'POST',
      body: JSON.stringify({
        hook,
        model: thumbPreset.model || 'cx/gpt-5.5-image',
        size: '1080x1350',
        quality: thumbPreset.quality || 'high',
        aspect_ratio: '4:5',
        crop_4_3: true,
        custom_only: true,
        extra: buildThumbExtra(hook),
        overlay_white_panel: true,
        overlay_title: coverTitle,
        overlay_handle: String(document.getElementById('brand')?.value || '').replace(/^@/, '').trim(),
        overlay_cta: '',
        overlay_template: activeCoverTemplate(),
      }),
    });
    const url = data.path || data.local_path || data.image_url;
    if (!url) throw new Error('URL thumbnail kosong');
    draftThumbs[i] = url;
    showThumbOnDraft(i, data.path || data.local_path || url);
    if (!quiet) Threads.toast('Thumbnail 4:5 siap', true);
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

async function autoThumbAllDrafts(coverBrief) {
  if (!thumbEnabled || !document.getElementById('auto-thumb')?.checked) return;
  if (!lastDrafts.length) return;
  const n = lastDrafts.length;
  let ok = 0;
  let lastErr = '';
  markStep('preview', 'is-active');
  setPipelineStatus('Generate cover preview…');
  for (let i = 0; i < n; i++) {
    showAlert(`Thumbnail ${i + 1}/${n} (${thumbPreset.model} · 4:5) — bisa 30–90 dtk…`);
    // inject cover into build via lastPipelineResult
    if (coverBrief && lastPipelineResult) lastPipelineResult.cover_brief = coverBrief;
    const url = await generateThumbnailForDraft(i, { quiet: true });
    if (url) ok += 1;
    else {
      const ph = document.querySelector(`[data-thumb-ph="${i}"]`);
      if (ph?.textContent) lastErr = ph.textContent;
    }
  }
  markStep('preview', 'is-done');
  showAlert(ok === n ? '' : (lastErr || `Thumbnail sebagian gagal (${ok}/${n})`));
  Threads.toast(
    ok ? `Thumbnail siap (${ok}/${n})` : (lastErr || 'Thumbnail gagal — teks tetap tersedia'),
    ok > 0,
  );
}

async function loadMemory() {
  const mem = await Threads.api('/api/ai/memory');
  renderMemory(mem);
  return mem;
}

document.getElementById('editorial-prompt').addEventListener('input', () => {
  document.getElementById('editorial-prompt').dataset.dirty = '1';
  document.getElementById('editorial-prompt-status').textContent = 'Belum disimpan';
});

document.getElementById('niche').addEventListener('input', () => {
  document.getElementById('niche').dataset.dirty = '1';
});

document.getElementById('brand').addEventListener('input', () => {
  document.getElementById('brand').dataset.dirty = '1';
});

const PRODUCT_FIELD_IDS = ['product-knowledge'];
PRODUCT_FIELD_IDS.forEach((id) => {
  document.getElementById(id)?.addEventListener('input', (event) => {
    event.currentTarget.dataset.dirty = '1';
    document.getElementById('product-status').textContent = 'Belum disimpan';
  });
});

function productPayload() {
  return {
    knowledge: document.getElementById('product-knowledge').value.trim(),
  };
}

async function saveProduct({ quiet = false } = {}) {
  try {
    const data = await Threads.api('/api/ai/product', {
      method: 'PUT',
      body: JSON.stringify(productPayload()),
    });
    PRODUCT_FIELD_IDS.forEach((id) => delete document.getElementById(id).dataset.dirty);
    renderMemory(data.memory);
    document.getElementById('product-status').textContent = 'Aktif untuk soft selling';
    if (!quiet) Threads.toast('Profil produk disimpan', true);
    return true;
  } catch (e) {
    Threads.toast(e.message, false);
    return false;
  }
}

document.getElementById('btn-save-product').onclick = () => saveProduct();

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

async function saveEditorialPrompt() {
  const el = document.getElementById('editorial-prompt');
  try {
    const data = await Threads.api('/api/ai/editorial-prompt', {
      method: 'PUT',
      body: JSON.stringify({ prompt: el.value }),
    });
    el.value = data.prompt || el.value;
    delete el.dataset.dirty;
    document.getElementById('editorial-prompt-status').textContent = data.custom ? 'Prompt custom tersimpan' : 'Memakai prompt default';
    Threads.toast('Prompt utas disimpan', true);
    return true;
  } catch (e) {
    Threads.toast(e.message, false);
    return false;
  }
}

document.getElementById('btn-save-editorial-prompt').onclick = saveEditorialPrompt;

document.getElementById('btn-reset-editorial-prompt').onclick = async () => {
  try {
    const data = await Threads.api('/api/ai/editorial-prompt');
    const el = document.getElementById('editorial-prompt');
    el.value = data.default || data.prompt || '';
    el.dataset.dirty = '1';
    document.getElementById('editorial-prompt-status').textContent = 'Prompt default dimuat, klik Simpan';
  } catch (e) {
    Threads.toast(e.message, false);
  }
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
  const cancel = document.getElementById('btn-cancel-generate');
  if (cancel) cancel.hidden = !busy;
}

let generateAbort = null;
/** @type {any} */
let lastPipelineResult = null;

const PIPELINE_STEPS = ['intent', 'research', 'strategy', 'story', 'copy', 'visual', 'preview', 'review', 'export'];
const EDITORIAL_SUBSTEPS = ['intent', 'strategy', 'story', 'copy', 'visual'];

function resetPipelineUI() {
  const root = document.getElementById('gen-pipeline');
  if (!root) return;
  root.querySelectorAll('li').forEach((li) => {
    li.classList.remove('is-done', 'is-active', 'is-failed');
  });
  const st = document.getElementById('gen-pipeline-status');
  if (st) st.textContent = 'Menyiapkan pipeline…';
  document.getElementById('strategy-box')?.classList.add('hidden');
  document.getElementById('sources-box')?.classList.add('hidden');
}

function setPipelineStatus(msg) {
  const st = document.getElementById('gen-pipeline-status');
  if (st) st.textContent = msg || '';
}

function markStep(step, state) {
  const li = document.querySelector(`#gen-pipeline [data-step="${step}"]`);
  if (!li) return;
  li.classList.remove('is-done', 'is-active', 'is-failed');
  if (state) li.classList.add(state);
}

function applyPhaseEvent(ev) {
  if (!ev || !ev.stage) return;
  const stage = String(ev.stage);
  const status = String(ev.status || '');
  if (ev.message) setPipelineStatus(ev.message);

  if (stage === 'research') {
    if (status === 'started' || status === 'running') markStep('research', 'is-active');
    if (status === 'done') markStep('research', 'is-done');
    return;
  }
  if (stage === 'editorial') {
    if (status === 'running' || status === 'started') {
      EDITORIAL_SUBSTEPS.forEach((s) => markStep(s, 'is-active'));
    }
    if (status === 'done') {
      // Jujur: satu call selesai → centang sekaligus Intent…Visual
      EDITORIAL_SUBSTEPS.forEach((s) => markStep(s, 'is-done'));
    }
    return;
  }
  if (stage === 'preview') {
    if (status === 'started' || status === 'running') markStep('preview', 'is-active');
    if (status === 'done') markStep('preview', 'is-done');
    return;
  }
  if (stage === 'critic' || stage === 'review') {
    if (status === 'started' || status === 'running') markStep('review', 'is-active');
    if (status === 'done') markStep('review', 'is-done');
    return;
  }
  if (stage === 'revision') {
    if (status === 'started' || status === 'running') {
      markStep('review', 'is-active');
      EDITORIAL_SUBSTEPS.forEach((s) => markStep(s, 'is-active'));
    }
    if (status === 'done') {
      EDITORIAL_SUBSTEPS.forEach((s) => markStep(s, 'is-done'));
      markStep('review', 'is-active');
    }
    return;
  }
  if (stage === 'export') {
    if (status === 'done') {
      markStep('export', 'is-done');
      markStep('review', 'is-done');
      markStep('preview', 'is-done');
    }
  }
}

function renderStrategyPanel(result) {
  const box = document.getElementById('strategy-box');
  const body = document.getElementById('strategy-body');
  const s = result?.strategy;
  if (!box || !body || !s) {
    box?.classList.add('hidden');
    return;
  }
  box.classList.remove('hidden');
  const rows = [
    ['Angle', s.angle],
    ['Why this angle', s.why_this_angle],
    ['Why this story', s.why_this_story],
    ['Why this visual', s.why_this_visual],
    ['Promise', s.content_promise],
    ['Audience', s.target_audience],
    ['Arc', s.arc],
    ['Selling', s.selling_level != null ? String(s.selling_level) : ''],
  ].filter(([, v]) => v != null && String(v).trim());
  body.innerHTML = `<dl class="gen-strategy-dl">${rows.map(([k, v]) =>
    `<div><dt>${Threads.escapeHtml(k)}</dt><dd>${Threads.escapeHtml(String(v))}</dd></div>`).join('')}</dl>`;
}

function renderSourcesPanel(result) {
  const box = document.getElementById('sources-box');
  const body = document.getElementById('sources-body');
  const sources = Array.isArray(result?.sources) ? result.sources.filter(Boolean) : [];
  if (!box || !body || !sources.length) {
    box?.classList.add('hidden');
    return;
  }
  box.classList.remove('hidden');
  body.innerHTML = `<ul class="gen-sources-list">${sources.map((u) =>
    `<li>${Threads.escapeHtml(String(u))}</li>`).join('')}</ul>`;
}

function friendlyStreamError(e) {
  const msg = String(e?.message || e || '');
  if (/input stream|network error|Failed to fetch|Load failed|aborted|NS_BASE_STREAM_CLOSED/i.test(msg)) {
    return 'Stream generate putus (sering nginx timeout atau 9router error). Cek: journalctl -u lazybussiness -n 50, curl http://127.0.0.1:20128/v1/models, proxy_read_timeout 600s untuk /api/ai/.';
  }
  return msg || 'generate gagal';
}

async function readGenerateSSE(res, onEvent) {
  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const chunks = buf.split('\n\n');
      buf = chunks.pop() || '';
      for (const chunk of chunks) {
        const line = chunk.split('\n').find((l) => l.startsWith('data: '));
        if (!line) continue;
        const raw = line.slice(6).trim();
        if (!raw || raw === '[DONE]') continue;
        let ev;
        try { ev = JSON.parse(raw); } catch { continue; }
        onEvent(ev);
      }
    }
  } catch (e) {
    throw new Error(friendlyStreamError(e));
  }
}

document.getElementById('btn-generate-foot')?.addEventListener('click', () => {
  document.getElementById('btn-generate')?.click();
});

document.getElementById('btn-cancel-generate')?.addEventListener('click', () => {
  if (generateAbort) generateAbort.abort();
});

document.getElementById('btn-generate').onclick = async () => {
  showAlert('');
  if (generateAbort) generateAbort.abort();
  generateAbort = new AbortController();
  setGenerateBusy(true);
  resetPipelineUI();
  showAlert('Pipeline generate…');
  lastPipelineResult = null;
  try {
	const product = productPayload();
	if (!product.knowledge) {
	  throw new Error('Isi knowledge produk sebelum generate');
	}
	if (PRODUCT_FIELD_IDS.some((id) => document.getElementById(id).dataset.dirty)) {
	  if (!(await saveProduct({ quiet: true }))) throw new Error('Profil produk gagal disimpan');
	}
    if (document.getElementById('editorial-prompt').dataset.dirty) {
      if (!(await saveEditorialPrompt())) throw new Error('Prompt utas gagal disimpan');
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
    const res = await fetch('/api/ai/generate?stream=1', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      body: JSON.stringify({
        topic: document.getElementById('topic').value.trim(),
        count: 1,
      }),
      signal: generateAbort.signal,
    });
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try {
        const j = await res.json();
        if (j?.error) msg = j.error;
      } catch {}
      throw new Error(msg);
    }

    let result = null;
    await readGenerateSSE(res, (ev) => {
      if (!ev || !ev.type) return;
      if (ev.type === 'phase') {
        applyPhaseEvent(ev);
      } else if (ev.type === 'done') {
        applyPhaseEvent({ stage: 'export', status: 'done' });
        result = ev.result || null;
      } else if (ev.type === 'error') {
        throw new Error(ev.error || 'generate gagal');
      }
    });

    lastPipelineResult = result;
    updateThumbPromptPreview();
    showAlert('');
    if (!result) {
      throw new Error('Tidak ada hasil pipeline');
    }
    const go = result.pipeline?.go !== false && Array.isArray(result.drafts) && result.drafts.length > 0;
    renderDrafts(result);
    renderStrategyPanel(result);
    renderSourcesPanel(result);
    if (!go) {
      const why = result.pipeline?.hold_reason || result.consideration || 'HOLD';
      showAlert(why, false);
      setPipelineStatus('HOLD — ' + why);
      PIPELINE_STEPS.forEach((s) => {
        const li = document.querySelector(`#gen-pipeline [data-step="${s}"]`);
        if (li && !li.classList.contains('is-done')) li.classList.add('is-failed');
      });
      Threads.toast('Generate HOLD', false);
      return;
    }
    setPipelineStatus('Export siap');
    Threads.toast('Draf siap', true);
    await autoThumbAllDrafts(result.cover_brief || '');
  } catch (e) {
    if (e?.name === 'AbortError') {
      showAlert('Generate dibatalkan', false);
      setPipelineStatus('Dibatalkan');
    } else {
      showAlert(e.message);
      Threads.toast(e.message, false);
      setPipelineStatus(e.message || 'Gagal');
    }
  } finally {
    setGenerateBusy(false);
    generateAbort = null;
  }
};

async function enqueueLazyDraft(i) {
  const d = lastDrafts[i];
  if (!d) return;
  const parts = Array.isArray(d?.parts) && d.parts.length
    ? d.parts.map(p => String(p || '').replace(/\\n/g, '\n').trim()).filter(Boolean)
    : draftFullText(d).split(/\n\s*\n/).map(s => s.trim()).filter(Boolean);
  if (parts.length < 2) {
    Threads.toast('Utas minimal 2 bagian untuk Lazy', false);
    return;
  }
  const thumb = draftThumbs[i];
  if (!thumb) {
    Threads.toast('Generate thumbnail dulu (auto thumbnail atau tombol Thumbnail)', false);
    return;
  }
  try {
    const data = await Threads.api('/api/lazy/handoff', {
      method: 'POST',
      body: JSON.stringify({
        parts,
        title: d.title || '',
        cover_title: draftCoverTitle(d),
        thumb_url: thumb,
      }),
    });
    Threads.toast(data.message || 'Diantre ke Lazy', true);
    location.href = '/app/lazy';
  } catch (e) {
    Threads.toast(e.message || String(e), false);
  }
}

document.getElementById('drafts').addEventListener('click', async e => {
  const copy = e.target.closest('[data-copy]');
  const use = e.target.closest('[data-use]');
  const lazyBtn = e.target.closest('[data-lazy]');
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
  if (lazyBtn) {
    await enqueueLazyDraft(Number(lazyBtn.dataset.lazy));
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
    location.href = '/app/buat?from=ai';
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
    // Handoff editorial slides jika ada (bukan regenerate di generate)
    try {
      const slides = lastPipelineResult?.strategy?.slides || lastPipelineResult?.package?.story?.slides;
      if (Array.isArray(slides) && slides.length) {
        localStorage.setItem('threads_carousel_editorial_slides', JSON.stringify(slides));
      }
      if (lastPipelineResult?.package?.visual_direction) {
        localStorage.setItem('threads_carousel_visual_direction', JSON.stringify(lastPipelineResult.package.visual_direction));
      }
    } catch { /* ignore */ }
    location.href = '/app/ig-carousel?from=utas';
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
  loadThumbExtra();
  updateThumbPromptPreview();
  thumbExtraEl()?.addEventListener('input', updateThumbPromptPreview);
  thumbExtraEl()?.addEventListener('change', saveThumbExtra);
  thumbExtraEl()?.addEventListener('blur', saveThumbExtra);
  document.getElementById('topic')?.addEventListener('input', updateThumbPromptPreview);
  document.getElementById('btn-reset-thumb-prompt')?.addEventListener('click', () => {
    const el = thumbExtraEl();
    if (!el) return;
    el.value = DEFAULT_THUMB_EXTRA;
    saveThumbExtra();
    updateThumbPromptPreview();
    Threads.toast('Prompt gambar direset', true);
  });
  try {
    await loadMemory();
  } catch {
    /* ignore */
  }
  try {
    const data = await Threads.api('/api/ai/editorial-prompt');
    const el = document.getElementById('editorial-prompt');
    if (el && !el.dataset.dirty) el.value = data.prompt || '';
    const status = document.getElementById('editorial-prompt-status');
    if (status) status.textContent = data.custom ? 'Prompt custom aktif' : 'Prompt default aktif';
  } catch {
    /* ignore */
  }
  try {
    const def = await Threads.api('/api/ai/thumbnail/defaults');
    thumbEnabled = !!def.enabled;
    if (def.preset) {
      thumbPreset = {
        model: def.preset.model || thumbPreset.model,
        size: '1080x1350',
        quality: def.preset.quality || thumbPreset.quality,
        crop_4_3: true,
      };
    }
    const chip = document.getElementById('thumb-preset');
    if (chip) {
      chip.textContent = thumbEnabled
        ? `${thumbPreset.model} · 4:5 · ${thumbPreset.quality}`
        : 'Thumbnail off';
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
