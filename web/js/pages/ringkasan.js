Threads.pageShell('ringkasan');

(async () => {
  const chip = document.getElementById('overview-account');
  const stats = document.getElementById('overview-stats');
  try {
    const st = await Threads.api('/api/status');
    if (!st.connected) return;
    const me = await Threads.api('/api/me');
    if (me?.username) {
      chip.textContent = '@' + me.username;
      chip.className = 'th-chip th-chip-ok';
    } else {
      chip.textContent = 'Terhubung';
      chip.className = 'th-chip th-chip-ok';
    }
    try {
      const insights = await Threads.api('/api/insights?aggregate=1');
      const n = insights.post_count || 0;
      if (n > 0) {
        stats.querySelectorAll('.th-metric-hint').forEach((el, i) => {
          if (i === 0) return; // pengikut tetap total akun
          el.textContent = `Agregat ${n} post terbaru`;
        });
      }
      Threads.applyInsights(insights, stats, { preferPostTotals: true });
    } catch (err) {
      Threads.toast(err.message || 'Gagal muat insight', false);
    }
  } catch {
    chip.textContent = 'Server offline';
    chip.className = 'th-chip th-chip-warn';
  }
})();
