Threads.pageShell('gambar');

(() => {
  const RULES_KEY = 'threads_gambar_rules_v8';
  const RULES_KEY_LEGACY = ['threads_gambar_rules_v7', 'threads_gambar_rules_v6', 'threads_gambar_rules_v5', 'threads_gambar_rules_v4', 'threads_gambar_rules_v3', 'threads_gambar_rules_v2'];
  const FORMAT_KEY = 'threads_gambar_format_v1';
  const DEFAULT_HEADER_THREADS = `buatkan thumbnail Threads landscape 4:3 (lebar > tinggi), kanvas penuh tanpa letterbox.
konteks:
{{hook}}

`;
  const DEFAULT_HEADER_IG = `buatkan gambar feed Instagram portrait 4:5 (tinggi > lebar, mirip 1080x1350), isi penuh frame.
konteks:
{{hook}}

`;
  const DEFAULT_RULES = `desain clean, minimalis, visual-first, kualitas rapi.
warna & cahaya: bright editorial daylight, exposure terang bersih, white balance netral, warna natural hidup, detail shadow terjaga. Jangan sepia/vintage, brown/olive cast, gloomy, underexposed, faded, washed-out, desaturated, atau vignette gelap.
teks di gambar: maksimal 3–6 kata saja (satu judul pendek), huruf tajam & terbaca.
utamakan 1 subjek utama + 1 metafora visual besar — jangan ramai.
penting: safe area kuat — teks & elemen penting minimal ~10–12% dari tepi (terutama atas).
dont:
- jangan menaruh judul mepet ke tepi atas
- jangan UI kecil di tepi, ikon app, kartu "Sponsored", watermark, teks mikro tak terbaca
- jangan tangan/jari/wajah cacat, blob, artefak, teks gibberish
- jangan menulis ulang hook/narasi atau banyak label/bullet
- jangan potong elemen penting di pinggir`;
  const DEFAULT_RULES_REF = `pakai gambar referensi sebagai template gaya: salin palet warna, gaya ilustrasi/ikon, tipografi, spacing, latar, dan tingkat flat/detail.
komposisi boleh mirip kerangka umum, TAPI sesuaikan subjek, pose, aksi, ekspresi, dan metafora visual agar relevan dengan hook (jangan copy pose/objek referensi kalau tidak cocok konteks).
teks di gambar: maksimal 3–6 kata (satu judul pendek terkait hook), gaya huruf mengikuti referensi, tajam & terbaca.
penting: safe area kuat (~10–12% dari tepi, terutama atas); komposisi bersih 1 fokus utama.
dont:
- jangan menaruh judul mepet ke tepi atas
- jangan UI kecil di tepi, ikon app, kartu iklan mikro, teks gibberish
- jangan tangan/jari/wajah cacat atau artefak blur aneh
- jangan 100% menjiplak adegan/pose/objek dari referensi
- jangan ganti mood/palet/style tipografi secara drastis
- jangan menulis ulang hook/narasi sebagai paragraf`;

  const $ = (id) => document.getElementById(id);
  const alertEl = $('img-alert');
  const statusEl = $('img-status');
  const metaEl = $('img-meta');
  const dimEl = $('img-dim');
  const preview = $('img-preview');
  const placeholder = $('img-placeholder');
  const busyEl = $('img-busy');
  const stage = $('img-preview-wrap');
  const openLink = $('img-open');
  const pathEl = $('img-path');
  const rawEl = $('img-raw');
  const hookEl = $('hook');
  const rulesEl = $('rules');
  const formatEl = $('format');
  const previewEl = $('prompt-preview');
  const refFile = $('ref-file');
  const refPreview = $('ref-preview');
  const refPreviewWrap = $('ref-preview-wrap');
  const refName = $('ref-name');
  const btnClearRef = $('btn-clear-ref');
  const btns = [$('btn-gen'), $('btn-gen-side')].filter(Boolean);
  const btnDefaults = $('btn-defaults');
  const btnResetRules = $('btn-reset-rules');

  let headerThreads = DEFAULT_HEADER_THREADS;
  let headerIG = DEFAULT_HEADER_IG;
  let defaultRules = DEFAULT_RULES;
  let defaultRulesRef = DEFAULT_RULES_REF;
  let referenceImage = '';
  let rulesLockedToRef = false;

  function showAlert(msg, ok) {
    if (!alertEl) return;
    if (!msg) {
      alertEl.classList.add('hidden');
      alertEl.textContent = '';
      return;
    }
    alertEl.classList.remove('hidden', 'th-alert-ok', 'th-alert-err');
    alertEl.classList.add(ok ? 'th-alert-ok' : 'th-alert-err');
    alertEl.textContent = msg;
  }

  function setMetaChips(items) {
    if (!metaEl) return;
    metaEl.innerHTML = items
      .filter((x) => x && x.value != null && x.value !== '')
      .map((x) => `<span class="th-chip">${x.label}: <b>${String(x.value)}</b></span>`)
      .join('');
  }

  function aspect() {
    return (formatEl?.value || '4:5') === '4:3' ? '4:3' : '4:5';
  }

  function setBusy(on) {
    btns.forEach((btn) => {
      btn.disabled = !!on;
      btn.innerHTML = on
        ? '<i class="bi bi-hourglass-split"></i> Generating…'
        : btn.id === 'btn-gen'
          ? '<i class="bi bi-magic"></i> Generate'
          : '<i class="bi bi-magic"></i> Generate gambar';
    });
    if (busyEl) busyEl.hidden = !on;
    stage?.classList.toggle('is-busy', !!on);
  }

  function syncFormatUI() {
    const ar = aspect();
    const ig = ar === '4:5';
    stage?.classList.toggle('is-ig', ig);
    stage?.classList.toggle('is-threads', !ig);
    if (dimEl) dimEl.textContent = ig ? '1080×1350' : '1024×768';
    try { localStorage.setItem(FORMAT_KEY, ar); } catch {}
    refreshPreview();
  }

  function showImage(path, width, height) {
    if (!path) return;
    const url = path + (path.includes('?') ? '&' : '?') + 't=' + Date.now();
    preview.hidden = false;
    placeholder.hidden = true;
    preview.src = url;
    openLink.hidden = false;
    openLink.href = path;
    pathEl.textContent = path;
    if (width && height && dimEl) dimEl.textContent = `${width}×${height}`;
  }

  function currentHeader() {
    return aspect() === '4:5' ? (headerIG || DEFAULT_HEADER_IG) : (headerThreads || DEFAULT_HEADER_THREADS);
  }

  function activeDefaultRules() {
    return referenceImage ? defaultRulesRef : defaultRules;
  }

  function buildFullPrompt() {
    const hook = String(hookEl?.value || '').trim() || '(hook kosong)';
    const rules = String(rulesEl?.value || '').trim() || activeDefaultRules();
    return String(currentHeader()).split('{{hook}}').join(hook) + rules;
  }

  function refreshPreview() {
    if (previewEl) previewEl.textContent = buildFullPrompt();
  }

  function persistRules() {
    try { localStorage.setItem(RULES_KEY, rulesEl?.value || ''); } catch {}
  }

  function resetRules() {
    if (rulesEl) rulesEl.value = activeDefaultRules();
    rulesLockedToRef = !!referenceImage;
    persistRules();
    refreshPreview();
  }

  function clearRef() {
    referenceImage = '';
    if (refFile) refFile.value = '';
    if (refPreview) refPreview.removeAttribute('src');
    if (refPreviewWrap) refPreviewWrap.hidden = true;
    if (btnClearRef) btnClearRef.hidden = true;
    if (refName) refName.textContent = '';
    if (rulesLockedToRef && rulesEl) {
      rulesEl.value = defaultRules;
      rulesLockedToRef = false;
      persistRules();
    }
    refreshPreview();
  }

  function setRefFromFile(file) {
    if (!file) return;
    if (file.size > 4 * 1024 * 1024) {
      showAlert('Referensi maksimal 4MB', false);
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      referenceImage = String(reader.result || '');
      if (refPreview) refPreview.src = referenceImage;
      if (refPreviewWrap) refPreviewWrap.hidden = false;
      if (btnClearRef) btnClearRef.hidden = false;
      if (refName) refName.textContent = `${file.name} · ${(file.size / 1024).toFixed(0)} KB`;
      if (formatEl) formatEl.value = '4:5';
      syncFormatUI();
      if (rulesEl) {
        rulesEl.value = defaultRulesRef;
        rulesLockedToRef = true;
        persistRules();
      }
      refreshPreview();
    };
    reader.onerror = () => showAlert('Gagal baca file referensi', false);
    reader.readAsDataURL(file);
  }

  async function loadDefaults() {
    try {
      const def = await Threads.api('/api/ai/thumbnail/defaults');
      const model = def.preset?.model || def.model;
      statusEl.textContent = def.enabled ? `${model} · ${aspect()}` : 'thumbnail OFF';
      statusEl.classList.toggle('th-chip-ok', !!def.enabled);
      statusEl.classList.toggle('th-chip-warn', !def.enabled);
      if (model) $('model').value = model;
      if (def.preset?.quality) $('quality').value = def.preset.quality;
      if (def.prompt_header) headerThreads = def.prompt_header;
      if (def.prompt_rules) defaultRules = def.prompt_rules;
      if (def.prompt_rules_ref) defaultRulesRef = def.prompt_rules_ref;
      setMetaChips([
        { label: 'provider', value: def.provider },
        { label: 'model', value: model },
        { label: 'format', value: aspect() },
        { label: 'ref', value: referenceImage ? 'on' : 'off' },
      ]);
    } catch (e) {
      statusEl.textContent = 'defaults gagal';
      statusEl.classList.add('th-chip-warn');
      showAlert(e.message || String(e), false);
    }
    refreshPreview();
  }

  async function generate() {
    showAlert('');
    const hook = (hookEl?.value || '').trim();
    if (!hook) {
      showAlert('Hook / konteks wajib diisi', false);
      return;
    }
    const rules = (rulesEl?.value || '').trim();
    if (!rules) {
      showAlert('Aturan desain wajib diisi', false);
      return;
    }
    const ar = aspect();
    const prompt = buildFullPrompt();
    setBusy(true);
    statusEl.textContent = referenceImage ? `generate ${ar} + ref…` : `generate ${ar}…`;
    statusEl.classList.remove('th-chip-ok', 'th-chip-warn');
    rawEl.textContent = 'Menunggu response…';
    refreshPreview();
    const started = Date.now();
    try {
      const body = {
        custom_only: true,
        extra: prompt,
        model: ($('model').value || 'cx/gpt-5.5-image').trim(),
        size: ar === '4:5' ? '1024x1536' : '1536x1024',
        aspect_ratio: ar,
        quality: $('quality').value || 'high',
        crop_4_3: true,
      };
      if (referenceImage) body.reference_image = referenceImage;
      const data = await Threads.api('/api/ai/thumbnail', {
        method: 'POST',
        body: JSON.stringify(body),
      });
      const path = data.path || data.local_path || data.image_url;
      const sec = ((Date.now() - started) / 1000).toFixed(1);
      rawEl.textContent = JSON.stringify(data, null, 2);
      if (!path) throw new Error('Response tanpa path/image_url');
      showImage(path, data.width, data.height);
      statusEl.textContent = `${data.model || 'ok'} · ${ar} · ${sec}s`;
      statusEl.classList.add('th-chip-ok');
      setMetaChips([
        { label: 'model', value: data.model },
        { label: 'size', value: data.size || `${data.width}x${data.height}` },
        { label: 'format', value: ar },
        { label: 'ref', value: referenceImage ? 'on' : 'off' },
        { label: 'waktu', value: `${sec}s` },
      ]);
      showAlert(`OK — ${path}`, true);
      Threads.toast?.('Gambar siap', true);
    } catch (e) {
      const msg = e.message || String(e);
      statusEl.textContent = 'gagal';
      statusEl.classList.add('th-chip-warn');
      rawEl.textContent = msg;
      showAlert(msg, false);
      Threads.toast?.(msg, false);
    } finally {
      setBusy(false);
    }
  }

  hookEl?.addEventListener('input', refreshPreview);
  rulesEl?.addEventListener('input', () => {
    rulesLockedToRef = false;
    persistRules();
    refreshPreview();
  });
  formatEl?.addEventListener('change', syncFormatUI);
  refFile?.addEventListener('change', () => {
    const file = refFile.files?.[0];
    if (file) setRefFromFile(file);
  });
  btnClearRef?.addEventListener('click', clearRef);
  btnResetRules?.addEventListener('click', resetRules);
  btns.forEach((btn) => btn.addEventListener('click', generate));
  btnDefaults?.addEventListener('click', loadDefaults);

  function looksLikeStaleRules(text) {
    const s = String(text || '').toLowerCase();
    return s.includes('avatar') ||
      s.includes('tokoh utama') ||
      s.includes('mascot') ||
      s.includes('karakter dari lampiran') ||
      s.includes('hanya sebagai inspirasi') ||
      s.includes('wajib mirip gambar referensi') ||
      s.includes('jangan invent layout') ||
      (s.includes('margin aman') && !s.includes('10–12%') && !s.includes('10-12%')) ||
      (!s.includes('cacat') && !s.includes('gibberish') && s.includes('safe area')) ||
      (s.includes('buat design clean') && !s.includes('3–6') && !s.includes('3-6')) ||
      (s.includes('hasil original') && s.includes('boleh beda'));
  }

  (async () => {
    let savedFormat = '4:5';
    try { savedFormat = localStorage.getItem(FORMAT_KEY) || '4:5'; } catch {}
    if (formatEl) formatEl.value = savedFormat === '4:3' ? '4:3' : '4:5';
    syncFormatUI();

    let saved = '';
    try {
      saved = localStorage.getItem(RULES_KEY) || '';
      for (const k of RULES_KEY_LEGACY) {
        try { localStorage.removeItem(k); } catch {}
      }
      if (looksLikeStaleRules(saved)) {
        saved = '';
        try { localStorage.removeItem(RULES_KEY); } catch {}
      }
    } catch {}
    await loadDefaults();
    if (rulesEl) {
      rulesEl.value = saved || activeDefaultRules();
      if (!saved) persistRules();
    }
    refreshPreview();
  })();
})();
