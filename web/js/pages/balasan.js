Threads.pageShell('balasan');

let posts = [];
let selectedId = null;
let repliesPayload = null;
let filter = 'all';
let selectSeq = 0;
const repliesCache = new Map();
const postStats = new Map(); // id -> { answered, pending, total }

function previewText(t, n = 90) {
  const s = String(t || '').replace(/\s+/g, ' ').trim();
  if (!s) return '(tanpa teks)';
  return s.length > n ? s.slice(0, n) + '…' : s;
}

function relTime(iso) {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return Threads.fmtDate(iso);
  const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (sec < 60) return 'baru saja';
  const min = Math.round(sec / 60);
  if (min < 60) return min + ' menit lalu';
  const hr = Math.round(min / 60);
  if (hr < 48) return hr + ' jam lalu';
  const day = Math.round(hr / 24);
  if (day < 14) return day + ' hari lalu';
  return Threads.fmtDate(iso);
}

function setSummary(text, ok) {
  const el = document.getElementById('inbox-summary');
  el.textContent = text;
  el.className = 'th-chip' + (ok ? ' th-chip-ok' : '');
}

function thumbUrl(p) {
  return p?.thumbnail_url || p?.media_url || '';
}

function badgeLabel(n) {
  if (n == null) return '';
  if (n > 99) return '99+';
  return String(n);
}

