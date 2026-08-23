Threads.pageShell('insights');

let lastPayload = null;
let lastPrev = null;
let lastKpis = [];
let kpiSortMode = 'drop';
let activeDays = 30;
let replizAccountId = '';

const MIX_PARTS = [
  { key: 'likes', label: 'Suka', tone: 'bg-iris' },
  { key: 'replies', label: 'Balasan', tone: 'bg-mint' },
  { key: 'reposts', label: 'Repost', tone: 'bg-iris2' },
  { key: 'quotes', label: 'Kutipan', tone: 'bg-edge2' },
];

const DAY_ORDER = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const DAY_ID = { Monday: 'Sen', Tuesday: 'Sel', Wednesday: 'Rab', Thursday: 'Kam', Friday: 'Jum', Saturday: 'Sab', Sunday: 'Min' };
const PART_ORDER = ['dini_hari', 'pagi', 'siang', 'sore', 'malam'];
const PART_ID = {
  dini_hari: 'Dini',
  pagi: 'Pagi',
  siang: 'Siang',
  sore: 'Sore',
  malam: 'Malam',
};

function showAlert(msg) {
  const el = document.getElementById('insights-alert');
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
  }
  el.textContent = msg;
  el.classList.remove('hidden');
}

function numField(obj, ...keys) {
  if (!obj || typeof obj !== 'object') return 0;
  for (const k of keys) {
    const v = Number(obj[k]);
    if (Number.isFinite(v) && v !== 0) return v;
  }
  for (const k of keys) {
    const v = Number(obj[k]);
    if (Number.isFinite(v)) return v;
  }
  return 0;
}

function metricMap(data) {
  const map = {};
  if (data?.metrics && typeof data.metrics === 'object') Object.assign(map, data.metrics);
  if (data?.totals && typeof data.totals === 'object') {
    Object.entries(data.totals).forEach(([k, v]) => {
      if (map[k] == null || Number(map[k]) === 0) map[k] = v;
    });
  }
  (data?.data || []).forEach((m) => {
    if (!m?.name) return;
    let val = m.total_value?.value ?? m.value;
    if (val == null && Array.isArray(m.values)) {
      val = m.values.length === 1
        ? m.values[0]?.value
        : m.values.reduce((s, v) => s + (Number(v?.value) || 0), 0);
    }
    if (map[m.name] == null || Number(map[m.name]) === 0) map[m.name] = Number(val) || 0;
  });
  map.views = numField(map, 'views', 'videoViews', 'watched', 'impression', 'impressions', 'reach');
  map.likes = numField(map, 'likes', 'like', 'totalLikes');
  map.replies = numField(map, 'replies', 'comments', 'comment');
  map.reposts = numField(map, 'reposts', 'repost', 'shares', 'share');
  map.quotes = numField(map, 'quotes', 'quote');
  map.followers_count = numField(map, 'followers_count', 'followersCount');
  if (data?.followers_count != null && !map.followers_count) {
    map.followers_count = Number(data.followers_count) || 0;
  }
  return map;
}

function mixFromPosts(posts) {
  return (posts || []).reduce(
    (acc, p) => {
      const m = postMetrics(p);
      acc.views += m.views;
      acc.likes += m.likes;
      acc.replies += m.replies;
      acc.reposts += m.reposts;
      acc.quotes += m.quotes;
      return acc;
    },
    { views: 0, likes: 0, replies: 0, reposts: 0, quotes: 0 }
  );
}

function postMetrics(p) {
  const m = p?.metrics || p || {};
  return {
    views: numField(m, 'views', 'videoViews', 'watched', 'impression', 'impressions', 'reach'),
    likes: numField(m, 'likes', 'like', 'totalLikes', 'favourite'),
    replies: numField(m, 'replies', 'reply', 'comments', 'comment'),
    reposts: numField(m, 'reposts', 'repost', 'shares', 'share', 'retweet'),
    quotes: numField(m, 'quotes', 'quote'),
  };
}

function postEng(m) {
  return m.likes + m.replies + m.reposts + m.quotes;
}

function postER(p) {
  const m = postMetrics(p);
  if (m.views <= 0) return 0;
  return (postEng(m) / m.views) * 100;
}

function parsePostTime(p) {
  const raw = p?.timestamp ?? p?.createdAt ?? p?.created_at ?? p?.publishedAt;
  if (raw == null || raw === '') return null;
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    const ms = raw > 1e12 ? raw : raw * 1000;
    const d = new Date(ms);
    return Number.isNaN(d.getTime()) ? null : d;
  }
  const s = String(raw).trim();
  if (/^\d+(\.\d+)?$/.test(s)) {
    const n = Number(s);
    if (!Number.isFinite(n) || n <= 0) return null;
    const ms = n > 1e12 ? n : n * 1000;
    const d = new Date(ms);
    return Number.isNaN(d.getTime()) ? null : d;
  }
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? null : d;
}

