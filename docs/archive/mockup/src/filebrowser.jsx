// Scoped file browser — sidebar tree, breadcrumb, folder/file views,
// file preview (reuses the detail-panel chrome) and collection building.
const { useState: useStF, useEffect: useEffF, useMemo: useMemoF, useRef: useRefF } = React;

// ---- File preview thumbnails --------------------------------------------
function FilePlaceholder({ node, large }) {
  return (
    <div className={`file-ph ${large ? 'lg' : ''}`}>
      <FileGlyph ftype={node.ftype} size={large ? 30 : 22} stroke={1.4}/>
      <span className="file-ph-ext mono">{(node.ext || '').replace('.', '') || 'file'}</span>
    </div>
  );
}
function FileThumb({ node, large = false }) {
  const t = node.thumb;
  if (t) {
    if (t.kind === 'waveform') return <Waveform seed={t.seed || 1}/>;
    if (t.kind === 'font') return <FontThumb sample={t.sample} family={t.family}/>;
    if (t.kind === 'sprite') return <SpritePlaceholder palette={t.palette} pattern={t.pattern}/>;
  }
  return <FilePlaceholder node={node} large={large}/>;
}

// ---- Sidebar: full file tree + type filter ------------------------------
function collectOpen(tree, acc = new Set(), parent = []) {
  for (const n of tree) {
    if (n.type === 'folder') {
      const p = [...parent, n.name];
      if (n.open) acc.add(p.join('/'));
      collectOpen(n.children, acc, p);
    }
  }
  return acc;
}

function TreeNavNode({ node, parent, openSet, toggleOpen, path, openPath, onNavigate, onOpenFile, depth }) {
  const here = [...parent, node.name];
  const key = here.join('/');
  if (node.type === 'file') {
    const active = openPath === key;
    return (
      <div className={`ft-row file ${active ? 'active' : ''}`} style={{ paddingLeft: 8 + depth * 13 }}
        onClick={() => onOpenFile(node, parent)}
        draggable={false}>
        <span className="ft-caret"/>
        <FileGlyph ftype={node.ftype} size={12} stroke={1.5}/>
        <span className="ft-label">{node.name}</span>
      </div>
    );
  }
  const open = openSet.has(key);
  const activeFolder = path.join('/') === key;
  return (
    <>
      <div className={`ft-row folder ${activeFolder ? 'active' : ''}`} style={{ paddingLeft: 8 + depth * 13 }}
        onClick={() => onNavigate(here)}>
        <button className="ft-caret" onClick={(e) => { e.stopPropagation(); toggleOpen(key); }}>
          {open ? <I.ChevronDown size={11}/> : <I.ChevronRight size={11}/>}
        </button>
        {open ? <I.FolderOpen size={12} stroke={1.5}/> : <I.Folder size={12} stroke={1.5}/>}
        <span className="ft-label">{node.name}</span>
        <span className="ft-count">{countFiles(node)}</span>
      </div>
      {open ? node.children.map((c, i) => (
        <TreeNavNode key={i} node={c} parent={here} openSet={openSet} toggleOpen={toggleOpen}
          path={path} openPath={openPath} onNavigate={onNavigate} onOpenFile={onOpenFile} depth={depth + 1}/>
      )) : null}
    </>
  );
}

