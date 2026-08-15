// Filter sidebar — search + composable filters.
const { useState: useSt } = React;

function FGroupHead({ title, onClear, action, children }) {
  return (
    <div className="fgroup-head">
      <span>{title}</span>
      {action ? action : onClear ? <button onClick={onClear}>clear</button> : children}
    </div>
  );
}

function FRow({ active, onClick, icon, label, count, dot, check }) {
  return (
    <div className={`f-row ${active ? 'active' : ''}`} onClick={onClick} role="button" tabIndex={0}>
      {check ? (
        <span className="f-check">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        </span>
      ) : null}
      {dot ? <span className={`f-dot ${dot}`}/> : null}
      {icon ? <span className="f-icon">{icon}</span> : null}
      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{label}</span>
      {count != null ? <span className="f-count">{count}</span> : null}
    </div>
  );
}

function TagTreeNode({ node, depth = 0, active, toggle }) {
  const [open, setOpen] = useSt(depth < 1);
  const has = node.children && node.children.length > 0;
  return (
    <>
      <div className="f-row" style={{ paddingLeft: 8 + depth * 14 }} onClick={() => toggle(node.name)}>
        {has ? (
          <button onClick={(e) => { e.stopPropagation(); setOpen(!open); }} style={{ color: 'var(--text-4)', display: 'grid', placeItems: 'center' }}>
            {open ? <I.ChevronDown size={11}/> : <I.ChevronRight size={11}/>}
          </button>
        ) : <span style={{ width: 11 }}/>}
        <I.Tag size={12} stroke={1.5}/>
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} className={active.has(node.name) ? '' : ''}>
          {node.name}
        </span>
        <span className="f-count">{node.count}</span>
      </div>
      {has && open ? node.children.map(c => (
        <TagTreeNode key={c.name} node={c} depth={depth + 1} active={active} toggle={toggle}/>
      )) : null}
    </>
  );
}

function FilterSidebar({
  query, setQuery,
  filters, toggleFilter, clearFilters,
  width, onResizerDown,
  activeCollection, setActiveCollection, onNewCollection,
}) {
  return (
    <aside className="sidebar" style={{ width }}>
      {onResizerDown ? <div className="sidebar-resizer" onMouseDown={onResizerDown}/> : null}
      <div className="search">
        <div className="search-box">
          <I.Search size={13}/>
          <input
            placeholder="Search assets, tags, licences…"
            value={query}
            onChange={e => setQuery(e.target.value)}
          />
          <span className="kbd">⌘K</span>
        </div>
      </div>
      <div className="filter-scroll">
        <div className="fgroup">
          <FGroupHead title="Licence"/>
          {LICENCE_FILTERS.map(f => (
            <FRow key={f.id}
              active={filters.licence.has(f.id)}
              onClick={() => toggleFilter('licence', f.id)}
              dot={f.status}
              label={f.label}
              count={f.count}/>
          ))}
        </div>

        <div className="fgroup">
          <FGroupHead title="File type"/>
          {FILETYPE_FILTERS.map(f => (
            <FRow key={f.id}
              active={filters.type.has(f.id)}
              onClick={() => toggleFilter('type', f.id)}
              icon={<TypeIcon type={f.id} size={13} stroke={1.6}/>}
              label={f.label}
              count={f.count}/>
          ))}
        </div>

        <div className="fgroup">
          <FGroupHead title="Tags"/>
          {TAG_TREE.map(t => (
            <TagTreeNode key={t.name} node={t} active={filters.tag} toggle={(n) => toggleFilter('tag', n)}/>
          ))}
        </div>

        <div className="fgroup">
          <FGroupHead title="Projects"/>
          {PROJECTS.map(p => (
            <FRow key={p.id}
              active={filters.project.has(p.id)}
              onClick={() => toggleFilter('project', p.id)}
              icon={<span style={{ width: 8, height: 8, borderRadius: 2, background: p.colour, display: 'inline-block' }}/>}
              label={p.name}
              count={p.count}/>
          ))}
        </div>

        <div className="fgroup">
          <FGroupHead title="Collections"
            action={<button onClick={onNewCollection} title="New collection"><I.Plus size={11}/></button>}/>
          {COLLECTIONS.filter(c => c.kind === 'smart').map(c => (
            <div key={c.id}
              className={`sb-col ${activeCollection === c.id ? 'on' : ''}`}
              onClick={() => setActiveCollection(activeCollection === c.id ? null : c.id)}>
              <I.Sliders size={11} stroke={1.5} style={{ color: 'var(--accent)' }}/>
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.name}</span>
              <span className="count">{c.count}</span>
            </div>
          ))}
          <div style={{ height: 6 }}/>
          {COLLECTIONS.filter(c => c.kind === 'manual').slice(0, 8).map(c => (
            <div key={c.id}
              className={`sb-col ${activeCollection === c.id ? 'on' : ''}`}
              onClick={() => setActiveCollection(activeCollection === c.id ? null : c.id)}>
              <I.Folder size={11} stroke={1.5}/>
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.name}</span>
              <span className="count">{c.count}</span>
            </div>
          ))}
        </div>

        <div className="fgroup">
          <FGroupHead title="Source"/>
          {SOURCES.map(s => (
            <FRow key={s.id}
              active={filters.source.has(s.id)}
              onClick={() => toggleFilter('source', s.id)}
              icon={<I.ExternalLink size={11} stroke={1.5}/>}
              label={s.label}
              count={s.count}/>
          ))}
        </div>
      </div>
    </aside>
  );
}
Object.assign(window, { FilterSidebar });
