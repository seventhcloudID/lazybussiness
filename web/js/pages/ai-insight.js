Threads.pageShell('ai-insight');

const CACHE_KEY = 'threads_ai_insight_v8';
let lastData = null;
let replizAccountId = '';
let replizAccountLabel = '';

function replizLabel(a) {
  if (!a) return '';
  const u = String(a.username || a.name || '').replace(/^@/, '');
  const type = String(a.type || '').toLowerCase();
  if (!u) return type || '';
  return type ? `@${u} · ${type}` : `@${u}`;
}

function applyReplizAccount(accs) {
  const list = accs?.accounts || [];
  const id = accs?.active_id || '';
  const a = list.find((x) => (x.id || x._id) === id) || list[0];
  replizAccountId = (a && (a.id || a._id)) || id || '';
  replizAccountLabel = replizLabel(a);
  const lead = document.getElementById('ai-lead');
  if (lead) {
    lead.textContent = replizAccountLabel
      ? `Analisis ${replizAccountLabel} lewat API Repliz.`
      : 'Pilih akun Repliz di sidebar dulu.';
  }
  const copy = document.getElementById('ai-empty-copy');
  if (copy) {
    copy.textContent = replizAccountLabel
      ? `AI baca sampai 50 post ${replizAccountLabel} dari Repliz, lalu susun playbook, eksperimen, rewrite, dan rencana 7 hari.`
      : 'Belum ada akun Repliz aktif. Buka Akun, pilih atau hubungkan akun, lalu kembali ke sini.';
  }
  return !!replizAccountId;
}

const TOC = [
  ['sec-ringkas', 'Verdict'],
  ['sec-score', 'Score'],
  ['sec-account', 'Account'],
  ['sec-engage', 'Engage'],
  ['sec-hot', 'Hot'],
  ['sec-cold', 'Cold'],
  ['sec-rewrite', 'Rewrite'],
  ['sec-hooks', 'Hooks'],
  ['sec-pillars', 'Pilar'],
  ['sec-dna', 'DNA'],
  ['sec-patterns', 'Pola'],
  ['sec-playbook', 'Playbook'],
  ['sec-exp', 'Eksperimen'],
  ['sec-week', 'Week'],
  ['sec-next', 'Ide'],
];

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
      : '<i class="bi bi-stars"></i> ' + (btn.id === 'btn-ai-empty' ? 'Analisis sekarang' : 'Analisis deep');
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

function emptyList(msg) {
  return `<p class="ins-meta m-0">${Threads.escapeHtml(msg)}</p>`;
}

async function copyText(text) {
  const t = String(text || '').trim();
  if (!t) return;
  try {
    await navigator.clipboard.writeText(t);
    Threads.toast('Disalin', true);
  } catch {
    Threads.toast('Gagal salin', false);
  }
}

function sendToBuat(hook, format) {
  const text = String(hook || '').trim();
  if (!text) return;
  try {
    localStorage.setItem('threads_compose_parts', JSON.stringify([text]));
    localStorage.setItem('threads_compose_from_ai', '1');
    if (String(format || '').toUpperCase() === 'IMAGE') {
      localStorage.setItem('threads_compose_media_type', 'IMAGE');
    } else {
      localStorage.setItem('threads_compose_media_type', 'TEXT');
    }
  } catch {}
  location.href = Threads.appUrl('/buat.html');
}

function actionBtns(hook, format) {
  const h = Threads.escapeHtml(hook || '');
  const f = Threads.escapeHtml(format || 'TEXT');
  if (!hook) return '';
  return `<div class="ai-actions">
    <button type="button" class="th-btn th-btn-ghost !py-1 !px-2 text-xs" data-copy="${h}"><i class="bi bi-clipboard"></i> Salin</button>
    <button type="button" class="th-btn th-btn-soft !py-1 !px-2 text-xs" data-buat="${h}" data-format="${f}"><i class="bi bi-pencil-square"></i> Buat</button>
  </div>`;
}