function FileTreeSidebar({
  asset, tree, width, onResizerDown,
  query, setQuery, path, onNavigate, openPath, onOpenFile,
  typeFilter, toggleType, typeCountsObj, onExit,
}) {
  const [openSet, setOpenSet] = useStF(() => collectOpen(tree));
  useEffF(() => {
    setOpenSet(prev => {
      const n = new Set(prev); let acc = [];
      for (const seg of path) { acc = [...acc, seg]; n.add(acc.join('/')); }
      return n;
    });
  }, [path]);
  const toggleOpen = (k) => setOpenSet(prev => { const n = new Set(prev); n.has(k) ? n.delete(k) : n.add(k); return n; });

  const types = Object.keys(typeCountsObj).sort((a, b) => typeCountsObj[b] - typeCountsObj[a]);

  return (
    <aside className="sidebar" style={{ width }}>
      {onResizerDown ? <div className="sidebar-resizer" onMouseDown={onResizerDown}/> : null}

      <button className="ft-scope" onClick={onExit} title="Back to library">
        <I.ChevronLeft size={13}/>
        <span className="ft-scope-ico"><TypeIcon type={asset.type} size={13} stroke={1.6}/></span>
        <span className="ft-scope-name">{asset.name}</span>
      </button>

      <div className="search">
        <div className="search-box">
          <I.Search size={13}/>
          <input placeholder="Search files in this pack…" value={query} onChange={e => setQuery(e.target.value)}/>
          {query ? <button className="ft-clear" onClick={() => setQuery('')}><I.X size={11}/></button> : null}
        </div>
      </div>

      <div className="filter-scroll">
        <div className="fgroup">
          <FGroupHead title="Files"/>
          <div className={`ft-row folder ${path.length === 0 ? 'active' : ''}`} style={{ paddingLeft: 8 }}
            onClick={() => onNavigate([])}>
            <span className="ft-caret"/>
            <I.Cube size={12} stroke={1.5}/>
            <span className="ft-label">All files</span>
            <span className="ft-count">{asset.files}</span>
          </div>
          <div className="ft-tree">
            {tree.map((n, i) => (
              <TreeNavNode key={i} node={n} parent={[]} openSet={openSet} toggleOpen={toggleOpen}
                path={path} openPath={openPath} onNavigate={onNavigate} onOpenFile={onOpenFile} depth={0}/>
            ))}
          </div>
        </div>

        <div className="fgroup">
          <FGroupHead title="File type"/>
          {types.map(ft => (
            <FRow key={ft}
              active={typeFilter.has(ft)}
              onClick={() => toggleType(ft)}
              icon={<FileGlyph ftype={ft} size={13} stroke={1.6}/>}
              label={(FTYPES[ft] || { label: ft }).label}
              count={typeCountsObj[ft]}/>
          ))}
        </div>
      </div>
    </aside>
  );
}

// ---- Toolbar: breadcrumb path -------------------------------------------
function FileBrowserToolbar({
  asset, path, onNavigate, onExit, count, total,
  view, setView, typeFilter, toggleType, resultsMode, query, clearResults,
}) {
  return (
    <div className="toolbar">
      <div className="breadcrumb fb-crumb">
        <button className="crumb-seg" onClick={onExit}><I.Library size={12}/> Library</button>
        <I.ChevronRight size={11} stroke={1.5} className="crumb-sep"/>
        <button className="crumb-seg" onClick={() => onNavigate([])}>{asset.name}</button>
        {resultsMode ? (
          <>
            <I.ChevronRight size={11} stroke={1.5} className="crumb-sep"/>
            <span className="crumb-seg on">{query.trim() ? `“${query.trim()}”` : 'Filtered'}</span>
          </>
        ) : path.map((seg, i) => (
          <React.Fragment key={i}>
            <I.ChevronRight size={11} stroke={1.5} className="crumb-sep"/>
            <button className={`crumb-seg ${i === path.length - 1 ? 'on' : ''}`} onClick={() => onNavigate(path.slice(0, i + 1))}>{seg}</button>
          </React.Fragment>
        ))}
        <span className="count">{resultsMode ? `${count} ${count === 1 ? 'match' : 'matches'}` : `${count} ${count === 1 ? 'item' : 'items'}`}</span>
      </div>

      {[...typeFilter].length ? (
        <>
          <span className="divider-v"/>
          {[...typeFilter].map(ft => (
            <span key={ft} className="chip">
              <span className="muted">type:</span><strong>{(FTYPES[ft] || { label: ft }).label}</strong>
              <button onClick={() => toggleType(ft)}><I.X size={10}/></button>
            </span>
          ))}
        </>
      ) : null}

      <div className="toolbar-spacer"/>

      {resultsMode ? <button className="tbtn" onClick={clearResults} style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>Clear</button> : null}
      <span className="divider-v"/>
      <div className="seg">
        <button className={view === 'grid' ? 'on' : ''} onClick={() => setView('grid')} title="Grid view"><I.Grid size={13}/></button>
        <button className={view === 'list' ? 'on' : ''} onClick={() => setView('list')} title="List view"><I.List size={13}/></button>
      </div>
    </div>
  );
}

