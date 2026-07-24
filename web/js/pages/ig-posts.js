Threads.pageShell('ig-posts');

function typeLabel(t) {
  return String(t || 'MEDIA').toUpperCase();
}

(async () => {
  const list = document.getElementById('posts-list');
  try {
    const st = await Threads.api('/api/ig/status');
    if (!st.connected) {
      document.getElementById('ig-empty').classList.remove('hidden');
      return;
    }
    const data = await Threads.api('/api/ig/media?limit=24');
    const items = data?.data || [];
    document.getElementById('posts-meta').textContent = items.length
      ? `${items.length} media`
      : 'Belum ada media';
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
      </div>`;
    }).join('');
  } catch (e) {
    Threads.toast(e.message, false);
    document.getElementById('ig-empty').classList.remove('hidden');
  }
})();
