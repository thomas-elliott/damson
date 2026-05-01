// Projects & Collections views + dialogs.
const { useState: useStP } = React;

// ---------- Licence bar (used in project rows) ----------
function LicenceBar({ byStatus, compact = false }) {
  const total = byStatus.ok + byStatus.warn + byStatus.err + byStatus.unk;
  if (total === 0) return <span style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>empty</span>;
  const segs = [
    { k: 'ok', c: 'var(--ok)', v: byStatus.ok },
    { k: 'warn', c: 'var(--warn)', v: byStatus.warn },
    { k: 'err', c: 'var(--err)', v: byStatus.err },
    { k: 'unk', c: 'var(--unk)', v: byStatus.unk },
  ].filter(s => s.v > 0);
  return (
    <div className="lic-bar" style={{ width: compact ? 80 : 140 }}>
      {segs.map(s => (
        <span key={s.k} style={{ background: s.c, flex: s.v, height: '100%' }} title={`${s.v} ${s.k}`}/>
      ))}
    </div>
  );
}

function SyncPill({ sync }) {
  if (sync === 'synced') return <span className="lic-pill ok"><span className="dot"/>Synced</span>;
  if (sync === 'drift') return <span className="lic-pill warn"><span className="dot"/>Manifest drift</span>;
  return <span className="lic-pill unk"><span className="dot"/>No manifest</span>;
}

// ---------- Projects index (list) ----------
function ProjectsIndex({ indexLayout, onOpen, onNew }) {
  return (
    <>
      <div className="toolbar">
        <div className="breadcrumb">Projects <span className="count">{PROJECTS.length}</span></div>
        <div className="toolbar-spacer"/>
        <button className="tbtn primary" onClick={onNew}><I.Plus size={12}/> New project</button>
      </div>
      <div className="grid-wrap">
        <div className="page-head" style={{ maxWidth: 1100 }}>
          <h1>Projects</h1>
          <p>A project pins specific asset versions to a game. Damson can write a manifest to your repo so builds always pull the right files.</p>
        </div>
        {indexLayout === 'cards'
          ? <ProjectsCards onOpen={onOpen}/>
          : <ProjectsList onOpen={onOpen}/>}
      </div>
    </>
  );
}

function ProjectsList({ onOpen }) {
  return (
    <div className="proj-list" style={{ maxWidth: 1100, margin: '0 auto' }}>
      <div className="proj-list-head">
        <div>Project</div>
        <div>Engine</div>
        <div>Assets</div>
        <div>Licence mix</div>
        <div>Manifest</div>
        <div style={{ textAlign: 'right' }}>Updated</div>
      </div>
      {PROJECTS.map(p => {
        const s = projectStats(p.id);
        return (
          <div key={p.id} className="proj-row" onClick={() => onOpen(p.id)}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
              <span className="proj-glyph" style={{ background: p.colour }}>{p.glyph}</span>
              <div style={{ minWidth: 0 }}>
                <div className="proj-name">{p.name}</div>
                <div className="proj-path mono">{p.path}</div>
              </div>
            </div>
            <div className="proj-eng">{p.engine}</div>
            <div className="proj-count mono">{s.count} · {formatSize(s.size)}</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <LicenceBar byStatus={s.byStatus}/>
              {s.byStatus.err > 0 ? <span className="lic-pill err" style={{ fontFamily: 'var(--sans)' }}>{s.byStatus.err} violation</span>
                : s.byStatus.warn > 0 ? <span className="lic-pill warn" style={{ fontFamily: 'var(--sans)' }}>{s.byStatus.warn} review</span>
                : null}
            </div>
            <div><SyncPill sync={p.sync}/></div>
            <div style={{ textAlign: 'right', color: 'var(--text-3)', fontSize: 'var(--fs-small)' }}>{formatDate(p.updated)}</div>
          </div>
        );
      })}
    </div>
  );
}

