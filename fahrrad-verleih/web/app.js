'use strict';
// Generisches Verleih-Frontend (C2): Texte, Typen, Felder und Zubehör kommen aus /api/plugins.
// Hier steht kein Fahrrad-Wissen – siehe CLAUDE.md, Plugin-Regel.

// ---------- Kern-Texte (nicht plugin-spezifisch) ----------
const LABELS = {
  condition: { neu: 'Neu', gut: 'Gut', gebraucht: 'Gebraucht' },
  docType: { personalausweis: 'Personalausweis', reisepass: 'Reisepass', fuehrerschein: 'Führerschein' },
  status: {
    verfuegbar: 'Verfügbar', ausgeliehen: 'Ausgeliehen', ueberfaellig: 'Überfällig', reserviert: 'Reserviert',
    aktiv: 'Aktiv', zurueckgegeben: 'Zurückgegeben', storniert: 'Storniert',
  },
};

// ---------- Plugin-Metadaten ----------
let META = { appName: 'Verleih', plugins: [] };
const plugin = (cat) => META.plugins.find((p) => p.category === cat);
const kindOf = (cat, kind) => plugin(cat)?.kinds.find((k) => k.key === kind);
const kindLabel = (cat, kind) => kindOf(cat, kind)?.label || kind;
const applies = (def, kind) => !def.onlyKinds?.length || def.onlyKinds.includes(kind);
// Begriffe: bei genau einem Plugin dessen Namen, sonst neutral "Artikel".
const T = {
  get one() { return META.plugins.length === 1 ? META.plugins[0].name : 'Artikel'; },
  get many() { return META.plugins.length === 1 ? META.plugins[0].namePlural : 'Artikel'; },
};
const allKinds = () => META.plugins.flatMap((p) => p.kinds.map((k) => ({ ...k, category: p.category, value: `${p.category}/${k.key}` })));
const kindOptions = (selected) => allKinds().map((k) =>
  `<option value="${k.value}" ${k.value === selected ? 'selected' : ''}>${esc(k.label)}${META.plugins.length > 1 ? ` (${esc(plugin(k.category).name)})` : ''}</option>`).join('');

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

// Preis mit Rechenweg inkl. Zubehör (K8, C6) – alle Beträge vom Server.
function quoteHtml(q) {
  const lines = [];
  if (q.fullDays) lines.push(`${q.fullDays} × Tagespreis ${money(q.dailyCents)} = ${money(q.daysCents)}`);
  if (q.restHours) {
    lines.push(q.restCapped
      ? `${q.restHours} Std. × ${money(q.hourlyCents)} → gedeckelt auf Tagespreis ${money(q.restCents)}`
      : `${q.restHours} Std. × ${money(q.hourlyCents)} = ${money(q.restCents)}`);
  }
  q.extras.forEach((e) => lines.push(`+ ${esc(e.label)} (pauschal) ${money(e.priceCents)}`));
  return `<div class="quote-box">
    <div class="muted small">Preis für ${duration(q.hours)}${q.extras.length ? ' inkl. Zubehör' : ''}</div>
    <div class="total">${money(q.totalCents)}</div>
    <div class="small">${lines.join('<br>')}</div>
  </div>`;
}

