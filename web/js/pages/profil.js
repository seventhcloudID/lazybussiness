Threads.pageShell('profil');

(async () => {
  try {
    const accs = await Threads.api('/api/repliz/accounts');
    const list = accs.accounts || [];
    const id = accs.active_id || '';
    const me = list.find((x) => (x.id || x._id) === id) || list[0];
    if (!me) {
      Threads.toast('Belum ada akun Repliz', false);
      return;
    }
    document.getElementById('profile-name').textContent = me.name || me.username || '—';
    document.getElementById('profile-username').textContent = me.username ? '@' + String(me.username).replace(/^@/, '') : '@—';
    document.getElementById('profile-bio').textContent = me.biography || me.bio || me.type || '(tanpa bio)';
    document.getElementById('profile-id').textContent = me.id || me._id || '—';
    const av = document.getElementById('profile-avatar');
    if (me.picture) {
      av.innerHTML = '<img src="' + me.picture + '" alt="">';
    } else {
      const u = String(me.username || me.name || '@');
      av.textContent = u.replace(/^@/, '')[0].toUpperCase();
    }
  } catch (e) {
    Threads.toast(e.message, false);
  }
})();
