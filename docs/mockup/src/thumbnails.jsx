// Procedural SVG thumbnails — deterministic by seed/palette.

function Waveform({ seed = 1 }) {
  const bars = 56;
  let s = seed * 9301 + 49297;
  const rand = () => { s = (s * 9301 + 49297) % 233280; return s / 233280; };
  const heights = Array.from({ length: bars }, (_, i) => {
    const env = Math.sin((i / bars) * Math.PI) * 0.7 + 0.3;
    return 0.15 + rand() * 0.85 * env;
  });
  const w = 180, h = 180, bw = w / bars, gap = 1.5;
  return (
    <svg className="thumb-waveform" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <defs>
        <linearGradient id={`wg${seed}`} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="var(--accent)" stopOpacity="0.85"/>
          <stop offset="1" stopColor="var(--accent-2)" stopOpacity="0.55"/>
        </linearGradient>
      </defs>
      {heights.map((ht, i) => {
        const bh = ht * (h * 0.7);
        return <rect key={i} x={i * bw + gap / 2} y={(h - bh) / 2} width={bw - gap} height={bh}
          fill={`url(#wg${seed})`} rx="1" />;
      })}
      <line x1="0" x2={w} y1={h / 2} y2={h / 2} stroke="currentColor" strokeWidth="0.5" strokeOpacity="0.15" />
    </svg>
  );
}