function ProjectsCards({ onOpen }) {
  return (
    <div className="proj-cards" style={{ maxWidth: 1100, margin: '0 auto' }}>
      {PROJECTS.map(p => {
        const s = projectStats(p.id);
        return (
          <div key={p.id} className="proj-card" onClick={() => onOpen(p.id)}>
            <div className="proj-card-head">
              <span className="proj-glyph lg" style={{ background: p.colour }}>{p.glyph}</span>
              <div style={{ minWidth: 0, flex: 1 }}>
                <div className="proj-name" style={{ fontSize: 'var(--fs-ui)' }}>{p.name}</div>
                <div className="proj-path mono">{p.path}</div>
              </div>
              <SyncPill sync={p.sync}/>
            </div>
            <div className="proj-card-desc">{p.description}</div>
            <div className="proj-card-stats">
              <div><span className="mono">{s.count}</span><span className="muted">&nbsp;assets</span></div>
              <div><span className="mono">{formatSize(s.size)}</span></div>
              <div><span className="mono">{p.engine}</span></div>
            </div>
            <LicenceBar byStatus={s.byStatus} compact={false}/>
          </div>
        );
      })}
      <div className="proj-card new" onClick={() => {}}>
        <div style={{ textAlign: 'center' }}>
          <I.Plus size={20}/>
          <div style={{ fontSize: 'var(--fs-small)', marginTop: 6 }}>New project</div>
        </div>
      </div>
    </div>
  );
}

// ---------- Project detail ----------
function ProjectDetail({ projectId, onBack, onOpenAsset }) {
  const p = PROJECTS.find(x => x.id === projectId);
  const s = projectStats(projectId);
  const [tab, setTab] = useStP('assets');

  return (
    <>
      <div className="toolbar">
        <button className="nav-btn" onClick={onBack} title="Back"><I.ChevronLeft size={14}/></button>
        <div className="breadcrumb" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ color: 'var(--text-3)', cursor: 'pointer' }} onClick={onBack}>Projects</span>
          <I.ChevronRight size={11} stroke={1.5}/>
          <span>{p.name}</span>
        </div>
        <div className="toolbar-spacer"/>
        <button className="tbtn"><I.Download size={12}/> Write manifest</button>
        <button className="tbtn"><I.Plus size={12}/> Add assets</button>
      </div>

      <div className="grid-wrap">
        <div className="proj-header">
          <div className="proj-header-top">
            <span className="proj-glyph xl" style={{ background: p.colour }}>{p.glyph}</span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <h1 className="proj-title">{p.name}</h1>
              <div className="proj-meta">
                <span className="mono">{p.path}</span>
                <span className="sep">·</span>
                <span>{p.engine}</span>
                <span className="sep">·</span>
                <span>updated {formatDate(p.updated)}</span>
              </div>
              <p className="proj-desc">{p.description}</p>
            </div>
            <SyncPill sync={p.sync}/>
          </div>

          <div className="proj-stat-grid">
            <div className="proj-stat">
              <div className="label">Assets pinned</div>
              <div className="val mono">{s.count}</div>
              <div className="sub">{s.files.toLocaleString()} files · {formatSize(s.size)}</div>
            </div>
            <div className="proj-stat">
              <div className="label">Licence status</div>
              <div className="val mono" style={{ color: s.byStatus.err > 0 ? 'var(--err)' : s.byStatus.warn > 0 ? 'var(--warn)' : 'var(--ok)' }}>
                {s.byStatus.err > 0 ? 'At risk'
                  : s.byStatus.warn > 0 ? 'Review'
                  : s.count === 0 ? '—'
                  : 'Clear'}
              </div>
              <div style={{ marginTop: 6 }}><LicenceBar byStatus={s.byStatus}/></div>
            </div>
            <div className="proj-stat">
              <div className="label">Manifest</div>
              <div className="val mono" style={{ fontSize: 'var(--fs-h3)' }}>
                {p.manifestLast ? formatDate(p.manifestLast) : 'Never written'}
              </div>
              <div className="sub">
                {p.sync === 'synced' ? 'Up to date with library'
                  : p.sync === 'drift' ? '2 assets have newer versions available'
                  : 'No manifest file yet'}
              </div>
            </div>
            <div className="proj-stat">
              <div className="label">Updates available</div>
              <div className="val mono" style={{ fontSize: 'var(--fs-h3)' }}>
                {p.sync === 'drift' ? '2' : '0'}
              </div>
              <div className="sub">of {s.count} pinned assets</div>
            </div>
          </div>

          <div className="tab-row">
            {[
              ['assets', 'Assets', s.count],
              ['manifest', 'Manifest', null],
              ['activity', 'Activity', 12],
            ].map(([k, label, count]) => (
              <button key={k} className={`tab ${tab === k ? 'on' : ''}`} onClick={() => setTab(k)}>
                {label}{count != null ? <span className="tab-count">{count}</span> : null}
              </button>
            ))}
          </div>
        </div>

        {tab === 'assets' ? (
          <ProjectAssetsPanel project={p} items={s.items} onOpenAsset={onOpenAsset}/>
        ) : tab === 'manifest' ? (
          <ProjectManifest project={p} items={s.items}/>
        ) : (
          <ProjectActivity project={p}/>
        )}
      </div>
    </>
  );
}