function localDayKey(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function daypartOf(d) {
  const h = d.getHours();
  if (h < 6) return 'dini_hari';
  if (h < 12) return 'pagi';
  if (h < 17) return 'siang';
  if (h < 21) return 'sore';
  return 'malam';
}

function weekdayKey(d) {
  return DAY_ORDER[d.getDay() === 0 ? 6 : d.getDay() - 1];
}

function median(nums) {
  const a = [...nums].sort((x, y) => x - y);
  if (!a.length) return 0;
  const mid = Math.floor(a.length / 2);
  return a.length % 2 ? a[mid] : (a[mid - 1] + a[mid]) / 2;
}

function pctDelta(cur, prev) {
  if (prev == null || !Number.isFinite(prev) || prev === 0) {
    if (cur > 0) return { label: 'baru', tone: 'up', value: 100 };
    return null;
  }
  const d = ((cur - prev) / Math.abs(prev)) * 100;
  if (!Number.isFinite(d)) return null;
  const sign = d > 0 ? '+' : '';
  return {
    label: `${sign}${d.toFixed(0)}%`,
    tone: d > 3 ? 'up' : d < -3 ? 'down' : 'flat',
    value: d,
  };
}

function fmtPct(n, digits = 2) {
  if (!Number.isFinite(n) || n === 0) return '—';
  return String(n.toFixed(digits)).replace('.', ',') + '%';
}

function fmtIdNum(n) {
  if (!Number.isFinite(n)) return '—';
  return Threads.fmtNum(Math.round(n));
}

function fmtDateShort(v) {
  if (v == null || v === '') return '—';
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}/.test(v)) {
    const d = new Date(v.includes('T') ? v : `${v}T12:00:00`);
    if (!Number.isNaN(d.getTime())) {
      return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
    }
  }
  const n = Number(v);
  if (!Number.isFinite(n) || n === 0) return '—';
  const ms = n > 1e12 ? n : n * 1000;
  return new Date(ms).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
}

function formatLabel(raw) {
  const k = String(raw || 'TEXT').toUpperCase();
  if (k.includes('CAROUSEL')) return 'Carousel';
  if (k.includes('IMAGE') || k === 'PHOTO') return 'Gambar';
  if (k.includes('VIDEO') || k.includes('REEL')) return 'Video';
  if (k.includes('TEXT')) return 'Teks';
  return k.replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, (c) => c.toUpperCase());
}

function deriveAnalytics(posts) {
  const list = [...(posts || [])];
  const ers = [];
  const views = [];
  const byFormat = {};
  const byDay = {};
  const byPart = {};
  const heat = {};
  const daily = {};

  list.forEach((p) => {
    const m = postMetrics(p);
    const er = postER(p);
    if (m.views > 0) ers.push(er);
    views.push(m.views);
    const fmt = String(p.media_type || 'TEXT').toUpperCase() || 'TEXT';
    if (!byFormat[fmt]) byFormat[fmt] = { n: 0, views: 0, eng: 0, erSum: 0, erN: 0 };
    byFormat[fmt].n += 1;
    byFormat[fmt].views += m.views;
    byFormat[fmt].eng += postEng(m);
    if (m.views > 0) {
      byFormat[fmt].erSum += er;
      byFormat[fmt].erN += 1;
    }

    const t = parsePostTime(p);
    if (!t) return;
    const dayKey = localDayKey(t);
    if (!daily[dayKey]) daily[dayKey] = { views: 0, eng: 0 };
    daily[dayKey].views += m.views;
    daily[dayKey].eng += postEng(m);

    const day = weekdayKey(t);
    const part = daypartOf(t);
    if (!byDay[day]) byDay[day] = { n: 0, erSum: 0, erN: 0, views: 0 };
    if (!byPart[part]) byPart[part] = { n: 0, erSum: 0, erN: 0, views: 0 };
    byDay[day].n += 1;
    byDay[day].views += m.views;
    byPart[part].n += 1;
    byPart[part].views += m.views;
    if (m.views > 0) {
      byDay[day].erSum += er;
      byDay[day].erN += 1;
      byPart[part].erSum += er;
      byPart[part].erN += 1;
    }
    const hk = `${day}|${part}`;
    if (!heat[hk]) heat[hk] = { n: 0, erSum: 0, erN: 0, eng: 0 };
    heat[hk].n += 1;
    heat[hk].eng += postEng(m);
    if (m.views > 0) {
      heat[hk].erSum += er;
      heat[hk].erN += 1;
    }
  });

  const avgER = ers.length ? ers.reduce((s, v) => s + v, 0) / ers.length : 0;
  const medER = median(ers);
  const medViews = median(views);
  const avgViews = views.length ? views.reduce((s, v) => s + v, 0) / views.length : 0;

  let spanDays = 1;
  const times = list.map(parsePostTime).filter(Boolean).sort((a, b) => a - b);
  if (times.length >= 2) {
    spanDays = Math.max(1, (times[times.length - 1] - times[0]) / 86400000);
  } else if (activeDays > 0) {
    spanDays = activeDays;
  }
  const postsPerWeek = list.length ? (list.length / spanDays) * 7 : 0;

  const totals = list.reduce(
    (acc, p) => {
      const m = postMetrics(p);
      acc.views += m.views;
      acc.likes += m.likes;
      acc.replies += m.replies;
      return acc;
    },
    { views: 0, likes: 0, replies: 0 }
  );
  const likeRate = totals.views ? (totals.likes / totals.views) * 100 : 0;
  const replyRate = totals.views ? (totals.replies / totals.views) * 100 : 0;

  const rankedER = [...list]
    .filter((p) => postMetrics(p).views > 0)
    .sort((a, b) => postER(b) - postER(a));
  const q = Math.max(1, Math.ceil(rankedER.length * 0.25));
  const cold = rankedER.slice(-q).reverse();

  const bestFormat = Object.entries(byFormat)
    .map(([k, v]) => ({
      key: k,
      n: v.n,
      avgER: v.erN ? v.erSum / v.erN : 0,
      avgViews: v.n ? v.views / v.n : 0,
    }))
    .sort((a, b) => b.avgER - a.avgER || b.avgViews - a.avgViews)[0];

  const bestDay = Object.entries(byDay)
    .map(([k, v]) => ({ key: k, n: v.n, avgER: v.erN ? v.erSum / v.erN : 0 }))
    .sort((a, b) => b.avgER - a.avgER || b.n - a.n)[0];

  const bestPart = Object.entries(byPart)
    .map(([k, v]) => ({ key: k, n: v.n, avgER: v.erN ? v.erSum / v.erN : 0 }))
    .sort((a, b) => b.avgER - a.avgER || b.n - a.n)[0];

  let bestHeat = null;
  Object.entries(heat).forEach(([hk, v]) => {
    const avg = v.erN ? v.erSum / v.erN : 0;
    const score = v.erN ? avg : v.n;
    if (!bestHeat || score > bestHeat.score) {
      const [day, part] = hk.split('|');
      bestHeat = { day, part, n: v.n, avgER: avg, score };
    }
  });

  let spark = Object.keys(daily)
    .sort()
    .map((k) => {
      const d = daily[k];
      const er = d.views > 0 ? (d.eng / d.views) * 100 : d.eng;
      return { day: k, er };
    });
  if (spark.length < 2) {
    spark = list
      .map((p) => {
        const t = parsePostTime(p);
        if (!t) return null;
        const m = postMetrics(p);
        const eng = postEng(m);
        const er = m.views > 0 ? (eng / m.views) * 100 : eng;
        return { day: t.toISOString(), er };
      })
      .filter(Boolean)
      .sort((a, b) => a.day.localeCompare(b.day));
  }

  return {
    list,
    avgER,
    medER,
    medViews,
    avgViews,
    postsPerWeek,
    likeRate,
    replyRate,
    byFormat,
    byDay,
    byPart,
    heat,
    cold,
    bestFormat,
    bestDay,
    bestPart,
    bestHeat,
    spark,
    spanDays,
  };
}

