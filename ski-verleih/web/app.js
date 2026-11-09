'use strict';
// Ski-Verleih Frontend – reines JS, alle Daten nur über /api (siehe CLAUDE.md).

// ---------- Anzeige-Texte ----------
const LABELS = {
  skiType: { alpin: 'Alpin', carving: 'Carving', freeride: 'Freeride', langlauf: 'Langlauf', touren: 'Touren', snowboard: 'Snowboard' },
  condition: { neu: 'Neu', gut: 'Gut', gebraucht: 'Gebraucht' },
  docType: { personalausweis: 'Personalausweis', reisepass: 'Reisepass', fuehrerschein: 'Führerschein' },
  status: {
    verfuegbar: 'Verfügbar', ausgeliehen: 'Ausgeliehen', ueberfaellig: 'Überfällig', reserviert: 'Reserviert',
    aktiv: 'Aktiv', zurueckgegeben: 'Zurückgegeben', storniert: 'Storniert',
  },
};

// ---------- Hilfsfunktionen ----------
const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
const eur = new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' });
const money = (cents) => eur.format((cents || 0) / 100);
const dt = (s) => new Date(s).toLocaleString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const badge = (st) => `<span class="badge b-${st}">${LABELS.status[st] || st}</span>`;
const options = (map, selected) => Object.entries(map).map(([v, l]) => `<option value="${v}" ${v === selected ? 'selected' : ''}>${l}</option>`).join('');

function duration(h) {
  const d = Math.floor(h / 24), r = h % 24, parts = [];
  if (d) parts.push(`${d} ${d === 1 ? 'Tag' : 'Tage'}`);
  if (r) parts.push(`${r} Std.`);
  return parts.join(' ') || '0 Std.';
}

