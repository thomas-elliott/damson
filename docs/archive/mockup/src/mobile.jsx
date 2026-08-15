// Damson — Mobile views. Reuses LICENCES, ASSETS, PROJECTS, COLLECTIONS, etc.
const { useState: useStM, useMemo: useMemoM } = React;

function MIcon({ name, size = 16, stroke = 1.75 }) {
  const C = I[name];
  return C ? <C size={size} stroke={stroke}/> : null;
}

// ── Top app bar ──────────────────────────────────────────────
function MAppBar({ title, count, onMenu, right }) {
  return (
    <div className="m-appbar">
      <button className="m-iconbtn" onClick={onMenu}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M3 6h18M3 12h18M3 18h18"/></svg>
      </button>
      <div className="m-title">{title}{count != null ? <span className="count">{count}</span> : null}</div>
      {right}
    </div>
  );
}

// ── Search box ──────────────────────────────────────────────
function MSearch({ value, onChange, placeholder = 'Search assets, tags, sources…' }) {
  return (
    <div className="m-search">
      <div className="m-searchbox">
        <I.Search size={14} stroke={1.75}/>
        <input value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder}/>
      </div>
    </div>
  );
}

// ── Filter chip strip ──────────────────────────────────────
// Mobile-first: file type is the primary filter axis, plus a Recent toggle.
// Licence has been moved to the drawer for power-users.
const TYPE_LABELS_SHORT = { unitypackage: 'Packages', sprite: 'Sprites', audio: 'Audio', font: 'Fonts' };
function MFilterStrip({ filters, toggleFilter, clearAll, recent, setRecent }) {
  const typeChips = FILETYPE_FILTERS.map(f => ({
    id: f.id, label: TYPE_LABELS_SHORT[f.id] || f.label, count: f.count,
    active: filters.type.has(f.id), icon: f.id,
  }));
  const noFilters = !filters.licence.size && !filters.type.size && !filters.tag.size && !recent;
  return (
    <div className="m-fstrip">
      <button
        className={`m-fchip ${noFilters ? 'active' : ''}`}
        onClick={clearAll}>
        All
      </button>
      <button
        className={`m-fchip ${recent ? 'active' : ''}`}
        onClick={() => setRecent(!recent)}>
        <I.Queue size={11} stroke={1.75}/>
        Recent
      </button>
      <span className="m-fdivider"/>
      {typeChips.map(c => (
        <button
          key={c.id}
          className={`m-fchip ${c.active ? 'active' : ''}`}
          onClick={() => toggleFilter('type', c.id)}>
          <TypeIcon type={c.icon} size={11} stroke={1.5}/>
          {c.label}
          <span className="count">{c.count}</span>
        </button>
      ))}
    </div>
  );
}