function ProjectAssetsPanel({ project, items, onOpenAsset }) {
  if (items.length === 0) {
    return (
      <div className="empty" style={{ padding: 60 }}>
        <div style={{ fontSize: 'var(--fs-ui)', color: 'var(--text-2)', marginBottom: 4 }}>No assets pinned yet</div>
        <div style={{ fontSize: 'var(--fs-small)', marginBottom: 14 }}>Add library assets here, or drop files directly onto this page.</div>
        <button className="tbtn primary"><I.Plus size={12}/> Add from library</button>
      </div>
    );
  }
  return (
    <table className="list" style={{ marginTop: 4 }}>
      <thead>
        <tr>
          <th style={{ width: '30%' }}>Asset</th>
          <th>Pinned</th>
          <th>Latest</th>
          <th>Licence</th>
          <th style={{ textAlign: 'right' }}>Size</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {items.map(a => {
          const latest = a.versions[0].v;
          const hasUpdate = latest !== a.version && a.version && a.version !== '—';
          return (
            <tr key={a.id} onClick={() => onOpenAsset(a)}>
              <td className="name">
                <span className="mini-thumb"><TypeIcon type={a.type} size={12} stroke={1.5}/></span>
                {a.name}
              </td>
              <td className="mono">{a.version}</td>
              <td className="mono" style={{ color: hasUpdate ? 'var(--warn)' : 'var(--text-3)' }}>
                {latest} {hasUpdate ? '↑' : ''}
              </td>
              <td><LicencePill licence={a.licence} compact/></td>
              <td style={{ textAlign: 'right' }} className="mono">{formatSize(a.size)}</td>
              <td>
                {hasUpdate
                  ? <span style={{ fontSize: 'var(--fs-small)', color: 'var(--warn)' }}>Update available</span>
                  : <span style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)' }}>Up to date</span>}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function ProjectManifest({ project, items }) {
  const manifestText = JSON.stringify({
    project: project.name,
    engine: project.engine,
    path: project.path,
    generated: project.manifestLast,
    assets: items.map(a => ({
      id: a.id, name: a.name,
      version: a.version, licence: a.licence,
      hash: 'sha256:' + a.id.padEnd(6, '0') + 'f4a2e91c',
      size: a.size,
    })),
  }, null, 2);

  return (
    <div style={{ maxWidth: 1100, margin: '0 auto', padding: '0 14px' }}>
      <div className="manifest-head">
        <div>
          <div style={{ fontSize: 'var(--fs-ui)', color: 'var(--text)', fontWeight: 500 }}>damson.manifest.json</div>
          <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)', marginTop: 2 }}>
            Written to <span className="mono">{project.path}/.damson/</span>
            {project.manifestLast ? ` · last ${formatDate(project.manifestLast)}` : ' · never written'}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="tbtn" style={{ border: '1px solid var(--line)' }}><I.Copy size={11}/> Copy</button>
          <button className="tbtn primary"><I.Download size={11}/> Write manifest</button>
        </div>
      </div>
      <pre className="manifest-code">{manifestText}</pre>
    </div>
  );
}

function ProjectActivity({ project }) {
  const events = [
    { t: '2h ago', who: 'thomas', what: 'pinned', target: 'POLYGON - Fantasy Kingdom v1.12.3' },
    { t: '1d ago', who: 'thomas', what: 'wrote manifest', target: 'damson.manifest.json' },
    { t: '3d ago', who: 'mira', what: 'added tag', target: 'environment → POLYGON - Fantasy Kingdom' },
    { t: '1w ago', who: 'thomas', what: 'bumped version', target: 'DOTween Pro → v1.0.381' },
    { t: '2w ago', who: 'mira', what: 'created project', target: project.name },
  ];
  return (
    <div style={{ maxWidth: 700, margin: '0 auto', padding: '8px 14px' }}>
      {events.map((e, i) => (
        <div key={i} className="activity-row">
          <span className="mono activity-t">{e.t}</span>
          <span className="activity-who">{e.who}</span>
          <span className="activity-what">{e.what}</span>
          <span className="activity-target">{e.target}</span>
        </div>
      ))}
    </div>
  );
}

// ---------- Collection detail (header above grid) ----------
function CollectionHeader({ collection, onBack, onEdit }) {
  const c = collection;
  return (
    <div className="col-header">
      <button className="nav-btn" onClick={onBack} title="All collections"><I.ChevronLeft size={14}/></button>
      <I.Collections size={16} stroke={1.5}/>
      <div style={{ flex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ color: 'var(--text)', fontSize: 'var(--fs-ui)', fontWeight: 500 }}>{c.name}</span>
          {c.kind === 'smart'
            ? <span className="smart-badge"><I.Sliders size={10}/> Smart</span>
            : null}
        </div>
        {c.kind === 'smart' ? (
          <div style={{ fontSize: 'var(--fs-small)', color: 'var(--text-4)', marginTop: 1, fontFamily: 'var(--mono)' }}>
            {describeSmart(c.query)}
          </div>
        ) : null}
      </div>
      <button className="tbtn" onClick={onEdit}>
        {c.kind === 'smart' ? <><I.Sliders size={11}/> Edit query</> : <><I.Plus size={11}/> Add assets</>}
      </button>
    </div>
  );
}

function describeSmart(q) {
  const parts = [];
  if (q.licence) parts.push('licence ∈ ' + q.licence.join(','));
  if (q.recent) parts.push('imported < ' + q.recent + 'd');
  if (q.minSize) parts.push('size > ' + formatSize(q.minSize));
  if (q.orphan) parts.push('project = ∅');
  return parts.join(' · ');
}

// ---------- Dialogs ----------
function Dialog({ open, onClose, children, width = 440 }) {
  if (!open) return null;
  return (
    <div className="dlg-scrim" onClick={onClose}>
      <div className="dlg" style={{ width }} onClick={e => e.stopPropagation()}>
        {children}
      </div>
    </div>
  );
}

function NewProjectDialog({ open, onClose, onCreate }) {
  const [name, setName] = useStP('');
  const [path, setPath] = useStP('~/Dev/');
  const [engine, setEngine] = useStP('Unity 2023.3');
  const [colour, setColour] = useStP('oklch(0.7 0.14 244)');
  React.useEffect(() => {
    if (open) { setName(''); setPath('~/Dev/'); setEngine('Unity 2023.3'); setColour('oklch(0.7 0.14 244)'); }
  }, [open]);

  const colours = [
    'oklch(0.7 0.14 244)', 'oklch(0.7 0.12 155)', 'oklch(0.68 0.17 28)',
    'oklch(0.78 0.14 85)', 'oklch(0.68 0.18 330)', 'oklch(0.66 0.14 280)',
  ];

  return (
    <Dialog open={open} onClose={onClose}>
      <div className="dlg-head">
        <h3>New project</h3>
        <button className="nav-btn" onClick={onClose}><I.X size={13}/></button>
      </div>
      <div className="dlg-body">
        <div className="field">
          <label>Name</label>
          <input autoFocus value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Prologue"/>
        </div>
        <div className="field">
          <label>Project folder</label>
          <input value={path} onChange={e => setPath(e.target.value)} className="mono" style={{ fontSize: 'var(--fs-small)' }}/>
          <div className="hint">A <span className="mono">.damson/</span> folder will be created here for the manifest.</div>
        </div>
        <div className="field">
          <label>Engine</label>
          <select value={engine} onChange={e => setEngine(e.target.value)}>
            <option>Unity 2023.3</option>
            <option>Unity 6</option>
            <option>Godot 4.3</option>
            <option>Unreal 5.4</option>
            <option>Custom</option>
          </select>
        </div>
        <div className="field">
          <label>Colour</label>
          <div className="swatch-row">
            {colours.map(c => (
              <button key={c}
                className={`swatch-btn ${colour === c ? 'on' : ''}`}
                style={{ background: c }}
                onClick={() => setColour(c)}/>
            ))}
          </div>
        </div>
      </div>
      <div className="dlg-foot">
        <button className="tbtn" onClick={onClose}>Cancel</button>
        <button className="tbtn primary" disabled={!name.trim()}
          onClick={() => onCreate({ name: name.trim(), path, engine, colour })}>
          Create project
        </button>
      </div>
    </Dialog>
  );
}

function NewCollectionDialog({ open, onClose, onCreate }) {
  const [name, setName] = useStP('');
  const [kind, setKind] = useStP('manual');
  React.useEffect(() => { if (open) { setName(''); setKind('manual'); } }, [open]);

  return (
    <Dialog open={open} onClose={onClose}>
      <div className="dlg-head">
        <h3>New collection</h3>
        <button className="nav-btn" onClick={onClose}><I.X size={13}/></button>
      </div>
      <div className="dlg-body">
        <div className="field">
          <label>Name</label>
          <input autoFocus value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Boss fight audio"/>
        </div>
        <div className="field">
          <label>Type</label>
          <div className="kind-opts">
            <button className={`kind-opt ${kind === 'manual' ? 'on' : ''}`} onClick={() => setKind('manual')}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                <I.Collections size={12}/>
                <strong>Manual</strong>
              </div>
              <div className="hint">You add assets one by one.</div>
            </button>
            <button className={`kind-opt ${kind === 'smart' ? 'on' : ''}`} onClick={() => setKind('smart')}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                <I.Sliders size={12}/>
                <strong>Smart</strong>
              </div>
              <div className="hint">Saved filter — auto-updates as assets change.</div>
            </button>
          </div>
        </div>
        {kind === 'smart' ? (
          <div className="field">
            <label>Query</label>
            <div style={{ padding: 10, background: 'var(--bg)', border: '1px solid var(--line)', borderRadius: 6, fontSize: 'var(--fs-small)', color: 'var(--text-3)' }}>
              The current library filters will be saved as this collection's query. You can edit it afterwards.
            </div>
          </div>
        ) : null}
      </div>
      <div className="dlg-foot">
        <button className="tbtn" onClick={onClose}>Cancel</button>
        <button className="tbtn primary" disabled={!name.trim()} onClick={() => onCreate({ name: name.trim(), kind })}>
          Create collection
        </button>
      </div>
    </Dialog>
  );
}

Object.assign(window, {
  ProjectsIndex, ProjectDetail, CollectionHeader,
  NewProjectDialog, NewCollectionDialog,
  ProjectsCards, ProjectsList, describeSmart,
  LicenceBar, SyncPill,
});