function debounce(fn, ms = 250) {
  let t;
  return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

async function api(method, path, body) {
  const opt = { method, headers: {} };
  if (body instanceof FormData) opt.body = body;
  else if (body !== undefined) { opt.headers['Content-Type'] = 'application/json'; opt.body = JSON.stringify(body); }
  const res = await fetch(path, opt);
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Fehler ${res.status}`);
  return data;
}

function toast(msg, kind = 'ok') {
  const el = document.createElement('div');
  el.className = `toast ${kind}`;
  el.textContent = msg;
  $('#toasts').append(el);
  setTimeout(() => el.remove(), kind === 'err' ? 6000 : 3500);
}

// Preis mit Rechenweg (B3, B4) – Werte kommen vollständig vom Server.
function quoteHtml(q) {
  const lines = [];
  if (q.fullDays) lines.push(`${q.fullDays} × Tagespreis ${money(q.dailyCents)} = ${money(q.daysCents)}`);
  if (q.restHours) {
    lines.push(q.restCapped
      ? `${q.restHours} Std. × ${money(q.hourlyCents)} → gedeckelt auf Tagespreis ${money(q.restCents)}`
      : `${q.restHours} Std. × ${money(q.hourlyCents)} = ${money(q.restCents)}`);
  }
  return `<div class="quote-box">
    <div class="muted small">Preis für ${duration(q.hours)}</div>
    <div class="total">${money(q.totalCents)}</div>
    <div class="small">${lines.join('<br>')}</div>
  </div>`;
}

// ---------- Dialog ----------
const modal = {
  el: $('#modal'),
  open({ title, body, submitLabel = 'Speichern', submitClass = 'btn-primary', onSubmit, onOpen }) {
    $('#modal-title').textContent = title;
    $('#modal-body').innerHTML = body;
    const submit = $('#modal-submit');
    submit.textContent = submitLabel;
    submit.className = `btn ${submitClass}`;
    submit.disabled = false;
    this.error('');
    this.onSubmit = onSubmit;
    this.el.showModal();
    onOpen?.($('#modal-body'));
    $('#modal-body input, #modal-body select')?.focus();
  },
  close() { this.el.close(); },
  error(msg) { const e = $('#modal-error'); e.textContent = msg; e.hidden = !msg; },
};

$('#modal-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const submit = $('#modal-submit');
  submit.disabled = true;
  try {
    await modal.onSubmit?.($('#modal-body'));
    modal.close();
    render();
  } catch (err) {
    modal.error(err.message);
  } finally {
    submit.disabled = false;
  }
});
document.querySelectorAll('[data-close]').forEach((b) => b.addEventListener('click', () => modal.close()));

function confirmDialog(title, text, label, cls, action) {
  modal.open({ title, body: `<p>${text}</p>`, submitLabel: label, submitClass: cls, onSubmit: action });
}

// ---------- Router ----------
const state = { skis: [], persons: [], rentals: [], skiFilter: { q: '', type: '', status: '' }, personQ: '', rentalTab: 'offen', rentalQ: '' };
const pages = { dashboard: renderDashboard, skis: renderSkis, persons: renderPersons, rentals: renderRentals, prices: renderPrices };

async function render() {
  const page = (location.hash.slice(1) || 'dashboard');
  const fn = pages[page] || renderDashboard;
  document.querySelectorAll('.topbar nav a').forEach((a) => a.classList.toggle('active', a.dataset.page === page));
  document.body.classList.remove('nav-open');
  try {
    await fn($('#app'));
  } catch (err) {
    $('#app').innerHTML = `<section class="panel"><p class="form-error">Daten konnten nicht geladen werden: ${esc(err.message)}</p></section>`;
  }
}
window.addEventListener('hashchange', render);

// ---------- Dashboard (A1) ----------
async function renderDashboard(app) {
  const d = await api('GET', '/api/dashboard');
  const item = (r, action) => `<li>
      <div><b>${esc(r.skiLabel)}</b><div class="small muted">${esc(r.personName)} · ${action === 'cancel' ? 'ab ' + dt(r.start) : 'bis ' + dt(r.plannedEnd)}</div></div>
      ${action === 'return' ? `<button class="btn btn-sm btn-success" data-action="return" data-id="${r.id}">Rückgabe</button>` : badge(r.status)}
    </li>`;
  const list = (title, rows, action, cls = '') => `<div class="list-card ${cls}">
      <h3>${title} <span class="muted">(${rows.length})</span></h3>
      <ul>${rows.map((r) => item(r, action)).join('') || '<li class="muted small">Keine Einträge</li>'}</ul>
    </div>`;

  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Dashboard</h1><span class="muted">${dt(new Date())}</span></div>
    <div class="tiles">
      <a class="tile t1" href="#skis"><div class="label">Verfügbare Ski</div><div class="value">${d.availableSkis}</div></a>
      <a class="tile t2" href="#rentals"><div class="label">Ausgeliehene Ski</div><div class="value">${d.rentedSkis}</div></a>
      <a class="tile t3" href="#skis"><div class="label">Gesamt Ski</div><div class="value">${d.totalSkis}</div></a>
      <a class="tile t4" href="#persons"><div class="label">Registrierte Personen</div><div class="value">${d.persons}</div></a>
    </div>
    <div class="mini-stats">
      <div class="mini"><span>Offene Reservierungen</span><b>${d.reservations}</b></div>
      <div class="mini ${d.overdueCount ? 'warn' : ''}"><span>Überfällig</span><b>${d.overdueCount}</b></div>
      <div class="mini"><span>Umsatz heute</span><b>${money(d.revenueTodayCents)}</b></div>
      <div class="mini ${d.personsWithoutDoc ? 'warn' : ''}"><span>Ohne Ausweis-Scan</span><b>${d.personsWithoutDoc}</b></div>
    </div>
    <h2>Schnellzugriff</h2>
    <div class="quick">
      <a class="btn" href="#skis">Ski verwalten</a>
      <a class="btn" href="#persons">Personen verwalten</a>
      <button class="btn btn-success" data-action="new-rental">Neue Ausleihe</button>
    </div>
    <h2>Heute im Blick</h2>
    <div class="lists">
      ${list('⚠️ Überfällig', d.overdue, 'return', d.overdue.length ? 'danger' : '')}
      ${list('Heute fällig', d.dueToday, 'return')}
      ${list('Nächste Reservierungen', d.upcomingReservations, 'cancel')}
    </div>
  </section>`;
}

// ---------- Ski-Verwaltung (A2) ----------
async function renderSkis(app) {
  state.skis = await api('GET', '/api/skis');
  const f = state.skiFilter;
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Ski-Verwaltung</h1><button class="btn btn-success" data-action="new-ski">+ Neuer Ski</button></div>
    <div class="toolbar">
      <input id="ski-q" type="search" placeholder="Suche Marke / Modell …" value="${esc(f.q)}">
      <select id="ski-type"><option value="">Alle Typen</option>${options(LABELS.skiType, f.type)}</select>
      <select id="ski-status"><option value="">Alle Status</option>${options({ verfuegbar: 'Verfügbar', ausgeliehen: 'Ausgeliehen', ueberfaellig: 'Überfällig' }, f.status)}</select>
    </div>
    <div class="table-wrap"><table>
      <thead><tr><th>#</th><th>Ski</th><th>Typ</th><th class="num">Länge</th><th>Zustand</th><th>Status</th><th>Nächste Reservierung</th><th></th></tr></thead>
      <tbody id="ski-rows"></tbody>
    </table></div>
  </section>`;
  const update = () => {
    f.q = $('#ski-q').value.toLowerCase(); f.type = $('#ski-type').value; f.status = $('#ski-status').value;
    const rows = state.skis.filter((k) =>
      (!f.q || `${k.brand} ${k.model}`.toLowerCase().includes(f.q)) && (!f.type || k.type === f.type) && (!f.status || k.status === f.status));
    $('#ski-rows').innerHTML = rows.map((k) => `<tr>
        <td class="muted">${k.id}</td>
        <td><b>${esc(k.brand)}</b> ${esc(k.model)}${k.notes ? `<div class="small muted">${esc(k.notes)}</div>` : ''}</td>
        <td>${LABELS.skiType[k.type]}</td>
        <td class="num">${k.lengthCm} cm</td>
        <td>${LABELS.condition[k.condition]}</td>
        <td>${badge(k.status)}</td>
        <td class="small">${k.nextReservation ? dt(k.nextReservation) : '<span class="muted">–</span>'}</td>
        <td><div class="actions">
          <button class="btn btn-sm btn-success" data-action="rent-ski" data-id="${k.id}" ${k.status !== 'verfuegbar' ? 'disabled title="Ski ist gerade nicht verfügbar"' : ''}>Verleihen</button>
          <button class="btn btn-sm btn-ghost" data-action="edit-ski" data-id="${k.id}">Bearbeiten</button>
          <button class="btn btn-sm btn-ghost" data-action="delete-ski" data-id="${k.id}" title="Löschen">🗑</button>
        </div></td>
      </tr>`).join('') || `<tr><td colspan="8" class="empty">Keine Ski gefunden.</td></tr>`;
  };
  ['#ski-q', '#ski-type', '#ski-status'].forEach((s) => $(s).addEventListener('input', update));
  update();
}

function openSkiForm(ski) {
  const k = ski || { brand: '', model: '', type: 'alpin', lengthCm: 170, condition: 'gut', notes: '' };
  modal.open({
    title: ski ? `Ski #${ski.id} bearbeiten` : 'Neuer Ski',
    body: `<div class="grid-2">
        <div class="field"><label>Marke <span class="req">*</span></label><input name="brand" required value="${esc(k.brand)}" placeholder="z. B. Atomic"></div>
        <div class="field"><label>Modell <span class="req">*</span></label><input name="model" required value="${esc(k.model)}" placeholder="z. B. Redster S9"></div>
        <div class="field"><label>Typ <span class="req">*</span></label><select name="type">${options(LABELS.skiType, k.type)}</select></div>
        <div class="field"><label>Länge (cm) <span class="req">*</span></label><input name="lengthCm" type="number" min="80" max="220" value="${k.lengthCm}"><div class="hint">80–220 cm</div></div>
        <div class="field"><label>Zustand <span class="req">*</span></label><select name="condition">${options(LABELS.condition, k.condition)}</select></div>
      </div>
      <div class="field"><label>Notiz</label><textarea name="notes" rows="2">${esc(k.notes)}</textarea></div>`,
    onSubmit: async (body) => {
      const v = (n) => $(`[name=${n}]`, body).value;
      const data = { brand: v('brand'), model: v('model'), type: v('type'), lengthCm: Number(v('lengthCm')), condition: v('condition'), notes: v('notes') };
      if (ski) await api('PUT', `/api/skis/${ski.id}`, data); else await api('POST', '/api/skis', data);
      toast(ski ? 'Ski gespeichert' : 'Ski angelegt');
    },
  });
}

