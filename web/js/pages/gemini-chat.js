Threads.pageShell('gemini-chat');

/** @type {{ role: string, text: string }[]} */
let messages = [];
let busy = false;
let statusInfo = { model: '', search_model: '', enabled: false };

function showAlert(msg) {
  const el = document.getElementById('chat-alert');
  if (!el) return;
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function setMeta(text) {
  const el = document.getElementById('chat-meta');
  if (el) el.textContent = text || '';
}

function updateChips() {
  const chip = document.getElementById('chat-model-chip');
  const searchOn = document.getElementById('use-search')?.checked;
  if (!chip) return;
  if (!statusInfo.enabled) {
    chip.textContent = 'AI off';
    chip.classList.add('bad');
    return;
  }
  chip.classList.remove('bad');
  chip.textContent = searchOn
    ? (statusInfo.search_model || 'search') + ' · search'
    : (statusInfo.model || 'model');
}

function renderLog() {
  const log = document.getElementById('chat-log');
  if (!log) return;
  if (!messages.length) {
    log.innerHTML = `<div class="gchat-empty">
      <i class="bi bi-chat-square-text"></i>
      <p>Chat dengan Gemini.</p>
    </div>`;
    return;
  }
  log.innerHTML = messages.map((m) => {
    const role = m.role === 'model' || m.role === 'assistant' ? 'model' : 'user';
    const label = role === 'model' ? 'Gemini' : 'You';
    return `<article class="gchat-msg gchat-msg-${role}">
      <div class="gchat-msg-role">${label}</div>
      <div class="gchat-msg-body">${formatBody(m.text)}</div>
    </article>`;
  }).join('');
  log.scrollTop = log.scrollHeight;
}

function formatBody(text) {
  const raw = String(text || '');
  let html = Threads.escapeHtml(raw);
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\n/g, '<br>');
  return html;
}

function setBusy(on) {
  busy = on;
  const btn = document.getElementById('btn-send');
  const input = document.getElementById('chat-input');
  if (btn) {
    btn.disabled = on;
    btn.innerHTML = on
      ? '<i class="bi bi-hourglass-split"></i><span>…</span>'
      : '<i class="bi bi-send-fill"></i><span>Send</span>';
  }
  if (input) input.disabled = on;
}

/** History for API — drop UI error rows. */
function apiMessages() {
  return messages.filter((m) => !String(m.text || '').startsWith('⚠️'));
}

async function sendChat(ev) {
  if (ev) {
    ev.preventDefault();
    ev.stopPropagation();
  }
  const input = document.getElementById('chat-input');
  const text = (input?.value || '').trim();
  if (!text || busy) return false;
  showAlert('');
  messages.push({ role: 'user', text });
  input.value = '';
  renderLog();
  setBusy(true);
  setMeta('…');
  try {
    const useSearch = !!document.getElementById('use-search')?.checked;
    const data = await Threads.api('/api/ai/chat', {
      method: 'POST',
      body: JSON.stringify({
        messages: apiMessages(),
        use_search: useSearch,
        // no system — raw chat
      }),
    });
    const reply = String(data.reply ?? '');
    messages.push({ role: 'model', text: reply || '(empty)' });
    renderLog();
    const bits = [];
    if (data.model) bits.push(data.model);
    if (data.search) bits.push('search');
    if (data.usage?.total_tokens != null) bits.push(data.usage.total_tokens + ' tok');
    setMeta(bits.join(' · '));
  } catch (e) {
    messages.push({
      role: 'model',
      text: '⚠️ ' + (e.message || 'error'),
    });
    renderLog();
    showAlert(e.message || 'error');
    setMeta('');
  } finally {
    setBusy(false);
    input?.focus();
  }
  return false;
}

document.getElementById('btn-send')?.addEventListener('click', sendChat);
document.getElementById('chat-input')?.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    e.stopPropagation();
    sendChat(e);
  }
});
document.getElementById('use-search')?.addEventListener('change', updateChips);
document.getElementById('btn-clear-chat')?.addEventListener('click', () => {
  messages = [];
  showAlert('');
  setMeta('');
  renderLog();
});

(async () => {
  try {
    const st = await Threads.api('/api/ai/status');
    statusInfo = {
      enabled: !!st.enabled,
      model: st.model || '',
      search_model: st.search_model || '',
    };
    updateChips();
  } catch (e) {
    showAlert(e.message || 'status error');
  }
  renderLog();
  document.getElementById('chat-input')?.focus();
})();