function drawSpark(series) {
  const line = document.getElementById('spark-line');
  const fill = document.getElementById('spark-fill');
  const peak = document.getElementById('spark-peak');
  const label = document.getElementById('spark-peak-label');
  if (!line || !fill || !peak) return;

  const vals = (series || []).map((s) => s.er).filter((v) => Number.isFinite(v));
  if (vals.length < 2) {
    line.removeAttribute('points');
    fill.removeAttribute('d');
    peak.setAttribute('cx', -20);
    peak.setAttribute('cy', -20);
    if (label) label.textContent = 'Belum cukup titik';
    return;
  }

  const W = 560;
  const H = 96;
  const pad = 8;
  const max = Math.max(...vals, 0.01);
  const min = 0;
  const pts = vals.map((v, i) => {
    const x = (i / (vals.length - 1)) * W;
    const y = H - pad - ((v - min) / (max - min)) * (H - pad * 2 - 16);
    return [x, y];
  });
  line.setAttribute('points', pts.map((p) => p.join(',')).join(' '));
  fill.setAttribute(
    'd',
    `M${pts[0][0]},${H} ` + pts.map((p) => `L${p[0]},${p[1]}`).join(' ') + ` L${W},${H} Z`
  );
  const pi = vals.indexOf(Math.max(...vals));
  peak.setAttribute('cx', pts[pi][0]);
  peak.setAttribute('cy', pts[pi][1]);
  if (label) {
    const d = series[pi]?.day;
    let dateLabel = '';
    if (d) {
      const dt = new Date(/^\d{4}-\d{2}-\d{2}$/.test(d) ? `${d}T12:00:00` : d);
      if (!Number.isNaN(dt.getTime())) {
        dateLabel = dt.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
      }
    }
    label.textContent = `puncak ${fmtPct(vals[pi], 1)} · ${dateLabel}`;
  }
}