// ---------- Personen (A3, A4) ----------
async function renderPersons(app) {
  state.persons = await api('GET', '/api/persons');
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Personen</h1><button class="btn btn-success" data-action="new-person">+ Neue Person</button></div>
    <div class="toolbar"><input id="person-q" type="search" placeholder="Suche Name, E-Mail, Ausweisnummer …" value="${esc(state.personQ)}"></div>
    <div class="table-wrap"><table>
      <thead><tr><th>Name</th><th>Kontakt</th><th>Ausweis</th><th>Scan</th><th class="num">Offene Ausleihen</th><th></th></tr></thead>
      <tbody id="person-rows"></tbody>
    </table></div>
  </section>`;
  const update = () => {
    const q = (state.personQ = $('#person-q').value).toLowerCase();
    const rows = state.persons.filter((p) => !q || `${p.firstName} ${p.lastName} ${p.email} ${p.phone} ${p.idDocNumber}`.toLowerCase().includes(q));
    $('#person-rows').innerHTML = rows.map((p) => `<tr>
        <td><b>${esc(p.lastName)}</b>, ${esc(p.firstName)}</td>
        <td class="small">${esc(p.email)}${p.email && p.phone ? '<br>' : ''}${esc(p.phone)}</td>
        <td class="small">${LABELS.docType[p.idDocType]}<br><code>${esc(p.idDocNumber)}</code></td>
        <td>${p.document
          ? `<a class="badge b-doc-ok" href="/api/persons/${p.id}/document" target="_blank" rel="noopener">✓ ansehen</a>`
          : '<span class="badge b-doc-missing">fehlt</span>'}</td>
        <td class="num">${p.openRentals}</td>
        <td><div class="actions">
          <button class="btn btn-sm btn-success" data-action="rent-person" data-id="${p.id}">Ausleihe</button>
          <button class="btn btn-sm btn-ghost" data-action="edit-person" data-id="${p.id}">Bearbeiten</button>
          <button class="btn btn-sm btn-ghost" data-action="delete-person" data-id="${p.id}" title="Löschen">🗑</button>
        </div></td>
      </tr>`).join('') || `<tr><td colspan="6" class="empty">Keine Personen gefunden.</td></tr>`;
  };
  $('#person-q').addEventListener('input', update);
  update();
}

const ALLOWED_DOC = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf'];

function openPersonForm(person) {
  const p = person || { firstName: '', lastName: '', email: '', phone: '', idDocType: 'personalausweis', idDocNumber: '' };
  let personId = person?.id;
  let file = null;
  let removeDoc = false;

  const docPreview = () => {
    if (file) {
      return file.type.startsWith('image/')
        ? `<div>📎 ${esc(file.name)}</div><img src="${URL.createObjectURL(file)}" alt="Vorschau">`
        : `<div>📄 ${esc(file.name)} (${Math.round(file.size / 1024)} KB)</div>`;
    }
    if (person?.document && !removeDoc) {
      const d = person.document;
      return `<div>Hinterlegt: <a href="/api/persons/${person.id}/document" target="_blank" rel="noopener">${esc(d.fileName || 'Ausweis')}</a>
        <button type="button" class="btn btn-sm btn-ghost" id="doc-remove">Entfernen</button></div>
        ${d.contentType.startsWith('image/') ? `<img src="/api/persons/${person.id}/document?ts=${Date.now()}" alt="Ausweis">` : ''}`;
    }
    return '<div>📤 Ausweis-Scan hierher ziehen oder <u>klicken</u></div><div class="hint">JPEG, PNG, WebP oder PDF · max. 5 MB</div>';
  };

  modal.open({
    title: person ? `${person.firstName} ${person.lastName} bearbeiten` : 'Neue Person',
    body: `<div class="grid-2">
        <div class="field"><label>Vorname <span class="req">*</span></label><input name="firstName" value="${esc(p.firstName)}"></div>
        <div class="field"><label>Nachname <span class="req">*</span></label><input name="lastName" value="${esc(p.lastName)}"></div>
        <div class="field"><label>E-Mail</label><input name="email" type="email" value="${esc(p.email)}"></div>
        <div class="field"><label>Telefon</label><input name="phone" type="tel" value="${esc(p.phone)}"><div class="hint">E-Mail oder Telefon ist Pflicht</div></div>
        <div class="field"><label>Ausweistyp <span class="req">*</span></label><select name="idDocType">${options(LABELS.docType, p.idDocType)}</select></div>
        <div class="field"><label>Ausweisnummer <span class="req">*</span></label><input name="idDocNumber" value="${esc(p.idDocNumber)}" placeholder="z. B. L01X00T47" style="text-transform:uppercase"></div>
      </div>
      <div class="field"><label>Ausweis-Scan</label>
        <div class="dropzone" id="dropzone" tabindex="0">${docPreview()}</div>
        <input type="file" id="doc-file" accept="image/jpeg,image/png,image/webp,application/pdf" hidden>
      </div>`,
    onOpen: (body) => {
      const dz = $('#dropzone', body), input = $('#doc-file', body);
      const refresh = () => { dz.innerHTML = docPreview(); };
      const take = (f) => {
        if (!f) return;
        if (!ALLOWED_DOC.includes(f.type)) return modal.error('Nur JPEG, PNG, WebP oder PDF erlaubt.');
        if (f.size > 5 * 1024 * 1024) return modal.error('Datei ist größer als 5 MB.');
        modal.error(''); file = f; removeDoc = false; refresh();
      };
      dz.addEventListener('click', (e) => {
        if (e.target.id === 'doc-remove') { removeDoc = true; refresh(); return; }
        if (e.target.closest('a')) return;
        input.click();
      });
      dz.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); input.click(); } });
      input.addEventListener('change', () => take(input.files[0]));
      dz.addEventListener('dragover', (e) => { e.preventDefault(); dz.classList.add('drag'); });
      dz.addEventListener('dragleave', () => dz.classList.remove('drag'));
      dz.addEventListener('drop', (e) => { e.preventDefault(); dz.classList.remove('drag'); take(e.dataTransfer.files[0]); });
    },
    onSubmit: async (body) => {
      const v = (n) => $(`[name=${n}]`, body).value;
      const data = { firstName: v('firstName'), lastName: v('lastName'), email: v('email'), phone: v('phone'), idDocType: v('idDocType'), idDocNumber: v('idDocNumber') };
      // Nach dem Anlegen merken wir uns die ID: schlägt der Upload fehl, wird beim erneuten Speichern aktualisiert statt doppelt angelegt.
      if (personId) await api('PUT', `/api/persons/${personId}`, data);
      else personId = (await api('POST', '/api/persons', data)).id;
      if (file) {
        const fd = new FormData();
        fd.append('file', file);
        await api('POST', `/api/persons/${personId}/document`, fd);
      } else if (removeDoc) {
        await api('DELETE', `/api/persons/${personId}/document`);
      }
      toast(person ? 'Person gespeichert' : 'Person angelegt');
    },
  });
}

// ---------- Ausleihen (A5, A6) ----------
const RENTAL_TABS = {
  offen: ['reserviert', 'aktiv', 'ueberfaellig'], reserviert: ['reserviert'], aktiv: ['aktiv'], ueberfaellig: ['ueberfaellig'],
  abgeschlossen: ['zurueckgegeben', 'storniert'], alle: null,
};
const TAB_LABELS = { offen: 'Offen', reserviert: 'Reserviert', aktiv: 'Aktiv', ueberfaellig: 'Überfällig', abgeschlossen: 'Abgeschlossen', alle: 'Alle' };

async function renderRentals(app) {
  state.rentals = await api('GET', '/api/rentals');
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Ausleihen &amp; Reservierungen</h1><button class="btn btn-success" data-action="new-rental">+ Neue Ausleihe</button></div>
    <div class="tabs" id="rental-tabs"></div>
    <div class="toolbar"><input id="rental-q" type="search" placeholder="Suche Person oder Ski …" value="${esc(state.rentalQ)}"></div>
    <div class="table-wrap"><table>
      <thead><tr><th>#</th><th>Ski</th><th>Person</th><th>Beginn</th><th>Ende (geplant)</th><th>Dauer</th><th>Status</th><th class="num">Preis</th><th></th></tr></thead>
      <tbody id="rental-rows"></tbody>
    </table></div>
  </section>`;
  const update = () => {
    const q = (state.rentalQ = $('#rental-q').value).toLowerCase();
    $('#rental-tabs').innerHTML = Object.entries(TAB_LABELS).map(([k, l]) => {
      const n = state.rentals.filter((r) => !RENTAL_TABS[k] || RENTAL_TABS[k].includes(r.status)).length;
      return `<button class="tab ${state.rentalTab === k ? 'active' : ''}" data-tab="${k}">${l} (${n})</button>`;
    }).join('');
    const allowed = RENTAL_TABS[state.rentalTab];
    const rows = state.rentals.filter((r) => (!allowed || allowed.includes(r.status)) && (!q || `${r.personName} ${r.skiLabel}`.toLowerCase().includes(q)));
    $('#rental-rows').innerHTML = rows.map((r) => `<tr>
        <td class="muted">${r.id}</td>
        <td>${esc(r.skiLabel)}<div class="small muted">${LABELS.skiType[r.skiType] || ''}</div></td>
        <td>${esc(r.personName)}</td>
        <td class="small">${dt(r.start)}</td>
        <td class="small">${dt(r.plannedEnd)}${r.returnedAt ? `<div class="muted">zurück: ${dt(r.returnedAt)}</div>` : ''}</td>
        <td class="small">${duration(r.durationHours)}</td>
        <td>${badge(r.status)}</td>
        <td class="num">${r.finalPriceCents != null
          ? `<b>${money(r.finalPriceCents)}</b><div class="small muted">geplant ${money(r.plannedPriceCents)}</div>`
          : r.status === 'storniert' ? `<s class="muted">${money(r.plannedPriceCents)}</s>` : money(r.plannedPriceCents)}</td>
        <td><div class="actions">
          ${['aktiv', 'ueberfaellig'].includes(r.status) ? `<button class="btn btn-sm btn-success" data-action="return" data-id="${r.id}">Rückgabe</button>` : ''}
          ${r.status === 'reserviert' ? `<button class="btn btn-sm btn-ghost" data-action="cancel" data-id="${r.id}">Stornieren</button>` : ''}
        </div></td>
      </tr>`).join('') || `<tr><td colspan="9" class="empty">Keine Ausleihen in dieser Ansicht.</td></tr>`;
  };
  $('#rental-tabs').addEventListener('click', (e) => {
    const t = e.target.closest('[data-tab]');
    if (t) { state.rentalTab = t.dataset.tab; update(); }
  });
  $('#rental-q').addEventListener('input', update);
  update();
}

