Threads.pageShell('posts');

let meCache = null;

function typeIcon(t) {
  const m = String(t || 'TEXT').toUpperCase();
  if (m === 'IMAGE') return 'bi-image';
  if (m === 'VIDEO') return 'bi-camera-video';
  if (m === 'CAROUSEL_ALBUM' || m === 'CAROUSEL') return 'bi-images';
  return 'bi-chat-left-text';
}

function typeLabel(t) {
  const m = String(t || 'TEXT').toUpperCase();
  if (m === 'CAROUSEL_ALBUM') return 'CAROUSEL';
  return m;
}

function skeleton() {
  return Array.from({ length: 6 }).map(() => `
    <div class="post-card p-4 h-full">
      <div class="flex gap-2.5 mb-3">
        <div class="skel w-9 h-9 rounded-full shrink-0"></div>
        <div class="flex-1 space-y-2 pt-1">
          <div class="skel h-2.5 w-24 rounded"></div>
          <div class="skel h-2.5 w-16 rounded"></div>
        </div>
      </div>
      <div class="space-y-2 mb-3">
        <div class="skel h-2.5 w-full rounded"></div>
        <div class="skel h-2.5 w-5/6 rounded"></div>
        <div class="skel h-2.5 w-2/3 rounded"></div>
      </div>
      <div class="skel h-28 w-full rounded-xl"></div>
    </div>`).join('');
}

function emptyState(msg, sub) {
  return `<div class="col-span-full th-panel">
    <div class="th-empty">
      <div class="th-empty-icon"><i class="bi bi-collection"></i></div>
      <p class="font-semibold text-ink mb-1">${Threads.escapeHtml(msg)}</p>
      <p class="text-sm text-muted mb-5">${Threads.escapeHtml(sub || '')}</p>
      <a href="/buat.html" class="th-btn th-btn-primary">Buat post pertama</a>
    </div>
  </div>`;
}

function renderPosts(data) {
  const list = document.getElementById('posts-list');
  const items = (data?.data || []).filter(p => {
    const t = String(p.media_type || '').toUpperCase();
    return t !== 'REPOST_FACADE';
  });
  const meta = document.getElementById('posts-meta');
  meta.textContent = items.length
    ? `${items.length} post · menyesuaikan lebar layar`
    : 'Belum ada post di rentang ini';

  if (!items.length) {
    list.innerHTML = emptyState('Belum ada post', 'Coba ubah filter tanggal, atau buat post baru.');
    return;
  }

  const uname = meCache?.username ? '@' + meCache.username : '@akun';
  const avatar = meCache?.threads_profile_picture_url
    ? `<img src="${Threads.escapeHtml(meCache.threads_profile_picture_url)}" class="w-9 h-9 rounded-full object-cover" alt="">`
    : `<div class="w-9 h-9 rounded-full bg-ink text-white flex items-center justify-center text-xs font-semibold">${Threads.escapeHtml((meCache?.username || 'T')[0].toUpperCase())}</div>`;

  list.innerHTML = items.map(p => {
    const text = (p.text || '').trim();
    const preview = text.length > 160 ? text.slice(0, 160) + '…' : text;
    const media = p.media_url
      ? `<div class="mt-3 rounded-xl overflow-hidden border border-line bg-canvas aspect-[16/10]">
           <img src="${Threads.escapeHtml(p.media_url)}" class="w-full h-full object-cover" alt="" loading="lazy">
         </div>`
      : '';
    const permalink = p.permalink
      ? `<a href="${Threads.escapeHtml(p.permalink)}" target="_blank" rel="noopener" class="action-btn" title="Buka di Threads"><i class="bi bi-box-arrow-up-right"></i></a>`
      : '';

    return `<article class="post-card p-4 flex flex-col h-full">
      <div class="flex gap-2.5 mb-2">
        <div class="shrink-0">${avatar}</div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5 min-w-0">
            <span class="font-semibold text-sm truncate text-ink">${Threads.escapeHtml(uname)}</span>
            <span class="text-muted text-[11px] shrink-0 mono">· ${Threads.fmtDate(p.timestamp)}</span>
          </div>
          <div class="text-[10px] uppercase tracking-wide text-muted mt-0.5 font-semibold">
            <i class="bi ${typeIcon(p.media_type)}"></i> ${Threads.escapeHtml(typeLabel(p.media_type))}
          </div>
        </div>
      </div>
      <p class="whitespace-pre-wrap text-sm leading-relaxed text-ink flex-1 ${preview ? '' : 'text-muted italic'}">${Threads.escapeHtml(preview || 'Tanpa teks')}</p>
      ${media}
      <div class="mt-3 pt-3 border-t border-line flex items-center gap-0.5">
        <button data-insights="${Threads.escapeHtml(p.id)}" class="action-btn" title="Metrik"><i class="bi bi-bar-chart"></i></button>
        <a href="/balasan.html?media_id=${encodeURIComponent(p.id || '')}" class="action-btn" title="Balasan"><i class="bi bi-chat"></i></a>
        ${permalink}
        <button data-copy="${Threads.escapeHtml(p.id)}" class="action-btn" title="Salin ID"><i class="bi bi-copy"></i></button>
        <button data-delete="${Threads.escapeHtml(p.id)}" class="action-btn danger ml-auto" title="Hapus"><i class="bi bi-trash3"></i></button>
      </div>
    </article>`;
  }).join('');
}