function renderMix(map, prevMap, postCount) {
  const parts = MIX_PARTS.map((p) => ({ ...p, v: Number(map[p.key]) || 0 }));
  const total = parts.reduce((s, p) => s + p.v, 0);
  const bar = document.getElementById('mix-bar');
  const legend = document.getElementById('mix-legend');
  const heading = document.getElementById('mix-heading');
  const postsEl = document.getElementById('mix-posts');
  const note = document.getElementById('mix-note');

  if (heading) heading.textContent = total ? `Isi ${Threads.fmtNum(total)} interaksi` : 'Isi interaksi';
  if (postsEl) postsEl.textContent = postCount ? `${postCount} post` : '—';

  if (!total) {
    bar.innerHTML = '<div class="bg-line" style="width:100%"></div>';
    legend.innerHTML = '<li class="comp-row text-muted">Belum ada interaksi di rentang ini</li>';
    if (note) note.textContent = 'Belum cukup data untuk membaca komposisi.';
    return;
  }

  bar.innerHTML = parts
    .filter((p) => p.v > 0)
    .map((p) => `<div class="${p.tone}" style="width:${(p.v / total) * 100}%"></div>`)
    .join('');

  legend.innerHTML = parts
    .filter((p) => p.v > 0)
    .map((p) => {
      const pct = (p.v / total) * 100;
      const pctLabel = pct < 1 ? '<1%' : `${pct.toFixed(0)}%`;
      let deltaHtml = '<span class="w-14 text-right text-[12px] text-muted tnum">—</span>';
      if (prevMap) {
        const d = pctDelta(p.v, Number(prevMap[p.key]) || 0);
        if (d) {
          const cls = d.tone === 'up' ? 'text-up' : d.tone === 'down' ? 'text-down' : 'text-muted';
          deltaHtml = `<span class="w-14 text-right text-[12px] font-semibold ${cls} tnum">${d.label.replace('-', '−')}</span>`;
        }
      }
      return `<li class="comp-row">
        <span class="dot ${p.tone}"></span>
        <span class="flex-1">${p.label}</span>
        <span class="tnum font-mono text-[13px]">${Threads.fmtNum(p.v)}</span>
        <span class="w-12 text-right text-[12px] text-muted tnum">${pctLabel}</span>
        ${deltaHtml}
      </li>`;
    })
    .join('');

  if (note) {
    const replyShare = total ? ((Number(map.replies) || 0) / total) * 100 : 0;
    const replyDelta = prevMap ? pctDelta(Number(map.replies) || 0, Number(prevMap.replies) || 0) : null;
    if (replyDelta && replyDelta.tone === 'down' && Math.abs(replyDelta.value) >= 15) {
      note.textContent = 'Balasan turun paling dalam. Itu sinyal kuat karena balasan yang paling menaikkan ER.';
    } else if (replyShare >= 25) {
      note.textContent = 'Porsi balasan relatif besar — konten diskusi / hook tanya sedang bekerja.';
    } else if (replyShare < 10) {
      note.textContent = 'Interaksi masih didominasi suka. Coba hook yang memancing komentar.';
    } else {
      note.textContent = 'Komposisi di atas diambil dari metrik agregat periode terpilih.';
    }
  }
}

function buildKpis(an, prevAn) {
  const row = (label, hint, now, prev, fmt) => {
    const d = prevAn ? pctDelta(now, prev) : null;
    return {
      label,
      hint,
      now: fmt(now),
      prev: prevAn ? fmt(prev) : '—',
      d: d && Number.isFinite(d.value) ? Math.round(d.value) : null,
    };
  };
  return [
    row('ER per post', 'rata-rata tiap post', an.avgER, prevAn?.avgER, (v) => fmtPct(v)),
    row('ER median', 'post di tengah', an.medER, prevAn?.medER, (v) => fmtPct(v)),
    row('Tayangan median', 'post di tengah', an.medViews, prevAn?.medViews, (v) => fmtIdNum(v)),
    row('Post per minggu', 'ritme posting', an.postsPerWeek, prevAn?.postsPerWeek, (v) => (v ? String(v.toFixed(1)).replace('.', ',') : '—')),
    row('Rasio suka', 'suka ÷ tayangan', an.likeRate, prevAn?.likeRate, (v) => fmtPct(v)),
    row('Rasio balasan', 'balasan ÷ tayangan', an.replyRate, prevAn?.replyRate, (v) => fmtPct(v)),
  ];
}

function renderKpi(mode) {
  const body = document.getElementById('kpiBody');
  if (!body) return;
  const list = [...lastKpis];
  if (mode === 'drop') {
    list.sort((a, b) => {
      const av = a.d == null ? 0 : a.d;
      const bv = b.d == null ? 0 : b.d;
      return av - bv;
    });
  }
  if (!list.length) {
    body.innerHTML = `<tr><td class="px-5 py-4 text-muted" colspan="4">Belum ada KPI.</td></tr>`;
    return;
  }
  body.innerHTML = list
    .map((k) => {
      const has = k.d != null;
      const up = has && k.d >= 0;
      const w = has ? Math.min(Math.abs(k.d), 60) / 60 * 100 : 0;
      const color = up ? 'bg-up' : 'bg-down';
      const text = !has ? 'text-muted' : up ? 'text-up' : 'text-down';
      const label = !has ? '—' : `${up ? '+' : '−'}${Math.abs(k.d)}%`;
      return `<tr class="hover:bg-white/[.03] transition">
        <td class="px-5 py-3.5">
          <span class="block text-[13.5px] font-semibold">${Threads.escapeHtml(k.label)}</span>
          <span class="block text-[11.5px] text-muted">${Threads.escapeHtml(k.hint)}</span>
        </td>
        <td class="px-3 py-3.5 text-right font-mono text-[15px] font-semibold tnum">${Threads.escapeHtml(k.now)}</td>
        <td class="px-3 py-3.5 text-right font-mono text-[13px] text-muted tnum hidden sm:table-cell">${Threads.escapeHtml(k.prev)}</td>
        <td class="px-3 py-3.5 pr-5">
          <div class="flex items-center gap-3">
            <div class="relative h-1.5 flex-1 rounded-full bg-canvas">
              <div class="absolute left-1/2 top-1/2 h-3 w-px -translate-y-1/2 bg-line"></div>
              ${has ? `<div class="absolute top-0 h-1.5 rounded-full ${color}" style="${up ? `left:50%;width:${w / 2}%` : `right:50%;width:${w / 2}%`}"></div>` : ''}
            </div>
            <span class="w-14 shrink-0 text-right font-mono text-[13px] font-semibold ${text} tnum">${label}</span>
          </div>
        </td>
      </tr>`;
    })
    .join('');
}

