Threads.pageShell('cari');

document.getElementById('btn-search').onclick = async () => {
  const q = document.getElementById('search-q').value.trim();
  if (!q) return Threads.toast('Isi kata kunci', false);
  if (!(await Threads.requireConnected())) return;
  try {
    const type = document.getElementById('search-type').value;
    const data = await Threads.api('/api/search?q=' + encodeURIComponent(q) + '&search_type=' + encodeURIComponent(type));
    const items = data?.data || [];
    const box = document.getElementById('search-results');
    if (!items.length) {
      box.innerHTML = '<p class="text-muted text-sm">Tidak ada hasil.</p>';
      return;
    }
    box.innerHTML = items.map((m, i) => `<div class="${i ? 'border-t border-line pt-4 mt-4' : ''}">
      <div class="text-xs font-semibold text-accent mb-1">@${Threads.escapeHtml(m.username || '—')} · <span class="text-muted font-medium">${Threads.fmtDate(m.timestamp)}</span></div>
      <div class="whitespace-pre-wrap text-sm leading-relaxed">${Threads.escapeHtml(m.text || '')}</div>
      ${m.permalink ? `<a href="${Threads.escapeHtml(m.permalink)}" target="_blank" rel="noopener" class="th-btn th-btn-ghost !py-1 !px-3 mt-3 text-xs">Buka ↗</a>` : ''}
    </div>`).join('');
  } catch (err) {
    let msg = err.message;
    const box = document.getElementById('search-results');
    if (/permission/i.test(msg)) {
      box.innerHTML = `<div class="text-sm text-red-700">
        <p class="font-semibold mb-1">${Threads.escapeHtml(msg)}</p>
        <p class="text-muted">Pencarian butuh <code>threads_keyword_search</code> aktif di Meta App <strong>dan</strong> tercantum di token. Izin di app saja belum cukup — regenerate token lalu hubungkan ulang. Cek status di halaman Token &amp; Izin.</p>
      </div>`;
    }
    Threads.toast(msg, false);
  }
};