function SpritePlaceholder({ palette = ['#888'], pattern = 'abstract' }) {
  const w = 180, h = 180;
  const [c0, c1, c2, c3] = [palette[0], palette[1] || palette[0], palette[2] || palette[0], palette[3] || palette[0]];

  if (pattern === 'landscape') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%" preserveAspectRatio="xMidYMid slice">
        <rect width={w} height={h} fill={c1} opacity="0.8" />
        <rect y={h * 0.55} width={w} height={h * 0.45} fill={c0} />
        <path d={`M0 ${h*0.55} L30 ${h*0.35} L55 ${h*0.5} L85 ${h*0.28} L115 ${h*0.48} L150 ${h*0.32} L180 ${h*0.5} L180 ${h*0.55} Z`} fill={c2} opacity="0.85"/>
        <circle cx={w*0.75} cy={h*0.22} r="14" fill={c3} opacity="0.6"/>
        <rect x="28" y={h*0.68} width="8" height="18" fill={c2} opacity="0.8"/>
        <rect x="44" y={h*0.72} width="10" height="14" fill={c2} opacity="0.6"/>
      </svg>
    );
  }
  if (pattern === 'character') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <rect width={w} height={h} fill={c1}/>
        <g transform={`translate(${w/2-30},${h/2-46})`}>
          <rect x="20" y="0" width="24" height="24" fill={c2}/>
          <rect x="16" y="24" width="32" height="36" fill={c0}/>
          <rect x="8" y="30" width="8" height="22" fill={c0}/>
          <rect x="48" y="30" width="8" height="22" fill={c0}/>
          <rect x="18" y="60" width="12" height="28" fill={c2}/>
          <rect x="34" y="60" width="12" height="28" fill={c2}/>
          <rect x="26" y="8" width="4" height="4" fill={c1}/>
          <rect x="34" y="8" width="4" height="4" fill={c1}/>
        </g>
      </svg>
    );
  }
  if (pattern === 'tiles') {
    const tiles = [];
    for (let y = 0; y < 6; y++) for (let x = 0; x < 6; x++) {
      const i = (x * 7 + y * 11) % 4;
      tiles.push(<rect key={`${x}-${y}`} x={x*30} y={y*30} width="30" height="30"
        fill={[c0, c1, c2, c3][i]} />);
    }
    return <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">{tiles}</svg>;
  }
  if (pattern === 'ui') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <rect width={w} height={h} fill={c2}/>
        <rect x="18" y="24" width="144" height="36" fill="none" stroke={c1} strokeWidth="1.5" rx="4"/>
        <rect x="24" y="30" width="44" height="4" fill={c1}/>
        <rect x="24" y="40" width="84" height="10" fill={c1} opacity="0.5"/>
        <rect x="18" y="72" width="68" height="28" fill={c1} rx="3"/>
        <rect x="94" y="72" width="68" height="28" fill="none" stroke={c1} strokeWidth="1.5" rx="3"/>
        <rect x="18" y="114" width="144" height="4" fill={c1} opacity="0.3"/>
        <rect x="18" y="124" width="100" height="4" fill={c1} opacity="0.3"/>
        <circle cx="30" cy="148" r="7" fill={c1}/>
        <rect x="44" y="145" width="80" height="6" fill={c1} opacity="0.5"/>
      </svg>
    );
  }
  if (pattern === 'portrait') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <rect width={w} height={h} fill={c2}/>
        <ellipse cx={w/2} cy={h*0.42} rx="32" ry="38" fill={c1}/>
        <path d={`M${w/2-46} ${h} Q${w/2} ${h*0.65} ${w/2+46} ${h} Z`} fill={c0}/>
        <circle cx={w/2-10} cy={h*0.4} r="2.5" fill={c2}/>
        <circle cx={w/2+10} cy={h*0.4} r="2.5" fill={c2}/>
      </svg>
    );
  }
  if (pattern === 'gradient') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <defs>
          <linearGradient id={`g-${palette.join('')}`} x1="0" x2="1" y1="0" y2="1">
            <stop offset="0" stopColor={c0}/>
            <stop offset="0.6" stopColor={c1}/>
            <stop offset="1" stopColor={c2}/>
          </linearGradient>
        </defs>
        <rect width={w} height={h} fill={`url(#g-${palette.join('')})`}/>
        <circle cx={w*0.7} cy={h*0.3} r="40" fill="white" opacity="0.12"/>
      </svg>
    );
  }
  if (pattern === 'fire') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <rect width={w} height={h} fill={c2}/>
        <path d={`M90 30 Q70 70 80 110 Q50 100 60 140 Q90 160 100 140 Q130 150 120 110 Q110 70 90 30 Z`} fill={c0} opacity="0.85"/>
        <path d={`M90 60 Q80 85 85 110 Q75 105 80 130 Q95 138 100 128 Q110 132 105 108 Q100 80 90 60 Z`} fill={c1} opacity="0.9"/>
      </svg>
    );
  }
  if (pattern === 'icons') {
    return (
      <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
        <rect width={w} height={h} fill={c1}/>
        {[[40,40],[100,40],[40,100],[100,100]].map(([x,y],i) => (
          <g key={i} transform={`translate(${x},${y})`}>
            <path d={`M0 0 L0 28 L8 22 L14 34 L18 32 L12 20 L22 20 Z`} fill={c0}/>
          </g>
        ))}
      </svg>
    );
  }
  // abstract default
  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" height="100%">
      <rect width={w} height={h} fill={c1}/>
      <circle cx={w*0.35} cy={h*0.4} r="36" fill={c0} opacity="0.85"/>
      <rect x={w*0.5} y={h*0.5} width="60" height="60" fill={c0} opacity="0.6"/>
    </svg>
  );
}

function FontThumb({ sample, family }) {
  return (
    <div className="thumb-font" style={{ fontFamily: family }}>
      {sample}
    </div>
  );
}

function UnknownThumb() {
  return <div className="thumb-unknown"><I.File size={28} stroke={1.25}/></div>;
}

function AssetThumb({ asset, large = false }) {
  const t = asset.thumb || {};
  if (t.kind === 'waveform') return <Waveform seed={t.seed || 1}/>;
  if (t.kind === 'font') return <FontThumb sample={t.sample} family={t.family}/>;
  if (t.kind === 'sprite') return <SpritePlaceholder palette={t.palette} pattern={t.pattern}/>;
  return <UnknownThumb/>;
}

Object.assign(window, { AssetThumb, Waveform, SpritePlaceholder, FontThumb });