// ---------- Eingabe Verleihdauer + Verleihbeginn (B1) ----------
const DURATION_PRESETS = [[1, '1 Std.'], [2, '2 Std.'], [3, '3 Std.'], [4, 'Halbtag (4 Std.)'], [24, '1 Tag'], [48, '2 Tage'], [72, '3 Tage'], [168, '1 Woche']];
const LIFT_CLOSE = { h: 16, m: 30 };

function durationHtml(hours, { lift = false } = {}) {
  return `<div class="presets">${DURATION_PRESETS.map(([h, l]) => `<button type="button" class="preset" data-h="${h}">${l}</button>`).join('')}
      ${lift ? '<button type="button" class="preset" data-lift>bis Liftschluss (16:30)</button>' : ''}</div>
    <div class="dur">
      <input name="days" type="number" min="0" max="30" value="${Math.floor(hours / 24)}" aria-label="Tage"><span>Tage</span>
      <input name="hrs" type="number" min="0" max="23" value="${hours % 24}" aria-label="Stunden"><span>Std.</span>
    </div>`;
}

// Verbindet Schnellwahl und Tage/Stunden-Felder; liefert hours(). getStart wird für „bis Liftschluss" gebraucht.
function bindDuration(root, onChange, getStart) {
  const days = $('[name=days]', root), hrs = $('[name=hrs]', root);
  const hours = () => Math.max(0, (parseInt(days.value, 10) || 0) * 24 + (parseInt(hrs.value, 10) || 0));
  const mark = () => $$('.preset[data-h]', root).forEach((b) => b.classList.toggle('active', Number(b.dataset.h) === hours()));
  const set = (h) => { days.value = Math.floor(h / 24); hrs.value = h % 24; mark(); onChange(); };
  root.addEventListener('click', (e) => {
    const b = e.target.closest('.preset');
    if (!b) return;
    if (!b.hasAttribute('data-lift')) return set(Number(b.dataset.h));
    const s = getStart?.();
    if (!s) return;
    const end = new Date(s);
    end.setHours(LIFT_CLOSE.h, LIFT_CLOSE.m, 0, 0);
    if (end <= s) end.setDate(end.getDate() + 1); // nach Liftschluss → nächster Tag
    set(Math.ceil((end - s) / 3600e3));
  });
  [days, hrs].forEach((el) => {
    el.addEventListener('input', () => { mark(); onChange(); });
    el.addEventListener('change', () => set(hours())); // normalisiert z. B. 0 Tage 26 Std. → 1 Tag 2 Std.
  });
  mark();
  return { hours };
}

