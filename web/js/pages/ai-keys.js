Threads.pageShell('ai-keys');

function showAlert(msg) {
  const el = document.getElementById('keys-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function renderStatus(st) {
  document.getElementById('st-provider').textContent = st.provider || '—';
  document.getElementById('st-model').textContent = st.model || '—';
  document.getElementById('st-total').textContent = String(st.total ?? 0);
  document.getElementById('st-store').textContent = String(st.from_store ?? 0);
  document.getElementById('st-env').textContent = String(st.from_env ?? 0);
  document.getElementById('st-note').textContent = st.note || '';

  const chip = document.getElementById('st-enabled');
  if (st.enabled) {
    chip.textContent = 'AI aktif';
    chip.className = 'th-chip ok';
  } else {
    chip.textContent = 'AI nonaktif — belum ada key';
    chip.className = 'th-chip bad';
  }

  const ul = document.getElementById('st-masked');
  const masked = st.stored_masked || [];
  ul.innerHTML = masked.length
    ? masked.map(m => `<li>${Threads.escapeHtml(m)}</li>`).join('')
    : '<li class="list-none -ml-4">Belum ada key dari UI</li>';
}

async function loadStatus() {
  showAlert('');
  const st = await Threads.api('/api/ai/keys');
  renderStatus(st);
  return st;
}

document.getElementById('btn-refresh').onclick = () =>
  loadStatus().then(() => Threads.toast('Status diperbarui', true)).catch(e => Threads.toast(e.message, false));

document.getElementById('btn-save').onclick = async () => {
  const text = document.getElementById('keys-input').value.trim();
  if (!text) return Threads.toast('Tempel minimal 1 key', false);
  try {
    const data = await Threads.api('/api/ai/keys', {
      method: 'PUT',
      body: JSON.stringify({ keys_text: text }),
    });
    renderStatus(data.keys);
    document.getElementById('keys-input').value = '';
    Threads.toast(`${data.keys.from_store} key UI disimpan · total aktif ${data.keys.total}`, true);
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-clear').onclick = async () => {
  if (!confirm('Hapus semua API key yang disimpan lewat UI? Key di .env tidak diubah.')) return;
  try {
    const data = await Threads.api('/api/ai/keys', { method: 'DELETE' });
    renderStatus(data.keys);
    Threads.toast('Key UI dihapus', true);
  } catch (e) {
    Threads.toast(e.message, false);
  }
};

(async () => {
  try {
    await loadStatus();
  } catch (e) {
    showAlert(e.message);
  }
})();
