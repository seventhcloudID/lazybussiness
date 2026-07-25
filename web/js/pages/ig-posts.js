Threads.pageShell('ig-posts');

function typeLabel(t) {
  return String(t || 'MEDIA').toUpperCase();
}

function canBuffer(t) {
  const u = String(t || '').toUpperCase();
  return u === 'IMAGE' || u === 'CAROUSEL_ALBUM';
}

async function sendToBuffer(mediaId, btn) {
  if (!mediaId) return;
  const prev = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i>';
  try {
    const data = await Threads.api('/api/buffer/from-ig', {
      method: 'POST',
      body: JSON.stringify({ media_id: mediaId }),
    });
    Threads.toast(
      `Buffer TikTok Notify Me · ${data.slides || '?'} slide` +
        (data.buffer?.post_id ? ` · ${data.buffer.post_id}` : ''),
      true,
    );
  } catch (e) {
    Threads.toast(e.message || String(e), false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = prev;
  }
}

(async () => {
  const list = document.getElementById('posts-list');
  try {
    const st = await Threads.api('/api/ig/status');
    if (!st.connected) {
      document.getElementById('ig-empty').classList.remove('hidden');
      return;
    }
    let bufferOK = false;
    try {
      const bst = await Threads.api('/api/buffer/status');
      bufferOK = !!bst.enabled;
    } catch { /* ignore */ }

    const data = await Threads.api('/api/ig/media?limit=24');
    const items = data?.data || [];
    const metaBits = [];
    metaBits.push(items.length ? `${items.length} media` : 'Belum ada media');
    metaBits.push(bufferOK ? 'Buffer TikTok siap' : 'Buffer belum (isi di Akun → Kelola)');
    document.getElementById('posts-meta').textContent = metaBits.join(' · ');

    if (!items.length) {
      list.innerHTML = `<div class="col-span-full th-panel"><div class="th-empty">
        <p class="font-semibold">Tidak ada media</p>
      </div></div>`;
      return;
    }
    list.innerHTML = items.map(p => {
      const cap = (p.caption || '').trim();
      const preview = cap.length > 120 ? cap.slice(0, 120) + '…' : cap;
      const src = p.media_url || p.thumbnail_url || '';
      const media = src
        ? `<div class="rounded-xl overflow-hidden border border-line bg-canvas aspect-square mb-3">
             <img src="${Threads.escapeHtml(src)}" class="w-full h-full object-cover" alt="" loading="lazy">
           </div>`
        : '';
      const link = p.permalink
        ? `<a href="${Threads.escapeHtml(p.permalink)}" target="_blank" rel="noopener" class="action-btn" title="Buka di IG"><i class="bi bi-box-arrow-up-right"></i></a>`
        : '';
      const bufBtn = canBuffer(p.media_type) && bufferOK
        ? `<button type="button" class="th-btn th-btn-soft text-xs mt-3 w-full justify-center btn-buffer-ig" data-id="${Threads.escapeHtml(p.id)}">
             <i class="bi bi-tiktok"></i> Kirim ke Buffer TikTok
           </button>`
        : canBuffer(p.media_type)
          ? `<p class="text-[11px] text-muted mt-3 mb-0">Isi Buffer key di <a href="/akun.html" class="underline">Akun → Kelola</a>.</p>`
          : `<p class="text-[11px] text-muted mt-3 mb-0">Video/Reels belum didukung Buffer foto.</p>`;
      return `<div class="post-card p-4">
        ${media}
        <div class="flex items-center justify-between gap-2 mb-2">
          <span class="text-[11px] font-semibold uppercase tracking-wide text-muted">${Threads.escapeHtml(typeLabel(p.media_type))}</span>
          ${link}
        </div>
        <p class="text-sm leading-relaxed text-ink whitespace-pre-wrap">${Threads.escapeHtml(preview || '(tanpa caption)')}</p>
        <div class="flex gap-3 mt-3 text-xs text-muted">
          <span><i class="bi bi-heart"></i> ${Threads.fmtNum(p.like_count)}</span>
          <span><i class="bi bi-chat"></i> ${Threads.fmtNum(p.comments_count)}</span>
          <span>${p.timestamp ? new Date(p.timestamp).toLocaleDateString('id-ID') : ''}</span>
        </div>
        ${bufBtn}
      </div>`;
    }).join('');

    list.addEventListener('click', e => {
      const btn = e.target.closest('.btn-buffer-ig');
      if (!btn) return;
      sendToBuffer(btn.dataset.id, btn);
    });
  } catch (e) {
    Threads.toast(e.message, false);
    document.getElementById('ig-empty').classList.remove('hidden');
  }
})();
