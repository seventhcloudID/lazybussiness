Threads.pageShell('ai-insight');

const CACHE_KEY = 'threads_ai_insight_v3';
let lastData = null;

function normalizePayload(data) {
  if (!data || typeof data !== 'object') return data;
  const maybe = String(data.raw || data.summary || '').trim();
  if ((!data.hot_content || !data.hot_content.length) && maybe.startsWith('{')) {
    try {
      const parsed = JSON.parse(maybe);
      return {
        ...parsed,
        provider: data.provider || parsed.provider,
        model: data.model || parsed.model,
        usage: data.usage || parsed.usage,
        quota: data.quota || parsed.quota,
      };
    } catch {}
  }
  return data;
}

function fillQuotaBar(prefix, bucket) {
  const used = bucket?.used ?? 0;
  const limit = bucket?.limit ?? 0;
  const elUsed = document.getElementById(`q-${prefix}-used`);
  const elLimit = document.getElementById(`q-${prefix}-limit`);
  const elBar = document.getElementById(`q-${prefix}-bar`);
  if (elUsed) elUsed.textContent = Number(used).toLocaleString('id-ID');
  if (elLimit) elLimit.textContent = Number(limit).toLocaleString('id-ID');
  if (elBar && limit > 0) {
    const pct = Math.min(100, (used / limit) * 100);
    elBar.style.width = pct + '%';
    elBar.style.background = pct >= 90 ? 'var(--danger)' : pct >= 70 ? 'var(--warn)' : 'var(--accent)';
  }
}

function renderQuota(q) {
  if (!q) return;
  const tier = document.getElementById('ai-quota-tier');
  if (tier) tier.textContent = `${q.tier || 'free'} · ${q.model || ''}`.trim();
  fillQuotaBar('rpd', q.rpd);
  fillQuotaBar('rpm', q.rpm);
  fillQuotaBar('tpm', q.tpm);
  const note = document.getElementById('ai-quota-note');
  if (note) {
    const parts = [q.note || ''];
    if (q.tokens_today != null) parts.unshift(`Token hari ini: ${Number(q.tokens_today).toLocaleString('id-ID')}`);
    if (q.rpd?.resets_at) parts.push(`RPD reset: ${q.rpd.resets_at}`);
    note.textContent = parts.filter(Boolean).join(' · ');
  }
}