function renderFormats(byFormat) {
  const root = document.getElementById('formatList');
  if (!root) return;
  const rows = Object.entries(byFormat)
    .map(([k, v]) => ({
      key: k,
      n: v.n,
      avgER: v.erN ? v.erSum / v.erN : 0,
      avgViews: v.n ? v.views / v.n : 0,
    }))
    .sort((a, b) => b.avgER - a.avgER || b.n - a.n);
  if (!rows.length) {
    root.innerHTML = '<li class="text-muted text-[13px]">Belum ada breakdown format.</li>';
    return;
  }
  const maxER = Math.max(...rows.map((r) => r.avgER), 0.01);
  root.innerHTML = rows
    .map((r) => {
      const w = Math.max(4, Math.round((r.avgER / maxER) * 100));
      const small = r.n < 3;
      return `<li>
        <div class="flex items-baseline justify-between gap-3">
          <p class="text-[14px] font-semibold">${Threads.escapeHtml(formatLabel(r.key))}
            ${small ? '<span class="ml-1.5 rounded bg-sun-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-sun-500 align-middle">sampel kecil</span>' : ''}
          </p>
          <p class="font-mono text-[14px] font-semibold tnum">${fmtPct(r.avgER)}</p>
        </div>
        <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-canvas">
          <div class="h-full rounded-full ${small ? 'bg-edge2' : 'bg-iris'}" style="width:${w}%"></div>
        </div>
        <p class="mt-1.5 text-[11.5px] text-muted tnum">${r.n} post · rata-rata ${Threads.fmtNum(Math.round(r.avgViews))} tayangan</p>
      </li>`;
    })
    .join('');
}

function shade(v) {
  if (v >= 5) return 'bg-sun-500 text-white';
  if (v >= 2.5) return 'bg-sun-400 text-white';
  if (v >= 1.5) return 'bg-sun-300 text-ink';
  if (v >= 0.9) return 'bg-sun-200 text-ink';
  return 'bg-sun-100 text-ink-600';
}

function renderHeat(an) {
  const root = document.getElementById('heatmap');
  const bestEl = document.getElementById('heat-best');
  if (!root) return;

  let html = `<div class="grid gap-1.5" style="grid-template-columns:42px repeat(5,1fr)">
    <div></div>${PART_ORDER.map((p) => `<div class="pb-1 text-center text-[11px] font-semibold text-muted">${PART_ID[p]}</div>`).join('')}`;

  let any = false;
  DAY_ORDER.forEach((d) => {
    html += `<div class="flex items-center text-[11.5px] font-semibold text-muted">${DAY_ID[d]}</div>`;
    PART_ORDER.forEach((p) => {
      const cell = an.heat[`${d}|${p}`];
      const er = cell?.erN ? cell.erSum / cell.erN : 0;
      const n = cell?.n || 0;
      if (!n) {
        html += `<div class="ns-heat-empty">–</div>`;
        return;
      }
      any = true;
      const tip = cell?.erN
        ? `${DAY_ID[d]} ${PART_ID[p]} · ER ${fmtPct(er, 1)} · ${n} post`
        : `${DAY_ID[d]} ${PART_ID[p]} · ${n} post`;
      const label = cell?.erN ? fmtPct(er, 1).replace('%', '') : String(n);
      const tone = cell?.erN ? shade(er) : shade(Math.min(5, n));
      html += `<button type="button" class="ns-heat-cell cell ${tone} hover:ring-2 hover:ring-iris/40 transition" data-tip="${Threads.escapeHtml(tip)}">${Threads.escapeHtml(label)}</button>`;
    });
  });
  html += '</div>';
  root.innerHTML = html;

  if (!any) {
    root.innerHTML = '<div class="text-muted text-[13px] py-4">Belum cukup timestamp untuk heatmap.</div>';
  }

  if (bestEl) {
    if (an.bestHeat) {
      const erBit = an.bestHeat.avgER
        ? ` · ${fmtPct(an.bestHeat.avgER, 1)}`
        : '';
      bestEl.innerHTML = `Slot terbaik: <span class="font-semibold text-ink">${DAY_ID[an.bestHeat.day]} ${PART_ID[an.bestHeat.part]}</span>${erBit} dari ${an.bestHeat.n} post`;
    } else {
      bestEl.textContent = 'Slot terbaik: —';
    }
  }

  const tip = document.getElementById('tip');
  root.querySelectorAll('.cell').forEach((c) => {
    c.addEventListener('mouseenter', () => {
      tip.textContent = c.dataset.tip || '';
      tip.classList.remove('hidden');
    });
    c.addEventListener('mousemove', (e) => {
      tip.style.left = e.clientX + 12 + 'px';
      tip.style.top = e.clientY - 34 + 'px';
    });
    c.addEventListener('mouseleave', () => tip.classList.add('hidden'));
  });
}