// ---- Main: folder + file cards / list -----------------------------------
function FolderCard({ node, parent, onOpen }) {
  const n = countFiles(node);
  return (
    <div className="fb-card folder" onClick={() => onOpen([...parent, node.name])}>
      <div className="fb-folder-tile">
        <I.Folder size={34} stroke={1.25}/>
      </div>
      <div className="fb-meta">
        <div className="fb-name">{node.name}</div>
        <div className="fb-sub">{n} {n === 1 ? 'item' : 'items'}</div>
      </div>
    </div>
  );
}

function FileCard({ node, dir, selected, open, onOpen, onSelect, onDragStart, onDragEnd, showPath }) {
  return (
    <div className={`fb-card file ${selected ? 'selected' : ''} ${open ? 'open' : ''}`}
      draggable
      onDragStart={(e) => onDragStart(e, node, dir)}
      onDragEnd={onDragEnd}
      onClick={(e) => e.shiftKey || e.metaKey || e.ctrlKey ? onSelect(node, dir) : onOpen(node, dir)}>
      <div className="fb-thumb">
        <span className="fb-type-badge mono">{(node.ext || '').replace('.', '')}</span>
        <FileThumb node={node}/>
        <span className="fb-check"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6 9 17l-5-5"/></svg></span>
      </div>
      <div className="fb-meta">
        <div className="fb-name">{node.name}</div>
        <div className="fb-sub">
          {showPath && dir.length ? <span className="fb-path mono">{dir.join('/')}</span> : <span>{formatSize(node.size)}</span>}
        </div>
      </div>
    </div>
  );
}