// Attribut-Anzeige gemäß Felddefinition (mit Einheit).
function attrText(item, f) {
  const v = item.attributes?.[f.key];
  if (!v) return '<span class="muted">–</span>';
  return esc(f.unit === 'Zoll' ? `${v}"` : f.unit ? `${v} ${f.unit}` : v);
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
$$('[data-close]').forEach((b) => b.addEventListener('click', () => modal.close()));

function confirmDialog(title, text, label, cls, action) {
  modal.open({ title, body: `<p>${text}</p>`, submitLabel: label, submitClass: cls, onSubmit: action });
}

// ---------- Router ----------
const state = { items: [], persons: [], rentals: [], itemFilter: { q: '', kind: '', status: '' }, personQ: '', rentalTab: 'offen', rentalQ: '' };
const pages = { dashboard: renderDashboard, items: renderItems, persons: renderPersons, rentals: renderRentals, prices: renderPrices };

async function render() {
  const page = location.hash.slice(1) || 'dashboard';
  const fn = pages[page] || renderDashboard;
  $$('.topbar nav a').forEach((a) => a.classList.toggle('active', a.dataset.page === page));
  document.body.classList.remove('nav-open');
  try {
    await fn($('#app'));
  } catch (err) {
    $('#app').innerHTML = `<section class="panel"><p class="form-error">Daten konnten nicht geladen werden: ${esc(err.message)}</p></section>`;
  }
}
window.addEventListener('hashchange', render);

// ---------- Dashboard (K1, C5) ----------
async function renderDashboard(app) {
  const d = await api('GET', '/api/dashboard');
  const item = (r, action) => `<li>
      <div><b>${esc(r.itemLabel)}</b><div class="small muted">${esc(r.personName)} · ${action === 'return' ? 'bis ' + dt(r.plannedEnd) : 'ab ' + dt(r.start)}</div></div>
      ${action === 'return' ? `<button class="btn btn-sm btn-success" data-action="return" data-id="${r.id}">Rückgabe</button>` : badge(r.status)}
    </li>`;
  const list = (title, rows, action, cls = '') => `<div class="list-card ${cls}">
      <h3>${title} <span class="muted">(${rows.length})</span></h3>
      <ul>${rows.map((r) => item(r, action)).join('') || '<li class="muted small">Keine Einträge</li>'}</ul>
    </div>`;
  const kinds = d.byKind.filter((k) => k.total > 0).map((k) => `<div class="kind-card">
      <div class="row"><span>${esc(k.label)}</span><span class="count">${k.available} / ${k.total}</span></div>
      <div class="bar ${k.available === 0 ? 'low' : ''}"><span style="width:${k.total ? (100 * k.available) / k.total : 0}%"></span></div>
    </div>`).join('');

  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Dashboard</h1><span class="muted">${dt(new Date())}</span></div>
    <div class="tiles">
      <a class="tile t1" href="#items"><div class="label">Verfügbare ${esc(T.many)}</div><div class="value">${d.availableItems}</div></a>
      <a class="tile t2" href="#rentals"><div class="label">Ausgeliehene ${esc(T.many)}</div><div class="value">${d.rentedItems}</div></a>
      <a class="tile t3" href="#items"><div class="label">Gesamt ${esc(T.many)}</div><div class="value">${d.totalItems}</div></a>
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
      <a class="btn" href="#items">${esc(T.many)} verwalten</a>
      <a class="btn" href="#persons">Personen verwalten</a>
      <button class="btn btn-success" data-action="new-rental">Neue Ausleihe</button>
    </div>
    <h2>Verfügbar nach Typ</h2>
    <div class="kind-grid">${kinds || '<p class="muted">Noch keine Artikel.</p>'}</div>
    <h2>Heute im Blick</h2>
    <div class="lists">
      ${list('⚠️ Überfällig', d.overdue, 'return', d.overdue.length ? 'danger' : '')}
      ${list('Heute fällig', d.dueToday, 'return')}
      ${list('Nächste Reservierungen', d.upcomingReservations, 'cancel')}
    </div>
  </section>`;
}

// ---------- Item-Verwaltung (K2, C1, C2) ----------
async function renderItems(app) {
  state.items = await api('GET', '/api/items');
  const f = state.itemFilter;
  const cols = META.plugins.flatMap((p) => p.fields.filter((x) => x.inTable).map((x) => ({ ...x, category: p.category })));
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>${esc(T.one)}-Verwaltung</h1><button class="btn btn-success" data-action="new-item">+ ${esc(T.one)} anlegen</button></div>
    <div class="toolbar">
      <input id="item-q" type="search" placeholder="Suche Marke / Modell …" value="${esc(f.q)}">
      <select id="item-kind"><option value="">Alle Typen</option>${kindOptions(f.kind)}</select>
      <select id="item-status"><option value="">Alle Status</option>${options({ verfuegbar: 'Verfügbar', ausgeliehen: 'Ausgeliehen', ueberfaellig: 'Überfällig' }, f.status)}</select>
    </div>
    <div class="table-wrap"><table>
      <thead><tr><th>#</th><th>${esc(T.one)}</th><th>Typ</th>${cols.map((c) => `<th>${esc(c.label)}</th>`).join('')}<th>Zustand</th><th>Status</th><th>Nächste Reservierung</th><th></th></tr></thead>
      <tbody id="item-rows"></tbody>
    </table></div>
  </section>`;
  const update = () => {
    f.q = $('#item-q').value.toLowerCase(); f.kind = $('#item-kind').value; f.status = $('#item-status').value;
    const rows = state.items.filter((it) =>
      (!f.q || `${it.brand} ${it.model}`.toLowerCase().includes(f.q)) && (!f.kind || `${it.category}/${it.kind}` === f.kind) && (!f.status || it.status === f.status));
    $('#item-rows').innerHTML = rows.map((it) => `<tr>
        <td class="muted">${it.id}</td>
        <td><b>${esc(it.brand)}</b> ${esc(it.model)}${it.notes ? `<div class="small muted">${esc(it.notes)}</div>` : ''}</td>
        <td>${esc(kindLabel(it.category, it.kind))}</td>
        ${cols.map((c) => `<td>${c.category === it.category ? attrText(it, c) : '<span class="muted">–</span>'}</td>`).join('')}
        <td>${LABELS.condition[it.condition]}</td>
        <td>${badge(it.status)}</td>
        <td class="small">${it.nextReservation ? dt(it.nextReservation) : '<span class="muted">–</span>'}</td>
        <td><div class="actions">
          <button class="btn btn-sm btn-success" data-action="rent-item" data-id="${it.id}" ${it.status !== 'verfuegbar' ? 'disabled title="Gerade nicht verfügbar"' : ''}>Verleihen</button>
          <button class="btn btn-sm btn-ghost" data-action="edit-item" data-id="${it.id}">Bearbeiten</button>
          <button class="btn btn-sm btn-ghost" data-action="delete-item" data-id="${it.id}" title="Löschen">🗑</button>
        </div></td>
      </tr>`).join('') || `<tr><td colspan="${7 + cols.length}" class="empty">Nichts gefunden.</td></tr>`;
  };
  ['#item-q', '#item-kind', '#item-status'].forEach((s) => $(s).addEventListener('input', update));
  update();
}

// Eingabefeld aus Plugin-Felddefinition erzeugen (C2).
function fieldInput(f, value) {
  const req = f.required ? ' <span class="req">*</span>' : '';
  const unit = f.unit ? ` (${esc(f.unit)})` : '';
  let input;
  if (f.type === 'select') {
    input = `<select name="attr:${f.key}"><option value="">– bitte wählen –</option>${f.options.map((o) => `<option ${o === value ? 'selected' : ''}>${esc(o)}</option>`).join('')}</select>`;
  } else if (f.type === 'number') {
    input = `<input name="attr:${f.key}" type="number" min="${f.min}" max="${f.max}" value="${esc(value)}">`;
  } else {
    input = `<input name="attr:${f.key}" value="${esc(value)}">`;
  }
  const hint = f.type === 'number' ? `<div class="hint">${f.min}–${f.max}${f.unit ? ' ' + esc(f.unit) : ''}</div>` : '';
  return `<div class="field" data-field="${f.key}"><label>${esc(f.label)}${unit}${req}</label>${input}${hint}</div>`;
}

function openItemForm(existing) {
  const first = allKinds()[0];
  const it = existing || { category: first.category, kind: first.key, brand: '', model: '', condition: 'gut', attributes: {}, notes: '' };
  modal.open({
    title: existing ? `${T.one} #${existing.id} bearbeiten` : `${T.one} anlegen`,
    body: `<div class="grid-2">
        <div class="field"><label>Typ <span class="req">*</span></label><select name="kind">${kindOptions(`${it.category}/${it.kind}`)}</select></div>
        <div class="field"><label>Zustand <span class="req">*</span></label><select name="condition">${options(LABELS.condition, it.condition)}</select></div>
        <div class="field"><label>Marke <span class="req">*</span></label><input name="brand" value="${esc(it.brand)}"></div>
        <div class="field"><label>Modell <span class="req">*</span></label><input name="model" value="${esc(it.model)}"></div>
      </div>
      <div class="grid-2" id="plugin-fields"></div>
      <div class="field"><label>Notiz</label><textarea name="notes" rows="2">${esc(it.notes)}</textarea></div>`,
    onOpen: (body) => {
      const values = { ...it.attributes };
      const renderFields = () => {
        $$('[name^="attr:"]', body).forEach((el) => { values[el.name.slice(5)] = el.value; });
        const [cat, kind] = $('[name=kind]', body).value.split('/');
        $('#plugin-fields', body).innerHTML = plugin(cat).fields.filter((f) => applies(f, kind)).map((f) => fieldInput(f, values[f.key] || '')).join('');
      };
      $('[name=kind]', body).addEventListener('input', renderFields);
      renderFields();
    },
    onSubmit: async (body) => {
      const v = (n) => $(`[name="${n}"]`, body).value;
      const [category, kind] = v('kind').split('/');
      const attributes = {};
      $$('[name^="attr:"]', body).forEach((el) => { if (el.value.trim()) attributes[el.name.slice(5)] = el.value.trim(); });
      const data = { category, kind, brand: v('brand'), model: v('model'), condition: v('condition'), notes: v('notes'), attributes };
      if (existing) await api('PUT', `/api/items/${existing.id}`, data); else await api('POST', '/api/items', data);
      toast(existing ? 'Gespeichert' : `${T.one} angelegt`);
    },
  });
}

// ---------- Personen (K3, K4) ----------
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
      // ID merken: schlägt der Upload fehl, wird beim erneuten Speichern aktualisiert statt doppelt angelegt.
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

// ---------- Ausleihen (K5, K6, C3) ----------
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
    <div class="toolbar"><input id="rental-q" type="search" placeholder="Suche Person oder ${esc(T.one)} …" value="${esc(state.rentalQ)}"></div>
    <div class="table-wrap"><table>
      <thead><tr><th>#</th><th>${esc(T.one)}</th><th>Person</th><th>Beginn</th><th>Ende (geplant)</th><th>Dauer</th><th>Status</th><th class="num">Preis</th><th></th></tr></thead>
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
    const rows = state.rentals.filter((r) => (!allowed || allowed.includes(r.status)) && (!q || `${r.personName} ${r.itemLabel}`.toLowerCase().includes(q)));
    $('#rental-rows').innerHTML = rows.map((r) => `<tr>
        <td class="muted">${r.id}</td>
        <td>${esc(r.itemLabel)}<div class="small muted">${esc(kindLabel(r.tariff.category, r.tariff.kind))}</div>
          ${r.extras.map((e) => `<span class="chip">${esc(e.label)}</span>`).join('')}</td>
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

// Zubehör-Checkboxen für Kategorie/Typ (C3).
function extrasHtml(category, kind, checked = []) {
  const list = (plugin(category)?.extras || []).filter((e) => applies(e, kind));
  if (!list.length) return '<span class="muted small">Kein Zubehör für diesen Typ.</span>';
  return list.map((e) => `<label class="extra"><input type="checkbox" value="${e.key}" ${checked.includes(e.key) ? 'checked' : ''}> ${esc(e.label)} <span class="muted small">+${money(e.priceCents)}</span></label>`).join('');
}
const checkedExtras = (root) => $$('.extras input:checked', root).map((c) => c.value);

// ---------- Eingabe Verleihdauer + Verleihbeginn (K11) ----------
const DURATION_PRESETS = [[1, '1 Std.'], [2, '2 Std.'], [3, '3 Std.'], [4, 'Halbtag (4 Std.)'], [24, '1 Tag'], [48, '2 Tage'], [72, '3 Tage'], [168, '1 Woche']];
const CLOSING_TIME = { h: 19, m: 0 }; // Ladenschluss – Einstellung des Verleihs, nicht des Plugins

function durationHtml(hours, { closing = false } = {}) {
  const p = (n) => String(n).padStart(2, '0');
  return `<div class="presets">${DURATION_PRESETS.map(([h, l]) => `<button type="button" class="preset" data-h="${h}">${l}</button>`).join('')}
      ${closing ? `<button type="button" class="preset" data-closing>bis Ladenschluss (${p(CLOSING_TIME.h)}:${p(CLOSING_TIME.m)})</button>` : ''}</div>
    <div class="dur">
      <input name="days" type="number" min="0" max="30" value="${Math.floor(hours / 24)}" aria-label="Tage"><span>Tage</span>
      <input name="hrs" type="number" min="0" max="23" value="${hours % 24}" aria-label="Stunden"><span>Std.</span>
    </div>`;
}

// Verbindet Schnellwahl und Tage/Stunden-Felder; liefert hours(). getStart wird für „bis Ladenschluss" gebraucht.
function bindDuration(root, onChange, getStart) {
  const days = $('[name=days]', root), hrs = $('[name=hrs]', root);
  const hours = () => Math.max(0, (parseInt(days.value, 10) || 0) * 24 + (parseInt(hrs.value, 10) || 0));
  const mark = () => $$('.preset[data-h]', root).forEach((b) => b.classList.toggle('active', Number(b.dataset.h) === hours()));
  const set = (h) => { days.value = Math.floor(h / 24); hrs.value = h % 24; mark(); onChange(); };
  root.addEventListener('click', (e) => {
    const b = e.target.closest('.preset');
    if (!b) return;
    if (!b.hasAttribute('data-closing')) return set(Number(b.dataset.h));
    const s = getStart?.();
    if (!s) return;
    const end = new Date(s);
    end.setHours(CLOSING_TIME.h, CLOSING_TIME.m, 0, 0);
    if (end <= s) end.setDate(end.getDate() + 1); // nach Ladenschluss → nächster Tag
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

async function openRentalForm({ personId, itemId } = {}) {
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
        ${durationHtml(3, { closing: true })}
      </div>
      <div class="period" id="period"></div>
      <div class="grid-2">
        <div class="field"><label>Typ (Filter)</label><select name="kindFilter"><option value="">Alle Typen</option>${kindOptions()}</select></div>
        <div class="field"><label>${esc(T.one)} <span class="req">*</span></label><select name="itemId"><option value="">lädt …</option></select>
          <div class="hint" id="item-hint"></div></div>
      </div>
      <div class="field"><label>Zubehör</label><div class="extras" id="extras"></div></div>
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
      let wanted = itemId;

      const selectedItem = () => available.find((it) => it.id === Number(f('itemId').value));
      const docHint = () => {
        const p = persons.find((x) => x.id === Number(f('personId').value));
        $('#doc-hint').innerHTML = p && !p.document ? '⚠️ Für diese Person ist noch kein Ausweis-Scan hinterlegt (unter „Personen" hochladbar).' : '';
      };
      const renderExtras = () => {
        const it = selectedItem();
        const keep = checkedExtras(body);
        $('#extras').innerHTML = it ? extrasHtml(it.category, it.kind, keep) : '';
        updateQuote();
      };
      const fillItems = () => {
        const kf = f('kindFilter').value;
        const list = available.filter((it) => !kf || `${it.category}/${it.kind}` === kf);
        const current = wanted || Number(f('itemId').value);
        f('itemId').innerHTML = list.length
          ? list.map((it) => `<option value="${it.id}" ${it.id === current ? 'selected' : ''}>${esc(it.label)} · ${esc(kindLabel(it.category, it.kind))}</option>`).join('')
          : '<option value="">Nichts frei</option>';
        $('#item-hint').textContent = `${list.length} im Zeitraum frei`;
        if (wanted && !available.some((it) => it.id === wanted)) $('#item-hint').textContent = '⚠️ Der gewählte Artikel ist im Zeitraum belegt.';
        wanted = null;
        renderExtras();
      };
      const updateQuote = debounce(async () => {
        const h = hours(), id = f('itemId').value;
        if (!h || !id) { $('#quote').innerHTML = ''; return; }
        try {
          $('#quote').innerHTML = quoteHtml(await api('GET', `/api/rentals/quote?itemId=${id}&hours=${h}&extras=${checkedExtras(body).join(',')}`));
          modal.error('');
        } catch (err) { $('#quote').innerHTML = ''; modal.error(err.message); }
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
      const loadItems = debounce(async () => {
        if (!showPeriod()) { $('#quote').innerHTML = ''; return; }
        try { available = await api('GET', `/api/items/available?start=${encodeURIComponent(startISO())}&hours=${hours()}`); fillItems(); }
        catch (err) { modal.error(err.message); }
      });
      const { hours } = bindDuration($('#duration', body), loadItems, getStart);

      $('#start-mode', body).addEventListener('click', (e) => {
        const b = e.target.closest('[data-mode]');
        if (!b) return;
        mode = b.dataset.mode;
        $$('#start-mode button', body).forEach((x) => x.classList.toggle('active', x === b));
        $('#start-later', body).hidden = mode === 'now';
        loadItems();
      });
      ['startDate', 'startHour'].forEach((n) => f(n).addEventListener('input', loadItems));
      f('kindFilter').addEventListener('input', fillItems);
      f('itemId').addEventListener('input', renderExtras);
      $('#extras').addEventListener('change', updateQuote);
      f('personId').addEventListener('input', docHint);
      docHint();
      loadItems();

      modal.onSubmit = async () => {
        if (!showPeriod()) throw new Error('Bitte Verleihbeginn und -dauer prüfen.');
        if (!f('itemId').value) throw new Error('Bitte einen freien Artikel auswählen.');
        const r = await api('POST', '/api/rentals', {
          personId: Number(f('personId').value), itemId: Number(f('itemId').value), start: startISO(), durationHours: hours(), extras: checkedExtras(body),
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
    body: `<p><b>${esc(r.itemLabel)}</b><br>${esc(r.personName)}</p>
      <p class="small">Beginn: ${dt(r.start)}<br>Geplantes Ende: ${dt(r.plannedEnd)} · geplanter Preis ${money(r.plannedPriceCents)}</p>
      ${r.extras.length ? `<p class="small">Zubehör zurücknehmen: ${r.extras.map((e) => `<span class="chip">${esc(e.label)}</span>`).join('')}</p>` : ''}
      ${r.status === 'ueberfaellig' ? '<p class="form-error" style="margin:0 0 1rem">Die Ausleihe ist überfällig.</p>' : ''}
      <p class="small muted">Abgerechnet wird die tatsächliche Dauer, jede angefangene Stunde zählt.</p>
      ${quoteHtml(q)}`,
    onSubmit: async () => {
      const done = await api('POST', `/api/rentals/${id}/return`);
      toast(`Rückgabe gebucht – Endpreis ${money(done.finalPriceCents)}`);
    },
  });
}

// ---------- Preise (K8, K10, C3, C6) ----------
async function renderPrices(app) {
  const tariffs = await api('GET', '/api/prices');
  const euro = (c) => (c / 100).toFixed(2);
  const extras = META.plugins.flatMap((p) => p.extras.map((e) => ({ ...e, category: p.category })));
  app.innerHTML = `<section class="panel">
    <div class="panel-head"><h1>Preise</h1><span class="muted">stundenweise Abrechnung mit Tagesdeckel</span></div>
    <p>Miete = <b>volle Tage × Tagespreis</b> + <b>Reststunden × Stundenpreis</b> (höchstens ein Tagespreis), zzgl. Zubehör pauschal pro Ausleihe.
       Bei der Rückgabe wird nach tatsächlicher Dauer abgerechnet – jede angefangene Stunde zählt.
       Änderungen gelten nur für <b>neue</b> Ausleihen.</p>
    <div class="table-wrap"><table>
      <thead><tr><th>Typ</th><th>Stundenpreis (€)</th><th>Tagespreis (€)</th><th></th></tr></thead>
      <tbody>${tariffs.map((t) => `<tr data-category="${t.category}" data-kind="${t.kind}">
          <td><b>${esc(kindLabel(t.category, t.kind))}</b></td>
          <td><input type="number" step="0.5" min="0.5" name="hourly" value="${euro(t.hourlyCents)}" style="max-width:130px"></td>
          <td><input type="number" step="0.5" min="0.5" name="daily" value="${euro(t.dailyCents)}" style="max-width:130px"></td>
          <td><button class="btn btn-sm" data-action="save-price">Speichern</button></td>
        </tr>`).join('')}</tbody>
    </table></div>
    <h2>Zubehör</h2>
    <div class="extras">${extras.map((e) => `<span class="extra">${esc(e.label)} <b>${money(e.priceCents)}</b>
      ${e.onlyKinds?.length ? `<span class="muted small">(nur ${e.onlyKinds.map((k) => esc(kindLabel(e.category, k))).join(', ')})</span>` : ''}</span>`).join('')}</div>
    <h2>Preisrechner</h2>
    <div class="grid-2" style="max-width:640px">
      <div class="field"><label>Typ</label><select id="calc-kind">${kindOptions()}</select></div>
    </div>
    <div class="field" id="calc-duration" style="max-width:640px"><label>Dauer</label>${durationHtml(3)}</div>
    <div class="field" style="max-width:640px"><label>Zubehör</label><div class="extras" id="calc-extras"></div></div>
    <div id="calc-result" style="max-width:640px"></div>
  </section>`;
  const calc = debounce(async () => {
    const [category, kind] = $('#calc-kind').value.split('/');
    const h = dur.hours();
    if (!h) { $('#calc-result').innerHTML = '<p class="muted">Bitte eine Dauer von mindestens 1 Stunde wählen.</p>'; return; }
    try {
      $('#calc-result').innerHTML = quoteHtml(await api('GET',
        `/api/rentals/quote?category=${category}&kind=${kind}&hours=${h}&extras=${checkedExtras($('#calc-extras').parentElement).join(',')}`));
    } catch (err) { $('#calc-result').innerHTML = `<p class="form-error" style="margin:0">${esc(err.message)}</p>`; }
  }, 150);
  const renderCalcExtras = () => {
    const [category, kind] = $('#calc-kind').value.split('/');
    const keep = checkedExtras($('#calc-extras').parentElement);
    $('#calc-extras').innerHTML = extrasHtml(category, kind, keep);
    calc();
  };
  const dur = bindDuration($('#calc-duration'), calc);
  $('#calc-kind').addEventListener('input', renderCalcExtras);
  $('#calc-extras').addEventListener('change', calc);
  renderCalcExtras();
}

async function savePrice(btn) {
  const row = btn.closest('tr');
  const cents = (n) => Math.round(Number($(`[name=${n}]`, row).value) * 100);
  try {
    await api('PUT', `/api/prices/${row.dataset.category}/${row.dataset.kind}`, { hourlyCents: cents('hourly'), dailyCents: cents('daily') });
    toast(`Tarif ${kindLabel(row.dataset.category, row.dataset.kind)} gespeichert`);
    $('#calc-kind').dispatchEvent(new Event('input'));
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
      case 'new-item': return openItemForm();
      case 'edit-item': return openItemForm(state.items.find((x) => x.id === id));
      case 'delete-item': {
        const it = state.items.find((x) => x.id === id);
        return confirmDialog(`${T.one} löschen?`, `<b>${esc(it.label)}</b> wird endgültig gelöscht.`, 'Löschen', 'btn-danger',
          async () => { await api('DELETE', `/api/items/${id}`); toast('Gelöscht'); });
      }
      case 'rent-item': return openRentalForm({ itemId: id });
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
        return confirmDialog('Reservierung stornieren?', `Reservierung #${id} wird storniert, der Artikel wird wieder frei.`, 'Stornieren', 'btn-danger',
          async () => { await api('POST', `/api/rentals/${id}/cancel`); toast('Reservierung storniert'); });
      case 'save-price': return savePrice(el);
    }
  } catch (err) {
    toast(err.message, 'err');
  }
});

// ---------- Start: Plugin-Metadaten laden, dann rendern ----------
(async () => {
  try {
    META = await api('GET', '/api/plugins');
    document.title = META.appName;
    $('#brand').textContent = META.appName;
    $('#nav-items').textContent = `${T.one}-Verwaltung`;
  } catch (err) {
    toast(`Plugin-Daten nicht ladbar: ${err.message}`, 'err');
  }
  render();
})();
