Threads.pageShell('mentions');

(async () => {
  const tbody = document.getElementById('mentions-tbody');
  if (!(await Threads.requireConnected())) {
    tbody.innerHTML = '<tr><td colspan="4" class="px-4 py-6 text-center text-muted">Hubungkan token dulu.</td></tr>';
    return;
  }
  try {
    const data = await Threads.api('/api/mentions');
    const items = data?.data || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="px-4 py-6 text-center text-muted">Tidak ada mention.</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(m => `<tr>
      <td class="font-semibold text-accent">@${Threads.escapeHtml(m.username || '—')}</td>
      <td>${Threads.escapeHtml(m.text || '')}</td>
      <td class="text-muted whitespace-nowrap">${Threads.fmtDate(m.timestamp)}</td>
      <td class="text-right whitespace-nowrap">
        ${m.permalink ? `<a href="${Threads.escapeHtml(m.permalink)}" target="_blank" rel="noopener" class="th-btn th-btn-ghost !py-1 !px-2.5 text-xs">Buka</a>` : ''}
        <button data-repost="${Threads.escapeHtml(m.id)}" class="th-btn th-btn-soft !py-1 !px-2.5 text-xs">Repost</button>
      </td>
    </tr>`).join('');
  } catch (e) {
    const perm = /permission/i.test(e.message);
    tbody.innerHTML = `<tr><td colspan="4" class="px-4 py-6 text-sm text-red-700">
      <p class="font-semibold mb-1">${Threads.escapeHtml(e.message)}</p>
      ${perm ? `<p class="text-muted">Mention butuh scope <code>threads_manage_mentions</code> di app + di token (bukan hanya threads_basic). Aktifkan di App Dashboard, regenerate token, lalu cek di halaman Token &amp; Izin.</p>` : ''}
    </td></tr>`;
    Threads.toast(e.message, false);
  }
})();

document.getElementById('mentions-tbody').addEventListener('click', async e => {
  const btn = e.target.closest('[data-repost]');
  if (!btn) return;
  try {
    await Threads.api('/api/repost', { method: 'POST', body: JSON.stringify({ media_id: btn.dataset.repost }) });
    Threads.toast('Repost berhasil', true);
  } catch (err) { Threads.toast(err.message, false); }
});