function FileBrowserMain({
  listing, resultsMode, path, onNavigate,
  selected, openPath, onOpenFile, onSelectFile,
  onDragStart, onDragEnd, view,
}) {
  const folders = resultsMode ? [] : listing.filter(n => n.type === 'folder');
  const files = listing.filter(n => n.type === 'file');

  if (listing.length === 0) {
    return (
      <div className="empty">
        <div style={{ fontSize: 'var(--fs-ui)', color: 'var(--text-2)', marginBottom: 4 }}>
          {resultsMode ? 'No files match' : 'This folder is empty'}
        </div>
        <div style={{ fontSize: 'var(--fs-small)' }}>
          {resultsMode ? 'Try a different search or clear the file-type filter.' : 'Navigate up to find files.'}
        </div>
      </div>
    );
  }

  if (view === 'list') {
    return (
      <table className="list fb-list">
        <thead>
          <tr><th style={{ width: '44%' }}>Name</th><th>Type</th>{resultsMode ? <th>Folder</th> : null}<th style={{ textAlign: 'right' }}>Size</th><th>Modified</th></tr>
        </thead>
        <tbody>
          {folders.map((n, i) => (
            <tr key={'f' + i} onClick={() => onNavigate([...path, n.name])}>
              <td className="name"><span className="mini-thumb"><I.Folder size={12} stroke={1.5}/></span>{n.name}</td>
              <td>folder</td>
              <td style={{ textAlign: 'right' }} className="mono">{countFiles(n)} items</td>
              <td/>
            </tr>
          ))}
          {files.map((n, i) => {
            const dir = resultsMode ? n.dir : path;
            const key = pathStr(dir, n.name);
            return (
              <tr key={'x' + i} className={selected.has(key) ? 'selected' : ''}
                draggable onDragStart={(e) => onDragStart(e, n, dir)} onDragEnd={onDragEnd}
                onClick={(e) => e.shiftKey || e.metaKey || e.ctrlKey ? onSelectFile(n, dir) : onOpenFile(n, dir)}>
                <td className="name"><span className="mini-thumb"><FileGlyph ftype={n.ftype} size={12} stroke={1.5}/></span>{n.name}</td>
                <td>{(FTYPES[n.ftype] || { label: n.ftype }).label}</td>
                {resultsMode ? <td className="mono" style={{ color: 'var(--text-4)', fontSize: 'var(--fs-small)' }}>{n.dir.join('/') || '—'}</td> : null}
                <td style={{ textAlign: 'right' }} className="mono">{formatSize(n.size)}</td>
                <td>{formatDate(n.modified)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    );
  }

  return (
    <div className="fb-grid">
      {folders.map((n, i) => <FolderCard key={'f' + i} node={n} parent={path} onOpen={onNavigate}/>)}
      {files.map((n, i) => {
        const dir = resultsMode ? n.dir : path;
        const key = pathStr(dir, n.name);
        return (
          <FileCard key={'x' + i} node={n} dir={dir}
            selected={selected.has(key)} open={openPath === key}
            onOpen={onOpenFile} onSelect={onSelectFile}
            onDragStart={onDragStart} onDragEnd={onDragEnd}
            showPath={resultsMode}/>
        );
      })}
    </div>
  );
}

// ---- File detail panel (reuses .detail-panel chrome) --------------------
function FileDetailBody({ asset, node, dir, onAddToCollection }) {
  const lic = LICENCES[asset.licence] || LICENCES.Unknown;
  const isImage = node.ftype === 'texture' || node.ftype === 'sprite';
  return (
    <div className="detail-scroll">
      <div className="detail-hero">
        <div className="detail-thumb">
          <FileThumb node={node} large/>
        </div>
        <h2 className="detail-title" style={{ wordBreak: 'break-all' }}>{node.name}</h2>
        <div className="detail-sub">
          <FileGlyph ftype={node.ftype} size={11} stroke={1.5}/> {(FTYPES[node.ftype] || { label: node.ftype }).label}
          <span className="sep">·</span>
          <span>{formatSize(node.size)}</span>
          <span className="sep">·</span>
          <span>modified {formatDate(node.modified)}</span>
        </div>
        <div className="fb-pathbar mono">{[asset.name, ...dir, node.name].join(' / ')}</div>
        <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
          <button className="tbtn" style={{ border: '1px solid var(--line)' }}><I.Download size={11}/> Download</button>
          <button className="tbtn" style={{ border: '1px solid var(--line)' }} onClick={onAddToCollection}><I.Collections size={11}/> Add to collection</button>
        </div>
      </div>

      <Section title="Details">
        <dl className="kv">
          <dt>Path</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)', wordBreak: 'break-all' }}>{[...dir, node.name].join('/')}</dd>
          <dt>Type</dt><dd>{(FTYPES[node.ftype] || { label: node.ftype }).label} <span className="mono" style={{ color: 'var(--text-4)' }}>({node.ext})</span></dd>
          <dt>Size</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>{formatSize(node.size)}</dd>
          <dt>Modified</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>{node.modified}</dd>
          {isImage ? (<><dt>Dimensions</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>2048 × 2048 · RGBA</dd></>) : null}
          {node.ftype === 'mesh' ? (<><dt>Tris</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>{(1200 + (node.size % 9000)).toLocaleString()} · 1 LOD</dd></>) : null}
        </dl>
      </Section>

      <Section title="Licence" defaultOpen={false}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <LicencePill licence={asset.licence}/>
          <span className="mono" style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>inherited from pack</span>
        </div>
        <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-3)', lineHeight: 1.5 }}>
          This file is governed by the licence of <strong style={{ color: 'var(--text-2)' }}>{asset.name}</strong>. Individual files cannot be re-licensed.
        </div>
      </Section>

      <Section title="Referenced by" defaultOpen={false}>
        <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)', padding: '2px 0' }}>
          {node.ftype === 'texture' ? '3 materials reference this texture.' :
           node.ftype === 'material' ? '2 prefabs use this material.' :
           node.ftype === 'mesh' ? '1 prefab references this mesh.' :
           'No internal references tracked.'}
        </div>
      </Section>

      <div style={{ height: 30 }}/>
    </div>
  );
}

function FileDetailPanel({ asset, node, dir, index, total, onPrev, onNext, onClose, onAddToCollection }) {
  const [width, setWidth] = useStF(() => {
    const saved = parseInt(localStorage.getItem('damson.detailW'), 10);
    return Number.isFinite(saved) ? saved : 440;
  });
  const draggingRef = useRefF(false);
  const widthRef = useRefF(width);
  useEffF(() => { widthRef.current = width; }, [width]);
  useEffF(() => {
    function onMove(e) {
      if (!draggingRef.current) return;
      setWidth(Math.max(320, Math.min(window.innerWidth * 0.8, window.innerWidth - e.clientX)));
    }
    function onUp() {
      if (!draggingRef.current) return;
      draggingRef.current = false;
      document.body.classList.remove('detail-resizing');
      document.querySelector('.detail-resizer')?.classList.remove('dragging');
      localStorage.setItem('damson.detailW', String(Math.round(widthRef.current)));
    }
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp); };
  }, []);
  function startDrag(e) { draggingRef.current = true; document.body.classList.add('detail-resizing'); e.currentTarget.classList.add('dragging'); e.preventDefault(); }
  if (!node) return null;
  return (
    <div className="detail-panel" style={{ '--detail-w': `${width}px` }}>
      <div className="detail-resizer" onMouseDown={startDrag} onDoubleClick={() => { setWidth(440); localStorage.setItem('damson.detailW', '440'); }} role="separator"/>
      <div className="detail-head">
        <button className="nav-btn" onClick={onPrev} title="Previous (↑)"><I.ChevronLeft size={14}/></button>
        <button className="nav-btn" onClick={onNext} title="Next (↓)"><I.ChevronRight size={14}/></button>
        <span className="counter">{index + 1} / {total}</span>
        <div className="spacer"/>
        <button className="nav-btn" title="Close (esc)" onClick={onClose}><I.X size={14}/></button>
      </div>
      <FileDetailBody asset={asset} node={node} dir={dir} onAddToCollection={onAddToCollection}/>
    </div>
  );
}

