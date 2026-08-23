Threads.pageShell('chat');

const STORE_KEY = 'threads_chat_v1';
const RAIL_KEY = 'threads_chat_rail';
const esc = (s) => Threads.escapeHtml(s);

const STARTERS = [
  { icon: 'bi-graph-up', title: 'Performa aku', text: 'Berdasarkan data real akun yang terhubung: gimana performa Threads aku belakangan? Apa yang harus didobel, apa yang harus distop?' },
  { icon: 'bi-lightbulb', title: 'Ide post hari ini', text: 'Ide 5 post Threads untuk aku, berdasarkan niche + post yang lagi nge-hit. Kasih hook-nya, jangan generic.' },
  { icon: 'bi-chat-quote', title: 'Critique post terakhir', text: 'Critique post terbaru aku pakai metriknya: kenapa performanya begitu, dan kasih rewrite hook.' },
  { icon: 'bi-lightning', title: 'Hook dari yang nge-hit', text: 'Tulis 8 hook baru dengan DNA post terkuat aku. Jangan jiplak kalimat aslinya.' },
];

const $ = (id) => document.getElementById(id);
const rail = $('chat-rail');
const logEl = $('chat-log');
const listEl = $('chat-list');
const inputEl = $('chat-input');
const formEl = $('chat-form');
const sendBtn = $('btn-send');
const stopBtn = $('btn-stop');
const titleEl = $('chat-title');
const subEl = $('chat-sub');
const metaEl = $('chat-meta');
const sysWrap = $('chat-sys');
const sysEl = $('chat-system');
const searchEl = $('use-search');
const searchToggle = $('chat-search-toggle');
const jumpBtn = $('btn-jump');
const searchFilter = $('chat-search');
const fileEl = $('chat-file');
const attachEl = $('chat-attach');
const wantImageEl = $('want-image');
const imageToggle = $('chat-image-toggle');
const reasoningEl = $('chat-reasoning');

let store = loadStore();
let status = null;
let busy = false;
let abortCtl = null;
let stickBottom = true;
let pendingImages = [];
let accountCtx = { handle: '', posts: 0 };

function uid() {
  return 'c_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7);
}

function blankConv() {
  return {
    id: uid(),
    title: 'Chat baru',
    updatedAt: Date.now(),
    system: '',
    useSearch: false,
    wantImage: false,
    reasoning: 'auto',
    responseId: '',
    messages: [],
  };
}

function loadStore() {
  try {
    const raw = JSON.parse(localStorage.getItem(STORE_KEY) || 'null');
    if (raw && Array.isArray(raw.conversations)) {
      if (!raw.activeId && raw.conversations[0]) raw.activeId = raw.conversations[0].id;
      return raw;
    }
  } catch {}
  const first = blankConv();
  return { activeId: first.id, conversations: [first] };
}

function saveStore() {
  try { localStorage.setItem(STORE_KEY, JSON.stringify(store)); } catch {}
}

function active() {
  return store.conversations.find((c) => c.id === store.activeId) || store.conversations[0];
}

function ensureActive() {
  if (!active()) {
    const c = blankConv();
    store.conversations.unshift(c);
    store.activeId = c.id;
  }
  return active();
}

function touch(conv) {
  conv.updatedAt = Date.now();
  store.conversations.sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
  saveStore();
}

function setBusy(on) {
  busy = on;
  inputEl.disabled = on;
  sendBtn.hidden = on;
  stopBtn.hidden = !on;
  sendBtn.disabled = on || !inputEl.value.trim();
  formEl.classList.toggle('is-busy', on);
}

function resizeInput() {
  inputEl.style.height = 'auto';
  inputEl.style.height = Math.min(inputEl.scrollHeight, 180) + 'px';
  sendBtn.disabled = busy || (!inputEl.value.trim() && !pendingImages.length);
}

function scrollLog(force) {
  if (!force && !stickBottom) return;
  logEl.scrollTop = logEl.scrollHeight;
}

function onLogScroll() {
  const gap = logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight;
  stickBottom = gap < 80;
  jumpBtn.hidden = stickBottom || logEl.scrollHeight < logEl.clientHeight + 40;
}

