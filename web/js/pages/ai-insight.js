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
  if (tier) tier.textContent = `Kuota: ${q.tier || 'free'} · ${q.model || ''}`.trim();
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
  btns.forEach((btn) => {
    if (!btn) return;
    btn.disabled = on;
    btn.innerHTML = on
      ? '<i class="bi bi-hourglass-split"></i> Menganalisis…'
      : '<i class="bi bi-stars"></i> ' + (btn.id === 'btn-ai-empty' ? 'Analisis sekarang' : 'Analisis akun');
  });
}

function chips(list) {
  return (list || [])
    .map((t) => `<span class="ins-ai-tag">${Threads.escapeHtml(t)}</span>`)
    .join('');
}

function dlRow(title, body) {
  if (!body) return '';
  return `<div><dt>${Threads.escapeHtml(title)}</dt><dd>${Threads.escapeHtml(body)}</dd></div>`;
}

function dlChips(title, list) {
  const html = chips(list);
  if (!html) return `<div><dt>${Threads.escapeHtml(title)}</dt><dd class="ins-meta">—</dd></div>`;
  return `<div><dt>${Threads.escapeHtml(title)}</dt><dd class="ins-ai-tags">${html}</dd></div>`;
}

/** Compact content row: metrics line + hook/why — no card chrome. */
function contentRow(item, i) {
  const metrics = String(item.proof || item.label || '').trim();
  const note = String(item.why || item.excerpt || item.pattern || '').trim();
  const title = String(item.label || '').trim();
  const showTitle = title && title !== metrics && title.length < 80;
  return `<div class="ins-ai-row">
    <span class="ins-ai-n mono">${i + 1}</span>
    <div class="ins-ai-row-body">
      ${metrics ? `<p class="ins-ai-metrics mono">${Threads.escapeHtml(metrics)}</p>` : ''}
      ${showTitle ? `<p class="ins-ai-title">${Threads.escapeHtml(title)}</p>` : ''}
      ${note ? `<p class="ins-ai-note">${Threads.escapeHtml(note)}</p>` : ''}
    </div>
  </div>`;
}

function emptyList(msg) {
  return `<p class="ins-meta m-0">${Threads.escapeHtml(msg)}</p>`;
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
    document.getElementById('ai-meta').textContent = [data.provider, data.model].filter(Boolean).join(' · ') || '—';
    return;
  }

  document.getElementById('ai-headline').textContent = data.headline || 'Breakdown akun';
  document.getElementById('ai-summary').textContent = data.summary || '—';

  const sc = data.scorecard || {};
  document.getElementById('hero-strength').textContent = sc.strength || '—';
  document.getElementById('hero-weakness').textContent = sc.weakness || '—';
  document.getElementById('hero-signal').textContent = sc.opportunity || '—';

  const meta = [data.provider, data.model].filter(Boolean);
  if (data.usage?.total_tokens) {
    const u = data.usage;
    meta.push(
      `token ${Number(u.total_tokens).toLocaleString('id-ID')} ` +
        `(prompt ${Number(u.prompt_tokens || 0).toLocaleString('id-ID')} + ` +
        `output ${Number(u.completion_tokens || 0).toLocaleString('id-ID')})`,
    );
  }
  document.getElementById('ai-meta').textContent = meta.length ? meta.join(' · ') : 'Sumber: AI';
  if (data.quota) renderQuota(data.quota);

  const ar = data.account_read || {};
  document.getElementById('ai-account-read').innerHTML = [
    dlRow('Niche', ar.niche),
    dlRow('Voice', ar.voice),
    dlRow('Audience', ar.audience),
    dlRow('Positioning', ar.positioning),
  ].filter(Boolean).join('') || emptyList('Belum ada account read.');

  const ep = data.engagement_profile || {};
  document.getElementById('ai-engagement').innerHTML = [
    dlRow('Views', ep.what_drives_views),
    dlRow('Replies', ep.what_drives_replies),
    dlRow('Format', ep.format_bias),
    dlRow('Panjang', ep.length_bias),
  ].filter(Boolean).join('') || emptyList('Belum ada profil engagement.');

  const hot = data.hot_content || [];
  document.getElementById('ai-hot').innerHTML = hot.length
    ? hot.map((item, i) => contentRow(item, i)).join('')
    : emptyList('Tidak ada data hot content.');

  const cold = data.cold_content || [];
  document.getElementById('ai-cold').innerHTML = cold.length
    ? cold.map((item, i) => contentRow(item, i)).join('')
    : emptyList('Tidak ada data cold content.');

  const dna = data.content_dna || {};
  document.getElementById('ai-dna').innerHTML = [
    dlChips('Tema berulang', dna.recurring_themes),
    dlChips('Signature moves', dna.signature_moves),
    dlChips('Blind spots', dna.blind_spots),
  ].join('');

  const patterns = data.patterns || [];
  document.getElementById('ai-patterns').innerHTML = patterns.length
    ? patterns.map((p) => `<li>${Threads.escapeHtml(p)}</li>`).join('')
    : emptyList('Belum ada pola yang terbaca.');
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