function renderActions(an, map) {
  const root = document.getElementById('action-cards');
  const count = document.getElementById('action-count');
  if (!root) return;
  const cards = [];

  if (an.bestHeat && an.bestHeat.n >= 1 && an.bestHeat.avgER > an.avgER * 1.4) {
    const day = DAY_ID[an.bestHeat.day];
    const part = PART_ID[an.bestHeat.part];
    cards.push({
      tag: 'Waktu',
      title: `Pindahkan post ke ${day} ${part.toLowerCase()}`,
      body: `ER ${day} ${part.toLowerCase()} <span class="font-mono font-semibold text-ink">${fmtPct(an.bestHeat.avgER, 1)}</span> — jauh di atas rata-rata. Baru ${an.bestHeat.n} post yang tayang di situ.`,
      href: '/app/kalender',
      cta: `Jadwalkan di ${day}`,
    });
  }

  if (an.bestFormat && Object.keys(an.byFormat).length >= 2) {
    const weak = Object.entries(an.byFormat)
      .map(([k, v]) => ({
        key: k,
        n: v.n,
        avgER: v.erN ? v.erSum / v.erN : 0,
      }))
      .filter((r) => r.n >= 3)
      .sort((a, b) => a.avgER - b.avgER)[0];
    if (weak && an.bestFormat.key !== weak.key && an.bestFormat.avgER > weak.avgER * 1.3) {
      cards.push({
        tag: 'Format',
        title: `Kurangi post ${formatLabel(weak.key).toLowerCase()}`,
        body: `${weak.n} post ${formatLabel(weak.key).toLowerCase()} cuma dapat ER <span class="font-mono font-semibold text-ink">${fmtPct(weak.avgER)}</span>, sementara ${an.bestFormat.n} post ${formatLabel(an.bestFormat.key).toLowerCase()} dapat <span class="font-mono font-semibold text-ink">${fmtPct(an.bestFormat.avgER)}</span>.`,
        href: '/app/posts',
        cta: `Lihat post ${formatLabel(weak.key).toLowerCase()}`,
      });
    }
  }

  if (an.cold?.length) {
    const threshold = an.medER || an.avgER || 0;
    cards.push({
      tag: 'Perbaikan',
      title: `Tulis ulang ${an.cold.length} post terlemah`,
      body: `${an.cold.length} post ada di kuartil ER terbawah${threshold ? ` (di bawah <span class="font-mono font-semibold text-ink">${fmtPct(threshold)}</span>)` : ''} tapi masih punya tayangan.`,
      href: '/app/generate',
      cta: 'Buka di Generate',
    });
  }

  if (an.replyRate < an.likeRate * 0.12 && an.likeRate > 0) {
    cards.push({
      tag: 'Hook',
      title: 'Tambah hook yang memancing balasan',
      body: `Rasio balasan hanya <span class="font-mono font-semibold text-ink">${fmtPct(an.replyRate)}</span> vs rasio suka <span class="font-mono font-semibold text-ink">${fmtPct(an.likeRate)}</span>.`,
      href: '/app/buat',
      cta: 'Buat post baru',
    });
  }

  const shown = cards.slice(0, 3);
  if (count) count.textContent = shown.length ? `${shown.length} saran` : '0 saran';
  if (!shown.length) {
    root.innerHTML = `<article class="action-card md:col-span-3"><p class="action-tag">Info</p><p class="mt-2.5 text-[14.5px] font-semibold">Belum cukup sinyal aksi</p><p class="mt-1.5 text-[12.5px] text-ink-600">Butuh lebih banyak post di rentang ini supaya saran otomatis muncul.</p><a class="action-btn" href="/app/buat">Buat post</a></article>`;
    return;
  }
  root.innerHTML = shown
    .map(
      (c) => `<article class="action-card">
      <p class="action-tag">${Threads.escapeHtml(c.tag)}</p>
      <p class="mt-2.5 text-[14.5px] font-semibold leading-snug">${Threads.escapeHtml(c.title)}</p>
      <p class="mt-1.5 text-[12.5px] leading-relaxed text-ink-600">${c.body}</p>
      <a class="action-btn" href="${c.href}">${Threads.escapeHtml(c.cta)}</a>
    </article>`
    )
    .join('');
}

function renderRaw(map, prevMap, followers) {
  const root = document.getElementById('raw-grid');
  if (!root) return;
  const items = [
    { key: 'views', label: 'Tayangan' },
    { key: 'likes', label: 'Suka' },
    { key: 'replies', label: 'Balasan' },
    { key: 'reposts', label: 'Repost' },
    { key: 'quotes', label: 'Kutipan' },
  ];
  const cells = items.map((it) => {
    const cur = Number(map[it.key]) || 0;
    const d = prevMap ? pctDelta(cur, Number(prevMap[it.key]) || 0) : null;
    const cls = !d ? 'text-muted' : d.tone === 'up' ? 'text-up' : d.tone === 'down' ? 'text-down' : 'text-muted';
    return `<div class="raw"><p class="raw-label">${it.label}</p><p class="raw-val">${Threads.fmtNum(cur)}</p><p class="raw-delta ${cls}">${d ? d.label.replace('-', '−') : '—'}</p></div>`;
  });
  cells.push(
    `<div class="raw"><p class="raw-label">Pengikut</p><p class="raw-val">${followers != null ? Threads.fmtNum(followers) : '—'}</p><p class="raw-delta text-muted">total akun</p></div>`
  );
  root.innerHTML = cells.join('');
}