function postRow(item, i) {
  const text = String(item.text || item.excerpt || item.label || '').trim();
  const why = String(item.why || item.pattern || '').trim();
  const metrics = [
    item.views != null ? `👁 ${Threads.fmtNum(item.views)}` : '',
    item.likes != null ? `❤ ${Threads.fmtNum(item.likes)}` : '',
    item.replies != null ? `💬 ${Threads.fmtNum(item.replies)}` : '',
  ].filter(Boolean);
  const proof = metrics.length ? metrics.join(' · ') : String(item.proof || '').trim();
  const open = item.permalink
    ? `<a class="th-btn th-btn-ghost !py-1 !px-2 text-xs" href="${Threads.escapeHtml(item.permalink)}" target="_blank" rel="noopener">Buka</a>`
    : '';
  const meta = [item.media_type, item.timestamp ? Threads.fmtDate?.(item.timestamp) : '']
    .filter(Boolean)
    .join(' · ');
  return `<article class="ins-ai-row ins-ai-post">
    <span class="ins-ai-n mono">${i + 1}</span>
    <div class="ins-ai-row-body">
      ${proof ? `<p class="ins-ai-metrics mono">${Threads.escapeHtml(proof)}</p>` : ''}
      ${text ? `<p class="ins-ai-title">${Threads.escapeHtml(text)}</p>` : ''}
      ${why ? `<p class="ins-ai-note">${Threads.escapeHtml(why)}</p>` : ''}
      ${meta ? `<p class="ins-ai-note" style="opacity:.75">${Threads.escapeHtml(meta)}</p>` : ''}
    </div>
    <div class="ins-ai-post-actions">${open}</div>
  </article>`;
}

function ideaRow(item, i) {
  const hook = String(item.hook || '').trim();
  const angle = String(item.angle || '').trim();
  const why = String(item.why || '').trim();
  const proof = [item.priority, item.format, item.pillar, item.cta].filter(Boolean).join(' · ');
  return `<article class="ins-ai-row ins-ai-post">
    <span class="ins-ai-n mono">${i + 1}</span>
    <div class="ins-ai-row-body">
      ${proof ? `<p class="ins-ai-metrics mono">${Threads.escapeHtml(proof)}</p>` : ''}
      ${hook ? `<p class="ins-ai-title">${Threads.escapeHtml(hook)}</p>` : ''}
      ${angle ? `<p class="ins-ai-note"><b>Angle:</b> ${Threads.escapeHtml(angle)}</p>` : ''}
      ${why ? `<p class="ins-ai-note">${Threads.escapeHtml(why)}</p>` : ''}
      ${actionBtns(hook, item.format)}
    </div>
  </article>`;
}

function rewriteRow(item, i) {
  const hook = String(item.new_hook || '').trim();
  return `<article class="ins-ai-row ins-ai-post">
    <span class="ins-ai-n mono">${i + 1}</span>
    <div class="ins-ai-row-body">
      ${item.problem ? `<p class="ins-ai-metrics">${Threads.escapeHtml(item.problem)}</p>` : ''}
      ${hook ? `<p class="ins-ai-title">${Threads.escapeHtml(hook)}</p>` : ''}
      ${item.why_better ? `<p class="ins-ai-note">${Threads.escapeHtml(item.why_better)}</p>` : ''}
      ${item.post_id ? `<p class="ins-ai-note" style="opacity:.7">post ${Threads.escapeHtml(item.post_id)}</p>` : ''}
      ${actionBtns(hook, 'TEXT')}
    </div>
  </article>`;
}

function experimentRow(item, i) {
  return `<article class="ins-ai-row">
    <span class="ins-ai-n mono">${i + 1}</span>
    <div class="ins-ai-row-body">
      <p class="ins-ai-metrics mono">${Threads.escapeHtml([item.effort, item.success_metric].filter(Boolean).join(' · '))}</p>
      <p class="ins-ai-title">${Threads.escapeHtml(item.name || 'Eksperimen')}</p>
      ${item.hypothesis ? `<p class="ins-ai-note"><b>Hipotesis:</b> ${Threads.escapeHtml(item.hypothesis)}</p>` : ''}
      ${item.how_to_test ? `<p class="ins-ai-note">${Threads.escapeHtml(item.how_to_test)}</p>` : ''}
    </div>
  </article>`;
}

