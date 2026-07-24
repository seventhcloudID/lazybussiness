Threads.pageShell('token');

function badgeEl(scope) {
  return document.querySelector(`[data-scope="${scope}"] .scope-badge`);
}

function setBadge(scope, state, detail) {
  const b = badgeEl(scope);
  if (!b) return;
  const styles = {
    ok: 'scope-badge bg-emerald-50 text-emerald-700',
    fail: 'scope-badge bg-red-50 text-red-700',
    skip: 'scope-badge bg-stone-100 text-stone-600',
    idle: 'scope-badge border border-line text-muted',
  };
  const labels = { ok: 'OK', fail: 'Gagal', skip: 'Tidak diuji', idle: 'Belum terhubung' };
  b.className = styles[state] || styles.idle;
  b.textContent = labels[state] || labels.idle;
  if (detail) b.title = detail;
}

function resetBadges(state) {
  document.querySelectorAll('[data-scope]').forEach(tr => {
    setBadge(tr.dataset.scope, state);
  });
}

function setStatus(me) {
  document.getElementById('token-status').textContent = me
    ? 'Status: terhubung sebagai @' + me.username
    : 'Status: belum terhubung';
}

function showHelp(missing) {
  const el = document.getElementById('scope-help');
  if (!missing.length) {
    el.classList.add('hidden');
    el.innerHTML = '';
    return;
  }
  el.classList.remove('hidden');
  el.innerHTML = `
    <p class="font-semibold mb-1">Scope berikut belum aktif di token/app:</p>
    <ul class="list-disc pl-5 mb-2">${missing.map(s => `<li><code class="th-code">${Threads.escapeHtml(s)}</code></li>`).join('')}</ul>
    <ol class="list-decimal pl-5 space-y-1 text-[13px]">
      <li>Buka <strong>Meta App Dashboard</strong> → Use cases → Threads API → Permissions → aktifkan scope di atas.</li>
      <li>Pastikan akun Threads kamu jadi <strong>Tester</strong> (atau Advanced Access sudah disetujui).</li>
      <li>Generate ulang long-lived token dengan scope itu <strong>dicentang</strong>, lalu hubungkan lagi di sini.</li>
      <li>Cek di <a class="underline" href="https://developers.facebook.com/tools/debug/accesstoken/" target="_blank" rel="noopener">Access Token Debugger</a> bahwa scope benar-benar ada di token.</li>
    </ol>`;
}

async function probeScopes() {
  const help = document.getElementById('scope-help');
  help.classList.add('hidden');
  document.querySelectorAll('[data-scope] .scope-badge').forEach(b => {
    b.textContent = 'Menguji…';
    b.className = 'scope-badge border border-line text-muted';
  });
  const data = await Threads.api('/api/permissions');
  if (!data.connected) {
    resetBadges('idle');
    return;
  }
  const missing = [];
  Object.entries(data.scopes || {}).forEach(([scope, info]) => {
    if (info.ok === true) setBadge(scope, 'ok');
    else if (info.ok === false) {
      setBadge(scope, 'fail', info.error || '');
      missing.push(scope);
    } else setBadge(scope, 'skip', info.note || '');
  });
  showHelp(missing);
}

document.getElementById('btn-connect').onclick = async () => {
  const token = document.getElementById('token-input').value.trim();
  if (!token) return Threads.toast('Isi token dulu', false);
  try {
    const data = await Threads.api('/api/token', { method: 'POST', body: JSON.stringify({ token }) });
    setStatus(data.me);
    document.getElementById('token-input').value = '';
    Threads.toast('Terhubung sebagai @' + data.me.username, true);
    await probeScopes();
  } catch (err) { Threads.toast(err.message, false); }
};

document.getElementById('btn-refresh-token').onclick = async () => {
  try {
    await Threads.api('/api/token/refresh', { method: 'POST', body: '{}' });
    Threads.toast('Token di-refresh', true);
    await probeScopes();
  } catch (err) { Threads.toast(err.message, false); }
};

document.getElementById('btn-probe').onclick = async () => {
  try {
    await probeScopes();
    Threads.toast('Uji izin selesai', true);
  } catch (err) { Threads.toast(err.message, false); }
};

document.getElementById('btn-disconnect').onclick = async () => {
  await Threads.api('/api/token', { method: 'DELETE' });
  setStatus(null);
  resetBadges('idle');
  showHelp([]);
  Threads.toast('Token diputuskan', true);
};

(async () => {
  try {
    const st = await Threads.api('/api/status');
    if (st.connected) {
      const me = await Threads.api('/api/me');
      setStatus(me);
      await probeScopes();
    }
  } catch {
    document.getElementById('token-status').textContent = 'Status: server Go belum jalan — jalankan: go run .';
  }
})();