function dateInput(d) {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

async function openRentalForm({ personId, skiId } = {}) {
  const persons = await api('GET', '/api/persons');
  if (!persons.length) { toast('Bitte zuerst eine Person anlegen.', 'err'); return; }
  const tomorrow = new Date();
  tomorrow.setDate(tomorrow.getDate() + 1);
  const hourOptions = Array.from({ length: 24 }, (_, h) => {
    const v = `${String(h).padStart(2, '0')}:00`;
    return `<option value="${v}" ${h === 9 ? 'selected' : ''}>${v} Uhr</option>`;
  }).join('');

  modal.open({
    title: 'Neue Ausleihe',
    submitLabel: 'Ausleihe anlegen',
    submitClass: 'btn-success',
    body: `<div class="field"><label>Person <span class="req">*</span></label>
        <select name="personId">${persons.map((p) => `<option value="${p.id}" ${p.id === personId ? 'selected' : ''}>
          ${esc(p.lastName)}, ${esc(p.firstName)} – ${LABELS.docType[p.idDocType]} ${esc(p.idDocNumber)}${p.document ? '' : ' (Scan fehlt)'}</option>`).join('')}</select>
        <div class="hint" id="doc-hint"></div></div>
      <div class="field"><label>Verleihbeginn <span class="req">*</span></label>
        <div class="seg" id="start-mode">
          <button type="button" data-mode="now" class="active">Sofort</button>
          <button type="button" data-mode="later">Später (Reservierung)</button>
        </div>
        <div class="inline" id="start-later" hidden>
          <input name="startDate" type="date" min="${dateInput(new Date())}" value="${dateInput(tomorrow)}" aria-label="Datum">
          <select name="startHour" aria-label="Uhrzeit">${hourOptions}</select>
        </div>
      </div>
      <div class="field" id="duration"><label>Verleihdauer <span class="req">*</span></label>
        ${durationHtml(24, { lift: true })}
      </div>
      <div class="period" id="period"></div>
      <div class="grid-2">
        <div class="field"><label>Ski-Typ (Filter)</label><select name="typeFilter"><option value="">Alle Typen</option>${options(LABELS.skiType)}</select></div>
        <div class="field"><label>Ski <span class="req">*</span></label><select name="skiId"><option value="">lädt …</option></select>
          <div class="hint" id="ski-hint"></div></div>
      </div>
      <div id="quote"></div>`,
    onOpen: (body) => {
      const f = (n) => $(`[name=${n}]`, body);
      let mode = 'now';
      const getStart = () => {
        if (mode === 'now') { const d = new Date(); d.setSeconds(0, 0); return d; }
        return f('startDate').value ? new Date(`${f('startDate').value}T${f('startHour').value}:00`) : null;
      };
      const startISO = () => getStart()?.toISOString() || '';
      let available = [];
      let wantedSki = skiId;

      const docHint = () => {
        const p = persons.find((x) => x.id === Number(f('personId').value));
        $('#doc-hint').innerHTML = p && !p.document ? '⚠️ Für diese Person ist noch kein Ausweis-Scan hinterlegt (unter „Personen" hochladbar).' : '';
      };
      const fillSkis = () => {
        const type = f('typeFilter').value;
        const list = available.filter((k) => !type || k.type === type);
        const current = wantedSki || Number(f('skiId').value);
        f('skiId').innerHTML = list.length
          ? list.map((k) => `<option value="${k.id}" ${k.id === current ? 'selected' : ''}>${esc(k.brand)} ${esc(k.model)} · ${k.lengthCm} cm · ${LABELS.skiType[k.type]}</option>`).join('')
          : '<option value="">Kein Ski frei</option>';
        $('#ski-hint').textContent = `${list.length} Ski im Zeitraum frei`;
        if (wantedSki && !available.some((k) => k.id === wantedSki)) $('#ski-hint').textContent = '⚠️ Der gewählte Ski ist im Zeitraum belegt.';
        wantedSki = null;
        updateQuote();
      };
      const updateQuote = debounce(async () => {
        const h = hours(), id = f('skiId').value;
        if (!h || !id) { $('#quote').innerHTML = ''; return; }
        try { $('#quote').innerHTML = quoteHtml(await api('GET', `/api/rentals/quote?skiId=${id}&hours=${h}`)); modal.error(''); }
        catch (err) { $('#quote').innerHTML = ''; modal.error(err.message); }
      }, 150);
      const showPeriod = () => {
        const h = hours(), s = getStart();
        if (!s) { $('#period').innerHTML = '<span class="warn-text">Bitte Datum für den Beginn wählen.</span>'; return false; }
        if (!h) { $('#period').innerHTML = '<span class="warn-text">Bitte eine Dauer von mindestens 1 Stunde wählen.</span>'; return false; }
        if (h > 720) { $('#period').innerHTML = '<span class="warn-text">Maximal 30 Tage möglich.</span>'; return false; }
        $('#period').innerHTML = `<div><span class="muted">Ab</span> <b>${mode === 'now' ? 'sofort' : dt(s)}</b></div>
          <div class="arrow">→</div>
          <div><span class="muted">Rückgabe bis</span> <b>${dt(new Date(s.getTime() + h * 3600e3))}</b></div>
          <div class="muted">${duration(h)}</div>`;
        return true;
      };
      const loadSkis = debounce(async () => {
        if (!showPeriod()) { $('#quote').innerHTML = ''; return; }
        try { available = await api('GET', `/api/skis/available?start=${encodeURIComponent(startISO())}&hours=${hours()}`); fillSkis(); }
        catch (err) { modal.error(err.message); }
      });
      const { hours } = bindDuration($('#duration', body), loadSkis, getStart);

      $('#start-mode', body).addEventListener('click', (e) => {
        const b = e.target.closest('[data-mode]');
        if (!b) return;
        mode = b.dataset.mode;
        $$('#start-mode button', body).forEach((x) => x.classList.toggle('active', x === b));
        $('#start-later', body).hidden = mode === 'now';
        loadSkis();
      });
      ['startDate', 'startHour'].forEach((n) => f(n).addEventListener('input', loadSkis));
      f('typeFilter').addEventListener('input', fillSkis);
      f('skiId').addEventListener('input', updateQuote);
      f('personId').addEventListener('input', docHint);
      docHint();
      loadSkis();

      modal.onSubmit = async () => {
        if (!showPeriod()) throw new Error('Bitte Verleihbeginn und -dauer prüfen.');
        if (!f('skiId').value) throw new Error('Bitte einen freien Ski auswählen.');
        const r = await api('POST', '/api/rentals', {
          personId: Number(f('personId').value), skiId: Number(f('skiId').value), start: startISO(), durationHours: hours(),
        });
        toast(`Ausleihe #${r.id} angelegt – ${money(r.plannedPriceCents)}`);
      };
    },
  });
}

async function openReturnDialog(id) {
  const { rental: r, quote: q } = await api('GET', `/api/rentals/${id}/return-quote`);
  modal.open({
    title: `Rückgabe Ausleihe #${r.id}`,
    submitLabel: 'Rückgabe buchen',
    submitClass: 'btn-success',
    body: `<p><b>${esc(r.skiLabel)}</b><br>${esc(r.personName)}</p>
      <p class="small">Beginn: ${dt(r.start)}<br>Geplantes Ende: ${dt(r.plannedEnd)} · geplanter Preis ${money(r.plannedPriceCents)}</p>
      ${r.status === 'ueberfaellig' ? '<p class="form-error" style="margin:0 0 1rem">Die Ausleihe ist überfällig.</p>' : ''}
      <p class="small muted">Abgerechnet wird die tatsächliche Dauer, jede angefangene Stunde zählt.</p>
      ${quoteHtml(q)}`,
    onSubmit: async () => {
      const done = await api('POST', `/api/rentals/${id}/return`);
      toast(`Rückgabe gebucht – Endpreis ${money(done.finalPriceCents)}`);
    },
  });
}

// ---------- Preise (B1–B6) ----------
async function renderPrices(app) {
  const tariffs = await api('GET', '/api/prices');
  const euro = (c) => (c / 100).toFixed(2);
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Preise</h1><span class="muted">stundenweise Abrechnung mit Tagesdeckel</span></div>
    <p>Preis = <b>volle Tage × Tagespreis</b> + <b>Reststunden × Stundenpreis</b> (höchstens ein Tagespreis).
       Bei der Rückgabe wird nach tatsächlicher Dauer abgerechnet – jede angefangene Stunde zählt.
       Änderungen gelten nur für <b>neue</b> Ausleihen; bestehende behalten ihren Tarif.</p>
    <div class="table-wrap"><table>
      <thead><tr><th>Ski-Typ</th><th>Stundenpreis (€)</th><th>Tagespreis (€)</th><th></th></tr></thead>
      <tbody>${tariffs.map((t) => `<tr data-type="${t.skiType}">
          <td><b>${LABELS.skiType[t.skiType]}</b></td>
          <td><input type="number" step="0.5" min="0.5" name="hourly" value="${euro(t.hourlyCents)}" style="max-width:130px"></td>
          <td><input type="number" step="0.5" min="0.5" name="daily" value="${euro(t.dailyCents)}" style="max-width:130px"></td>
          <td><button class="btn btn-sm" data-action="save-price">Speichern</button></td>
        </tr>`).join('')}</tbody>
    </table></div>
    <h2>Preisrechner</h2>
    <div class="grid-2" style="max-width:640px">
      <div class="field"><label>Ski-Typ</label><select id="calc-type">${options(LABELS.skiType)}</select></div>
    </div>
    <div class="field" id="calc-duration" style="max-width:640px"><label>Dauer</label>${durationHtml(26)}</div>
    <div id="calc-result" style="max-width:640px"></div>
  </section>`;
  const calc = debounce(async () => {
    const h = dur.hours();
    if (!h) { $('#calc-result').innerHTML = '<p class="muted">Bitte eine Dauer von mindestens 1 Stunde wählen.</p>'; return; }
    try { $('#calc-result').innerHTML = quoteHtml(await api('GET', `/api/rentals/quote?type=${$('#calc-type').value}&hours=${h}`)); }
    catch (err) { $('#calc-result').innerHTML = `<p class="form-error" style="margin:0">${esc(err.message)}</p>`; }
  }, 150);
  const dur = bindDuration($('#calc-duration'), calc);
  $('#calc-type').addEventListener('input', calc);
  calc();
}

async function savePrice(btn) {
  const row = btn.closest('tr');
  const cents = (n) => Math.round(Number($(`[name=${n}]`, row).value) * 100);
  try {
    await api('PUT', `/api/prices/${row.dataset.type}`, { hourlyCents: cents('hourly'), dailyCents: cents('daily') });
    toast(`Tarif ${LABELS.skiType[row.dataset.type]} gespeichert`);
    $('#calc-type').dispatchEvent(new Event('input'));
  } catch (err) {
    toast(err.message, 'err');
  }
}

// ---------- Zentrale Klick-Aktionen ----------
document.addEventListener('click', async (e) => {
  const el = e.target.closest('[data-action]');
  if (!el || el.disabled) return;
  const id = Number(el.dataset.id);
  try {
    switch (el.dataset.action) {
      case 'new-ski': return openSkiForm();
      case 'edit-ski': return openSkiForm(state.skis.find((k) => k.id === id));
      case 'delete-ski': {
        const k = state.skis.find((x) => x.id === id);
        return confirmDialog('Ski löschen?', `<b>${esc(k.brand)} ${esc(k.model)}</b> (${k.lengthCm} cm) wird endgültig gelöscht.`, 'Löschen', 'btn-danger',
          async () => { await api('DELETE', `/api/skis/${id}`); toast('Ski gelöscht'); });
      }
      case 'rent-ski': return openRentalForm({ skiId: id });
      case 'new-person': return openPersonForm();
      case 'edit-person': return openPersonForm(state.persons.find((p) => p.id === id));
      case 'delete-person': {
        const p = state.persons.find((x) => x.id === id);
        return confirmDialog('Person löschen?', `<b>${esc(p.firstName)} ${esc(p.lastName)}</b> inkl. Ausweis-Scan wird endgültig gelöscht.`, 'Löschen', 'btn-danger',
          async () => { await api('DELETE', `/api/persons/${id}`); toast('Person gelöscht'); });
      }
      case 'rent-person': return openRentalForm({ personId: id });
      case 'new-rental': return openRentalForm();
      case 'return': return openReturnDialog(id);
      case 'cancel':
        return confirmDialog('Reservierung stornieren?', `Reservierung #${id} wird storniert, der Ski wird wieder frei.`, 'Stornieren', 'btn-danger',
          async () => { await api('POST', `/api/rentals/${id}/cancel`); toast('Reservierung storniert'); });
      case 'save-price': return savePrice(el);
    }
  } catch (err) {
    toast(err.message, 'err');
  }
});

render();