function renderToc() {
  const el = document.getElementById('ai-toc');
  if (!el) return;
  el.innerHTML = TOC.map(([id, label]) =>
    `<a href="#${id}">${Threads.escapeHtml(label)}</a>`
  ).join('');
}

function buildBrief(data) {
  const lines = [];
  lines.push(`# AI Insight — ${data.headline || ''}`);
  lines.push('');
  lines.push(data.summary || '');
  if (data.north_star) {
    lines.push('');
    lines.push(`North star: ${data.north_star}`);
  }
  lines.push('');
  lines.push('## Next posts');
  (data.next_posts || []).forEach((p, i) => {
    lines.push(`${i + 1}. [${p.priority || 'P1'}][${p.format || 'TEXT'}] ${p.hook || ''}`);
    if (p.why) lines.push(`   ${p.why}`);
  });
  lines.push('');
  lines.push('## This week');
  (data.playbook?.this_week || []).forEach((s, i) => lines.push(`${i + 1}. ${s}`));
  lines.push('');
  lines.push('## Week plan');
  (data.week_plan || []).forEach((s) => {
    lines.push(`- ${s.day || ''} ${s.daypart || ''}: ${s.hook || s.angle || ''}`);
  });
  return lines.join('\n');
}

function renderReport(raw) {
  const data = normalizePayload(raw);
  lastData = data;
  document.getElementById('ai-empty').classList.add('hidden');
  document.getElementById('ai-report').classList.remove('hidden');
  const exportBtn = document.getElementById('btn-export');
  if (exportBtn) exportBtn.hidden = false;
  renderToc();

  const summaryLooksJson = String(data.summary || '').trim().startsWith('{');
  if (summaryLooksJson && !data.hot_content?.length) {
    document.getElementById('ai-headline').textContent = 'Breakdown gagal dirender';
    document.getElementById('ai-summary').textContent = 'Hasil AI masih format mentah. Klik Analisis deep lagi.';
    document.getElementById('ai-meta').textContent = [data.provider, data.model].filter(Boolean).join(' · ') || '—';
    return;
  }

  document.getElementById('ai-headline').textContent = data.headline || 'Breakdown akun';
  document.getElementById('ai-summary').textContent = data.summary || '—';
  const north = document.getElementById('ai-north');
  if (north) {
    if (data.north_star) {
      north.hidden = false;
      north.innerHTML = `<b>North star:</b> ${Threads.escapeHtml(data.north_star)}`;
    } else {
      north.hidden = true;
      north.textContent = '';
    }
  }

  const sc = data.scorecard || {};
  document.getElementById('hero-strength').textContent = sc.strength || '—';
  document.getElementById('hero-weakness').textContent = sc.weakness || '—';
  document.getElementById('hero-signal').textContent = sc.opportunity || '—';
  const scoresEl = document.getElementById('ai-scores');
  const scoreBits = [
    ['Jangkauan', sc.reach],
    ['Percakapan', sc.conversation],
    ['Konsistensi', sc.consistency],
    ['Hook', sc.hook_power],
    ['Orisinal', sc.originality],
    ['Reply magnet', sc.reply_magnet],
  ].filter(([, v]) => v != null && v !== '');
  if (scoresEl) {
    if (scoreBits.length) {
      scoresEl.hidden = false;
      scoresEl.innerHTML = scoreBits.map(([k, v]) =>
        `<div class="ins-ai-score"><b>${Threads.escapeHtml(String(v))}/10</b><span>${Threads.escapeHtml(k)}</span></div>`
      ).join('');
    } else {
      scoresEl.hidden = true;
      scoresEl.innerHTML = '';
    }
  }

  const meta = ['Repliz', replizAccountLabel || data.repliz_account?.username, data.provider, data.model].filter(Boolean);
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
    dlRow('Pembedaan', ar.differentiator),
  ].filter(Boolean).join('') || emptyList('Belum ada account read.');

  const ep = data.engagement_profile || {};
  document.getElementById('ai-engagement').innerHTML = [
    dlRow('Views', ep.what_drives_views),
    dlRow('Replies', ep.what_drives_replies),
    dlRow('Format', ep.format_bias),
    dlRow('Panjang', ep.length_bias),
    dlRow('Waktu kuat', ep.best_time),
    dlRow('Waktu lemah', ep.worst_time),
  ].filter(Boolean).join('') || emptyList('Belum ada profil engagement.');

  const hot = data.hot_content || [];
  document.getElementById('ai-hot').innerHTML = hot.length
    ? hot.map((item, i) => postRow(item, i)).join('')
    : emptyList('Tidak ada data hot content.');

  const cold = data.cold_content || [];
  document.getElementById('ai-cold').innerHTML = cold.length
    ? cold.map((item, i) => postRow(item, i)).join('')
    : emptyList('Tidak ada data cold content.');

  const rewrites = data.cold_rewrites || [];
  document.getElementById('ai-rewrites').innerHTML = rewrites.length
    ? rewrites.map((item, i) => rewriteRow(item, i)).join('')
    : emptyList('Belum ada rewrite.');

  const hl = data.hook_lab || {};
  document.getElementById('ai-hooks').innerHTML = [
    dlChips('Winning openers', hl.winning_openers),
    dlChips('Losing openers', hl.losing_openers),
    dlChips('Do more', hl.do_more),
    dlChips('Stop doing', hl.stop_doing),
  ].join('');

  const pillars = data.pillars || [];
  const pillarsEl = document.getElementById('ai-pillars');
  if (pillarsEl) {
    pillarsEl.innerHTML = pillars.length
      ? pillars.map((p) => `
        <article class="ai-pillar">
          <div class="ai-pillar-top">
            <strong>${Threads.escapeHtml(p.name || 'Pilar')}</strong>
            <span class="mono">${Threads.escapeHtml(String(p.weight_pct ?? '—'))}%</span>
          </div>
          <div class="ai-pillar-bar"><span style="width:${Math.max(4, Math.min(100, Number(p.weight_pct) || 0))}%"></span></div>
          ${p.why ? `<p>${Threads.escapeHtml(p.why)}</p>` : ''}
          ${p.example_angles?.length ? `<div class="ins-ai-tags">${chips(p.example_angles)}</div>` : ''}
        </article>`).join('')
      : emptyList('Belum ada pilar.');
  }

  const dna = data.content_dna || {};
  document.getElementById('ai-dna').innerHTML = [
    dlChips('Tema berulang', dna.recurring_themes),
    dlChips('Signature moves', dna.signature_moves),
    dlChips('Blind spots', dna.blind_spots),
    dlChips('Rumus hook', dna.hook_formulas),
  ].join('');

  const patterns = data.patterns || [];
  document.getElementById('ai-patterns').innerHTML = patterns.length
    ? patterns.map((p) => `<li>${Threads.escapeHtml(p)}</li>`).join('')
    : emptyList('Belum ada pola yang terbaca.');

  const risks = data.risks || [];
  const risksEl = document.getElementById('ai-risks');
  if (risksEl) {
    risksEl.innerHTML = risks.length
      ? risks.map((r) => `<li>${Threads.escapeHtml(r)}</li>`).join('')
      : '';
    risksEl.hidden = !risks.length;
  }

  const timing = document.getElementById('ai-timing');
  if (timing) timing.textContent = data.timing_read || '—';

  const pb = data.playbook || {};
  const pbEl = document.getElementById('ai-playbook');
  if (pbEl) {
    pbEl.innerHTML = [
      dlChips('Stop', pb.stop),
      dlChips('Start', pb.start),
      dlChips('Continue', pb.continue),
    ].join('');
  }
  const week = pb.this_week || [];
  const weekEl = document.getElementById('ai-this-week');
  if (weekEl) {
    weekEl.innerHTML = week.length
      ? week.map((p) => `<li>${Threads.escapeHtml(p)}</li>`).join('')
      : '';
    weekEl.classList.toggle('hidden', !week.length);
  }

  const rp = data.reply_play || {};
  document.getElementById('ai-reply').innerHTML = [
    dlRow('Kapan bertanya', rp.when_to_ask),
    dlChips('Gaya pertanyaan', rp.question_styles),
    dlChips('Hindari', rp.avoid),
  ].filter(Boolean).join('') || emptyList('Belum ada reply play.');

  const experiments = data.experiments || [];
  document.getElementById('ai-experiments').innerHTML = experiments.length
    ? experiments.map((item, i) => experimentRow(item, i)).join('')
    : emptyList('Belum ada eksperimen.');

  const plan = data.week_plan || [];
  const weekPlanEl = document.getElementById('ai-week');
  if (weekPlanEl) {
    weekPlanEl.innerHTML = plan.length
      ? plan.map((s) => `
        <article class="ai-week-card">
          <header>
            <strong>${Threads.escapeHtml(s.day || 'Hari')}</strong>
            <span>${Threads.escapeHtml([s.daypart, s.format].filter(Boolean).join(' · '))}</span>
          </header>
          <p class="ai-week-hook">${Threads.escapeHtml(s.hook || s.angle || '—')}</p>
          ${s.angle && s.hook ? `<p class="ins-ai-note">${Threads.escapeHtml(s.angle)}</p>` : ''}
          ${actionBtns(s.hook || s.angle, s.format)}
        </article>`).join('')
      : emptyList('Belum ada week plan.');
  }

  const next = data.next_posts || [];
  const nextEl = document.getElementById('ai-next');
  if (nextEl) {
    nextEl.innerHTML = next.length
      ? next.map((item, i) => ideaRow(item, i)).join('')
      : emptyList('Belum ada ide post.');
  }
}