function applyPayload(data, prev) {
  lastPayload = data;
  lastPrev = prev || null;
  let map = metricMap(data);
  const postMix = mixFromPosts(data.posts || []);
  if ((Number(map.likes) || 0) + (Number(map.replies) || 0) + (Number(map.reposts) || 0) + (Number(map.quotes) || 0) === 0) {
    map = { ...map, ...postMix };
  }
  if (!(Number(map.views) > 0) && postMix.views > 0) map.views = postMix.views;
  const prevMap = prev ? metricMap(prev) : null;
  const an = deriveAnalytics(data.posts || []);
  const prevAn = prev ? deriveAnalytics(prev.posts || []) : null;

  const views = Number(map.views) || 0;
  const eng =
    Number(data.engagement) ||
    ((map.likes || 0) + (map.replies || 0) + (map.reposts || 0) + (map.quotes || 0));
  let engRate = Number(data.engagement_rate) || 0;
  if (!engRate && views > 0 && eng > 0) engRate = (eng / views) * 100;
  if (!engRate && an.avgER > 0) engRate = an.avgER;
  const posts = Number(data.post_count) || (data.posts || []).length || 0;

  const erEl = document.getElementById('hero-er');
  if (erEl) {
    erEl.innerHTML = engRate
      ? `${String(engRate.toFixed(2)).replace('.', ',')}<span class="text-[32px] align-top">%</span>`
      : '—';
  }

  const deltaEl = document.getElementById('hero-delta');
  if (deltaEl) {
    if (prev && Number.isFinite(Number(prev.engagement_rate))) {
      const d = pctDelta(engRate, Number(prev.engagement_rate) || 0);
      if (d && d.tone !== 'flat') {
        deltaEl.hidden = false;
        const up = d.tone === 'up';
        deltaEl.className = `mb-1.5 inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12.5px] font-semibold ${up ? 'bg-up/10 text-up' : 'bg-down/10 text-down'}`;
        deltaEl.innerHTML = `${up ? '↑' : '↓'} ${Math.abs(Math.round(d.value))}% vs periode lalu`;
      } else {
        deltaEl.hidden = true;
      }
    } else {
      deltaEl.hidden = true;
    }
  }

  const copy = document.getElementById('hero-copy');
  if (copy) {
    if (views || eng) {
      let extra = '';
      if (prevMap) {
        const rd = pctDelta(Number(map.replies) || 0, Number(prevMap.replies) || 0);
        const vd = pctDelta(views, Number(prevMap.views) || 0);
        if (rd && rd.tone === 'down' && (!vd || vd.tone !== 'down' || Math.abs(rd.value) > Math.abs(vd.value))) {
          extra = ' Penurunan datang dari balasan, bukan dari jangkauan.';
        } else if (vd && vd.tone === 'down') {
          extra = ' Jangkauan ikut turun di periode ini.';
        }
      }
      copy.innerHTML = `${Threads.fmtNum(eng)} interaksi dari <span class="font-semibold text-ink">${Threads.fmtNum(views)} tayangan</span> di ${posts} post.${extra}`;
    } else {
      copy.textContent = 'Belum ada data engagement di rentang terpilih.';
    }
  }

  const rangeLabel = document.getElementById('insights-range-label');
  if (rangeLabel) {
    const acc = data.repliz_account;
    const accBit = acc?.username
      ? `@${String(acc.username).replace(/^@/, '')}${acc.type ? ' · ' + acc.type : ''} <span class="text-muted/50">·</span> `
      : '';
    const cur = data.since || data.until ? `${fmtDateShort(data.since)} – ${fmtDateShort(data.until)}` : 'Semua waktu';
    const cmp = prev?.since || prev?.until ? ` <span class="text-muted/50">·</span> dibanding ${fmtDateShort(prev.since)} – ${fmtDateShort(prev.until)}` : '';
    rangeLabel.innerHTML = accBit + cur + cmp;
  }

  const kpiLead = document.getElementById('kpi-lead');
  if (kpiLead) {
    const cap = data.sample_capped || data.source === 'repliz' ? ' (sampel Repliz, maks. 40)' : '';
    kpiLead.textContent = `Dihitung dari ${posts} post di rentang ini${cap}, bukan total akun.`;
  }

  drawSpark(an.spark);
  renderMix(map, prevMap, posts);
  renderActions(an, map);
  lastKpis = buildKpis(an, prevAn);
  renderKpi(kpiSortMode);
  renderFormats(an.byFormat);
  renderHeat(an);
  renderRaw(map, prevMap, map.followers_count ?? data.followers_count ?? null);
}

function insightsEarliestDate() {
  const d = new Date();
  d.setFullYear(d.getFullYear() - 2);
  d.setDate(d.getDate() + 1);
  const apiFloor = new Date('2024-04-13T00:00:00');
  return d > apiFloor ? d : apiFloor;
}

