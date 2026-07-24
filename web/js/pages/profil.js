Threads.pageShell('profil');

(async () => {
  if (!(await Threads.requireConnected())) return;
  try {
    const me = await Threads.api('/api/me');
    document.getElementById('profile-name').textContent = me?.name || '—';
    document.getElementById('profile-username').textContent = me?.username ? '@' + me.username : '@—';
    document.getElementById('profile-bio').textContent = me?.threads_biography || '(tanpa bio)';
    document.getElementById('profile-id').textContent = me?.id || '—';
    const av = document.getElementById('profile-avatar');
    if (me?.threads_profile_picture_url) {
      av.innerHTML = '<img src="' + me.threads_profile_picture_url + '" alt="" class="w-full h-full object-cover">';
    } else {
      av.textContent = me?.username ? me.username[0].toUpperCase() : '@';
    }
  } catch (e) {
    Threads.toast(e.message, false);
  }
})();
