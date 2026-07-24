Threads.pageShell('ig-token');

function setStatus(me) {
  document.getElementById('token-status').textContent = me
    ? 'Status: terhubung sebagai @' + me.username + (me.account_type ? ' · ' + me.account_type : '')
    : 'Status: belum terhubung';
}

document.getElementById('btn-connect').onclick = async () => {
  const token = document.getElementById('token-input').value.trim();
  if (!token) return Threads.toast('Isi token Instagram dulu', false);
  try {
    const data = await Threads.api('/api/ig/token', { method: 'POST', body: JSON.stringify({ token }) });
    setStatus(data.me);
    document.getElementById('token-input').value = '';
    Threads.toast('IG terhubung sebagai @' + data.me.username, true);
  } catch (err) {
    Threads.toast(err.message, false);
  }
};

document.getElementById('btn-refresh-token').onclick = async () => {
  try {
    await Threads.api('/api/ig/token/refresh', { method: 'POST', body: '{}' });
    const me = await Threads.api('/api/ig/me');
    setStatus(me);
    Threads.toast('Token Instagram di-refresh', true);
  } catch (err) {
    Threads.toast(err.message, false);
  }
};

document.getElementById('btn-exchange').onclick = async () => {
  const token = document.getElementById('exchange-input').value.trim();
  if (!token) return Threads.toast('Isi short-lived token', false);
  try {
    const data = await Threads.api('/api/ig/token/exchange', {
      method: 'POST',
      body: JSON.stringify({ token }),
    });
    setStatus(data.me || null);
    document.getElementById('exchange-input').value = '';
    Threads.toast(data.me ? 'Long-lived tersimpan · @' + data.me.username : 'Exchange OK', true);
  } catch (err) {
    Threads.toast(err.message, false);
  }
};

document.getElementById('btn-disconnect').onclick = async () => {
  await Threads.api('/api/ig/token', { method: 'DELETE' });
  setStatus(null);
  Threads.toast('Token Instagram diputuskan', true);
};

(async () => {
  try {
    const st = await Threads.api('/api/ig/status');
    if (st.connected) {
      const me = await Threads.api('/api/ig/me');
      setStatus(me);
    }
  } catch {
    document.getElementById('token-status').textContent = 'Status: server Go belum jalan — jalankan: go run .';
  }
})();