async function generate() {
  showAlert('');
  try {
    const accs = await Threads.api('/api/repliz/accounts');
    if (!applyReplizAccount(accs)) {
      Threads.toast('Pilih akun Repliz di sidebar dulu', false);
      return;
    }
  } catch (e) {
    Threads.toast(e.message || 'Repliz belum tersambung', false);
    return;
  }
  setLoading(true);
  document.getElementById('ai-empty').classList.add('hidden');
  document.getElementById('ai-report').classList.add('hidden');
  showAlert(`Menganalisis ${replizAccountLabel || 'akun Repliz'} — sampai 50 post, biasanya 1–2 menit.`);
  try {
    const data = await Threads.api('/api/insights/ai', {
      method: 'POST',
      body: JSON.stringify({ account_id: replizAccountId }),
    });
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify({ at: Date.now(), account_id: replizAccountId, data }));
    } catch {}
    showAlert('');
    renderReport(data);
    if (data.quota) renderQuota(data.quota);
    Threads.toast('Deep insight siap', true);
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
document.getElementById('btn-export')?.addEventListener('click', () => {
  if (!lastData) return;
  copyText(buildBrief(lastData));
});

document.getElementById('ai-report')?.addEventListener('click', (e) => {
  const copyBtn = e.target.closest('[data-copy]');
  if (copyBtn) {
    copyText(copyBtn.getAttribute('data-copy'));
    return;
  }
  const buatBtn = e.target.closest('[data-buat]');
  if (buatBtn) {
    sendToBuat(buatBtn.getAttribute('data-buat'), buatBtn.getAttribute('data-format'));
  }
});

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
    localStorage.removeItem('threads_ai_insight_v3');
    localStorage.removeItem('threads_ai_insight_v4');
    localStorage.removeItem('threads_ai_insight_v5');
    localStorage.removeItem('threads_ai_insight_v6');
    const cached = JSON.parse(localStorage.getItem(CACHE_KEY) || 'null');
    const accs = await Threads.api('/api/repliz/accounts').catch(() => null);
    applyReplizAccount(accs);
    if (cached?.data && cached.account_id && cached.account_id === replizAccountId) renderReport(cached.data);
  } catch {}
})();
