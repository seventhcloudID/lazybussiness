Threads.pageShell('ig-profil');

function fillProfile(me) {
  document.getElementById('ig-empty').classList.add('hidden');
  document.getElementById('ig-profile-panel').classList.remove('hidden');
  document.getElementById('profile-name').textContent = me?.name || me?.username || '—';
  const uname = me?.username ? '@' + String(me.username).replace(/^@/, '') : '@—';
  const userEl = document.getElementById('profile-username');
  if (me?.username) {
    userEl.innerHTML = `<a class="underline" href="https://instagram.com/${encodeURIComponent(String(me.username).replace(/^@/, ''))}" target="_blank" rel="noopener">${Threads.escapeHtml(uname)}</a>`;
  } else {
    userEl.textContent = uname;
  }
  const bio = String(me?.biography || '').trim();
  document.getElementById('profile-bio').textContent = bio || (me?.source === 'repliz' ? 'Profil dari Repliz' : '(tanpa bio)');
  const web = String(me?.website || '').trim();
  const webEl = document.getElementById('profile-website');
  if (web) {
    const href = /^https?:\/\//i.test(web) ? web : 'https://' + web;
    webEl.innerHTML = `<a class="underline" href="${Threads.escapeHtml(href)}" target="_blank" rel="noopener">${Threads.escapeHtml(web)}</a>`;
  } else {
    webEl.textContent = '';
  }
  const type = me?.account_type ? String(me.account_type) : '';
  const src = me?.source === 'repliz' ? 'Repliz' : '';
  document.getElementById('profile-meta').textContent = [type, src].filter(Boolean).join(' · ');
  document.getElementById('profile-id').textContent = me?.id || me?.user_id || '—';
  const setNum = (id, v) => {
    const el = document.getElementById(id);
    if (el) el.textContent = Threads.fmtNum(v);
  };
  setNum('stat-reach', me?.reach);
  setNum('stat-views', me?.views);
  setNum('stat-likes', me?.likes);
  setNum('stat-comments', me?.comments);
  setNum('stat-shares', me?.shares);
  setNum('stat-saves', me?.saves);
  const av = document.getElementById('profile-avatar');
  if (me?.profile_picture_url) {
    av.innerHTML = '<img src="' + Threads.escapeHtml(me.profile_picture_url) + '" alt="" class="w-full h-full object-cover">';
  } else {
    av.textContent = me?.username ? String(me.username).replace(/^@/, '')[0].toUpperCase() : '@';
  }
}

async function loadProfile(accountId) {
  const q = accountId ? ('?account_id=' + encodeURIComponent(accountId)) : '';
  const st = await Threads.api('/api/ig/status' + q);
  if (!st.connected) {
    document.getElementById('ig-profile-panel').classList.add('hidden');
    document.getElementById('ig-empty').classList.remove('hidden');
    return;
  }
  const me = await Threads.api('/api/ig/me' + q);
  fillProfile(me);
}

(async () => {
  try {
    const accs = await Threads.api('/api/repliz/accounts').catch(() => null);
    const igList = (accs?.accounts || []).filter((a) => String(a.type || '').toLowerCase() === 'instagram');
    const sel = document.getElementById('ig-account');
    if (sel && igList.length) {
      const active = accs.active_id || '';
      const pick = igList.find((a) => (a.id || a._id) === active) || igList[0];
      sel.innerHTML = igList.map((a) => {
        const id = a.id || a._id;
        const u = '@' + String(a.username || a.name || id).replace(/^@/, '');
        return `<option value="${Threads.escapeHtml(id)}">${Threads.escapeHtml(u)}</option>`;
      }).join('');
      sel.value = pick.id || pick._id;
      sel.classList.remove('hidden');
      sel.addEventListener('change', () => loadProfile(sel.value).catch((e) => Threads.toast(e.message, false)));
      await loadProfile(sel.value);
      return;
    }
    await loadProfile('');
  } catch (e) {
    Threads.toast(e.message, false);
    document.getElementById('ig-empty').classList.remove('hidden');
  }
})();
