// Asset detail panel & full-page mode.
const { useState: useStD } = React;

function Section({ title, count, children, defaultOpen = true }) {
  const [open, setOpen] = useStD(defaultOpen);
  return (
    <div className="section">
      <button className={`section-head ${open ? 'open' : ''}`} onClick={() => setOpen(!open)}>
        <span className="caret"><I.ChevronRight size={11}/></span>
        <span>{title}</span>
        {count != null ? <span className="section-count">{count}</span> : null}
      </button>
      {open ? <div className="section-body">{children}</div> : null}
    </div>
  );
}

function TreeNode({ node, depth = 0 }) {
  const [open, setOpen] = useStD(node.open || false);
  const isFolder = node.type === 'folder';
  const hasChildren = isFolder && Array.isArray(node.children);
  return (
    <>
      <div className={`tree-node ${isFolder ? 'tree-folder' : ''}`} style={{ paddingLeft: 6 + depth * 14 }}
        onClick={() => isFolder && setOpen(!open)}>
        {isFolder ? <span className="caret">{open ? '▾' : '▸'}</span> : <span className="caret"/>}
        {isFolder ? <I.Folder size={11} stroke={1.5}/> : <I.File size={11} stroke={1.5}/>}
        <span>{node.name}</span>
        {typeof node.children === 'number'
          ? <span className="size">{node.children} files</span>
          : node.size ? <span className="size">{node.size}</span> : null}
      </div>
      {hasChildren && open ? node.children.map((c, i) => <TreeNode key={i} node={c} depth={depth + 1}/>) : null}
    </>
  );
}