function showAlert(msg) {
  const el = document.getElementById('ai-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function setLoading(on) {
  const btns = [document.getElementById('btn-ai-insight'), document.getElementById('btn-ai-empty')];
  btns.forEach(btn => {
    if (!btn) return;
    btn.disabled = on;
    btn.innerHTML = on
      ? '<i class="bi bi-hourglass-split"></i> Menganalisis…'
      : '<i class="bi bi-stars"></i> ' + (btn.id === 'btn-ai-empty' ? 'Analisis sekarang' : 'Analisis akun');
  });
}

function chips(list) {
  return (list || []).map(t => `<span class="th-chip">${Threads.escapeHtml(t)}</span>`).join('');
}

function contentCard(item) {
  return `<article class="ai-content-card">
    <div class="ai-content-label">${Threads.escapeHtml(item.label || 'Post')}</div>
    ${item.excerpt ? `<p class="ai-content-excerpt">${Threads.escapeHtml(item.excerpt)}</p>` : ''}
    <p class="ai-content-why">${Threads.escapeHtml(item.why || '')}</p>
    <div class="foot">
      ${item.proof ? `<div><strong>Bukti</strong><span>${Threads.escapeHtml(item.proof)}</span></div>` : ''}
      ${item.pattern ? `<div><strong>Pola</strong><span>${Threads.escapeHtml(item.pattern)}</span></div>` : ''}
    </div>
  </article>`;
}

function renderReport(raw) {
  const data = normalizePayload(raw);
  lastData = data;
  document.getElementById('ai-empty').classList.add('hidden');
  document.getElementById('ai-report').classList.remove('hidden');

  const summaryLooksJson = String(data.summary || '').trim().startsWith('{');
  if (summaryLooksJson && !data.hot_content?.length) {
    document.getElementById('ai-headline').textContent = 'Breakdown gagal dirender';
    document.getElementById('ai-summary').textContent = 'Hasil AI masih format mentah. Klik Analisis akun lagi.';
    document.getElementById('ai-meta').textContent = [data.provider, data.model].filter(Boolean).join(' · ');
    return;
  }

  document.getElementById('ai-headline').textContent = data.headline || 'Breakdown akun';
  document.getElementById('ai-summary').textContent = data.summary || '';
  const meta = [data.provider, data.model].filter(Boolean);
  if (data.usage?.total_tokens) {
    const u = data.usage;
    meta.push(
      `token ${Number(u.total_tokens).toLocaleString('id-ID')} ` +
      `(prompt ${Number(u.prompt_tokens || 0).toLocaleString('id-ID')} + ` +
      `output ${Number(u.completion_tokens || 0).toLocaleString('id-ID')})`
    );
  }
  document.getElementById('ai-meta').textContent = meta.join(' · ');
  if (data.quota) renderQuota(data.quota);

  const ar = data.account_read || {};
  document.getElementById('ai-account-read').innerHTML = [
    { t: 'Niche', v: ar.niche, k: 'opportunity', icon: 'bi-bullseye' },
    { t: 'Voice', v: ar.voice, k: 'strength', icon: 'bi-mic' },
    { t: 'Audience', v: ar.audience, k: 'weakness', icon: 'bi-people' },
    { t: 'Positioning', v: ar.positioning, k: 'opportunity', icon: 'bi-geo' },
  ].filter(x => x.v).map(x => `
    <div class="ai-score ${x.k}">
      <div class="ai-score-label"><i class="bi ${x.icon}"></i> ${x.t}</div>
      <p>${Threads.escapeHtml(x.v)}</p>
    </div>`).join('') || '';

  const sc = data.scorecard || {};
  document.getElementById('ai-scorecard').innerHTML = [
    { t: 'Kekuatan', v: sc.strength, k: 'strength', icon: 'bi-graph-up-arrow' },
    { t: 'Kelemahan', v: sc.weakness, k: 'weakness', icon: 'bi-exclamation-triangle' },
    { t: 'Signal', v: sc.opportunity, k: 'opportunity', icon: 'bi-radar' },
  ].map(x => `
    <div class="ai-score ${x.k}">
      <div class="ai-score-label"><i class="bi ${x.icon}"></i> ${x.t}</div>
      <p>${Threads.escapeHtml(x.v || '—')}</p>
    </div>`).join('');

  const ep = data.engagement_profile || {};
  document.getElementById('ai-engagement').innerHTML = `
    <div class="ai-formula">
      <div class="ai-formula-step"><div class="n">Views</div><div class="t">Pendorong</div><p>${Threads.escapeHtml(ep.what_drives_views || '—')}</p></div>
      <div class="ai-formula-step"><div class="n">Replies</div><div class="t">Pendorong</div><p>${Threads.escapeHtml(ep.what_drives_replies || '—')}</p></div>
      <div class="ai-formula-step"><div class="n">Format</div><div class="t">Bias</div><p>${Threads.escapeHtml(ep.format_bias || '—')}</p></div>
      <div class="ai-formula-step"><div class="n">Panjang</div><div class="t">Bias</div><p>${Threads.escapeHtml(ep.length_bias || '—')}</p></div>
    </div>`;

  document.getElementById('ai-hot').innerHTML = (data.hot_content || []).length
    ? data.hot_content.map(i => contentCard(i)).join('')
    : '<p class="text-sm text-muted m-0">Tidak ada data hot content.</p>';

  document.getElementById('ai-cold').innerHTML = (data.cold_content || []).length
    ? data.cold_content.map(i => contentCard(i)).join('')
    : '<p class="text-sm text-muted m-0">Tidak ada data cold content.</p>';

  const dna = data.content_dna || {};
  document.getElementById('ai-dna').innerHTML = `
    <div class="ai-topics">
      <div class="ai-topic-box go">
        <div class="t">Tema berulang</div>
        <div class="flex flex-wrap gap-2">${chips(dna.recurring_themes) || '—'}</div>
      </div>
      <div class="ai-topic-box go">
        <div class="t">Signature moves</div>
        <div class="flex flex-wrap gap-2">${chips(dna.signature_moves) || '—'}</div>
      </div>
      <div class="ai-topic-box no">
        <div class="t">Blind spots</div>
        <div class="flex flex-wrap gap-2">${chips(dna.blind_spots) || '—'}</div>
      </div>
    </div>`;

  document.getElementById('ai-patterns').innerHTML = (data.patterns || []).length
    ? `<ul class="text-sm text-muted pl-5 list-disc space-y-1.5 m-0">${data.patterns.map(p => `<li>${Threads.escapeHtml(p)}</li>`).join('')}</ul>`
    : '<p class="text-sm text-muted m-0">—</p>';
}

async function generate() {
  showAlert('');
  if (!(await Threads.requireConnected())) return;
  setLoading(true);
  document.getElementById('ai-empty').classList.add('hidden');
  document.getElementById('ai-report').classList.add('hidden');
  showAlert('Membaca akun + metrik post… Gemini menganalisis breakdown.');
  try {
    const data = await Threads.api('/api/insights/ai', { method: 'POST', body: '{}' });
    try { localStorage.setItem(CACHE_KEY, JSON.stringify({ at: Date.now(), data })); } catch {}
    showAlert('');
    renderReport(data);
    if (data.quota) renderQuota(data.quota);
    Threads.toast('Breakdown akun siap', true);
  } catch (e) {
    showAlert(e.message);
    document.getElementById('ai-empty').classList.remove('hidden');
    Threads.toast(e.message, false);
    try {
      const st = await Threads.api('/api/ai/status');
      if (st.quota) renderQuota(st.quota);
    } catch {}
  } finally {
    setLoading(false);
  }
}

document.getElementById('btn-ai-insight').onclick = () => generate();
document.getElementById('btn-ai-empty').onclick = () => generate();

(async () => {
  try {
    const st = await Threads.api('/api/ai/status');
    const badge = document.getElementById('ai-badge');
    if (st.enabled) {
      badge.textContent = `${st.provider || 'AI'} · ${st.model || ''}`;
      badge.className = 'th-chip th-chip-ok';
      if (st.quota) renderQuota(st.quota);
    } else {
      badge.textContent = 'AI off — set AI_API_KEY';
      badge.className = 'th-chip th-chip-warn';
    }
  } catch {}

  try {
    localStorage.removeItem('threads_ai_insight_v1');
    localStorage.removeItem('threads_ai_insight_v2');
    const cached = JSON.parse(localStorage.getItem(CACHE_KEY) || 'null');
    if (cached?.data) renderReport(cached.data);
  } catch {}
})();
