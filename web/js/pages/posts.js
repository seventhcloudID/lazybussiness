Threads.pageShell('posts');

let meCache = null;
/** @type {Map<string, any>} */
const postsById = new Map();

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

function metricIcon(name) {
  const n = String(name || '').toLowerCase();
  if (n.includes('view')) return 'bi-eye';
  if (n.includes('like')) return 'bi-heart';
  if (n.includes('repl')) return 'bi-chat';
  if (n.includes('quote')) return 'bi-chat-quote';
  if (n.includes('repost') || n.includes('share')) return 'bi-repeat';
  return 'bi-bar-chart';
}

function metricLabel(name) {
  const n = String(name || '').toLowerCase();
  if (n.includes('like')) return 'Suka';
  if (n.includes('repost')) return 'Repost';
  if (n.includes('quote')) return 'Quote';
  if (n.includes('repl')) return 'Balasan';
  if (n.includes('view')) return 'Views';
  return name || 'Metrik';
}

/** Ambil insight tiap post secara paralel. Gagal satu post tidak merusak lainnya. */
async function fetchPostInsights(items) {
  const out = new Map();
  const need = [];
  for (const p of items) {
    if (!p.id) continue;
    if (p.metrics && typeof p.metrics === 'object') {
      const metrics = [];
      for (const [name, value] of Object.entries(p.metrics)) {
        const n = Number(value) || 0;
        if (n) metrics.push({ name, value: n });
      }
      out.set(p.id, metrics);
    } else {
      need.push(p);
    }
  }
  await Promise.all(need.map(async (p) => {
    out.set(p.id, []);
  }));
  return out;
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
      <a href="/app/buat" class="th-btn th-btn-primary">Buat post pertama</a>
    </div>
  </div>`;
}

async function renderPosts(data) {
  const list = document.getElementById('posts-list');
  const items = (data?.data || []).filter(p => {
    const t = String(p.media_type || '').toUpperCase();
    return t !== 'REPOST_FACADE';
  });
  const meta = document.getElementById('posts-meta');
  const accBit = meCache?.username
    ? `@${String(meCache.username).replace(/^@/, '')}${meCache.type ? ' · ' + meCache.type : ''} · `
    : '';
  meta.textContent = items.length
    ? `${accBit}${items.length} post dari Repliz`
    : `${accBit}Belum ada post di rentang ini`;

  if (!items.length) {
    list.innerHTML = emptyState('Belum ada post', 'Coba ubah filter tanggal, atau buat post baru.');
    return;
  }

  // Muat insight semua post secara paralel.
  list.innerHTML = skeleton();
  postsById.clear();
  for (const p of items) {
    if (p.id) postsById.set(String(p.id), p);
  }
  let insights;
  try {
    insights = await fetchPostInsights(items);
  } catch {
    insights = new Map();
  }

  const uname = meCache?.username ? '@' + String(meCache.username).replace(/^@/, '') : '@akun';
  const avatar = meCache?.picture || meCache?.threads_profile_picture_url
    ? `<img src="${Threads.escapeHtml(meCache.picture || meCache.threads_profile_picture_url)}" class="w-9 h-9 rounded-full object-cover" alt="">`
    : `<div class="w-9 h-9 rounded-full bg-ink text-white flex items-center justify-center text-xs font-semibold">${Threads.escapeHtml((meCache?.username || 'R')[0].toUpperCase())}</div>`;

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

    const metrics = insights.get(p.id) || [];
    const hasViews = metrics.some(m => String(m.name).toLowerCase().includes('view'));
    const metricHtml = metrics.length
      ? `<div class="post-metrics">${metrics.map(m => {
          const isHigh = hasViews && String(m.name).toLowerCase().includes('view');
          return `<span class="post-metric${isHigh ? ' is-high' : ''}" title="${Threads.escapeHtml(metricLabel(m.name))}">
            <i class="bi ${metricIcon(m.name)}"></i> ${Threads.fmtNum(m.value)}
          </span>`;
        }).join('')}</div>`
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
      ${metricHtml}
      <div class="mt-3 pt-3 border-t border-line flex items-center gap-0.5">
        <a href="/app/balasan?media_id=${encodeURIComponent(p.id || '')}" class="action-btn" title="Balasan"><i class="bi bi-chat"></i></a>
        ${permalink}
        <button type="button" data-carousel="${Threads.escapeHtml(p.id)}" class="action-btn" title="Buka sebagai carousel IG" ${text ? '' : 'disabled'}><i class="bi bi-images"></i></button>
        <button data-copy="${Threads.escapeHtml(p.id)}" class="action-btn" title="Salin ID"><i class="bi bi-copy"></i></button>
        ${meCache?.source === 'repliz' ? '' : `<button data-delete="${Threads.escapeHtml(p.id)}" class="action-btn danger ml-auto" title="Hapus"><i class="bi bi-trash3"></i></button>`}
      </div>
    </article>`;
  }).join('');
}

