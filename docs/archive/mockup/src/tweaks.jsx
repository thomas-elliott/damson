// Tweaks panel — floating, hidden until host activates.
const { useState: useStT, useEffect: useEfT } = React;

function Tweaks({ tweaks, setTweaks }) {
  const [active, setActive] = useStT(false);

  useEfT(() => {
    const onMsg = (ev) => {
      const d = ev.data;
      if (!d || typeof d !== 'object') return;
      if (d.type === '__activate_edit_mode') setActive(true);
      if (d.type === '__deactivate_edit_mode') setActive(false);
    };
    window.addEventListener('message', onMsg);
    // Announce availability AFTER listener is live.
    window.parent.postMessage({ type: '__edit_mode_available' }, '*');
    return () => window.removeEventListener('message', onMsg);
  }, []);

  if (!active) return null;

  const update = (patch) => {
    const next = { ...tweaks, ...patch };
    setTweaks(next);
    window.parent.postMessage({ type: '__edit_mode_set_keys', edits: patch }, '*');
  };

  return (
    <div className="tweaks">
      <div className="tweaks-head">
        <span><I.Sliders size={11}/>&nbsp;&nbsp;Tweaks</span>
        <button onClick={() => setActive(false)}><I.X size={12}/></button>
      </div>
      <div className="tweaks-body">
        <div className="tweak-row">
          <label>Accent</label>
          <div className="seg-full">
            {['blue', 'yellow'].map(a => (
              <button key={a} className={tweaks.accent === a ? 'on' : ''} onClick={() => update({ accent: a })}>
                <span className="swatch-dot" style={{
                  background: a === 'blue' ? 'oklch(0.70 0.14 244)' : 'oklch(0.82 0.15 86)'
                }}/>
                {a[0].toUpperCase() + a.slice(1)}
              </button>
            ))}
          </div>
        </div>

        <div className="tweak-row">
          <label>Theme</label>
          <div className="seg-full">
            <button className={tweaks.theme === 'dark' ? 'on' : ''} onClick={() => update({ theme: 'dark' })}>
              <I.Moon size={11}/> Dark
            </button>
            <button className={tweaks.theme === 'light' ? 'on' : ''} onClick={() => update({ theme: 'light' })}>
              <I.Sun size={11}/> Light
            </button>
          </div>
        </div>

        <div className="tweak-row">
          <label>Sidebar width <span className="val">{tweaks.sidebarWidth}px</span></label>
          <input type="range" className="slider"
            min="200" max="340" step="2"
            value={tweaks.sidebarWidth}
            onChange={e => update({ sidebarWidth: parseInt(e.target.value, 10) })}/>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { Tweaks });
