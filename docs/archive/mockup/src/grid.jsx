// Grid + list view, toolbar, chip bar, selection bar.
const { useMemo: useMemoG } = React;

const SPDX_SHORT = {
  'Unity-Asset-Store-EULA': 'Unity EULA',
};

function LicencePill({ licence, compact = false }) {
  const data = LICENCES[licence] || LICENCES.Unknown;
  const label = compact
    ? (SPDX_SHORT[data.spdx] || data.spdx || '—')
    : (data.spdx || 'Unknown');
  return (
    <span className={`lic-pill ${data.status}`} title={licence}>
      <span className="dot"/>
      {label}
    </span>
  );
}

function AssetCard({ asset, selected, onClick, onShiftClick }) {
  return (
    <div
      className={`card ${selected ? 'selected' : ''}`}
      onClick={(e) => e.shiftKey || e.metaKey || e.ctrlKey ? onShiftClick(asset, e) : onClick(asset)}>
      <div className="thumb">
        <span className="type-badge">{asset.type === 'unitypackage' ? 'unitypkg' : asset.type}</span>
        {asset.needsReview ? <span className="review-dot" title="Needs review"/> : null}
        <AssetThumb asset={asset}/>
      </div>
      <div className="meta">
        <div className="name-col">
          <div className="name">{asset.name}</div>
          <div className="sub">
            <span>{formatSize(asset.size)}</span>
            <span className="dot"/>
            <span>{formatDate(asset.imported)}</span>
          </div>
        </div>
        <LicencePill licence={asset.licence} compact/>
      </div>
    </div>
  );
}

function Toolbar({
  count, total, view, setView, sort, setSort,
  activeChips, removeChip, clearAll,
  browseTarget, onBrowse,
}) {
  return (
    <div className="toolbar">
      <div className="breadcrumb">
        Library <span className="count">{count} of {total}</span>
      </div>

      {activeChips.length ? (
        <>
          <span className="divider-v"/>
          {activeChips.map(c => (
            <span key={c.key} className="chip">
              <span className="muted">{c.group}:</span><strong>{c.label}</strong>
              <button onClick={() => removeChip(c)}><I.X size={10}/></button>
            </span>
          ))}
          <button className="tbtn" onClick={clearAll} style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>
            Clear all
          </button>
        </>
      ) : null}

      <div className="toolbar-spacer"/>

      {browseTarget ? (
        <>
          <button className="tbtn browse-pill" onClick={() => onBrowse(browseTarget.id)} title={`Browse the ${browseTarget.files.toLocaleString()} files in ${browseTarget.name}`}>
            <I.Browse size={12}/> Browse files
          </button>
          <span className="divider-v"/>
        </>
      ) : null}

      <button className="tbtn" onClick={() => setSort(sort === 'name' ? 'imported' : 'name')}>
        <I.SortAsc size={12}/>
        Sort: <span style={{ color: 'var(--text)' }}>{sort === 'name' ? 'Name' : 'Imported'}</span>
      </button>
      <span className="divider-v"/>
      <div className="seg">
        <button className={view === 'grid' ? 'on' : ''} onClick={() => setView('grid')} title="Grid view">
          <I.Grid size={13}/>
        </button>
        <button className={view === 'list' ? 'on' : ''} onClick={() => setView('list')} title="List view">
          <I.List size={13}/>
        </button>
      </div>
    </div>
  );
}

function GridView({ assets, selected, onOpen, onSelect }) {
  if (assets.length === 0) {
    return (
      <div className="empty">
        <div style={{ fontSize: 'var(--fs-ui)', color: 'var(--text-2)', marginBottom: 4 }}>No assets match these filters</div>
        <div style={{ fontSize: 'var(--fs-small)' }}>Clear a filter to widen the search, or drop files here to import.</div>
      </div>
    );
  }
  return (
    <div className="grid">
      {assets.map(a => (
        <AssetCard key={a.id} asset={a}
          selected={selected.has(a.id)}
          onClick={onOpen}
          onShiftClick={onSelect}/>
      ))}
    </div>
  );
}

function ListView({ assets, selected, onOpen }) {
  return (
    <table className="list">
      <thead>
        <tr>
          <th style={{ width: '30%' }}>Name</th>
          <th>Type</th>
          <th>Source</th>
          <th>Licence</th>
          <th>Version</th>
          <th style={{ textAlign: 'right' }}>Size</th>
          <th>Imported</th>
        </tr>
      </thead>
      <tbody>
        {assets.map(a => (
          <tr key={a.id} className={selected.has(a.id) ? 'selected' : ''} onClick={() => onOpen(a)}>
            <td className="name">
              <span className="mini-thumb"><TypeIcon type={a.type} size={12} stroke={1.5}/></span>
              {a.name}
            </td>
            <td>{a.type}</td>
            <td>{a.source}</td>
            <td><LicencePill licence={a.licence} compact/></td>
            <td className="mono">{a.version}</td>
            <td style={{ textAlign: 'right' }} className="mono">{formatSize(a.size)}</td>
            <td>{formatDate(a.imported)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SelectionBar({ count, onClear }) {
  if (count < 2) return null;
  return (
    <div className="sel-bar">
      <span className="count">{count} selected</span>
      <span className="divider"/>
      <button className="btn"><I.Projects size={12}/> Add to project</button>
      <button className="btn"><I.Collections size={12}/> Add to collection</button>
      <button className="btn"><I.Tag size={12}/> Tag</button>
      <button className="btn" style={{ color: 'var(--err)' }}><I.Trash size={12}/> Delete</button>
      <span className="divider"/>
      <button className="btn" onClick={onClear}><I.X size={12}/></button>
    </div>
  );
}

Object.assign(window, { AssetCard, LicencePill, Toolbar, GridView, ListView, SelectionBar });
