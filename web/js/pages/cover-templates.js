Threads.pageShell('cover-templates');

const COVER_TEMPLATE_KEY = 'threads_cover_template_v1';
const TEMPLATES = [
  { id: 'edge-clean', name: 'Edge Clean', desc: 'Panel hitam-putih menyatu dengan sisi bawah.', rect: [0, 58.5, 100, 41.5] },
  { id: 'split-roomy', name: 'Split Roomy', desc: 'Area teks lebih lega untuk hook panjang.', rect: [0, 54, 100, 46] },
  { id: 'inset-editorial', name: 'Inset Editorial', desc: 'Panel masuk dari semua sisi dengan napas foto di bawah.', rect: [5.5, 56.5, 89, 38.3] },
  { id: 'left-cut', name: 'Left Cut', desc: 'Panel rata kiri dan menyisakan foto di sisi kanan.', rect: [0, 54.5, 84, 41] },
  { id: 'right-cut', name: 'Right Cut', desc: 'Panel rata kanan untuk focal point di sisi kiri.', rect: [16, 54.5, 84, 41] },
  { id: 'low-editorial', name: 'Low Editorial', desc: 'Panel lebih rendah agar porsi foto lebih besar.', rect: [4.5, 61, 91, 36.5] },
];

const $ = (id) => document.getElementById(id);
let saved = loadSaved();
let selected = saved;

function loadSaved() {
  try {
    const id = localStorage.getItem(COVER_TEMPLATE_KEY) || 'edge-clean';
    return TEMPLATES.some((t) => t.id === id) ? id : 'edge-clean';
  } catch {
    return 'edge-clean';
  }
}

function esc(value) {
  return Threads.escapeHtml(String(value || ''));
}

function headlineMarkup(value) {
  const words = String(value || '').trim().split(/\s+/).filter(Boolean);
  if (!words.length) return '';
  let boldFrom = words.length;
  for (let i = words.length - 2; i >= 0; i -= 1) {
    if (/[.!?:]$/.test(words[i])) {
      boldFrom = i + 1;
      break;
    }
  }
  if (boldFrom === words.length) {
    if (words.length >= 6) boldFrom = words.length - Math.floor((words.length + 2) / 3);
    else if (words.length >= 3) boldFrom = words.length - 1;
    else boldFrom = 0;
  }
  return words.map((word, index) => {
    const clean = word.replace(/[.,!?;:"'()[\]]/g, '');
    const keyword = clean.length >= 3 && clean === clean.toUpperCase() && clean !== clean.toLowerCase();
    return index >= boldFrom || keyword ? `<strong>${esc(word)}</strong>` : esc(word);
  }).join(' ');
}

function previewMarkup(t) {
  const [x, y, w, h] = t.rect;
  const title = headlineMarkup($('cover-preview-title')?.value || 'Ke Jakarta cuma sehari?');
  const handle = esc(($('cover-preview-handle')?.value || '').replace(/^@/, ''));
  const cta = esc($('cover-preview-cta')?.value || '');
  return `
    <span class="cover-preview" aria-hidden="true">
      <span class="cover-preview-photo">
        <i class="cover-preview-sun"></i>
        <i class="cover-preview-city"></i>
        <i class="cover-preview-person"></i>
      </span>
      <span class="cover-preview-panel" style="--px:${x}%;--py:${y}%;--pw:${w}%;--ph:${h}%">
        ${handle ? `<small>${handle}</small>` : ''}
        <span class="cover-preview-title">${title}</span>
        ${cta ? `<em>${cta}</em>` : ''}
      </span>
    </span>`;
}

function render() {
  const root = $('cover-template-gallery');
  root.innerHTML = TEMPLATES.map((t) => `
    <button type="button" class="cover-template-card${t.id === selected ? ' is-selected' : ''}${t.id === saved ? ' is-active' : ''}" data-template="${t.id}">
      ${previewMarkup(t)}
      <span class="cover-template-meta">
        <span><strong>${esc(t.name)}</strong>${t.id === saved ? '<i>Aktif</i>' : ''}</span>
        <small>${esc(t.desc)}</small>
      </span>
      <span class="cover-template-check"><i class="bi bi-check2"></i></span>
    </button>`).join('');
  const picked = TEMPLATES.find((t) => t.id === selected) || TEMPLATES[0];
  const active = TEMPLATES.find((t) => t.id === saved) || TEMPLATES[0];
  $('cover-selected-name').textContent = picked.name;
  $('cover-active-name').textContent = active.name;
  $('cover-active-desc').textContent = active.desc;
  const apply = $('btn-apply-cover-template');
  apply.disabled = selected === saved;
  apply.innerHTML = selected === saved
    ? '<i class="bi bi-check2"></i> Sudah digunakan'
    : '<i class="bi bi-check2"></i> Gunakan template ini';
}

$('cover-template-gallery').addEventListener('click', (event) => {
  const card = event.target.closest('[data-template]');
  if (!card) return;
  selected = card.dataset.template;
  render();
});

['cover-preview-title', 'cover-preview-handle', 'cover-preview-cta'].forEach((id) => {
  $(id)?.addEventListener('input', render);
});

$('btn-apply-cover-template').addEventListener('click', () => {
  try {
    localStorage.setItem(COVER_TEMPLATE_KEY, selected);
    saved = selected;
    render();
    Threads.toast(`Template ${TEMPLATES.find((t) => t.id === saved)?.name || saved} dipakai di Generate`, true);
  } catch {
    Threads.toast('Browser gagal menyimpan pilihan template', false);
  }
});

render();