async function loadPosts() {
  document.getElementById('posts-list').innerHTML = skeleton();
  const since = document.getElementById('posts-since').value;
  const until = document.getElementById('posts-until').value;
  const q = new URLSearchParams();
  if (since) q.set('since', Math.floor(new Date(since).getTime() / 1000));
  if (until) q.set('until', Math.floor(new Date(until + 'T23:59:59').getTime() / 1000));
  renderPosts(await Threads.api('/api/threads?' + q.toString()));
}

document.getElementById('btn-posts-filter').onclick = () => loadPosts().catch(e => Threads.toast(e.message, false));
document.getElementById('btn-posts-refresh').onclick = () => loadPosts().catch(e => Threads.toast(e.message, false));

document.getElementById('posts-list').addEventListener('click', async e => {
  const del = e.target.closest('[data-delete]');
  const insights = e.target.closest('[data-insights]');
  const copy = e.target.closest('[data-copy]');
  try {
    if (copy) {
      await navigator.clipboard.writeText(copy.dataset.copy);
      Threads.toast('ID disalin', true);
      return;
    }
    if (del) {
      if (!(await Threads.confirm('Hapus post ini?', {
        title: 'Hapus post',
        okLabel: 'Hapus post',
      }))) return;
      try {
        await Threads.api('/api/media/' + del.dataset.delete, { method: 'DELETE' });
        Threads.toast('Post dihapus', true);
        await loadPosts();
      } catch (err) {
        const msg = err.message || String(err);
        if (/permission|threads_delete|#10|#200/i.test(msg)) {
          Threads.toast(
            'Hapus gagal: token belum punya scope threads_delete. Aktifkan di Meta App → generate ulang token → hubungkan di halaman Token.',
            false,
          );
        } else {
          Threads.toast(msg, false);
        }
      }
      return;
    }
    if (insights) {
      const data = await Threads.api('/api/media/' + insights.dataset.insights + '/insights');
      const lines = (data?.data || []).map(m => {
        const v = m.values?.[0]?.value ?? m.total_value?.value ?? '—';
        return `${m.title || m.name}: ${Threads.fmtNum(v)}`;
      });
      if (!lines.length) return Threads.toast('Tidak ada metrik', false);
      await Threads.alert(lines.join('\n'), { title: 'Insight post' });
    }
  } catch (err) { Threads.toast(err.message, false); }
});

(async () => {
  document.getElementById('posts-list').innerHTML = skeleton();
  if (!(await Threads.requireConnected())) {
    document.getElementById('posts-list').innerHTML = emptyState(
      'Belum terhubung',
      'Hubungkan token Threads di Akun & API → Kelola.'
    );
    const cta = document.querySelector('#posts-list a');
    if (cta) { cta.href = '/akun.html'; cta.textContent = 'Ke Akun & API'; }
    return;
  }
  try {
    meCache = await Threads.api('/api/me');
  } catch {}
  try { await loadPosts(); } catch (e) { Threads.toast(e.message, false); }
})();