function fmtISODate(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function setDateRange(days) {
  activeDays = days || 0;
  const until = new Date();
  const since = new Date();
  const untilEl = document.getElementById('insights-until');
  const sinceEl = document.getElementById('insights-since');
  const earliest = insightsEarliestDate();
  if (!days) {
    sinceEl.value = fmtISODate(earliest);
    untilEl.value = fmtISODate(until);
    return;
  }
  since.setDate(until.getDate() - days);
  if (since < earliest) since.setTime(earliest.getTime());
  sinceEl.value = fmtISODate(since);
  untilEl.value = fmtISODate(until);
}

function rangeQuery(sinceISO, untilISO) {
  const q = new URLSearchParams();
  const nowSec = Math.floor(Date.now() / 1000);
  const floor = Math.floor(insightsEarliestDate().getTime() / 1000);
  if (sinceISO) {
    let s = Math.floor(new Date(sinceISO + 'T00:00:00').getTime() / 1000);
    if (s < floor) s = floor;
    q.set('since', String(s));
  }
  if (untilISO) {
    let u = Math.floor(new Date(untilISO + 'T23:59:59').getTime() / 1000);
    if (u > nowSec) u = nowSec;
    q.set('until', String(u));
  }
  return q;
}

function previousWindow(sinceISO, untilISO) {
  const since = new Date(sinceISO + 'T00:00:00');
  const until = new Date(untilISO + 'T23:59:59');
  const span = until.getTime() - since.getTime();
  if (!(span > 0)) return null;
  const prevUntil = new Date(since.getTime() - 1000);
  const prevSince = new Date(prevUntil.getTime() - span);
  const earliest = insightsEarliestDate();
  if (prevUntil < earliest) return null;
  if (prevSince < earliest) prevSince.setTime(earliest.getTime());
  return { since: fmtISODate(prevSince), until: fmtISODate(prevUntil) };
}

function escapeOpt(s) {
  return Threads.escapeHtml(String(s || ''));
}

async function loadReplizAccounts() {
  const sel = document.getElementById('repliz-account');
  if (!sel) return false;
  try {
    const d = await Threads.api('/api/repliz/accounts');
    const items = Array.isArray(d.accounts) ? d.accounts : [];
    if (!items.length) {
      sel.hidden = true;
      sel.classList.add('hidden');
      return false;
    }
    replizAccountId = d.active_id || items[0].id || items[0]._id || '';
    sel.innerHTML = items.map((a) => {
      const id = a.id || a._id || '';
      const user = String(a.username || a.name || id).replace(/^@/, '');
      const type = a.type || '';
      const off = a.isConnected === false ? ' (off)' : '';
      return `<option value="${escapeOpt(id)}">@${escapeOpt(user)} · ${escapeOpt(type)}${off}</option>`;
    }).join('');
    if (replizAccountId) sel.value = replizAccountId;
    sel.hidden = false;
    sel.removeAttribute('hidden');
    sel.classList.remove('hidden');
    return true;
  } catch {
    sel.hidden = true;
    sel.classList.add('hidden');
    return false;
  }
}

async function fetchInsights(sinceISO, untilISO) {
  const q = rangeQuery(sinceISO, untilISO);
  const id = replizAccountId || document.getElementById('repliz-account')?.value || '';
  if (id) q.set('account_id', id);
  return Threads.api('/api/insights?aggregate=1&posts=40&' + q.toString());
}

async function loadInsights() {
  showAlert('');
  const body = document.getElementById('insights-body');
  body.style.opacity = '0.55';
  const since = document.getElementById('insights-since').value;
  const until = document.getElementById('insights-until').value;
  try {
    const data = await fetchInsights(since, until);
    let prev = null;
    const win = previousWindow(since, until);
    if (win && activeDays !== 0) {
      try {
        prev = await fetchInsights(win.since, win.until);
      } catch {
        prev = null;
      }
    }
    applyPayload(data, prev);
    if (data?.warning) showAlert(data.warning);
    else if (!(data?.data || []).length && !(data?.posts || []).length && !data?.metrics) {
      showAlert('Tidak ada data insight untuk rentang ini.');
    }
  } finally {
    body.style.opacity = '1';
  }
}

document.getElementById('btn-insights').onclick = () =>
  loadInsights().catch((e) => {
    showAlert(e.message);
    Threads.toast(e.message, false);
  });

document.getElementById('kpi-sort').addEventListener('click', (e) => {
  const btn = e.target.closest('.sort-btn');
  if (!btn) return;
  document.querySelectorAll('#kpi-sort .sort-btn').forEach((x) => x.classList.remove('sort-on'));
  btn.classList.add('sort-on');
  kpiSortMode = btn.dataset.sort || 'drop';
  renderKpi(kpiSortMode);
});

document.getElementById('repliz-account')?.addEventListener('change', async (e) => {
  const id = e.target.value;
  if (!id) return;
  replizAccountId = id;
  try {
    await Threads.api('/api/repliz/accounts/switch', { method: 'POST', body: JSON.stringify({ id }) });
  } catch {
    /* tetap muat insight dengan id terpilih */
  }
  loadInsights().catch((err) => {
    showAlert(err.message);
    Threads.toast(err.message, false);
  });
});

document.getElementById('insights-presets').addEventListener('click', (e) => {
  const btn = e.target.closest('button[data-days]');
  if (!btn) return;
  document.querySelectorAll('#insights-presets .period').forEach((b) => {
    b.classList.remove('period-on');
    b.removeAttribute('aria-selected');
  });
  btn.classList.add('period-on');
  btn.setAttribute('aria-selected', 'true');
  setDateRange(Number(btn.dataset.days));
  loadInsights().catch((err) => {
    showAlert(err.message);
    Threads.toast(err.message, false);
  });
});

(async () => {
  const hasRepliz = await loadReplizAccounts();
  if (!hasRepliz) {
    showAlert('Hubungkan akun Repliz di halaman Akun dulu.');
    return;
  }
  setDateRange(30);
  try {
    await loadInsights();
  } catch (e) {
    showAlert(e.message);
    Threads.toast(e.message, false);
  }
})();