function LicenceAttrs({ attrs }) {
  const keys = [
    ['commercial', 'Commercial use'],
    ['modification', 'Modification'],
    ['redistribution', 'Redistribution'],
    ['attribution', 'Attribution req.'],
    ['shareAlike', 'Share-alike'],
    ['opensource', 'Open source'],
  ];
  return (
    <div className="lic-grid">
      {keys.map(([k, label]) => {
        const v = attrs[k];
        const has = v === true;
        return (
          <div key={k} className={`lic-attr ${has ? 'yes' : 'no'}`}>
            <span className="box">
              {has ? <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6 9 17l-5-5"/></svg> : null}
            </span>
            {label}
          </div>
        );
      })}
    </div>
  );
}

function DetailBody({ asset }) {
  const lic = LICENCES[asset.licence] || LICENCES.Unknown;
  const ov = asset.overview || {};
  const [teamNotes, setTeamNotes] = useStD(ov.teamNotes || '');
  const [editingNotes, setEditingNotes] = useStD(false);
  const hasGallery = ov.gallery && ov.gallery.length > 0;

  return (
    <div className="detail-scroll">
      <div className="detail-hero">
        <div className="detail-thumb">
          <AssetThumb asset={asset}/>
        </div>
        <h2 className="detail-title">{asset.name}</h2>
        {ov.tagline ? (
          <div style={{ color: 'var(--text-2)', fontSize: 'var(--fs-ui)', marginBottom: 8, lineHeight: 1.5 }}>
            {ov.tagline}
          </div>
        ) : null}
        <div className="detail-sub">
          <span className="mono">v{asset.version || '—'}</span>
          <span className="sep">·</span>
          <TypeIcon type={asset.type} size={11} stroke={1.5}/> {asset.type}
          <span className="sep">·</span>
          <span>{formatSize(asset.size)}</span>
          <span className="sep">·</span>
          <span>{asset.files} {asset.files === 1 ? 'file' : 'files'}</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
          <LicencePill licence={asset.licence}/>
          {asset.needsReview ? (
            <span className="lic-pill warn" style={{ fontFamily: 'var(--sans)' }}>
              <span className="dot"/> Needs review
            </span>
          ) : null}
          {ov.rating ? (
            <span style={{ fontSize: 'var(--fs-small)', color: 'var(--text-3)', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              <span style={{ color: 'var(--warn)' }}>★</span>
              <span className="mono">{ov.rating.stars}</span>
              <span style={{ color: 'var(--text-4)' }}>({ov.rating.count.toLocaleString()})</span>
            </span>
          ) : null}
        </div>
        <div className="tag-chips">
          {asset.tags.map(t => <span key={t} className="tag-chip">{t}</span>)}
          <span className="tag-chip add"><I.Plus size={10}/>&nbsp;tag</span>
        </div>
      </div>

      {hasGallery ? (
        <Section title="Gallery" count={ov.gallery.length}>
          <div className="gallery-grid">
            {ov.gallery.map((g, i) => (
              <div key={i} className="gallery-cell" style={{
                aspectRatio: '4/3',
                background: 'var(--bg)',
                border: '1px solid var(--line)',
                borderRadius: 'var(--r-sm)',
                overflow: 'hidden',
                position: 'relative',
                cursor: 'zoom-in',
              }}>
                <AssetThumb asset={{ thumb: g }}/>
                <div style={{
                  position: 'absolute', bottom: 0, left: 0, right: 0,
                  padding: '14px 6px 4px',
                  background: 'linear-gradient(to top, rgba(0,0,0,0.7), transparent)',
                  fontSize: 'var(--fs-small)', color: 'rgba(255,255,255,0.85)',
                  fontFamily: 'var(--mono)',
                }}>{g.caption}</div>
              </div>
            ))}
          </div>
        </Section>
      ) : null}

      <Section title="Description">
        <div style={{ fontSize: 'var(--fs-ui)', color: 'var(--text-2)', lineHeight: 1.55, whiteSpace: 'pre-wrap' }}>
          {ov.description || 'No description available.'}
        </div>
        <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 8, fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>
          <I.Download size={10}/>
          Scraped from source · <span className="mono">{formatDate(ov.archivedAt)}</span>
          <span style={{ marginLeft: 'auto' }}>
            <button className="tbtn" style={{ fontSize: 'var(--fs-small)', color: 'var(--text-3)' }}>
              <I.Download size={10}/> Re-scrape
            </button>
          </span>
        </div>
      </Section>

      <Section title="Team notes">
        {editingNotes ? (
          <>
            <textarea
              autoFocus
              value={teamNotes}
              onChange={e => setTeamNotes(e.target.value)}
              placeholder="Context for the team — why we picked this, caveats, who approved it…"
              style={{
                width: '100%', minHeight: 80,
                background: 'var(--bg)',
                border: '1px solid var(--line-2)',
                borderRadius: 'var(--r-sm)',
                padding: 8,
                fontSize: 'var(--fs-ui)', fontFamily: 'var(--sans)',
                color: 'var(--text)',
                resize: 'vertical', outline: 0,
              }}/>
            <div style={{ display: 'flex', gap: 6, marginTop: 8, justifyContent: 'flex-end' }}>
              <button className="tbtn" onClick={() => { setTeamNotes(ov.teamNotes || ''); setEditingNotes(false); }}>Cancel</button>
              <button className="tbtn primary" onClick={() => setEditingNotes(false)}>Save</button>
            </div>
          </>
        ) : teamNotes ? (
          <div
            onClick={() => setEditingNotes(true)}
            style={{
              fontSize: 'var(--fs-ui)', color: 'var(--text-2)', lineHeight: 1.55,
              padding: 8, margin: '-8px',
              borderRadius: 'var(--r-sm)', cursor: 'text',
            }}
            className="hoverable">
            {teamNotes}
            <div style={{ marginTop: 8, fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>
              <span className="mono">thomas</span> · 2d ago — click to edit
            </div>
          </div>
        ) : (
          <button
            className="tbtn"
            onClick={() => setEditingNotes(true)}
            style={{
              width: '100%', border: '1px dashed var(--line-2)',
              padding: '14px 10px', color: 'var(--text-4)',
              justifyContent: 'center',
            }}>
            <I.Plus size={12}/> Add a note for your team
          </button>
        )}
      </Section>

      <Section title="Publisher" defaultOpen={false}>
        {ov.publisher ? (
          <div className="row-item" style={{ padding: 4 }}>
            <span className="swatch">{ov.publisher.name[0]}</span>
            <div style={{ minWidth: 0, flex: 1 }}>
              <div style={{ color: 'var(--text)', fontSize: 'var(--fs-ui)' }}>{ov.publisher.name}</div>
              {ov.publisher.url ? (
                <a style={{ fontSize: 'var(--fs-small)', color: 'var(--accent)', textDecoration: 'none', fontFamily: 'var(--mono)' }}>
                  {ov.publisher.url} ↗
                </a>
              ) : null}
            </div>
            {ov.publisher.assets ? <span className="row-meta">{ov.publisher.assets} assets</span> : null}
          </div>
        ) : <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>Unknown publisher.</div>}
      </Section>

      <Section title="Versions" count={asset.versions.length} defaultOpen={false}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {asset.versions.map(v => (
            <div key={v.v} className={`v-item ${v.current ? 'current' : ''}`}>
              <div style={{ minWidth: 0, flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span className="v-num">{v.v}</span>
                  {v.current ? <span style={{ fontSize: 'var(--fs-small)', color: 'var(--accent)', fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.06em' }}>current</span> : null}
                  <span className="v-date">{formatDate(v.date)}</span>
                </div>
                <div className="v-notes">{v.notes} · <span className="mono" style={{ color: 'var(--text-4)' }}>{v.files} files</span></div>
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Files" count={asset.files} defaultOpen={false}>
        <div className="tree">
          {asset.fileTree.map((n, i) => <TreeNode key={i} node={n}/>)}
        </div>
        <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
          <button className="tbtn" style={{ border: '1px solid var(--line)' }}>
            <I.Download size={11}/> Download original
          </button>
          {asset.type === 'unitypackage'
            ? <button className="tbtn" style={{ border: '1px solid var(--line)' }}><I.Package size={11}/> Re-extract</button>
            : null}
        </div>
      </Section>

      <Section title="Licence" defaultOpen={false}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <LicencePill licence={asset.licence}/>
          <span className="mono" style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>
            confidence: {lic.status === 'unk' ? 'none' : '92%'}
          </span>
        </div>
        <LicenceAttrs attrs={lic.attrs}/>
        <div className="lic-text">{asset.licenceText}</div>
      </Section>

      <Section title="Projects" count={asset.projects.length} defaultOpen={false}>
        {asset.projects.length === 0
          ? <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)', padding: '4px 0' }}>Not pinned to any project.</div>
          : asset.projects.map(pn => {
              const p = PROJECTS.find(x => x.name === pn) || { name: pn, colour: 'var(--line-2)' };
              return (
                <div key={pn} className="row-item">
                  <span className="swatch" style={{ background: p.colour, color: 'var(--bg)' }}>{pn[0]}</span>
                  <div>
                    <div style={{ color: 'var(--text)', fontSize: 'var(--fs-ui)' }}>{pn}</div>
                    <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>pinned · thomas</div>
                  </div>
                  <span className="row-meta">v{asset.version}</span>
                </div>
              );
            })}
      </Section>

      <Section title="Collections" count={asset.collections.length} defaultOpen={false}>
        {asset.collections.length === 0
          ? <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)', padding: '4px 0' }}>Not in any collection.</div>
          : asset.collections.map(c => (
            <div key={c} className="row-item">
              <span className="swatch"><I.Collections size={11} stroke={1.5}/></span>
              <div style={{ color: 'var(--text)', fontSize: 'var(--fs-ui)' }}>{c}</div>
            </div>
          ))}
      </Section>

      <Section title="Source & archive" defaultOpen={false}>
        <dl className="kv">
          <dt>Origin</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>{asset.source}</dd>
          {ov.originUrl ? (<><dt>URL</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)', color: 'var(--accent)', wordBreak: 'break-all' }}>{ov.originUrl}</dd></>) : null}
          <dt>Archived</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)' }}>{formatDate(ov.archivedAt)} · description, gallery & licence text saved locally</dd>
          <dt>Hash</dt><dd className="mono" style={{ fontSize: 'var(--fs-small)', color: 'var(--text-3)' }}>sha256:{asset.id.padEnd(6, '0')}f4a2…e91c</dd>
        </dl>
      </Section>

      <Section title="Audit" defaultOpen={false}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, fontSize: 'var(--fs-small)' }}>
          <div style={{ color: 'var(--text-2)' }}>
            <span className="mono" style={{ color: 'var(--text-4)' }}>{formatDate(asset.imported)}</span>
            &nbsp;· thomas · <span style={{ color: 'var(--text-3)' }}>imported (v{asset.version})</span>
          </div>
          <div style={{ color: 'var(--text-2)' }}>
            <span className="mono" style={{ color: 'var(--text-4)' }}>3w ago</span>
            &nbsp;· thomas · <span style={{ color: 'var(--text-3)' }}>added tags: {asset.tags.slice(0,2).join(', ')}</span>
          </div>
          <a style={{ color: 'var(--accent)', fontSize: 'var(--fs-small)', textDecoration: 'none' }}>View full audit →</a>
        </div>
      </Section>

      <div style={{ height: 40 }}/>
    </div>
  );
}

function DetailPanel({ asset, onClose, onPrev, onNext, onExpand, index, total }) {
  const [width, setWidth] = useStD(() => {
    const saved = parseInt(localStorage.getItem('damson.detailW'), 10);
    return Number.isFinite(saved) ? saved : 440;
  });
  const draggingRef = React.useRef(false);

  React.useEffect(() => {
    function onMove(e) {
      if (!draggingRef.current) return;
      // Panel is anchored to the right edge — width grows as cursor moves left.
      const next = Math.max(320, Math.min(window.innerWidth * 0.8, window.innerWidth - e.clientX));
      setWidth(next);
    }
    function onUp() {
      if (!draggingRef.current) return;
      draggingRef.current = false;
      document.body.classList.remove('detail-resizing');
      document.querySelector('.detail-resizer')?.classList.remove('dragging');
      // Persist
      localStorage.setItem('damson.detailW', String(Math.round(widthRef.current)));
    }
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, []);

  // Keep latest width available to the mouseup handler without re-binding listeners.
  const widthRef = React.useRef(width);
  React.useEffect(() => { widthRef.current = width; }, [width]);

  function startDrag(e) {
    draggingRef.current = true;
    document.body.classList.add('detail-resizing');
    e.currentTarget.classList.add('dragging');
    e.preventDefault();
  }

  function resetWidth(e) {
    // Double-click resets to default 440.
    e.preventDefault();
    setWidth(440);
    localStorage.setItem('damson.detailW', '440');
  }

  if (!asset) return null;
  return (
    <div className="detail-panel" style={{ '--detail-w': `${width}px` }}>
      <div
        className="detail-resizer"
        onMouseDown={startDrag}
        onDoubleClick={resetWidth}
        title="Drag to resize · double-click to reset"
        role="separator"
        aria-orientation="vertical"
      />
      <div className="detail-head">
        <button className="nav-btn" onClick={onPrev} title="Previous (↑)"><I.ChevronLeft size={14}/></button>
        <button className="nav-btn" onClick={onNext} title="Next (↓)"><I.ChevronRight size={14}/></button>
        <span className="counter">{index + 1} / {total}</span>
        <div className="spacer"/>
        <button className="nav-btn" title="Open as full page" onClick={onExpand}><I.Expand size={13}/></button>
        <button className="nav-btn" title="More"><I.MoreH size={14}/></button>
        <button className="nav-btn" title="Close (esc)" onClick={onClose}><I.X size={14}/></button>
      </div>
      <DetailBody asset={asset}/>
    </div>
  );
}

function FullPage({ asset, onClose, onCollapse }) {
  return (
    <div className="fullpage">
      <div className="fullpage-head">
        <button className="nav-btn" onClick={onClose} title="Back to library"><I.ChevronLeft size={14}/></button>
        <div className="crumb">
          <span>Library</span>
          <I.ChevronRight size={11}/>
          <span className="on">{asset.name}</span>
        </div>
        <div style={{ flex: 1 }}/>
        <button className="tbtn" onClick={onCollapse} title="Back to side panel">
          <I.Collapse size={12}/> Side panel
        </button>
      </div>
      <div className="fullpage-body">
        <div className="fullpage-preview">
          <div className="pv-box">
            <AssetThumb asset={asset} large/>
          </div>
        </div>
        <div className="fullpage-side">
          <DetailBody asset={asset}/>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { DetailPanel, FullPage });