// ── Card ───────────────────────────────────────────────────
function MCard({ asset, onOpen }) {
  return (
    <div className="m-card" onClick={() => onOpen(asset)}>
      <div className="thumb">
        <span className="type-badge">{asset.type === 'unitypackage' ? 'unitypkg' : asset.type}</span>
        {asset.needsReview ? <span className="review-dot"/> : null}
        <AssetThumb asset={asset}/>
      </div>
      <div className="meta">
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="name">{asset.name}</div>
          <div className="sub">
            <span>{formatSize(asset.size)}</span>
            <span className="dot"/>
            <LicencePill licence={asset.licence} compact/>
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Bottom tabs ────────────────────────────────────────────
function MTabs({ route, setRoute, queueCount }) {
  const tabs = [
    { id: 'library', label: 'Library', icon: 'Library' },
    { id: 'projects', label: 'Projects', icon: 'Projects' },
    { id: 'collections', label: 'Collections', icon: 'Collections' },
    { id: 'queue', label: 'Queue', icon: 'Queue' },
  ];
  return (
    <div className="m-tabs">
      {tabs.map(t => (
        <button key={t.id} className={`m-tab ${route === t.id ? 'on' : ''}`} onClick={() => setRoute(t.id)}>
          <MIcon name={t.icon} size={20} stroke={1.75}/>
          {t.label}
          {t.id === 'queue' && queueCount ? <span className="badge">{queueCount}</span> : null}
        </button>
      ))}
    </div>
  );
}

// ── Drawer (combined nav + filters) ─────────────────────────
function MDrawer({ open, onClose, filters, toggleFilter, route, setRoute }) {
  if (!open) return null;
  return (
    <>
      <div className="m-scrim" onClick={onClose}/>
      <div className="m-drawer">
        <div className="m-drawer-head">
          <div className="brand">D</div>
          <div className="name">Damson</div>
          <button className="m-iconbtn" onClick={onClose}><I.X size={16}/></button>
        </div>
        <div className="m-drawer-body">
          <div className="m-dgroup">Browse</div>
          {[
            { id: 'library', l: 'Library', icon: 'Library' },
            { id: 'projects', l: 'Projects', icon: 'Projects' },
            { id: 'collections', l: 'Collections', icon: 'Collections' },
            { id: 'queue', l: 'Queue', icon: 'Queue' },
          ].map(r => (
            <div key={r.id} className={`m-drow ${route === r.id ? 'active' : ''}`} onClick={() => { setRoute(r.id); onClose(); }}>
              <span className="ico"><MIcon name={r.icon} size={16}/></span>
              {r.l}
            </div>
          ))}

          <div className="m-dgroup">Licence</div>
          {LICENCE_FILTERS.map(f => (
            <div key={f.id} className={`m-drow ${filters.licence.has(f.id) ? 'active' : ''}`} onClick={() => toggleFilter('licence', f.id)}>
              <span className={`f-dot ${f.status}`}/>{f.label}
              <span className="count">{f.count}</span>
            </div>
          ))}

          <div className="m-dgroup">Type</div>
          {FILETYPE_FILTERS.map(f => (
            <div key={f.id} className={`m-drow ${filters.type.has(f.id) ? 'active' : ''}`} onClick={() => toggleFilter('type', f.id)}>
              <span className="ico"><TypeIcon type={f.id} size={14}/></span>
              {f.label}
              <span className="count">{f.count}</span>
            </div>
          ))}

          <div className="m-dgroup">Tags</div>
          <div className="m-tagwrap">
            {TAG_TREE.map(t => (
              <button key={t.name} className={`tag ${filters.tag.has(t.name) ? 'on' : ''}`}
                onClick={() => toggleFilter('tag', t.name)}>
                {t.name}<span style={{ color: 'var(--text-4)', marginLeft: 4 }}>{t.count}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}

// ── Section helper ───────────────────────────────────────
function MSection({ title, count, children, defaultOpen = true, action }) {
  const [open, setOpen] = useStM(defaultOpen);
  return (
    <div className="m-section">
      <button className={`m-section-head ${open ? 'open' : ''}`} onClick={() => setOpen(!open)}>
        <span className="caret"><I.ChevronRight size={10}/></span>
        <span>{title}</span>
        {count != null && !action ? <span className="sec-count">{count}</span> : null}
        {action || null}
      </button>
      {open ? <div className="m-section-body">{children}</div> : null}
    </div>
  );
}

// ── Detail sheet ─────────────────────────────────────────
function MDetailSheet({ asset, index, total, onClose, onPrev, onNext, initialEditMode = false, initialEditField = null }) {
  const [edit, setEdit] = useStM(initialEditMode);
  const [editField, setEditField] = useStM(initialEditField);
  const [tags, setTags] = useStM(asset.tags);
  const [licence, setLicence] = useStM(asset.licence);
  const [project, setProject] = useStM(asset.projects);
  const [notes, setNotes] = useStM(asset.overview?.teamNotes || '');
  const [tagInput, setTagInput] = useStM('');
  const lic = LICENCES[licence] || LICENCES.Unknown;
  const ov = asset.overview || {};
  const showEditInline = (field) => edit || editField === field;

  const removeTag = (t) => setTags(s => s.filter(x => x !== t));
  const addTag = () => {
    const v = tagInput.trim();
    if (v && !tags.includes(v)) setTags([...tags, v]);
    setTagInput('');
  };

  return (
    <div className="m-sheet">
      <div className="m-sheet-bar">
        <button className="m-iconbtn back" onClick={onClose} aria-label="Back to library">
          <I.ChevronLeft size={16}/>
          <span className="back-label">Library</span>
        </button>
        <span className="spacer"/>
        <div className="m-pager">
          <button className="m-iconbtn" onClick={onPrev} aria-label="Previous"><I.ChevronLeft size={13}/></button>
          <span className="counter">{index + 1} / {total}</span>
          <button className="m-iconbtn" onClick={onNext} aria-label="Next"><I.ChevronRight size={13}/></button>
        </div>
        <button className={`edit-pill ${edit ? 'on' : ''}`} onClick={() => { setEdit(!edit); setEditField(null); }}>
          {edit ? <><I.Check size={11}/> Done</> : <>Edit</>}
        </button>
        <button className="m-iconbtn"><I.MoreH size={16}/></button>
      </div>

      <div className="m-sheet-body">
        <div className="m-hero">
          <div className="hero-thumb"><AssetThumb asset={asset}/></div>
          <h2>{asset.name}</h2>
          {ov.tagline ? <div className="tagline">{ov.tagline}</div> : null}
          <div className="meta-row">
            <span className="mono">v{asset.version || '—'}</span>
            <span className="sep">·</span>
            <TypeIcon type={asset.type} size={11} stroke={1.5}/> {asset.type}
            <span className="sep">·</span>
            <span>{formatSize(asset.size)}</span>
            <span className="sep">·</span>
            <span>{asset.files} {asset.files === 1 ? 'file' : 'files'}</span>
          </div>
          <div className="pills">
            {asset.needsReview ? <span className="lic-pill warn" style={{ fontFamily: 'var(--sans)' }}><span className="dot"/> Needs review</span> : null}
            {ov.rating ? <span style={{ fontSize: 11, color: 'var(--text-3)', display: 'inline-flex', alignItems: 'center', gap: 4 }}><span style={{ color: 'var(--warn)' }}>★</span><span className="mono">{ov.rating.stars}</span></span> : null}
            <LicencePill licence={licence} compact/>
          </div>
        </div>

        <MSection title="Tags" count={tags.length}
          action={!edit ? <span className="edit-link" onClick={(e) => { e.stopPropagation(); setEditField('tags'); }}>Edit</span> : null}>
          <div className="m-tag-edit">
            {tags.map(t => (
              <span key={t} className="tag-chip">
                {t}
                {showEditInline('tags') ? <button onClick={() => removeTag(t)}><I.X size={10}/></button> : null}
              </span>
            ))}
            {showEditInline('tags') ? (
              <>
                <input className="tag-input" value={tagInput} onChange={e => setTagInput(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && addTag()} placeholder="add tag…"/>
                <button className="tag-chip" style={{ borderStyle: 'dashed', color: 'var(--text-3)' }} onClick={addTag}>
                  <I.Plus size={10}/> Add
                </button>
              </>
            ) : null}
          </div>
        </MSection>

        <MSection title="Team notes"
          action={!edit ? <span className="edit-link" onClick={(e) => { e.stopPropagation(); setEditField('notes'); }}>Edit</span> : null}>
          {showEditInline('notes') ? (
            <textarea className="m-textarea" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Add a note for your team…"/>
          ) : notes ? (
            <div style={{ fontSize: 13, color: 'var(--text-2)', lineHeight: 1.55 }}>
              {notes}
              <div style={{ marginTop: 6, fontSize: 11, color: 'var(--text-4)' }}>
                <span className="mono">thomas</span> · 2d ago
              </div>
            </div>
          ) : (
            <div style={{ color: 'var(--text-4)', fontSize: 12 }}>No notes yet — tap Edit to add.</div>
          )}
        </MSection>

        <MSection title="Description" defaultOpen={false}>
          <div style={{ fontSize: 13, color: 'var(--text-2)', lineHeight: 1.55, whiteSpace: 'pre-wrap' }}>
            {ov.description || 'No description.'}
          </div>
        </MSection>

        <MSection title="Projects" count={project.length} defaultOpen={false}
          action={!edit ? <span className="edit-link" onClick={(e) => { e.stopPropagation(); setEditField('projects'); }}>Edit</span> : null}>
          {showEditInline('projects') ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {PROJECTS.map(p => {
                const on = project.includes(p.name);
                return (
                  <button key={p.id} className="m-drow" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, background: on ? 'var(--accent-tint)' : 'var(--bg)' }}
                    onClick={() => setProject(s => on ? s.filter(x => x !== p.name) : [...s, p.name])}>
                    <span className="swatch" style={{ background: p.colour, width: 22, height: 22, fontSize: 11, color: 'var(--bg)' }}>{p.glyph}</span>
                    {p.name}
                    <span style={{ marginLeft: 'auto' }}>{on ? <I.Check size={14}/> : <I.Plus size={14}/>}</span>
                  </button>
                );
              })}
            </div>
          ) : project.length === 0 ? (
            <div style={{ color: 'var(--text-4)', fontSize: 12 }}>Not pinned to any project.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {project.map(pn => {
                const p = PROJECTS.find(x => x.name === pn) || { name: pn, colour: 'var(--line-2)', glyph: pn[0] };
                return (
                  <div key={pn} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0' }}>
                    <span className="swatch" style={{ background: p.colour, width: 22, height: 22, borderRadius: 5, display: 'grid', placeItems: 'center', fontSize: 11, fontWeight: 700, color: 'var(--bg)' }}>{p.glyph}</span>
                    <span style={{ fontSize: 13, color: 'var(--text)' }}>{pn}</span>
                    <span className="mono" style={{ marginLeft: 'auto', color: 'var(--text-4)', fontSize: 11 }}>v{asset.version}</span>
                  </div>
                );
              })}
            </div>
          )}
        </MSection>

        <MSection title="Collections" count={asset.collections.length} defaultOpen={false}>
          {asset.collections.length === 0
            ? <div style={{ color: 'var(--text-4)', fontSize: 12 }}>Not in any collection.</div>
            : asset.collections.map(c => (
              <div key={c} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0', fontSize: 13, color: 'var(--text-2)' }}>
                <I.Folder size={12} stroke={1.5}/>{c}
              </div>
            ))}
        </MSection>

        <MSection title="Versions" count={asset.versions.length} defaultOpen={false}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {asset.versions.slice(0, 4).map(v => (
              <div key={v.v} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12 }}>
                <span className="mono" style={{ color: 'var(--text)', fontWeight: 500 }}>{v.v}</span>
                {v.current ? <span style={{ fontSize: 10, color: 'var(--accent)', textTransform: 'uppercase', letterSpacing: '0.06em', fontWeight: 600 }}>current</span> : null}
                <span style={{ color: 'var(--text-4)', marginLeft: 'auto', fontSize: 11 }}>{formatDate(v.date)}</span>
              </div>
            ))}
          </div>
        </MSection>

        <MSection title="Source" defaultOpen={false}>
          <dl className="m-kv">
            <dt>Origin</dt><dd className="mono">{asset.source}</dd>
            <dt>Imported</dt><dd className="mono">{formatDate(asset.imported)}</dd>
            <dt>Hash</dt><dd className="mono" style={{ color: 'var(--text-3)' }}>sha256:{asset.id.padEnd(6, '0')}f4a2…</dd>
          </dl>
        </MSection>

        <MSection title="Licence" defaultOpen={false}
          action={!edit ? <span className="edit-link" onClick={(e) => { e.stopPropagation(); setEditField('licence'); }}>Edit</span> : null}>
          {showEditInline('licence') ? (
            <>
              <select className="m-select" value={licence} onChange={e => setLicence(e.target.value)}>
                {Object.keys(LICENCES).map(k => <option key={k} value={k}>{k}</option>)}
              </select>
              <div style={{ fontSize: 11, color: 'var(--text-4)', marginTop: 4, fontFamily: 'var(--mono)' }}>
                detected: {asset.licence} · 92%
              </div>
            </>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <LicencePill licence={licence}/>
              <span className="mono" style={{ fontSize: 11, color: 'var(--text-4)' }}>confidence 92%</span>
            </div>
          )}
          <div className="m-lic-grid">
            {[['commercial', 'Commercial'], ['modification', 'Modify'], ['redistribution', 'Redistribute'], ['attribution', 'Attribution'], ['shareAlike', 'Share-alike'], ['opensource', 'Open source']].map(([k, l]) => {
              const has = lic.attrs[k] === true;
              return (
                <div key={k} className={`m-lic-attr ${has ? 'yes' : 'no'}`}>
                  <span className="box">{has ? <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round"><path d="M20 6 9 17l-5-5"/></svg> : null}</span>
                  {l}
                </div>
              );
            })}
          </div>
        </MSection>

        <div style={{ height: 40 }}/>
      </div>
    </div>
  );
}

// ── Library page ─────────────────────────────────────────
function MLibrary({ filters, setFilters, query, setQuery, onOpen, recent, setRecent }) {
  const toggleFilter = (g, id) => setFilters(f => {
    const n = { ...f, [g]: new Set(f[g]) };
    if (n[g].has(id)) n[g].delete(id); else n[g].add(id);
    return n;
  });
  const clearAll = () => { setFilters({ licence: new Set(), type: new Set(), tag: new Set(), project: new Set(), collection: new Set(), source: new Set() }); setRecent(false); };

  const filtered = useMemoM(() => {
    const q = query.trim().toLowerCase();
    let out = ASSETS.filter(a => {
      if (filters.licence.size) {
        const st = (LICENCES[a.licence] || LICENCES.Unknown).status;
        if (!filters.licence.has(st)) return false;
      }
      if (filters.type.size && !filters.type.has(a.type)) return false;
      if (filters.tag.size) {
        const any = [...filters.tag].some(t => a.tags.includes(t));
        if (!any) return false;
      }
      if (q) {
        const hay = (a.name + ' ' + a.tags.join(' ') + ' ' + a.source).toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
    if (recent) {
      out = [...out].sort((a, b) => (b.imported || '').localeCompare(a.imported || '')).slice(0, 8);
    }
    return out;
  }, [filters, query, recent]);

  return (
    <>
      <MSearch value={query} onChange={setQuery}/>
      <MFilterStrip filters={filters} toggleFilter={toggleFilter} clearAll={clearAll} recent={recent} setRecent={setRecent}/>
      <div className="m-body">
        <div className="m-grid">
          {filtered.map(a => <MCard key={a.id} asset={a} onOpen={onOpen}/>)}
        </div>
        {filtered.length === 0 ? (
          <div style={{ padding: 40, textAlign: 'center', color: 'var(--text-4)', fontSize: 12 }}>No assets match.</div>
        ) : null}
      </div>
    </>
  );
}

// ── Projects page ────────────────────────────────────────
function MProjects() {
  return (
    <div className="m-body">
      <div className="m-page-head">
        <h1>Projects</h1>
        <p>Pinned asset versions per game.</p>
      </div>
      {PROJECTS.map(p => {
        const stats = projectStats(p.id);
        const total = stats.count || 1;
        const segs = [
          { st: 'ok', n: stats.byStatus.ok },
          { st: 'warn', n: stats.byStatus.warn },
          { st: 'err', n: stats.byStatus.err },
          { st: 'unk', n: stats.byStatus.unk },
        ];
        return (
          <div key={p.id} className="m-list-row" style={{ alignItems: 'flex-start', flexDirection: 'column', gap: 6 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%' }}>
              <span className="swatch" style={{ background: p.colour }}>{p.glyph}</span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="name">{p.name}</div>
                <div className="sub">{p.engine} · updated {formatDate(p.updated)}</div>
              </div>
              <div className="right">
                <div style={{ fontSize: 12, color: 'var(--text)' }} className="mono">{stats.count}</div>
                <div className="sub">assets</div>
              </div>
            </div>
            <div className="m-lic-bar" style={{ width: '100%' }}>
              {segs.map(s => s.n > 0 ? <div key={s.st} style={{ width: `${(s.n/total)*100}%`, background: `var(--${s.st === 'unk' ? 'unk' : s.st})` }}/> : null)}
            </div>
            {stats.byStatus.warn + stats.byStatus.err > 0 ? (
              <div style={{ fontSize: 11, color: 'var(--warn)' }}>
                {stats.byStatus.warn + stats.byStatus.err} to review
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

// ── Collections page ─────────────────────────────────────
function MCollections() {
  return (
    <div className="m-body">
      <div className="m-page-head">
        <h1>Collections</h1>
        <p>Manual & smart filter groups.</p>
      </div>
      {COLLECTIONS.slice(0, 12).map(c => (
        <div key={c.id} className="m-list-row">
          <span className="swatch" style={{ background: c.kind === 'smart' ? 'var(--accent-tint)' : 'var(--surface-2)', color: c.kind === 'smart' ? 'var(--accent)' : 'var(--text-2)' }}>
            {c.kind === 'smart' ? <I.Sliders size={14}/> : <I.Folder size={14}/>}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div className="name">{c.name}</div>
            <div className="sub">{c.kind === 'smart' ? 'Smart' : 'Manual'} · {c.count || 0} assets</div>
          </div>
          {c.kind === 'smart' ? <span className="lic-pill" style={{ background: 'var(--accent-tint)', color: 'var(--accent)', fontFamily: 'var(--sans)', fontSize: 10 }}>Smart</span> : null}
        </div>
      ))}
    </div>
  );
}

// ── Queue page (read-only) ──────────────────────────────
function MQueue() {
  return (
    <div className="m-body">
      <div className="m-page-head">
        <h1>Import queue</h1>
        <p>In-flight imports. Add new files from desktop.</p>
      </div>
      {window.QUEUE_SEED.map(q => (
        <div key={q.id} className="m-queue-item">
          <div className="m-queue-thumb"><TypeIcon type={q.type} size={16} stroke={1.5}/></div>
          <div className="m-queue-meta">
            <div className="m-queue-name">{q.name}</div>
            <div className="m-queue-sub">
              <span>{q.detectedType}</span>
              <span style={{ color: 'var(--text-4)' }}>·</span>
              <span className="mono">{formatSize(q.size)}</span>
            </div>
            {q.progress < 100 ? (
              <div className="m-queue-progress"><div style={{ width: `${q.progress}%` }}/></div>
            ) : q.needsFields.length ? (
              <div style={{ marginTop: 4, fontSize: 11, color: 'var(--warn)' }}>
                Missing: {q.needsFields.join(', ')}
              </div>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Top-level mobile app ─────────────────────────────────
function MobileApp({ initialRoute = 'library', initialOpenAssetId = null, initialDrawer = false, initialEditMode = false, initialEditField = null, demo = null }) {
  const [route, setRoute] = useStM(initialRoute);
  const [drawer, setDrawer] = useStM(initialDrawer);
  const [openId, setOpenId] = useStM(initialOpenAssetId);
  const [filters, setFilters] = useStM({
    licence: new Set(demo?.licence || []),
    type: new Set(demo?.type || []),
    tag: new Set(demo?.tag || []),
    project: new Set(), collection: new Set(), source: new Set(),
  });
  const [recent, setRecent] = useStM(!!demo?.recent);
  const [query, setQuery] = useStM('');

  const openAsset = (a) => setOpenId(a.id);
  const openAsset_obj = openId ? ASSETS.find(x => x.id === openId) : null;
  const allFiltered = ASSETS;
  const idx = openAsset_obj ? allFiltered.findIndex(x => x.id === openAsset_obj.id) : -1;
  const step = (delta) => {
    if (!allFiltered.length) return;
    const next = allFiltered[(idx + delta + allFiltered.length) % allFiltered.length];
    setOpenId(next.id);
  };

  const titleByRoute = {
    library: 'Library', projects: 'Projects', collections: 'Collections', queue: 'Queue',
  };
  const countByRoute = {
    library: ASSETS.length, projects: PROJECTS.length, collections: COLLECTIONS.length, queue: window.QUEUE_SEED.length,
  };

  return (
    <div className="m-app">
      <MAppBar
        title={titleByRoute[route]}
        count={countByRoute[route]}
        onMenu={() => setDrawer(true)}
        right={route === 'library' ? (
          <button className="m-iconbtn"><I.SortAsc size={16}/></button>
        ) : null}/>

      {route === 'library' && <MLibrary filters={filters} setFilters={setFilters} query={query} setQuery={setQuery} onOpen={openAsset} recent={recent} setRecent={setRecent}/>}
      {route === 'projects' && <MProjects/>}
      {route === 'collections' && <MCollections/>}
      {route === 'queue' && <MQueue/>}

      <MTabs route={route} setRoute={(r) => { setRoute(r); setOpenId(null); }} queueCount={window.QUEUE_SEED.filter(q => q.progress >= 100).length}/>

      <MDrawer open={drawer} onClose={() => setDrawer(false)}
        filters={filters}
        toggleFilter={(g, id) => setFilters(f => { const n = { ...f, [g]: new Set(f[g]) }; if (n[g].has(id)) n[g].delete(id); else n[g].add(id); return n; })}
        route={route} setRoute={setRoute}/>

      {openAsset_obj ? (
        <MDetailSheet asset={openAsset_obj} index={idx >= 0 ? idx : 0} total={allFiltered.length}
          onClose={() => setOpenId(null)} onPrev={() => step(-1)} onNext={() => step(1)}
          initialEditMode={initialEditMode}
          initialEditField={initialEditField || (openAsset_obj.id === 'a05' ? 'tags' : null)}/>
      ) : null}
    </div>
  );
}

Object.assign(window, { MobileApp });
