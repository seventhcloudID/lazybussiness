Threads.pageShell('balasan');

let posts = [];
let selectedId = null;
let repliesPayload = null;
let selectSeq = 0;
let running = false;
/** pending roots for selected post */
let pending = [];
const repliesCache = new Map();
const postStats = new Map();

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

function showAlert(msg) {
  const el = document.getElementById('balasan-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
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

function draftReadyCount() {
  return pending.filter(p => String(p.draftText || '').trim() && p.approved !== false && p.selected !== false).length;
}

function hasAnyDraft() {
  return pending.some(p => String(p.draftText || '').trim() || p.statusKind === 'draft' || p.statusKind === 'skip');
}

function updateRunUI() {
  const intent = document.getElementById('ar-intent').value.trim();
  const n = pending.filter(p => p.selected !== false).length;
  const ready = draftReadyCount();
  const anyDraft = hasAnyDraft();
  document.getElementById('btn-run').disabled = running || !selectedId || !n || !intent;
  document.getElementById('btn-send').disabled = running || !selectedId || !ready;
  document.getElementById('btn-discard').disabled = running || !anyDraft;
  const hint = document.getElementById('ar-hint');
  if (!selectedId) {
    hint.textContent = 'Pilih post dulu.';
  } else if (!pending.length) {
    hint.textContent = 'Tidak ada reply yang belum dibalas di post ini.';
  } else if (anyDraft) {
    hint.textContent = ready
      ? `${ready} draf siap kirim. Edit kalau perlu, centang approve, lalu Kirim.`
      : 'Draf ada — centang approve + pastikan teks tidak kosong sebelum kirim.';
  } else {
    hint.textContent = `${n} reply dipilih. Generate draf dulu — belum dikirim otomatis.`;
  }
  document.getElementById('pending-count').textContent = pending.length
    ? `${pending.length} reply` + (ready ? ` · ${ready} draf` : '')
    : '0';
}

function syncPendingFromPayload() {
  const prev = new Map(pending.map(p => [p.id, p]));
  const collected = [];
  const walk = (nodes, depth, parentUser) => {
    for (const r of nodes || []) {
      const mine = r.is_mine || r.reply_status === 'mine';
      if (!mine && !r.answered) {
        const old = prev.get(r.id);
        collected.push({
          id: r.id,
          username: r.username,
          text: r.text,
          timestamp: r.timestamp,
          permalink: r.permalink,
          depth: depth || 0,
          parentUser: parentUser || '',
          selected: old ? old.selected !== false : true,
          approved: old?.approved !== false,
          status: old?.status || '',
          statusKind: old?.statusKind || '',
          draftText: old?.draftText || '',
        });
      }
      if (Array.isArray(r.children) && r.children.length) {
        walk(r.children, (depth || 0) + 1, r.username || parentUser);
      }
    }
  };
  walk(repliesPayload?.data || [], 0, '');
  pending = collected;
}

function renderPosts() {
  const q = (document.getElementById('post-search').value || '').trim().toLowerCase();
  const list = document.getElementById('posts-list');
  const items = posts.filter(p => {
    const t = String(p.media_type || '').toUpperCase();
    if (t === 'REPOST_FACADE') return false;
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
    const pend = st?.pending;
    const total = st?.total;
    const done = st && st.pending === 0 && st.total > 0;
    const ratio = st && st.total > 0 ? Math.max(8, Math.round((st.answered / st.total) * 100)) : (p.has_replies ? 35 : 0);
    const thumb = thumbUrl(p);
    const thumbHtml = thumb
      ? `<img class="ri-post-thumb" src="${Threads.escapeHtml(thumb)}" alt="" loading="lazy">`
      : `<div class="ri-post-thumb is-empty"><i class="bi bi-chat-left-text"></i></div>`;
    const badgeN = pend != null ? pend : (p.has_replies ? null : 0);
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
  const short = previewText(full, 160);
  el.innerHTML = `
    <div class="ri-hero">
      <div class="min-w-0">
        <div class="ri-hero-time">${Threads.fmtDate(post.timestamp)}</div>
        <div class="ri-hero-text" title="${Threads.escapeHtml(full)}">${Threads.escapeHtml(short)}</div>
        <div class="ri-hero-stats">
          <strong>${pending.length} belum</strong> · ${answered} sudah / ${total} total
        </div>
      </div>
      ${thumb ? `<img class="ri-hero-thumb" src="${Threads.escapeHtml(thumb)}" alt="">` : ''}
    </div>`;
}

function renderPending() {
  const root = document.getElementById('pending-list');
  if (!pending.length) {
    root.innerHTML = `<div class="th-empty py-12">
      <p class="font-semibold mb-1">Tidak ada yang pending</p>
      <p class="text-sm text-muted">Semua reply di post ini sudah dibalas.</p>
    </div>`;
    updateRunUI();
    return;
  }

  root.innerHTML = pending.map((p, i) => {
    const depth = Math.min(Number(p.depth) || 0, 6);
    const nest = depth
      ? `<span class="ar-pending-nest">↳ balas @${Threads.escapeHtml(p.parentUser || '…')}</span>`
      : '';
    const full = String(p.text || '(tanpa teks)');
    const hasDraft = String(p.draftText || '').trim() || p.statusKind === 'draft' || p.statusKind === 'skip';
    const draftBlock = hasDraft ? `
      <div class="ar-draft${p.statusKind === 'skip' ? ' is-skip' : ''}">
        <div class="ar-draft-head">
          <label>
            <input type="checkbox" data-approve ${p.approved !== false && p.statusKind !== 'skip' ? 'checked' : ''} ${running || p.statusKind === 'skip' ? 'disabled' : ''}>
            Approve kirim
          </label>
          <span class="ar-draft-meta">${p.statusKind === 'skip' ? 'Dilewati' : 'Draf AI — bisa diedit'}</span>
        </div>
        <textarea class="th-textarea" data-draft rows="3" ${running || p.statusKind === 'skip' ? 'disabled' : ''}>${Threads.escapeHtml(p.draftText || '')}</textarea>
      </div>` : '';
    return `
    <article class="ar-pending${p.selected !== false ? ' is-on' : ''}" data-idx="${i}" style="padding-left:${16 + depth * 14}px">
      <label class="ar-pending-check">
        <input type="checkbox" data-pick ${p.selected !== false ? 'checked' : ''} ${running ? 'disabled' : ''}>
      </label>
      <div class="ar-pending-body">
        <div class="ar-pending-head">
          <strong>@${Threads.escapeHtml(p.username || 'user')}</strong>
          ${nest}
          <span class="text-muted">${relTime(p.timestamp)}</span>
          ${p.status ? `<span class="ar-status ${p.statusKind || ''}">${Threads.escapeHtml(p.status)}</span>` : ''}
        </div>
        <p class="ar-pending-text">${Threads.escapeHtml(full)}</p>
        ${draftBlock}
      </div>
    </article>`;
  }).join('');
  updateRunUI();
}

function renderThreadChrome() {
  const answered = repliesPayload?.answered_count ?? 0;
  const total = repliesPayload?.count ?? 0;
  const pendN = pending.length;
  document.getElementById('thread-title').textContent = 'Belum dibalas';
  document.getElementById('thread-sub').textContent = total
    ? `${pendN} pending · ${answered} sudah / ${total} total`
    : 'Tidak ada balasan';
  setSummary(pendN ? `${pendN} belum dibalas` : (total ? 'Semua sudah dibalas' : '0 balasan'), pendN === 0 && total > 0);

  if (selectedId) {
    postStats.set(selectedId, {
      answered,
      pending: pendN,
      total,
    });
    renderPosts();
    renderSelectedPost(posts.find(p => p.id === selectedId));
  }
  renderPending();
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
  pending = [];
  document.getElementById('ar-progress').textContent = '';
  const post = posts.find(p => p.id === id);
  renderPosts();
  document.getElementById('thread-empty').classList.add('hidden');
  document.getElementById('thread-body').classList.remove('hidden');
  renderSelectedPost(post);
  updateRunUI();

  const cached = !refresh ? repliesCache.get(id) : null;
  const freshEnough = cached && (Date.now() - cached.at) < 60_000;
  if (freshEnough) {
    repliesPayload = cached.data;
    syncPendingFromPayload();
    renderThreadChrome();
  } else {
    document.getElementById('thread-sub').textContent = 'Memuat reply…';
    document.getElementById('pending-list').innerHTML =
      `<div class="th-empty py-10"><p class="text-sm text-muted">Memuat…</p></div>`;
  }

  try {
    const data = await fetchReplies(id, { refresh: refresh || !freshEnough });
    if (seq !== selectSeq || selectedId !== id) return;
    repliesPayload = data;
    syncPendingFromPayload();
    renderThreadChrome();
    const url = new URL(location.href);
    url.searchParams.set('media_id', id);
    history.replaceState(null, '', url);
  } catch (e) {
    if (seq !== selectSeq) return;
    if (!freshEnough) {
      Threads.toast(e.message, false);
      document.getElementById('pending-list').innerHTML =
        `<div class="th-empty py-10"><p class="text-sm text-muted">${Threads.escapeHtml(e.message)}</p></div>`;
    }
  }
}

async function generateDrafts() {
  showAlert('');
  if (!selectedId) return Threads.toast('Pilih post dulu', false);
  const intent = document.getElementById('ar-intent').value.trim();
  if (!intent) return Threads.toast('Isi instruksi dulu', false);

  const queue = pending.filter(p => p.selected !== false);
  if (!queue.length) return Threads.toast('Centang minimal 1 reply', false);

  const post = posts.find(p => p.id === selectedId);
  running = true;
  updateRunUI();
  const btn = document.getElementById('btn-run');
  const prog = document.getElementById('ar-progress');
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Generate…';
  btn.disabled = true;

  queue.forEach(p => {
    p.status = 'AI menulis…';
    p.statusKind = 'run';
  });
  renderPending();
  prog.textContent = 'Generate draf…';

  let drafts = [];
  try {
    const result = await Threads.api('/api/ai/replies', {
      method: 'POST',
      body: JSON.stringify({
        media_id: selectedId,
        post_text: post?.text || '',
        intent,
        only_pending: true,
        limit: queue.length,
        incoming: queue.map(p => ({ id: p.id, username: p.username, text: p.text })),
      }),
    });
    drafts = result.drafts || [];
    if (result.consideration) {
      document.getElementById('ar-hint').textContent = result.consideration;
    }
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
    queue.forEach(p => {
      p.status = 'Gagal AI';
      p.statusKind = 'err';
    });
    renderPending();
    running = false;
    btn.innerHTML = '<i class="bi bi-magic"></i> Generate draf';
    updateRunUI();
    return;
  }

  const byID = new Map(drafts.map(d => [d.reply_to_id, d]));
  let ready = 0;
  let skipped = 0;

  for (const item of pending) {
    if (item.selected === false) continue;
    const d = byID.get(item.id);
    if (!d || d.skip || !String(d.text || '').trim()) {
      item.draftText = '';
      item.approved = false;
      item.status = d?.skip_reason || 'Dilewati AI';
      item.statusKind = 'skip';
      skipped++;
      continue;
    }
    item.draftText = String(d.text).trim();
    item.approved = true;
    item.status = 'Draf siap — review dulu';
    item.statusKind = 'draft';
    ready++;
  }

  running = false;
  btn.innerHTML = '<i class="bi bi-magic"></i> Generate draf';
  prog.textContent = `Draf siap · ${ready}` + (skipped ? ` · ${skipped} skip` : '') + ' — belum dikirim';
  Threads.toast(prog.textContent, true);
  renderPending();
}

async function kirimApproved() {
  showAlert('');
  if (!selectedId) return Threads.toast('Pilih post dulu', false);

  // sinkron teks dari textarea sebelum kirim
  document.querySelectorAll('#pending-list [data-draft]').forEach(el => {
    const card = el.closest('[data-idx]');
    const i = Number(card?.dataset.idx);
    if (pending[i]) pending[i].draftText = el.value;
  });

  const queue = pending.filter(p =>
    p.selected !== false && p.approved !== false && String(p.draftText || '').trim()
  );
  if (!queue.length) return Threads.toast('Tidak ada draf yang di-approve', false);

  running = true;
  updateRunUI();
  const btn = document.getElementById('btn-send');
  const prog = document.getElementById('ar-progress');
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Mengirim…';
  btn.disabled = true;

  let ok = 0;
  let fail = 0;

  for (const item of pending) {
    if (item.selected === false || item.approved === false) continue;
    const text = String(item.draftText || '').trim();
    if (!text) continue;
    item.status = 'Mengirim…';
    item.statusKind = 'run';
    renderPending();
    prog.textContent = `Kirim @${item.username || 'user'}…`;
    try {
      await Threads.api('/api/publish', {
        method: 'POST',
        body: JSON.stringify({
          media_type: 'TEXT',
          text,
          reply_to_id: item.id,
          publish: true,
        }),
      });
      item.status = 'Terkirim';
      item.statusKind = 'ok';
      item.selected = false;
      item.approved = false;
      item.draftText = '';
      ok++;
    } catch (e) {
      item.status = 'Gagal: ' + (e.message || e);
      item.statusKind = 'err';
      fail++;
    }
    renderPending();
    await new Promise(r => setTimeout(r, 650));
  }

  running = false;
  btn.innerHTML = '<i class="bi bi-send"></i> Kirim yang di-approve';
  prog.textContent = `Selesai · ${ok} terkirim` + (fail ? ` · ${fail} gagal` : '');
  Threads.toast(prog.textContent, fail === 0);
  updateRunUI();

  repliesCache.delete(selectedId);
  if (selectedId) await selectPost(selectedId, { refresh: true });
}

function buangDraft() {
  if (running) return;
  pending.forEach(p => {
    if (p.statusKind === 'ok') return;
    p.draftText = '';
    p.approved = true;
    if (p.statusKind === 'draft' || p.statusKind === 'skip' || p.statusKind === 'err' || p.statusKind === 'run') {
      p.status = '';
      p.statusKind = '';
    }
  });
  document.getElementById('ar-progress').textContent = '';
  renderPending();
  Threads.toast('Draf dibuang', true);
}

document.getElementById('posts-list').addEventListener('click', e => {
  const item = e.target.closest('[data-id]');
  if (!item || running) return;
  selectPost(item.dataset.id);
});

document.getElementById('post-search').addEventListener('input', () => renderPosts());
document.getElementById('btn-refresh-posts').onclick = () => {
  if (running) return;
  repliesCache.clear();
  postStats.clear();
  loadPosts();
  if (selectedId) selectPost(selectedId, { refresh: true });
};

document.getElementById('ar-intent').addEventListener('input', updateRunUI);
document.getElementById('btn-run').onclick = () => generateDrafts();
document.getElementById('btn-send').onclick = () => kirimApproved();
document.getElementById('btn-discard').onclick = () => buangDraft();

document.getElementById('pending-list').addEventListener('change', e => {
  const card = e.target.closest('[data-idx]');
  const i = Number(card?.dataset.idx);
  if (i == null || !pending[i] || running) return;
  if (e.target.matches('[data-pick]')) {
    pending[i].selected = e.target.checked;
    card.classList.toggle('is-on', e.target.checked);
    updateRunUI();
    return;
  }
  if (e.target.matches('[data-approve]')) {
    pending[i].approved = e.target.checked;
    updateRunUI();
  }
});

document.getElementById('pending-list').addEventListener('input', e => {
  if (!e.target.matches('[data-draft]')) return;
  const card = e.target.closest('[data-idx]');
  const i = Number(card?.dataset.idx);
  if (!pending[i]) return;
  pending[i].draftText = e.target.value;
  updateRunUI();
});

(async () => {
  updateRunUI();
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
