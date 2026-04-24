// Main app — wires state, routing, filters, selection, drag-drop, keyboard.
const { useState, useEffect, useMemo, useRef, useCallback } = React;

function App() {
  const [tweaks, setTweaks] = useState(window.TWEAKS);
  const [route, setRoute] = useState('library');
  const [openProjectId, setOpenProjectId] = useState(null);
  const [activeCollection, setActiveCollection] = useState(null);
  const [view, setView] = useState('grid');
  const [sort, setSort] = useState('imported');
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState({
    licence: new Set(), type: new Set(), tag: new Set(),
    project: new Set(), collection: new Set(), source: new Set(),
  });
  const [selected, setSelected] = useState(new Set());
  const [openId, setOpenId] = useState(null);
  const [fullPage, setFullPage] = useState(false);
  const [queue, setQueue] = useState(window.QUEUE_SEED);
  const [dropActive, setDropActive] = useState(false);
  const [sidebarW, setSidebarW] = useState(tweaks.sidebarWidth);
  const [showNewProject, setShowNewProject] = useState(false);
  const [showNewCollection, setShowNewCollection] = useState(false);
  const [projectsLayout, setProjectsLayout] = useState(tweaks.projectsLayout || 'list');
  const mainRef = useRef(null);

  useEffect(() => {
    document.documentElement.dataset.theme = tweaks.theme;
    document.documentElement.dataset.accent = tweaks.accent;
    document.documentElement.style.setProperty('--sidebar-w', tweaks.sidebarWidth + 'px');
    setSidebarW(tweaks.sidebarWidth);
    if (tweaks.projectsLayout) setProjectsLayout(tweaks.projectsLayout);
  }, [tweaks]);

  const toggleFilter = (group, id) => {
    setFilters(f => {
      const n = { ...f, [group]: new Set(f[group]) };
      if (n[group].has(id)) n[group].delete(id); else n[group].add(id);
      return n;
    });
  };
  const clearFilters = () => {
    setFilters({
      licence: new Set(), type: new Set(), tag: new Set(),
      project: new Set(), collection: new Set(), source: new Set(),
    });
    setActiveCollection(null);
  };

  const activeCollectionObj = activeCollection ? COLLECTIONS.find(c => c.id === activeCollection) : null;

  const activeChips = useMemo(() => {
    const chips = [];
    filters.licence.forEach(id => {
      const f = LICENCE_FILTERS.find(x => x.id === id);
      if (f) chips.push({ key: `lic-${id}`, group: 'Licence', label: f.label, group_k: 'licence', id });
    });
    filters.type.forEach(id => {
      const f = FILETYPE_FILTERS.find(x => x.id === id);
      if (f) chips.push({ key: `type-${id}`, group: 'Type', label: f.label, group_k: 'type', id });
    });
    filters.tag.forEach(name => chips.push({ key: `tag-${name}`, group: 'Tag', label: name, group_k: 'tag', id: name }));
    filters.project.forEach(id => {
      const p = PROJECTS.find(x => x.id === id);
      if (p) chips.push({ key: `proj-${id}`, group: 'Project', label: p.name, group_k: 'project', id });
    });
    filters.source.forEach(id => {
      const s = SOURCES.find(x => x.id === id);
      if (s) chips.push({ key: `src-${id}`, group: 'Source', label: s.label, group_k: 'source', id });
    });
    if (query.trim()) chips.push({ key: 'q', group: 'Search', label: query.trim(), group_k: 'query' });
    return chips;
  }, [filters, query]);

  const removeChip = (c) => {
    if (c.group_k === 'query') { setQuery(''); return; }
    toggleFilter(c.group_k, c.id);
  };

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    // Base set — narrowed by active collection first
    let base = ASSETS;
    if (activeCollectionObj) base = collectionAssets(activeCollectionObj);

    let out = base.filter(a => {
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
        const hay = (a.name + ' ' + a.tags.join(' ') + ' ' + a.source + ' ' + a.licence).toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
    if (sort === 'name') out = [...out].sort((a, b) => a.name.localeCompare(b.name));
    else out = [...out].sort((a, b) => (b.imported || '').localeCompare(a.imported || ''));
    return out;
  }, [filters, query, sort, activeCollectionObj]);

  const openAsset = useCallback((a) => {
    setOpenId(a.id);
    setSelected(new Set([a.id]));
    setFullPage(false);
  }, []);
  const shiftSelect = (a) => {
    setSelected(s => {
      const n = new Set(s);
      if (n.has(a.id)) n.delete(a.id); else n.add(a.id);
      return n;
    });
    if (!openId) setOpenId(a.id);
  };

  const openAsset_obj = openId ? (ASSETS.find(x => x.id === openId) || null) : null;
  const openIdx = openAsset_obj ? filtered.findIndex(x => x.id === openAsset_obj.id) : -1;

  const step = (delta) => {
    if (!filtered.length) return;
    let idx = openIdx;
    if (idx < 0) idx = 0;
    const next = filtered[(idx + delta + filtered.length) % filtered.length];
    setOpenId(next.id);
    setSelected(new Set([next.id]));
  };

  useEffect(() => {
    const onKey = (e) => {
      if (e.key === 'Escape') {
        if (fullPage) { setFullPage(false); return; }
        setOpenId(null); setSelected(new Set());
      }
      if (e.target.matches('input, textarea, select')) return;
      if (e.key === 'ArrowDown' || e.key === 'ArrowRight') { e.preventDefault(); step(1); }
      if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') { e.preventDefault(); step(-1); }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [fullPage, openIdx, filtered]);

  useEffect(() => {
    const el = mainRef.current;
    if (!el) return;
    let depth = 0;
    const onOver = (e) => { e.preventDefault(); };
    const onEnter = (e) => {
      e.preventDefault(); depth++;
      if (e.dataTransfer && [...(e.dataTransfer.types || [])].includes('Files')) setDropActive(true);
    };
    const onLeave = () => { depth--; if (depth <= 0) { depth = 0; setDropActive(false); } };
    const onDrop = (e) => {
      e.preventDefault(); depth = 0; setDropActive(false);
      const files = e.dataTransfer && e.dataTransfer.files;
      if (!files || !files.length) return;
      const nowItems = [...files].slice(0, 3).map((f, i) => ({
        id: 'q' + Date.now() + i,
        name: f.name, type: guessType(f.name),
        size: f.size || 1024 * 1024 * 12,
        imported: 'just now', progress: 0,
        detectedLicence: null, detectedType: guessType(f.name),
        needsFields: ['Licence', 'Tags'],
      }));
      setQueue(q => [...nowItems, ...q]);
      setRoute('queue');
      nowItems.forEach((it, i) => {
        setTimeout(() => setQueue(q => q.map(x => x.id === it.id ? { ...x, progress: 100, imported: 'just now' } : x)), 900 + i * 300);
      });
    };
    el.addEventListener('dragover', onOver);
    el.addEventListener('dragenter', onEnter);
    el.addEventListener('dragleave', onLeave);
    el.addEventListener('drop', onDrop);
    return () => {
      el.removeEventListener('dragover', onOver);
      el.removeEventListener('dragenter', onEnter);
      el.removeEventListener('dragleave', onLeave);
      el.removeEventListener('drop', onDrop);
    };
  }, [route]);

  const sidebarWRef = useRef(sidebarW);
  useEffect(() => { sidebarWRef.current = sidebarW; }, [sidebarW]);
  const onResizerDown = (e) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = sidebarW;
    const target = e.currentTarget;
    target.classList.add('dragging');
    const move = (m) => {
      const next = Math.max(200, Math.min(380, startW + (m.clientX - startX)));
      setSidebarW(next);
      document.documentElement.style.setProperty('--sidebar-w', next + 'px');
    };
    const up = () => {
      target.classList.remove('dragging');
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
      setTweaks(t => {
        window.parent.postMessage({ type: '__edit_mode_set_keys', edits: { sidebarWidth: sidebarWRef.current } }, '*');
        return { ...t, sidebarWidth: sidebarWRef.current };
      });
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };

  const queueCount = queue.filter(q => q.progress >= 100).length;

  const gotoLibraryWithCollection = (cid) => {
    setRoute('library');
    setActiveCollection(cid);
    setOpenProjectId(null);
  };

  const onCreateProject = (p) => {
    // Prototype — just close the dialog and show a toast-like effect by jumping to the projects page.
    setShowNewProject(false);
    setRoute('projects');
  };
  const onCreateCollection = (c) => {
    setShowNewCollection(false);
  };

  return (
    <>
      <div className="app">
        <NavRail
          route={route}
          setRoute={(r) => {
            setRoute(r);
            setOpenId(null); setFullPage(false); setOpenProjectId(null);
            if (r !== 'library') setActiveCollection(null);
          }}
          queueCount={queueCount}/>

        {route === 'library' ? (
          <>
            <FilterSidebar
              query={query} setQuery={setQuery}
              filters={filters} toggleFilter={toggleFilter} clearFilters={clearFilters}
              width={sidebarW}
              onResizerDown={onResizerDown}
              activeCollection={activeCollection}
              setActiveCollection={setActiveCollection}
              onNewCollection={() => setShowNewCollection(true)}/>
            <main className="main" ref={mainRef}>
              {activeCollectionObj ? (
                <CollectionHeader
                  collection={activeCollectionObj}
                  onBack={() => setActiveCollection(null)}
                  onEdit={() => {}}/>
              ) : null}
              <Toolbar
                count={filtered.length} total={activeCollectionObj ? collectionAssets(activeCollectionObj).length : ASSETS.length}
                view={view} setView={setView}
                sort={sort} setSort={setSort}
                activeChips={activeChips} removeChip={removeChip} clearAll={() => { clearFilters(); setQuery(''); }}/>
              <div className="grid-wrap">
                {view === 'grid'
                  ? <GridView assets={filtered} selected={selected} onOpen={openAsset} onSelect={shiftSelect}/>
                  : <ListView assets={filtered} selected={selected} onOpen={openAsset}/>}
              </div>
              {dropActive ? (
                <div className="drop-overlay">
                  <div className="inner">
                    <I.Upload size={22}/>
                    <div style={{ marginTop: 8 }}>Drop to import</div>
                    <div className="hint">Files land in the import queue · Unity packages are extracted in the background</div>
                  </div>
                </div>
              ) : null}
              {openAsset_obj && !fullPage ? (
                <DetailPanel
                  asset={openAsset_obj}
                  index={openIdx >= 0 ? openIdx : 0}
                  total={filtered.length}
                  onClose={() => { setOpenId(null); setSelected(new Set()); }}
                  onPrev={() => step(-1)} onNext={() => step(1)}
                  onExpand={() => setFullPage(true)}/>
              ) : null}
              {openAsset_obj && fullPage ? (
                <FullPage asset={openAsset_obj}
                  onClose={() => { setFullPage(false); setOpenId(null); }}
                  onCollapse={() => setFullPage(false)}/>
              ) : null}
              <SelectionBar count={selected.size} onClear={() => setSelected(new Set())}/>
            </main>
          </>
        ) : route === 'projects' ? (
          <>
            <div style={{ gridColumn: '2 / 3', display: 'none' }}/>
            <main className="main" ref={mainRef} style={{ gridColumn: '2 / 4' }}>
              {openProjectId ? (
                <ProjectDetail
                  projectId={openProjectId}
                  onBack={() => setOpenProjectId(null)}
                  onOpenAsset={(a) => { setRoute('library'); openAsset(a); }}/>
              ) : (
                <>
                  <div className="toolbar">
                    <div className="breadcrumb">Projects <span className="count">{PROJECTS.length}</span></div>
                    <div className="toolbar-spacer"/>
                    <div className="seg">
                      <button className={projectsLayout === 'list' ? 'on' : ''} onClick={() => { setProjectsLayout('list'); window.parent.postMessage({ type: '__edit_mode_set_keys', edits: { projectsLayout: 'list' } }, '*'); }} title="List"><I.List size={13}/></button>
                      <button className={projectsLayout === 'cards' ? 'on' : ''} onClick={() => { setProjectsLayout('cards'); window.parent.postMessage({ type: '__edit_mode_set_keys', edits: { projectsLayout: 'cards' } }, '*'); }} title="Cards"><I.Grid size={13}/></button>
                    </div>
                    <span className="divider-v"/>
                    <button className="tbtn primary" onClick={() => setShowNewProject(true)}><I.Plus size={12}/> New project</button>
                  </div>
                  <div className="grid-wrap">
                    <div className="page-head" style={{ maxWidth: 1100 }}>
                      <h1>Projects</h1>
                      <p>A project pins specific asset versions to a game. Damson writes a manifest to your repo so builds always pull the right files.</p>
                    </div>
                    {projectsLayout === 'cards'
                      ? <ProjectsCards onOpen={setOpenProjectId}/>
                      : <ProjectsList onOpen={setOpenProjectId}/>}
                  </div>
                </>
              )}
            </main>
          </>
        ) : route === 'collections' ? (
          <>
            <div style={{ gridColumn: '2 / 3', display: 'none' }}/>
            <main className="main" ref={mainRef} style={{ gridColumn: '2 / 4' }}>
              <div className="toolbar">
                <div className="breadcrumb">Collections <span className="count">{COLLECTIONS.length}</span></div>
                <div className="toolbar-spacer"/>
                <button className="tbtn primary" onClick={() => setShowNewCollection(true)}><I.Plus size={12}/> New collection</button>
              </div>
              <div className="grid-wrap">
                <div className="page-head" style={{ maxWidth: 1100 }}>
                  <h1>Collections</h1>
                  <p>Group assets across projects. Smart collections save a filter query and auto-update.</p>
                </div>
                <div className="proj-cards" style={{ maxWidth: 1100, margin: '0 auto' }}>
                  {COLLECTIONS.map(c => {
                    const items = collectionAssets(c);
                    return (
                      <div key={c.id} className="proj-card" onClick={() => gotoLibraryWithCollection(c.id)}>
                        <div className="proj-card-head">
                          <span className="proj-glyph lg" style={{ background: c.kind === 'smart' ? 'var(--accent-tint)' : 'var(--surface-2)', color: c.kind === 'smart' ? 'var(--accent)' : 'var(--text-2)' }}>
                            {c.kind === 'smart' ? <I.Sliders size={15}/> : <I.Folder size={15}/>}
                          </span>
                          <div style={{ minWidth: 0, flex: 1 }}>
                            <div className="proj-name" style={{ fontSize: 14 }}>{c.name}</div>
                            <div style={{ fontSize: 11, color: 'var(--text-4)', marginTop: 1 }}>
                              {c.kind === 'smart' ? 'Smart collection' : 'Manual collection'}
                            </div>
                          </div>
                          {c.kind === 'smart' ? <span className="smart-badge"><I.Sliders size={10}/> Smart</span> : null}
                        </div>
                        <div className="proj-card-desc" style={{ minHeight: 36 }}>
                          {c.kind === 'smart'
                            ? <span className="mono" style={{ fontSize: 11 }}>{describeSmart(c.query)}</span>
                            : <span style={{ color: 'var(--text-4)' }}>{items.length} {items.length === 1 ? 'asset' : 'assets'}</span>}
                        </div>
                        <div className="proj-card-stats">
                          <div><span className="mono">{items.length}</span><span className="muted">&nbsp;assets</span></div>
                          <div><span className="mono">{formatSize(items.reduce((s, a) => s + a.size, 0))}</span></div>
                        </div>
                      </div>
                    );
                  })}
                  <div className="proj-card new" onClick={() => setShowNewCollection(true)}>
                    <div style={{ textAlign: 'center' }}><I.Plus size={20}/><div style={{ fontSize: 12, marginTop: 6 }}>New collection</div></div>
                  </div>
                </div>
              </div>
            </main>
          </>
        ) : route === 'queue' ? (
          <>
            <div style={{ gridColumn: '2 / 3', display: 'none' }}/>
            <main className="main" ref={mainRef} style={{ gridColumn: '2 / 4' }}>
              <div className="toolbar">
                <div className="breadcrumb">Import queue <span className="count">{queue.length} items</span></div>
                <div className="toolbar-spacer"/>
                <button className="tbtn"><I.Check size={12}/> Confirm all ready</button>
              </div>
              <QueueView queue={queue} onReview={() => setRoute('library')}/>
              {dropActive ? <div className="drop-overlay"><div className="inner"><I.Upload size={22}/><div style={{ marginTop: 8 }}>Drop to import</div></div></div> : null}
            </main>
          </>
        ) : (
          <>
            <div style={{ gridColumn: '2 / 3', display: 'none' }}/>
            <main className="main" style={{ gridColumn: '2 / 4' }}>
              <div className="toolbar">
                <div className="breadcrumb">{route[0].toUpperCase() + route.slice(1)}</div>
              </div>
              <div style={{ padding: 60, textAlign: 'center', color: 'var(--text-3)' }}>
                <div style={{ fontSize: 14, color: 'var(--text-2)', marginBottom: 4 }}>Not in this prototype</div>
                <button className="tbtn" style={{ marginTop: 14, border: '1px solid var(--line)' }} onClick={() => setRoute('library')}>
                  <I.Library size={12}/> Back to library
                </button>
              </div>
            </main>
          </>
        )}
      </div>
      <Tweaks tweaks={tweaks} setTweaks={setTweaks}/>
      <NewProjectDialog open={showNewProject} onClose={() => setShowNewProject(false)} onCreate={onCreateProject}/>
      <NewCollectionDialog open={showNewCollection} onClose={() => setShowNewCollection(false)} onCreate={onCreateCollection}/>
    </>
  );
}

function guessType(name) {
  const n = name.toLowerCase();
  if (n.endsWith('.unitypackage')) return 'unitypackage';
  if (n.match(/\.(wav|mp3|ogg|flac|aiff)$/)) return 'audio';
  if (n.match(/\.(ttf|otf|woff2?)$/)) return 'font';
  if (n.match(/\.(png|jpg|jpeg|gif|webp|psd)$/)) return 'sprite';
  return 'file';
}

ReactDOM.createRoot(document.getElementById('root')).render(<App/>);
