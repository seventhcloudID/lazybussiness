Threads.pageShell('kuota');

function fillBar(barId, used, limit) {
  const el = document.getElementById(barId);
  if (!el || !limit) return;
  const pct = Math.min(100, (used / limit) * 100);
  el.style.width = pct + '%';
  el.style.background = pct >= 90 ? 'var(--danger)' : pct >= 70 ? 'var(--warn)' : 'var(--accent)';
}

function renderAIQuota(q) {
  if (!q) return;
  document.getElementById('ai-quota-tier').textContent = `${q.tier || 'free'} · ${q.model || ''}`;
  const set = (prefix, b) => {
    document.getElementById(`q-${prefix}-used`).textContent = Number(b?.used || 0).toLocaleString('id-ID');
    document.getElementById(`q-${prefix}-limit`).textContent = Number(b?.limit || 0).toLocaleString('id-ID');
    fillBar(`q-${prefix}-bar`, b?.used || 0, b?.limit || 0);
  };
  set('rpd', q.rpd);
  set('rpm', q.rpm);
  set('tpm', q.tpm);
  const parts = [];
  if (q.tokens_today != null) parts.push(`Token hari ini: ${Number(q.tokens_today).toLocaleString('id-ID')}`);
  if (q.note) parts.push(q.note);
  if (q.rpd?.resets_at) parts.push(`Reset RPD: ${q.rpd.resets_at}`);
  document.getElementById('ai-quota-note').textContent = parts.join(' · ');
}

(async () => {
  try {
    const st = await Threads.api('/api/ai/status');
    if (st.quota) renderAIQuota(st.quota);
  } catch {}

  if (!(await Threads.requireConnected())) return;
  try {
    const data = await Threads.api('/api/quota');
    const row = Array.isArray(data?.data) ? data.data[0] : data;
    const usage = row?.quota_usage ?? 0;
    const limit = row?.config?.quota_total ?? 250;
    const rUsage = row?.reply_quota_usage ?? 0;
    const rLimit = row?.reply_config?.quota_total ?? 1000;
    document.getElementById('quota-usage').textContent = usage;
    document.getElementById('quota-limit').textContent = limit;
    fillBar('quota-bar', usage, limit);
    document.getElementById('reply-usage').textContent = rUsage;
    document.getElementById('reply-limit').textContent = Number(rLimit).toLocaleString('id-ID');
    fillBar('reply-bar', rUsage, rLimit);
  } catch (e) {
    Threads.toast(e.message, false);
  }
})();
