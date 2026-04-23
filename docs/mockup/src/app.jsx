// Main app — wires state, routing, filters, selection, drag-drop, keyboard.
const { useState, useEffect, useMemo, useRef, useCallback } = React;

function App() {
  const [tweaks, setTweaks] = useState(window.TWEAKS);
  const [route, setRoute] = useState('library');
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
  const mainRef = useRef(null);

  // Apply theme + accent + sidebar width to <html>
  useEffect(() => {
    document.documentElement.dataset.theme = tweaks.theme;
    document.documentElement.dataset.accent = tweaks.accent;
    document.documentElement.style.setProperty('--sidebar-w', tweaks.sidebarWidth + 'px');
    setSidebarW(tweaks.sidebarWidth);
  }, [tweaks]);

  const toggleFilter = (group, id) => {
    setFilters(f => {
      const n = { ...f, [group]: new Set(f[group]) };
      if (n[group].has(id)) n[group].delete(id); else n[group].add(id);
      return n;
    });
  };
  const clearFilters = () => setFilters({
    licence: new Set(), type: new Set(), tag: new Set(),
    project: new Set(), collection: new Set(), source: new Set(),
  });

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
    filters.collection.forEach(id => {
      const c = COLLECTIONS.find(x => x.id === id);
      if (c) chips.push({ key: `col-${id}`, group: 'Collection', label: c.name, group_k: 'collection', id });
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
        const hay = (a.name + ' ' + a.tags.join(' ') + ' ' + a.source + ' ' + a.licence).toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
    if (sort === 'name') out = [...out].sort((a, b) => a.name.localeCompare(b.name));
    else out = [...out].sort((a, b) => (b.imported || '').localeCompare(a.imported || ''));
    return out;
  }, [filters, query, sort]);

  const openAsset = useCallback((a) => {
    setOpenId(a.id);
    setSelected(new Set([a.id]));
    setFullPage(false);
  }, []);
  const shiftSelect = (a, e) => {
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

  // Keyboard shortcuts
  useEffect(() => {
    const onKey = (e) => {
      if (e.key === 'Escape') {
        if (fullPage) { setFullPage(false); return; }
        setOpenId(null); setSelected(new Set());
      }
      if (e.target.matches('input, textarea')) return;
      if (e.key === 'ArrowDown' || e.key === 'ArrowRight') { e.preventDefault(); step(1); }
      if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') { e.preventDefault(); step(-1); }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [fullPage, openIdx, filtered]);

  // Drag-drop simulation — any file drop pushes new queue items + flips to queue view
  useEffect(() => {
    const el = mainRef.current;
    if (!el) return;
    let depth = 0;
    const onOver = (e) => { e.preventDefault(); };
    const onEnter = (e) => {
      e.preventDefault();
      depth++;
      if (e.dataTransfer && [...(e.dataTransfer.types || [])].includes('Files')) setDropActive(true);
    };
    const onLeave = () => { depth--; if (depth <= 0) { depth = 0; setDropActive(false); } };
    const onDrop = (e) => {
      e.preventDefault();
      depth = 0; setDropActive(false);
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
      // Animate progress
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
  }, []);

  // Sidebar resizer drag
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
        const next = { ...t, sidebarWidth: sidebarWRef.current };
        window.parent.postMessage({ type: '__edit_mode_set_keys', edits: { sidebarWidth: sidebarWRef.current } }, '*');
        return next;
      });
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };
  const sidebarWRef = useRef(sidebarW);
  useEffect(() => { sidebarWRef.current = sidebarW; }, [sidebarW]);

  const queueCount = queue.filter(q => q.progress >= 100).length;

  return (
    <>
      <div className="app">
        <NavRail route={route} setRoute={(r) => { setRoute(r); setOpenId(null); setFullPage(false); }} queueCount={queueCount}/>

        {route === 'library' ? (
          <>
            <FilterSidebar
              query={query} setQuery={setQuery}
              filters={filters} toggleFilter={toggleFilter} clearFilters={clearFilters}
              width={sidebarW}
              onResizerDown={onResizerDown}/>
            <main className="main" ref={mainRef}>
              <Toolbar
                count={filtered.length} total={ASSETS.length}
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
                <FullPage
                  asset={openAsset_obj}
                  onClose={() => { setFullPage(false); setOpenId(null); }}
                  onCollapse={() => setFullPage(false)}/>
              ) : null}
              <SelectionBar count={selected.size} onClear={() => setSelected(new Set())}/>
            </main>
          </>
        ) : route === 'queue' ? (
          <>
            <div style={{ gridColumn: '2 / 3' }}/>
            <main className="main" ref={mainRef} style={{ gridColumn: '2 / 4' }}>
              <div className="toolbar">
                <div className="breadcrumb">Import queue <span className="count">{queue.length} items</span></div>
                <div className="toolbar-spacer"/>
                <button className="tbtn"><I.Check size={12}/> Confirm all ready</button>
              </div>
              <QueueView queue={queue} onReview={(it) => { setRoute('library'); }}/>
              {dropActive ? (
                <div className="drop-overlay"><div className="inner"><I.Upload size={22}/><div style={{ marginTop: 8 }}>Drop to import</div></div></div>
              ) : null}
            </main>
          </>
        ) : (
          <>
            <div style={{ gridColumn: '2 / 3' }}/>
            <main className="main" style={{ gridColumn: '2 / 4' }}>
              <div className="toolbar">
                <div className="breadcrumb">{route[0].toUpperCase() + route.slice(1)}</div>
              </div>
              <div style={{ padding: 60, textAlign: 'center', color: 'var(--text-3)' }}>
                <div style={{ fontSize: 14, color: 'var(--text-2)', marginBottom: 4 }}>Not in this prototype</div>
                <div style={{ fontSize: 12 }}>Library and Import queue are the two surfaces in this pass.</div>
                <button className="tbtn" style={{ marginTop: 14, border: '1px solid var(--line)' }} onClick={() => setRoute('library')}>
                  <I.Library size={12}/> Back to library
                </button>
              </div>
            </main>
          </>
        )}
      </div>
      <Tweaks tweaks={tweaks} setTweaks={setTweaks}/>
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
