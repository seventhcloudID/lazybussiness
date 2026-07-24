Threads.pageShell('ig-profil');

(async () => {
  try {
    const st = await Threads.api('/api/ig/status');
    if (!st.connected) {
      document.getElementById('ig-empty').classList.remove('hidden');
      return;
    }
    const me = await Threads.api('/api/ig/me');
    document.getElementById('ig-profile-panel').classList.remove('hidden');
    document.getElementById('profile-name').textContent = me?.name || me?.username || '—';
    document.getElementById('profile-username').textContent = me?.username ? '@' + me.username : '@—';
    document.getElementById('profile-meta').textContent = me?.account_type || '';
    document.getElementById('profile-id').textContent = me?.id || '—';
    document.getElementById('stat-followers').textContent = Threads.fmtNum(me?.followers_count);
    document.getElementById('stat-follows').textContent = Threads.fmtNum(me?.follows_count);
    document.getElementById('stat-media').textContent = Threads.fmtNum(me?.media_count);
    const av = document.getElementById('profile-avatar');
    if (me?.profile_picture_url) {
      av.innerHTML = '<img src="' + me.profile_picture_url + '" alt="" class="w-full h-full object-cover">';
    } else {
      av.textContent = me?.username ? me.username[0].toUpperCase() : '@';
    }
  } catch (e) {
    Threads.toast(e.message, false);
    document.getElementById('ig-empty').classList.remove('hidden');
  }
})();
