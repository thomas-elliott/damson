// Nav rail — global navigation.
function NavRail({ route, setRoute, queueCount }) {
  const items = [
    { id: 'library', label: 'Library', Icon: I.Library },
    { id: 'projects', label: 'Projects', Icon: I.Projects },
    { id: 'collections', label: 'Collections', Icon: I.Collections },
    { id: 'queue', label: 'Import queue', Icon: I.Queue, badge: queueCount },
    { id: 'audit', label: 'Audit log', Icon: I.Audit },
  ];
  return (
    <nav className="rail" aria-label="Primary">
      <div className="brand" title="Damson"><span>D</span></div>
      {items.map(it => (
        <button key={it.id}
          className="rail-btn"
          aria-current={route === it.id ? 'page' : undefined}
          onClick={() => setRoute(it.id)}>
          <it.Icon size={16}/>
          {it.badge ? <span className="badge">{it.badge}</span> : null}
          <span className="tip">{it.label}</span>
        </button>
      ))}
      <div className="spacer"/>
      <button className="rail-btn"
        aria-current={route === 'settings' ? 'page' : undefined}
        onClick={() => setRoute('settings')}>
        <I.Settings size={16}/>
        <span className="tip">Settings</span>
      </button>
    </nav>
  );
}
Object.assign(window, { NavRail });