function renderPosts() {
  const q = (document.getElementById('post-search').value || '').trim().toLowerCase();
  const list = document.getElementById('posts-list');
  const items = posts.filter(p => {
    const t = String(p.media_type || '').toUpperCase();
    if (t === 'REPOST_FACADE') return false;
    // Utas berantai: hanya starter (bukan lanjutan reply)
    if (p.is_reply === true) return false;
    if (p.replied_to?.id) return false;
    if (!q) return true;
    return String(p.text || '').toLowerCase().includes(q) || String(p.id || '').includes(q);
  });
  document.getElementById('posts-count').textContent = items.length ? items.length + '' : '';

  if (!items.length) {
    list.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Tidak ada post.</p></div>`;
    return;
  }

  list.innerHTML = items.map(p => {
    const active = p.id === selectedId ? ' is-active' : '';
    const st = postStats.get(p.id);
    const pending = st?.pending;
    const total = st?.total;
    const done = st && st.pending === 0 && st.total > 0;
    const ratio = st && st.total > 0 ? Math.max(8, Math.round((st.answered / st.total) * 100)) : (p.has_replies ? 35 : 0);
    const thumb = thumbUrl(p);
    const thumbHtml = thumb
      ? `<img class="ri-post-thumb" src="${Threads.escapeHtml(thumb)}" alt="" loading="lazy">`
      : `<div class="ri-post-thumb is-empty"><i class="bi bi-chat-left-text"></i></div>`;
    const badgeN = pending != null ? pending : (p.has_replies ? null : 0);
    const badge = badgeN == null
      ? (p.has_replies ? `<span class="ri-post-badge" title="Ada balasan">•</span>` : '')
      : `<span class="ri-post-badge${done ? ' is-clear' : ''}">${badgeLabel(done ? total : badgeN)}</span>`;
    return `<button type="button" class="ri-post${active}${done ? ' is-done' : ''}" data-id="${Threads.escapeHtml(p.id)}">
      <div class="ri-post-text">${Threads.escapeHtml(previewText(p.text, 100))}</div>
      ${thumbHtml}
      <div class="ri-post-bar${done ? ' is-done' : ''}"><span style="width:${ratio}%"></span></div>
      ${badge}
    </button>`;
  }).join('');
}

function renderSelectedPost(post) {
  const el = document.getElementById('selected-post');
  if (!post) {
    el.innerHTML = '';
    return;
  }
  const st = postStats.get(post.id) || {};
  const answered = st.answered ?? repliesPayload?.answered_count ?? 0;
  const total = st.total ?? repliesPayload?.count ?? 0;
  const thumb = thumbUrl(post);
  const full = String(post.text || '(tanpa teks)');
  const short = previewText(full, 140);
  el.innerHTML = `
    <div class="ri-hero">
      <div class="min-w-0">
        <div class="ri-hero-time">${Threads.fmtDate(post.timestamp)}</div>
        <div class="ri-hero-text" title="${Threads.escapeHtml(full)}">${Threads.escapeHtml(short)}</div>
        <div class="ri-hero-stats">
          <strong>${answered} replied</strong> / ${total} total
        </div>
      </div>
      ${thumb ? `<img class="ri-hero-thumb" src="${Threads.escapeHtml(thumb)}" alt="">` : ''}
    </div>`;
}

function filteredRoots() {
  const items = (repliesPayload?.data || []).filter(r => !r.is_mine);
  if (filter === 'pending') return items.filter(r => !r.answered);
  if (filter === 'answered') return items.filter(r => r.answered);
  return items;
}

function renderNode(r) {
  const answered = !!r.answered;
  const hidden = String(r.hide_status || '').toLowerCase().includes('hide')
    || String(r.hide_status || '').toLowerCase().includes('hushed');
  const stateClass = answered ? ' is-answered' : ' is-pending';
  const status = answered
    ? `<span class="ri-status ok">Sudah</span>`
    : `<span class="ri-status pending">Belum</span>`;
  const nestCount = Array.isArray(r.children) ? r.children.length : 0;
  const nestHint = nestCount
    ? `<span class="ri-nest-hint">${nestCount} balasan dalam utas · disembunyikan</span>`
    : '';
  const media = (r.media_url || r.thumbnail_url)
    ? `<div class="ri-media"><img src="${Threads.escapeHtml(r.media_url || r.thumbnail_url)}" alt="" loading="lazy"></div>`
    : '';
  return `<article class="ri-node${stateClass}${hidden ? ' opacity-70' : ''}" data-id="${Threads.escapeHtml(r.id)}">
    <div class="ri-node-head">
      <div class="ri-avatar">${Threads.escapeHtml((r.username || '?')[0].toUpperCase())}</div>
      <span class="ri-user">${Threads.escapeHtml(r.username || '—')}</span>
      <span class="ri-ago">${relTime(r.timestamp)}</span>
      ${status}
    </div>
    <p class="ri-body">${Threads.escapeHtml(previewText(r.text || '', 220))}</p>
    ${media}
    ${nestHint}
    <div class="ri-actions">
      <button type="button" class="th-btn th-btn-soft !py-1 !px-2.5 text-xs" data-compose="${Threads.escapeHtml(r.id)}">
        <i class="bi bi-reply-fill"></i> Balas
      </button>
      <button type="button" class="th-btn th-btn-ghost !py-1 !px-2.5 text-xs" data-hide="${Threads.escapeHtml(r.id)}" data-val="${hidden ? 'false' : 'true'}">
        ${hidden ? 'Tampilkan' : 'Sembunyikan'}
      </button>
      ${r.permalink ? `<a class="th-btn th-btn-ghost !py-1 !px-2.5 text-xs" href="${Threads.escapeHtml(r.permalink)}" target="_blank" rel="noopener"><i class="bi bi-box-arrow-up-right"></i></a>` : ''}
    </div>
    <div class="ri-composer hidden" data-composer-for="${Threads.escapeHtml(r.id)}">
      <textarea class="th-textarea" rows="2" placeholder="Balas @${Threads.escapeHtml(r.username || '')}…"></textarea>
      <div class="flex gap-2 mt-2 justify-end">
        <button type="button" class="th-btn th-btn-ghost !py-1 !px-2.5 text-xs" data-cancel-compose>Batal</button>
        <button type="button" class="th-btn th-btn-primary !py-1 !px-2.5 text-xs" data-send-reply="${Threads.escapeHtml(r.id)}">Kirim</button>
      </div>
    </div>
  </article>`;
}

function renderReplies() {
  const list = document.getElementById('replies-list');
  const items = filteredRoots();
  const pending = repliesPayload?.pending_count ?? 0;
  const answered = repliesPayload?.answered_count ?? 0;
  const total = repliesPayload?.count ?? 0;

  if (selectedId) {
    postStats.set(selectedId, { answered, pending, total });
    renderPosts();
    const post = posts.find(p => p.id === selectedId);
    renderSelectedPost(post);
  }

  document.getElementById('thread-title').textContent = 'Conversation';
  document.getElementById('thread-sub').textContent = total
    ? `${answered} replied / ${total} total`
    : 'Tidak ada balasan';
  setSummary(pending ? `${pending} belum dibalas` : (total ? 'Semua sudah dibalas' : '0 balasan'), pending === 0 && total > 0);

  if (!items.length) {
    list.innerHTML = `<div class="th-empty py-12">
      <p class="font-semibold mb-1">${filter === 'all' ? 'Belum ada balasan' : 'Kosong di filter ini'}</p>
      <p class="text-sm text-muted">Coba filter lain atau pilih post berbeda.</p>
    </div>`;
    return;
  }
  list.innerHTML = items.map(r => renderNode(r)).join('');
}

async function loadPosts() {
  const list = document.getElementById('posts-list');
  list.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Memuat post…</p></div>`;
  try {
    if (!(await Threads.requireConnected())) {
      list.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Hubungkan token dulu.</p></div>`;
      return;
    }
    const data = await Threads.api('/api/threads');
    posts = data?.data || [];
    renderPosts();
  } catch (e) {
    list.innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">${Threads.escapeHtml(e.message)}</p></div>`;
  }
}

async function fetchReplies(id, { refresh = false } = {}) {
  const q = new URLSearchParams({ media_id: id });
  if (refresh) q.set('refresh', '1');
  const data = await Threads.api('/api/replies?' + q.toString());
  repliesCache.set(id, { at: Date.now(), data });
  return data;
}

async function selectPost(id, { refresh = false } = {}) {
  const seq = ++selectSeq;
  selectedId = id;
  const post = posts.find(p => p.id === id);
  renderPosts();
  document.getElementById('thread-empty').classList.add('hidden');
  document.getElementById('thread-body').classList.remove('hidden');
  renderSelectedPost(post);

  const cached = !refresh ? repliesCache.get(id) : null;
  const freshEnough = cached && (Date.now() - cached.at) < 60_000;
  if (freshEnough) {
    repliesPayload = cached.data;
    renderReplies();
  } else {
    document.getElementById('thread-title').textContent = 'Conversation';
    document.getElementById('thread-sub').textContent = 'Memuat thread…';
    document.getElementById('replies-list').innerHTML = `<div class="th-empty py-10"><p class="text-sm text-muted">Memuat…</p></div>`;
  }

  try {
    const data = await fetchReplies(id, { refresh: refresh || !freshEnough });
    if (seq !== selectSeq || selectedId !== id) return;
    repliesPayload = data;
    renderReplies();
    const url = new URL(location.href);
    url.searchParams.set('media_id', id);
    history.replaceState(null, '', url);
  } catch (e) {
    if (seq !== selectSeq) return;
    if (!freshEnough) {
      Threads.toast(e.message, false);
      document.getElementById('replies-list').innerHTML =
        `<div class="th-empty py-10"><p class="text-sm text-muted">${Threads.escapeHtml(e.message)}</p></div>`;
    }
  }
}

document.getElementById('posts-list').addEventListener('click', e => {
  const item = e.target.closest('[data-id]');
  if (!item) return;
  selectPost(item.dataset.id);
});

document.getElementById('post-search').addEventListener('input', () => renderPosts());
document.getElementById('btn-refresh-posts').onclick = () => {
  repliesCache.clear();
  postStats.clear();
  loadPosts();
  if (selectedId) selectPost(selectedId, { refresh: true });
};

document.getElementById('reply-filter').addEventListener('click', e => {
  const btn = e.target.closest('[data-filter]');
  if (!btn) return;
  filter = btn.dataset.filter;
  document.querySelectorAll('#reply-filter [data-filter]').forEach(b => b.classList.toggle('active', b === btn));
  if (repliesPayload) renderReplies();
});

document.getElementById('replies-list').addEventListener('click', async e => {
  const compose = e.target.closest('[data-compose]');
  if (compose) {
    const id = compose.dataset.compose;
    document.querySelectorAll('.ri-composer').forEach(el => {
      el.classList.toggle('hidden', el.dataset.composerFor !== id);
    });
    document.querySelector(`[data-composer-for="${CSS.escape(id)}"] textarea`)?.focus();
    return;
  }
  if (e.target.closest('[data-cancel-compose]')) {
    e.target.closest('.ri-composer')?.classList.add('hidden');
    return;
  }
  const send = e.target.closest('[data-send-reply]');
  if (send) {
    const replyTo = send.dataset.sendReply;
    const text = send.closest('.ri-composer')?.querySelector('textarea')?.value?.trim();
    if (!text) return Threads.toast('Tulis balasan dulu', false);
    send.disabled = true;
    try {
      await Threads.api('/api/publish', {
        method: 'POST',
        body: JSON.stringify({ media_type: 'TEXT', text, reply_to_id: replyTo, publish: true }),
      });
      Threads.toast('Balasan terkirim', true);
      repliesCache.delete(selectedId);
      if (selectedId) await selectPost(selectedId, { refresh: true });
    } catch (err) {
      Threads.toast(err.message, false);
      send.disabled = false;
    }
    return;
  }
  const hideBtn = e.target.closest('[data-hide]');
  if (hideBtn) {
    try {
      await Threads.api('/api/replies/manage', {
        method: 'POST',
        body: JSON.stringify({ reply_id: hideBtn.dataset.hide, hide: hideBtn.dataset.val === 'true' }),
      });
      Threads.toast('Status diubah', true);
      repliesCache.delete(selectedId);
      if (selectedId) await selectPost(selectedId, { refresh: true });
    } catch (err) {
      Threads.toast(err.message, false);
    }
  }
});

(async () => {
  await loadPosts();
  const mid = new URLSearchParams(location.search).get('media_id');
  if (mid) {
    if (!posts.find(p => p.id === mid)) {
      posts.unshift({ id: mid, text: 'Post dari tautan', timestamp: '', has_replies: true });
      renderPosts();
    }
    await selectPost(mid);
  }
})();