function renderMarkdown(src) {
  const fences = [];
  let text = String(src || '').replace(/\r\n/g, '\n');
  text = text.replace(/```([a-zA-Z0-9_-]*)[ \t]*\n?([\s\S]*?)```/g, (_, lang, code) => {
    const i = fences.length;
    fences.push({ lang: lang || '', code: code.replace(/\n$/, '') });
    return `\0FENCE${i}\0`;
  });
  text = esc(text);

  text = text.replace(/`([^`\n]+)`/g, '<code>$1</code>');
  text = text.replace(/^###### (.*)$/gm, '<h6>$1</h6>');
  text = text.replace(/^##### (.*)$/gm, '<h5>$1</h5>');
  text = text.replace(/^#### (.*)$/gm, '<h4>$1</h4>');
  text = text.replace(/^### (.*)$/gm, '<h3>$1</h3>');
  text = text.replace(/^## (.*)$/gm, '<h2>$1</h2>');
  text = text.replace(/^# (.*)$/gm, '<h1>$1</h1>');
  text = text.replace(/^> (.*)$/gm, '<blockquote>$1</blockquote>');
  text = text.replace(/^(?:---|\*\*\*|___)\s*$/gm, '<hr>');
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  text = text.replace(/__([^_]+)__/g, '<strong>$1</strong>');
  text = text.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, '$1<em>$2</em>');
  text = text.replace(/~~([^~]+)~~/g, '<del>$1</del>');
  text = text.replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
  text = text.replace(/(^|[\s>(])(https?:\/\/[^\s<]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>');

  text = text.replace(/(?:^|\n)((?:\|.+\|\n)+)/g, (block) => {
    const rows = block.trim().split('\n').filter(Boolean);
    if (rows.length < 2 || !/^\|?\s*:?-{3,}/.test(rows[1])) return block;
    const cells = (row) => row.replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
    const head = cells(rows[0]);
    const body = rows.slice(2).map(cells);
    return `<table><thead><tr>${head.map((c) => `<th>${c}</th>`).join('')}</tr></thead><tbody>${
      body.map((r) => `<tr>${r.map((c) => `<td>${c}</td>`).join('')}</tr>`).join('')
    }</tbody></table>`;
  });

  text = text.replace(/(?:(?:^|\n)(?:[-*]|\d+\.) .+(?:\n(?:[-*]|\d+\.) .+)*)/g, (block) => {
    const lines = block.trim().split('\n');
    const ordered = /^\d+\./.test(lines[0]);
    const tag = ordered ? 'ol' : 'ul';
    const items = lines.map((ln) => `<li>${ln.replace(/^(?:[-*]|\d+\.)\s+/, '')}</li>`).join('');
    return `\n<${tag}>${items}</${tag}>`;
  });

  text = text.split(/\n{2,}/).map((chunk) => {
    const t = chunk.trim();
    if (!t) return '';
    if (/^<(h[1-6]|ul|ol|table|blockquote|hr|pre)/.test(t)) return t;
    return `<p>${t.replace(/\n/g, '<br>')}</p>`;
  }).join('\n');

  text = text.replace(/\0FENCE(\d+)\0/g, (_, i) => {
    const f = fences[Number(i)];
    if (!f) return '';
    const lang = f.lang ? `<span>${esc(f.lang)}</span>` : '<span>code</span>';
    return `<pre class="chat-code"><div class="chat-code-head">${lang}<button type="button" class="chat-copy-code">Salin</button></div><code>${esc(f.code)}</code></pre>`;
  });
  return text;
}

function titleFrom(text) {
  const t = String(text || '').replace(/\s+/g, ' ').trim();
  if (!t) return 'Chat baru';
  return t.length > 42 ? t.slice(0, 42) + '…' : t;
}

function timeLabel(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
}

function groupConvs(list) {
  const now = Date.now();
  const day = 86400000;
  const buckets = [
    { key: 'Hari ini', items: [] },
    { key: 'Kemarin', items: [] },
    { key: '7 hari', items: [] },
    { key: 'Lebih lama', items: [] },
  ];
  const startToday = new Date(); startToday.setHours(0, 0, 0, 0);
  list.forEach((c) => {
    const t = c.updatedAt || 0;
    if (t >= startToday.getTime()) buckets[0].items.push(c);
    else if (t >= startToday.getTime() - day) buckets[1].items.push(c);
    else if (t >= now - 7 * day) buckets[2].items.push(c);
    else buckets[3].items.push(c);
  });
  return buckets.filter((b) => b.items.length);
}

function renderRail() {
  const q = (searchFilter.value || '').trim().toLowerCase();
  const list = store.conversations.filter((c) => {
    if (!q) return true;
    const blob = (c.title + ' ' + (c.messages || []).map((m) => m.text).join(' ')).toLowerCase();
    return blob.includes(q);
  });
  if (!list.length) {
    listEl.innerHTML = `<p class="chat-rail-empty">${q ? 'Tidak ketemu.' : 'Belum ada riwayat.'}</p>`;
    return;
  }
  listEl.innerHTML = groupConvs(list).map((g) => `
    <div class="chat-rail-group">
      <div class="chat-rail-label">${esc(g.key)}</div>
      ${g.items.map((c) => `
        <div class="chat-rail-item${c.id === store.activeId ? ' is-on' : ''}" data-id="${esc(c.id)}">
          <button type="button" class="chat-rail-open" data-id="${esc(c.id)}">
            <span class="chat-rail-name">${esc(c.title || 'Chat baru')}</span>
            <span class="chat-rail-time">${esc(timeLabel(c.updatedAt))}</span>
          </button>
          <button type="button" class="chat-rail-more" data-del="${esc(c.id)}" title="Hapus" aria-label="Hapus">
            <i class="bi bi-trash3"></i>
          </button>
        </div>
      `).join('')}
    </div>
  `).join('');
}

function renderEmpty() {
  logEl.innerHTML = `
    <div class="chat-empty">
      <div class="chat-empty-mark" aria-hidden="true"></div>
      <h2>Mau bantu apa hari ini?</h2>
      <p>${accountCtx.handle
        ? `Terhubung ke <strong>${esc(accountCtx.handle)}</strong>${accountCtx.posts ? ` · ${accountCtx.posts} post` : ''}. Tanya soal performa, hook, atau ide — pakai data real akun ini.`
        : 'Chat ini bisa baca data real akun yang sedang aktif (post, metrik, niche).'}</p>
      <div class="chat-starters">
        ${STARTERS.map((s, i) => `
          <button type="button" class="chat-starter" data-starter="${i}">
            <i class="bi ${s.icon}"></i>
            <span>${esc(s.title)}</span>
          </button>
        `).join('')}
      </div>
    </div>`;
}

function msgImages(m) {
  const imgs = m.images || [];
  if (!imgs.length) return '';
  return `<div class="chat-msg-images">${imgs.map((src) =>
    `<a href="${esc(src)}" target="_blank" rel="noopener"><img src="${esc(src)}" alt=""></a>`
  ).join('')}</div>`;
}

function msgSources(m) {
  const sources = (m.sources || []).filter((s) => /^https?:\/\//i.test(s?.url || '')).slice(0, 12);
  if (!sources.length) return '';
  return `<div class="chat-sources"><span class="chat-sources-label">Sumber</span>${sources.map((s) => {
    let fallback = 'Sumber web';
    try { fallback = new URL(s.url).hostname.replace(/^www\./, ''); } catch {}
    return `<a href="${esc(s.url)}" target="_blank" rel="noopener noreferrer"><i class="bi bi-box-arrow-up-right"></i>${esc(s.title || fallback)}</a>`;
  }).join('')}</div>`;
}

function msgHtml(m, i, last) {
  const isUser = m.role === 'user';
  const isErr = m.error || (m.text || '').startsWith('⚠️');
  const who = isUser ? 'Kamu' : 'AI';
  const body = isUser
    ? (m.text ? `<p>${esc(m.text).replace(/\n/g, '<br>')}</p>` : '')
    : (m.text ? renderMarkdown(m.text) : (m.streaming ? '<div class="chat-think-row"><span class="chat-dots" aria-hidden="true"><i></i><i></i><i></i></span><span>Nulis jawaban…</span></div>' : ''));
  const actions = isUser
    ? `<button type="button" class="chat-act" data-edit="${i}" title="Edit & kirim ulang"><i class="bi bi-pencil"></i></button>`
    : `
      <button type="button" class="chat-act" data-copy-msg="${i}" title="Salin"><i class="bi bi-clipboard"></i></button>
      ${last && !isErr && !m.streaming ? `<button type="button" class="chat-act" data-regen="${i}" title="Generate ulang"><i class="bi bi-arrow-clockwise"></i></button>` : ''}
    `;
  return `
    <article class="chat-msg ${isUser ? 'is-user' : 'is-ai'}${isErr ? ' is-err' : ''}" data-i="${i}">
      <div class="chat-msg-avatar" aria-hidden="true">${isUser ? 'K' : '✦'}</div>
      <div class="chat-msg-col">
        <div class="chat-msg-meta">
          <span>${who}</span>
          ${m.model ? `<span class="chat-msg-model" title="${esc([m.requestedModel && `requested: ${m.requestedModel}`, m.route].filter(Boolean).join(' · '))}">${esc(String(m.model).replace(/^.*\//, ''))}</span>` : ''}
          ${m.search ? '<span class="chat-msg-model">web searched</span>' : ''}
        </div>
        ${msgImages(m)}
        <div class="chat-msg-body">${body}</div>
        ${msgSources(m)}
        <div class="chat-msg-acts">${actions}</div>
      </div>
    </article>`;
}

function thinkingHtml() {
  return `
    <article class="chat-msg is-ai is-think" id="chat-think">
      <div class="chat-msg-avatar" aria-hidden="true">✦</div>
      <div class="chat-msg-col">
        <div class="chat-msg-meta"><span>AI</span></div>
        <div class="chat-msg-body">
          <div class="chat-think-row">
            <span class="chat-dots" aria-hidden="true"><i></i><i></i><i></i></span>
            <span>Nulis jawaban…</span>
          </div>
        </div>
      </div>
    </article>`;
}

function renderStage() {
  const conv = ensureActive();
  titleEl.textContent = conv.title || 'Chat baru';
  const model = (status?.chat?.model || status?.model || '').replace(/^.*\//, '') || 'AI';
  const search = conv.useSearch ? ' · Search' : '';
  if (status?.enabled === false) {
    subEl.innerHTML = '<a href="/app/akun">Set API key dulu</a>';
  } else {
    const bits = [model];
    if (accountCtx.handle) bits.push(accountCtx.handle);
    if (search) bits.push(search.replace(/^ · /, ''));
    subEl.textContent = bits.join(' · ');
  }
  sysEl.value = conv.system || '';
  searchEl.checked = !!conv.useSearch;
  if (wantImageEl) wantImageEl.checked = !!conv.wantImage;
  if (reasoningEl) reasoningEl.value = conv.reasoning || 'auto';
  const msgs = conv.messages || [];
  const streaming = busy && msgs.length && msgs[msgs.length - 1].streaming;
  if (!msgs.length && !busy && !inputEl.value.trim() && !pendingImages.length) {
    renderEmpty();
  } else {
    logEl.innerHTML = msgs.map((m, i) => msgHtml(m, i, m.role !== 'user' && i === msgs.length - 1)).join('')
      + (busy && !streaming ? thinkingHtml() : '');
  }
  renderRail();
  updateMeta();
  scrollLog(true);
}

function updateMeta() {
  const parts = [];
  if (status?.quota?.tier) parts.push(status.quota.tier);
  if (status?.quota?.rpd) {
    const u = status.quota.rpd.used ?? 0;
    const lim = status.quota.rpd.limit ?? 0;
    if (lim) parts.push(`${u}/${lim} req`);
  }
  metaEl.textContent = parts.join(' · ');
}

function newChat() {
  if (busy) return;
  const empty = store.conversations.find((c) => !(c.messages || []).length);
  if (empty) {
    store.activeId = empty.id;
  } else {
    const c = blankConv();
    store.conversations.unshift(c);
    store.activeId = c.id;
  }
  saveStore();
  renderStage();
  inputEl.focus();
}

async function deleteConv(id) {
  const conv = store.conversations.find((c) => c.id === id);
  if (!conv) return;
  const ok = await Threads.confirm('Hapus percakapan ini? Tidak bisa dibatalkan.', {
    title: 'Hapus chat',
    okLabel: 'Hapus',
    cancelLabel: 'Batal',
  });
  if (!ok) return;
  store.conversations = store.conversations.filter((c) => c.id !== id);
  if (!store.conversations.length) store.conversations.push(blankConv());
  if (store.activeId === id) store.activeId = store.conversations[0].id;
  saveStore();
  renderStage();
}

function openConv(id) {
  if (busy) return Threads.toast('Tunggu jawaban selesai, atau tekan Stop.', false);
  store.activeId = id;
  saveStore();
  renderStage();
  closeRailMobile();
}

function applyStarter(i) {
  const s = STARTERS[i];
  if (!s) return;
  if (s.title === 'Buat gambar' && wantImageEl) {
    wantImageEl.checked = true;
    ensureActive().wantImage = true;
    saveStore();
  }
  inputEl.value = s.text || '';
  resizeInput();
  inputEl.focus();
  inputEl.setSelectionRange(inputEl.value.length, inputEl.value.length);
}

function payloadMessages(conv) {
  return (conv.messages || [])
    .filter((m) => !m.streaming)
    .filter((m) => m.role === 'user' || m.role === 'assistant' || m.role === 'model')
    .filter((m) => !m.error && !(m.text || '').startsWith('⚠️'))
    .map((m) => ({
      role: m.role === 'assistant' ? 'assistant' : m.role,
      text: m.text || '',
      images: m.images || [],
    }));
}

function patchAssistant(msg) {
  const el = logEl.querySelector('.chat-msg.is-ai:last-child');
  if (!el) {
    renderStage();
    return;
  }
  const body = el.querySelector('.chat-msg-body');
  if (body) {
    body.innerHTML = msg.text
      ? renderMarkdown(msg.text)
      : '<div class="chat-think-row"><span class="chat-dots" aria-hidden="true"><i></i><i></i><i></i></span><span>Nulis jawaban…</span></div>';
  }
  let imgs = el.querySelector('.chat-msg-images');
  if ((msg.images || []).length) {
    const html = msgImages(msg);
    if (imgs) imgs.outerHTML = html;
    else body?.insertAdjacentHTML('beforebegin', html);
  }
  scrollLog();
}

async function complete() {
  const conv = ensureActive();
  abortCtl = new AbortController();
  setBusy(true);
  stickBottom = true;
  const assistant = { role: 'assistant', text: '', images: [], sources: [], model: '', requestedModel: '', route: '', search: false, ts: Date.now(), streaming: true };
  conv.messages.push(assistant);
  renderStage();
  try {
    const body = JSON.stringify({
      messages: payloadMessages(conv),
      system: (conv.system || '').trim(),
      use_search: !!conv.useSearch,
      want_image: !!conv.wantImage,
      previous_response_id: conv.responseId || '',
      conversation_key: String(conv.id || '').slice(0, 64),
      reasoning: conv.reasoning || 'auto',
    });
    const res = await fetch('/api/ai/chat', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      body,
      signal: abortCtl.signal,
    });
    if (res.status === 401) {
      throw new Error('login required');
    }
    const ctype = res.headers.get('content-type') || '';
    if (!res.ok || !ctype.includes('text/event-stream') || !res.body) {
      const text = await res.text();
      let data = null;
      try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
      if (data?.reply) {
        assistant.text = data.reply;
        assistant.model = data.model || '';
        assistant.requestedModel = data.requested_model || '';
        assistant.route = data.route || '';
        assistant.search = !!data.search;
        assistant.images = data.images || [];
        assistant.sources = data.sources || [];
        conv.responseId = data.response_id || '';
        if (data.quota) status = { ...(status || {}), quota: data.quota };
      } else {
        throw new Error(Threads.apiErrorMessage(res, data, text));
      }
    } else {
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const parts = buf.split('\n\n');
        buf = parts.pop() || '';
        for (const block of parts) {
          const line = block.split('\n').find((l) => l.startsWith('data:'));
          if (!line) continue;
          let ev;
          try { ev = JSON.parse(line.slice(5).trim()); } catch { continue; }
          if (ev.type === 'delta' && ev.delta) {
            assistant.text += ev.delta;
            patchAssistant(assistant);
          } else if (ev.type === 'status') {
            const body = logEl.querySelector('.chat-msg.is-ai:last-child .chat-msg-body');
            if (body && ev.delta) {
              body.innerHTML = `<div class="chat-think-row"><span class="chat-dots" aria-hidden="true"><i></i><i></i><i></i></span><span>${esc(ev.delta)}</span></div>`
                + (assistant.text ? renderMarkdown(assistant.text) : '');
            }
          } else if (ev.type === 'image' && ev.image) {
            assistant.images = assistant.images || [];
            assistant.images.push(ev.image);
            patchAssistant(assistant);
          } else if (ev.type === 'done') {
            if (ev.reply && !assistant.text) assistant.text = ev.reply;
            assistant.model = ev.model || assistant.model;
            assistant.requestedModel = ev.requested_model || assistant.requestedModel;
            assistant.route = ev.route || assistant.route;
            assistant.search = !!ev.search;
            assistant.sources = ev.sources || assistant.sources || [];
            conv.responseId = ev.response_id || '';
            if (ev.quota) status = { ...(status || {}), quota: ev.quota };
          } else if (ev.type === 'error') {
            throw new Error(ev.error || 'Gagal memuat jawaban');
          }
        }
      }
    }
    if (!assistant.text && !(assistant.images || []).length) {
      throw new Error('AI tidak mengembalikan jawaban');
    }
  } catch (err) {
    const aborted = /dibatalkan|AbortError/i.test(err?.message || err?.name || '');
    if (aborted) {
      if (!assistant.text && !(assistant.images || []).length) {
        conv.messages.pop();
      }
    } else {
      const message = err.message || 'Gagal memuat jawaban';
      assistant.text = assistant.text
        ? `${assistant.text}\n\n⚠️ Jawaban terputus: ${message}`
        : '⚠️ ' + message;
      assistant.error = true;
    }
  } finally {
    delete assistant.streaming;
    abortCtl = null;
    setBusy(false);
    touch(conv);
    renderStage();
    inputEl.focus();
  }
}

async function sendText(text) {
  const t = String(text || '').trim();
  if ((!t && !pendingImages.length) || busy) return;
  if (status && status.enabled === false) {
    Threads.toast('AI belum dikonfigurasi — isi API key di Akun.', false);
    return;
  }
  const conv = ensureActive();
  const images = pendingImages.map((p) => p.url).filter(Boolean);
  pendingImages = [];
  renderAttach();
  conv.messages.push({ role: 'user', text: t, images, ts: Date.now() });
  if (!conv.title || conv.title === 'Chat baru') conv.title = titleFrom(t || 'Gambar');
  inputEl.value = '';
  resizeInput();
  touch(conv);
  await complete();
}

async function regen() {
  if (busy) return;
  const conv = ensureActive();
  const msgs = conv.messages || [];
  while (msgs.length && msgs[msgs.length - 1].role !== 'user') msgs.pop();
  if (!msgs.length) return;
  conv.responseId = '';
  touch(conv);
  await complete();
}

async function editAt(i) {
  if (busy) return;
  const conv = ensureActive();
  const m = conv.messages[i];
  if (!m || m.role !== 'user') return;
  conv.messages = conv.messages.slice(0, i);
  conv.responseId = '';
  inputEl.value = m.text;
  touch(conv);
  renderStage();
  resizeInput();
  inputEl.focus();
  inputEl.setSelectionRange(inputEl.value.length, inputEl.value.length);
}

function copyText(text, btn) {
  const done = () => {
    if (!btn) return Threads.toast('Disalin', true);
    const prev = btn.innerHTML;
    btn.innerHTML = '<i class="bi bi-check2"></i>';
    setTimeout(() => { btn.innerHTML = prev; }, 1200);
  };
  navigator.clipboard.writeText(text).then(done).catch(() => {
    Threads.toast('Gagal salin', false);
  });
}

function toggleRail() {
  const collapsed = document.body.classList.toggle('chat-rail-off');
  try { localStorage.setItem(RAIL_KEY, collapsed ? 'off' : 'on'); } catch {}
  if (window.innerWidth < 900) {
    document.body.classList.toggle('chat-rail-open', !collapsed);
  }
}

function closeRailMobile() {
  if (window.innerWidth < 900) {
    document.body.classList.add('chat-rail-off');
    document.body.classList.remove('chat-rail-open', 'chat-nav-open');
    const bd = $('chat-backdrop');
    if (bd) bd.hidden = true;
  }
}

function stopGen() {
  try { abortCtl?.abort(); } catch {}
}

async function bootStatus() {
  try {
    status = await Threads.api('/api/ai/status');
  } catch {
    status = { enabled: false };
  }
  const canSearch = status?.chat?.search !== false;
  searchToggle.hidden = !canSearch;
  if (imageToggle) imageToggle.hidden = !(status?.chat?.image ?? status?.thumbnail?.enabled ?? true);
  if (!canSearch) {
    const conv = ensureActive();
    conv.useSearch = false;
    searchEl.checked = false;
  }
  if (status.enabled === false) {
    subEl.innerHTML = '<a href="/app/akun">Set API key dulu</a>';
  }
}

formEl.addEventListener('submit', (e) => {
  e.preventDefault();
  sendText(inputEl.value);
});

inputEl.addEventListener('input', resizeInput);
inputEl.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendText(inputEl.value);
  }
});

function closeDrawers() {
  document.body.classList.remove('chat-nav-open', 'chat-rail-open');
  document.body.classList.add('chat-rail-off');
  $('chat-backdrop').hidden = true;
}

function syncBackdrop() {
  const open = document.body.classList.contains('chat-nav-open')
    || (window.innerWidth < 900 && document.body.classList.contains('chat-rail-open'));
  $('chat-backdrop').hidden = !open;
}

$('btn-app-nav')?.addEventListener('click', () => {
  document.body.classList.toggle('chat-nav-open');
  if (document.body.classList.contains('chat-nav-open')) {
    document.body.classList.add('chat-rail-off');
    document.body.classList.remove('chat-rail-open');
  }
  syncBackdrop();
});
$('chat-backdrop')?.addEventListener('click', closeDrawers);

titleEl.addEventListener('dblclick', async () => {
  const conv = ensureActive();
  const next = await Threads.prompt('Nama percakapan', {
    title: 'Ganti judul',
    okLabel: 'Simpan',
    defaultValue: conv.title === 'Chat baru' ? '' : conv.title,
    placeholder: 'Mis. Hook WA blast',
  });
  if (next == null) return;
  const t = String(next).trim();
  conv.title = t || 'Chat baru';
  touch(conv);
  renderStage();
});

stopBtn.addEventListener('click', stopGen);
$('btn-new-chat').addEventListener('click', newChat);
$('btn-rail-toggle').addEventListener('click', () => {
  document.body.classList.remove('chat-nav-open');
  toggleRail();
  if (window.innerWidth < 900) {
    document.body.classList.toggle('chat-rail-open', !document.body.classList.contains('chat-rail-off'));
  }
  syncBackdrop();
});
$('btn-sys').addEventListener('click', () => {
  sysWrap.hidden = !sysWrap.hidden;
  if (!sysWrap.hidden) sysEl.focus();
});
sysEl.addEventListener('input', () => {
  ensureActive().system = sysEl.value;
  saveStore();
});
searchEl.addEventListener('change', () => {
  ensureActive().useSearch = searchEl.checked;
  saveStore();
  renderStage();
});
wantImageEl?.addEventListener('change', () => {
  ensureActive().wantImage = wantImageEl.checked;
  saveStore();
});
reasoningEl?.addEventListener('change', () => {
  ensureActive().reasoning = reasoningEl.value || 'auto';
  saveStore();
  renderStage();
});
searchFilter.addEventListener('input', renderRail);
jumpBtn.addEventListener('click', () => { stickBottom = true; scrollLog(true); });
logEl.addEventListener('scroll', onLogScroll);

$('btn-clear-chat').addEventListener('click', async () => {
  const conv = ensureActive();
  if (!(conv.messages || []).length) return;
  const ok = await Threads.confirm('Kosongkan isi chat ini? Judul tetap, riwayat pesan hilang.', {
    title: 'Kosongkan chat',
    okLabel: 'Kosongkan',
  });
  if (!ok) return;
  conv.messages = [];
  conv.title = 'Chat baru';
  conv.responseId = '';
  touch(conv);
  renderStage();
});

listEl.addEventListener('click', (e) => {
  const del = e.target.closest('[data-del]');
  if (del) {
    e.stopPropagation();
    deleteConv(del.getAttribute('data-del'));
    return;
  }
  const open = e.target.closest('[data-id]');
  if (open) openConv(open.getAttribute('data-id'));
});

logEl.addEventListener('click', (e) => {
  const starter = e.target.closest('[data-starter]');
  if (starter) {
    applyStarter(Number(starter.getAttribute('data-starter')));
    return;
  }
  const copyCode = e.target.closest('.chat-copy-code');
  if (copyCode) {
    const code = copyCode.closest('pre')?.querySelector('code')?.textContent || '';
    copyText(code, copyCode);
    return;
  }
  const copyMsg = e.target.closest('[data-copy-msg]');
  if (copyMsg) {
    const i = Number(copyMsg.getAttribute('data-copy-msg'));
    const m = ensureActive().messages[i];
    if (m) copyText(m.text, copyMsg);
    return;
  }
  const regenBtn = e.target.closest('[data-regen]');
  if (regenBtn) {
    regen();
    return;
  }
  const editBtn = e.target.closest('[data-edit]');
  if (editBtn) editAt(Number(editBtn.getAttribute('data-edit')));
});

function renderAttach() {
  if (!attachEl) return;
  if (!pendingImages.length) {
    attachEl.hidden = true;
    attachEl.innerHTML = '';
    resizeInput();
    return;
  }
  attachEl.hidden = false;
  attachEl.innerHTML = pendingImages.map((p, i) => `
    <div class="chat-attach-item">
      <img src="${esc(p.preview || p.url)}" alt="">
      <button type="button" class="chat-attach-x" data-rm="${i}" aria-label="Hapus">×</button>
    </div>
  `).join('');
  resizeInput();
}

async function addImageFiles(files) {
  for (const file of files) {
    if (!file.type || !file.type.startsWith('image/')) continue;
    try {
      const preview = await fileToDataURL(file);
      let url = preview;
      try {
        const fd = new FormData();
        fd.append('file', file, file.name || 'image.png');
        const res = await fetch('/api/upload/image', { method: 'POST', body: fd, credentials: 'same-origin' });
        const data = await res.json();
        if (res.ok && (data.path || data.image_url)) url = data.path || data.image_url;
      } catch {}
      pendingImages.push({ url, preview });
    } catch (err) {
      Threads.toast(err.message || 'Gagal baca gambar', false);
    }
  }
  renderAttach();
}

function fileToDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(new Error('Gagal baca file'));
    reader.readAsDataURL(file);
  });
}

$('btn-attach')?.addEventListener('click', () => fileEl?.click());
fileEl?.addEventListener('change', () => {
  const files = [...(fileEl.files || [])];
  fileEl.value = '';
  addImageFiles(files);
});
attachEl?.addEventListener('click', (e) => {
  const btn = e.target.closest('[data-rm]');
  if (!btn) return;
  pendingImages.splice(Number(btn.getAttribute('data-rm')), 1);
  renderAttach();
});
document.addEventListener('paste', (e) => {
  const items = [...(e.clipboardData?.items || [])];
  const files = items.filter((it) => it.type.startsWith('image/')).map((it) => it.getAsFile()).filter(Boolean);
  if (!files.length) return;
  e.preventDefault();
  addImageFiles(files);
});

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if (busy) stopGen();
    else if (document.body.classList.contains('chat-nav-open') || document.body.classList.contains('chat-rail-open')) {
      closeDrawers();
    }
  }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    newChat();
  }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'b') {
    e.preventDefault();
    document.body.classList.remove('chat-nav-open');
    toggleRail();
    syncBackdrop();
  }
});

try {
  if (localStorage.getItem(RAIL_KEY) === 'off') document.body.classList.add('chat-rail-off');
} catch {}
if (window.innerWidth < 900) document.body.classList.add('chat-rail-off');

bootStatus().then(() => {
  renderStage();
  resizeInput();
  inputEl.focus();
  Threads.api('/api/ai/chat/context').then((ctx) => {
    accountCtx = { handle: ctx.handle || '', posts: Number(ctx.posts || 0) };
    renderStage();
  }).catch(() => {});
});
