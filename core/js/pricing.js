Core.mountShell('pricing');

function showAlert(msg, ok) {
  const el = document.getElementById('pricing-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.className = 'th-alert mb-3 ' + (ok ? 'th-alert-ok' : '');
  el.classList.remove('hidden');
}

function fmtIDR(n) {
  const x = Number(n) || 0;
  return x.toLocaleString('id-ID');
}

function syncPreview() {
  const name = document.getElementById('p-name')?.value?.trim() || 'Bulanan';
  const amount = document.getElementById('p-amount')?.value;
  document.querySelector('.core-price-label').textContent = name;
  document.getElementById('price-amount-view').textContent = fmtIDR(amount);
}

function fillForm(p) {
  document.getElementById('p-name').value = p.name || 'Bulanan';
  document.getElementById('p-amount').value = p.amount ?? 145000;
  document.getElementById('p-features').value = (p.features || []).join('\n');
  document.getElementById('p-notes').value = p.notes || '';
  document.getElementById('pricing-updated').textContent = p.updated_at
    ? `Update ${String(p.updated_at).slice(0, 16).replace('T', ' ')}`
    : 'Default';
  syncPreview();
}

async function load() {
  const data = await Threads.api('/api/admin/pricing');
  fillForm(data.pricing || {});
}

async function save() {
  const features = (document.getElementById('p-features').value || '')
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean);
  const body = {
    name: document.getElementById('p-name').value.trim(),
    amount: Number(document.getElementById('p-amount').value) || 145000,
    currency: 'IDR',
    interval: 'month',
    features,
    notes: document.getElementById('p-notes').value.trim(),
  };
  try {
    const res = await Threads.api('/api/admin/pricing', {
      method: 'PUT',
      body: JSON.stringify(body),
    });
    fillForm(res.pricing || body);
    showAlert('Pricing disimpan', true);
  } catch (err) {
    showAlert(err.message || 'Gagal simpan', false);
  }
}

document.getElementById('p-name')?.addEventListener('input', syncPreview);
document.getElementById('p-amount')?.addEventListener('input', syncPreview);
document.getElementById('btn-save')?.addEventListener('click', () => save());

(async () => {
  try {
    const me = await Threads.api('/api/auth/me');
    if (me.enabled && me.role && me.role !== 'admin') {
      location.href = '/app/ringkasan.html';
      return;
    }
    await load();
  } catch (err) {
    showAlert(err.message || 'Gagal muat pricing', false);
  }
})();