function partsFromPostText(text) {
  const raw = String(text || '').trim();
  if (!raw) return [];
  const paras = raw.split(/\n\s*\n/).map((s) => s.trim()).filter(Boolean);
  return paras.length ? paras.slice(0, 20) : [raw];
}

async function openCarouselFromPost(post, btn) {
  const fallback = partsFromPostText(post?.text || '');
  if (!fallback.length && !post?.id) {
    Threads.toast('Post tanpa teks — tidak bisa jadi carousel', false);
    return;
  }

  const prev = btn?.innerHTML;
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<i class="bi bi-hourglass-split"></i>';
  }
  try {
    let parts = fallback;
    // Ambil seluruh utas (root + balasan milik sendiri), bukan cuma 1 kartu.
    if (post?.id && meCache?.source !== 'repliz') {
      try {
        const tp = await Threads.api('/api/media/' + encodeURIComponent(post.id) + '/thread');
        if (Array.isArray(tp?.parts) && tp.parts.length) {
          parts = tp.parts.map((p) => String(p || '').trim()).filter(Boolean);
        }
      } catch {
        // fallback ke teks kartu
      }
    }
    if (!parts.length) {
      throw new Error('Tidak ada teks utas untuk carousel');
    }

    localStorage.setItem('threads_carousel_parts', JSON.stringify(parts));
    localStorage.setItem('threads_compose_parts', JSON.stringify(parts));
    localStorage.removeItem('threads_carousel_caption');
    localStorage.setItem('threads_carousel_gen_caption', '1');
    Threads.toast(`${parts.length} slide · caption AI menyusul…`, true);
    location.href = '/app/ig-carousel?from=posts';
  } catch (e) {
    Threads.toast(e.message || String(e), false);
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = prev;
    }
  }
}

async function loadPosts() {
  document.getElementById('posts-list').innerHTML = skeleton();
  const since = document.getElementById('posts-since').value;
  const until = document.getElementById('posts-until').value;
  const q = new URLSearchParams();
  if (since) q.set('since', Math.floor(new Date(since).getTime() / 1000));
  if (until) q.set('until', Math.floor(new Date(until + 'T23:59:59').getTime() / 1000));
  if (meCache?.id) q.set('account_id', meCache.id);
  await renderPosts(await Threads.api('/api/threads?' + q.toString()));
}

document.getElementById('btn-posts-filter').onclick = () => loadPosts().catch(e => Threads.toast(e.message, false));
document.getElementById('btn-posts-refresh').onclick = () => loadPosts().catch(e => Threads.toast(e.message, false));

document.getElementById('posts-list').addEventListener('click', async e => {
  const del = e.target.closest('[data-delete]');
  const copy = e.target.closest('[data-copy]');
  const car = e.target.closest('[data-carousel]');
  try {
    if (car) {
      const post = postsById.get(String(car.dataset.carousel || ''));
      if (!post) return Threads.toast('Post tidak ditemukan', false);
      await openCarouselFromPost(post, car);
      return;
    }
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
            'Hapus post terpublish tidak tersedia di Repliz. Batalkan jadwal di Kalender jika belum terkirim.',
            false,
          );
        } else {
          Threads.toast(msg, false);
        }
      }
      return;
    }
  } catch (err) { Threads.toast(err.message, false); }
});

(async () => {
  document.getElementById('posts-list').innerHTML = skeleton();
  try {
    const accs = await Threads.api('/api/repliz/accounts');
    const list = accs.accounts || [];
    const id = accs.active_id || '';
    const a = list.find((x) => (x.id || x._id) === id) || list[0];
    if (!a) {
      document.getElementById('posts-list').innerHTML = emptyState(
        'Belum ada akun Repliz',
        'Buka Akun, hubungkan atau pilih akun, lalu kembali ke Post.'
      );
      const cta = document.querySelector('#posts-list a');
      if (cta) { cta.href = '/app/akun'; cta.textContent = 'Ke Akun Repliz'; }
      return;
    }
    meCache = {
      id: a.id || a._id,
      username: a.username || a.name,
      name: a.name,
      picture: a.picture,
      source: 'repliz',
      type: a.type,
    };
    const meta = document.getElementById('posts-meta');
    if (meta) {
      const handle = '@' + String(meCache.username || '').replace(/^@/, '');
      meta.textContent = `${handle} · ${a.type || 'repliz'} — post dari Repliz`;
    }
  } catch (e) {
    document.getElementById('posts-list').innerHTML = emptyState(
      'Repliz belum tersambung',
      e.message || 'Set REPLIZ_ACCESS_KEY di .env'
    );
    return;
  }
  try { await loadPosts(); } catch (e) { Threads.toast(e.message, false); }
})();