// ---- Selection bar + collection dock + toast ----------------------------
function FileSelectionBar({ count, onClear, onAddToCollection, onNewCollection }) {
  if (count < 1) return null;
  return (
    <div className="sel-bar">
      <span className="count">{count} {count === 1 ? 'file' : 'files'} selected</span>
      <span className="divider"/>
      <button className="btn" onClick={onAddToCollection}><I.Collections size={12}/> Add to collection</button>
      <button className="btn" onClick={onNewCollection}><I.Plus size={12}/> New collection</button>
      <span className="divider"/>
      <span className="sel-hint mono">or drag onto a collection</span>
      <span className="divider"/>
      <button className="btn" onClick={onClear}><I.X size={12}/></button>
    </div>
  );
}

function CollectionDock({ visible, mode, count, onPick, onPickNew, onCancel }) {
  const [over, setOver] = useStF(null);
  if (!visible) return null;
  const pickMode = mode === 'pick';
  const cols = COLLECTIONS.filter(c => c.kind === 'manual').slice(0, 7);
  return (
    <div className={`col-dock ${pickMode ? 'pick' : ''}`}>
      <span className="col-dock-label mono">{count} {count === 1 ? 'file' : 'files'} {pickMode ? '→ pick a collection' : '→'}</span>
      {cols.map(c => (
        <button key={c.id}
          className={`col-drop ${over === c.id ? 'over' : ''}`}
          onClick={pickMode ? () => onPick(c) : undefined}
          onDragOver={(e) => { e.preventDefault(); setOver(c.id); }}
          onDragLeave={() => setOver(o => o === c.id ? null : o)}
          onDrop={(e) => { e.preventDefault(); setOver(null); onPick(c); }}>
          <I.Folder size={12} stroke={1.5}/> {c.name}
        </button>
      ))}
      <button className="col-drop new"
        onClick={pickMode ? onPickNew : undefined}
        onDragOver={(e) => { e.preventDefault(); setOver('new'); }}
        onDragLeave={() => setOver(o => o === 'new' ? null : o)}
        onDrop={(e) => { e.preventDefault(); setOver(null); onPickNew(); }}>
        <I.Plus size={12}/> New collection
      </button>
      {pickMode ? <button className="col-dock-x" onClick={onCancel}><I.X size={13}/></button> : null}
    </div>
  );
}

function Toast({ toast }) {
  if (!toast) return null;
  return (
    <div className="fb-toast">
      <I.Check size={13}/>
      <span>{toast}</span>
    </div>
  );
}

Object.assign(window, {
  FileThumb, FileTreeSidebar, FileBrowserToolbar, FileBrowserMain,
  FileDetailPanel, FileSelectionBar, CollectionDock, Toast,
});
