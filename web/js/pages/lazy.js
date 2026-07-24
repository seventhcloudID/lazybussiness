Threads.pageShell('lazy');

let pollTimer = null;

function showAlert(msg) {
  const el = document.getElementById('lazy-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function fmtTime(iso) {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  } catch {
    return iso;
  }
}

function statusBadge(st) {
  const map = {
    pending: ['Menunggu', 'pend'],
    running: ['Jalan', 'run'],
    done: ['Selesai', 'ok'],
    failed: ['Gagal', 'bad'],
    skipped_ig: ['Threads OK · IG skip', 'warn'],
  };
  const [label, cls] = map[st] || [st, ''];
  return `<span class="lazy-badge ${cls}">${Threads.escapeHtml(label)}</span>`;
}

function renderStatus(st) {
  const cfg = st.config || {};
  document.getElementById('enabled').checked = !!cfg.enabled;
  document.getElementById('enabled-label').textContent = cfg.enabled ? 'ON' : 'OFF';
  document.getElementById('posts-per-day').value = cfg.posts_per_day || 5;
  if (cfg.topic_hint != null && document.getElementById('topic').dataset.dirty !== '1') {
    document.getElementById('topic').value = cfg.topic_hint || '';
  }

  document.getElementById('stat-today').textContent = st.today || '—';
  const c = st.counts || {};
  document.getElementById('stat-done').textContent = String((c.done || 0) + (c.skipped_ig || 0));
  document.getElementById('stat-pending').textContent = String((c.pending || 0) + (c.running || 0));
  document.getElementById('stat-fail').textContent = String((c.failed || 0) + (c.skipped_ig || 0));

  const warns = st.warnings || [];
  const wEl = document.getElementById('lazy-warnings');
  if (warns.length) {
    wEl.classList.remove('hidden');
    wEl.innerHTML = warns.map(w => `<div class="lazy-warn">${Threads.escapeHtml(w)}</div>`).join('');
  } else {
    wEl.classList.add('hidden');
    wEl.innerHTML = '';
  }

  const bits = [];
  bits.push(`TZ ${st.timezone || '—'}`);
  bits.push(st.public_ok ? 'PUBLIC_BASE_URL OK' : 'PUBLIC_BASE_URL belum');
  bits.push(st.threads_ok ? 'Threads OK' : 'Threads —');
  bits.push(st.instagram_ok ? 'IG OK' : 'IG —');
  bits.push(st.ai_ok ? 'AI OK' : 'AI —');
  document.getElementById('lazy-meta').textContent = bits.join(' · ');

  if (st.next_pending) {
    document.getElementById('next-hint').textContent =
      `Berikutnya ${fmtTime(st.next_pending.scheduled_at)}`;
  } else {
    document.getElementById('next-hint').textContent = cfg.enabled ? 'Tidak ada antrian' : 'Otomasi OFF';
  }

  const jobs = st.jobs_today || [];
  const list = document.getElementById('job-list');
  if (!jobs.length) {
    list.innerHTML = '<p class="text-sm text-muted p-4 m-0">Belum ada rencana — nyalakan otomasi lalu simpan.</p>';
    return;
  }
  list.innerHTML = jobs.map(j => `
    <div class="lazy-job">
      <div class="lazy-job-main">
        <strong>${fmtTime(j.scheduled_at)}</strong>
        ${statusBadge(j.status)}
        <span class="lazy-job-title">${Threads.escapeHtml(j.title || j.id)}</span>
      </div>
      ${j.error ? `<div class="lazy-job-err">${Threads.escapeHtml(j.error)}</div>` : ''}
    </div>`).join('');
}

async function refresh() {
  try {
    const st = await Threads.api('/api/lazy/status');
    renderStatus(st);
    showAlert('');
  } catch (e) {
    showAlert(e.message);
  }
}

document.getElementById('enabled').addEventListener('change', () => {
  document.getElementById('enabled-label').textContent =
    document.getElementById('enabled').checked ? 'ON' : 'OFF';
});

document.getElementById('topic').addEventListener('input', () => {
  document.getElementById('topic').dataset.dirty = '1';
});

document.getElementById('btn-save').onclick = async () => {
  showAlert('');
  const posts = Math.max(5, Math.min(12, Number(document.getElementById('posts-per-day').value) || 5));
  document.getElementById('posts-per-day').value = posts;
  try {
    const brand = document.getElementById('brand').value.trim();
    if (brand) {
      await Threads.api('/api/ai/brand', { method: 'PUT', body: JSON.stringify({ brand }) });
    }
    await Threads.api('/api/lazy/config', {
      method: 'PUT',
      body: JSON.stringify({
        enabled: document.getElementById('enabled').checked,
        posts_per_day: posts,
        topic_hint: document.getElementById('topic').value.trim(),
      }),
    });
    delete document.getElementById('topic').dataset.dirty;
    Threads.toast('Pengaturan disimpan', true);
    await refresh();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  }
};

document.getElementById('btn-run-now').onclick = async () => {
  showAlert('');
  const btn = document.getElementById('btn-run-now');
  btn.disabled = true;
  btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Running…';
  try {
    const brand = document.getElementById('brand').value.trim();
    if (brand) {
      await Threads.api('/api/ai/brand', { method: 'PUT', body: JSON.stringify({ brand }) });
    }
    const job = await Threads.api('/api/lazy/run-now', { method: 'POST', body: '{}' });
    const box = document.getElementById('lazy-last');
    box.classList.remove('hidden');
    document.getElementById('lazy-title').textContent = job.title || job.id;
    const parts = job.parts || [];
    document.getElementById('lazy-parts').innerHTML = parts.map((p, i) => `
      <div class="gen-thread-part">
        <div class="gen-thread-n">${i + 1}</div>
        <div class="gen-thread-text"><p>${Threads.escapeHtml(p)}</p></div>
      </div>`).join('') || '<p class="text-muted text-sm">Tidak ada parts</p>';
    document.getElementById('lazy-caption').textContent =
      (job.caption || '') + (job.error ? `\n\n⚠️ ${job.error}` : '');
    Threads.toast(job.status === 'failed' ? 'Gagal — cek error' : `Selesai: ${job.status}`, job.status !== 'failed');
    await refresh();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="bi bi-play-fill"></i> Run 1 sekarang';
  }
};

document.getElementById('btn-refresh').onclick = () => refresh();

(async () => {
  try {
    const mem = await Threads.api('/api/ai/memory');
    if (mem?.brand) document.getElementById('brand').value = mem.brand;
  } catch {}
  await refresh();
  pollTimer = setInterval(refresh, 30000);
})();
