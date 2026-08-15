// File-tree data for the scoped file browser.
// The library has no real filesystem — when you drill into an asset "pack"
// (a .unitypackage, a multi-file asset) this fabricates a believable
// on-disk asset-library layout (Unity-style /Assets/… trees) so you can
// browse, preview and pull individual files into collections.

// ---- File-type taxonomy --------------------------------------------------
// ftype drives the icon, the "File type" sidebar filter, and preview style.
const FTYPES = {
  prefab:   { label: 'Prefabs',    icon: 'Cube' },
  mesh:     { label: 'Meshes',     icon: 'Cube' },
  texture:  { label: 'Textures',   icon: 'Image' },
  material: { label: 'Materials',  icon: 'Sphere' },
  scene:    { label: 'Scenes',     icon: 'Scene' },
  script:   { label: 'Scripts',    icon: 'Code' },
  shader:   { label: 'Shaders',    icon: 'Code' },
  anim:     { label: 'Animation',  icon: 'Play' },
  audio:    { label: 'Audio',      icon: 'Audio' },
  font:     { label: 'Fonts',      icon: 'Font' },
  sprite:   { label: 'Sprites',    icon: 'Image' },
  doc:      { label: 'Docs',       icon: 'File' },
  data:     { label: 'Data',       icon: 'File' },
};

const FTYPE_ICON = {
  prefab: 'Cube', mesh: 'Cube', texture: 'Image', material: 'Sphere',
  scene: 'Scene', script: 'Code', shader: 'Code', anim: 'Play',
  audio: 'Audio', font: 'Font', sprite: 'Image', doc: 'File', data: 'File',
};

function FileGlyph({ ftype, ...rest }) {
  const name = FTYPE_ICON[ftype] || 'File';
  const Cmp = I[name] || I.File;
  return <Cmp {...rest}/>;
}

// ---- Deterministic helpers ----------------------------------------------
function seedFromId(id) {
  let s = 0;
  for (const ch of String(id)) s = (s * 31 + ch.charCodeAt(0)) >>> 0;
  return s || 1;
}
function rng(seed) {
  let s = seed >>> 0;
  return () => { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296; };
}
function pad(n, w = 2) { return String(n).padStart(w, '0'); }

function dateOffset(iso, days) {
  if (!iso || !iso.includes('-')) return iso;
  const d = new Date(iso);
  d.setDate(d.getDate() - days);
  return d.toISOString().slice(0, 10);
}

// Builders
function dir(name, children, open = false) { return { name, type: 'folder', open, children }; }
function file(name, ftype, size, modified, thumb) {
  const ext = (name.match(/\.[a-z0-9]+$/i) || [''])[0];
  return { name, type: 'file', ftype, ext, size, modified, thumb: thumb || null };
}

// Generate a numbered sequence of files in one go.
function seq(rand, base, { prefix, suffixes, ext, ftype, count, min, max, baseDate, dateSpread, thumb }) {
  const out = [];
  for (let i = 0; i < count; i++) {
    const tag = suffixes ? suffixes[i % suffixes.length] : null;
    const num = pad(i + 1);
    const name = tag
      ? `${prefix}_${tag}_${num}${ext}`
      : `${prefix}_${num}${ext}`;
    const size = Math.round(min + rand() * (max - min));
    const modified = dateOffset(baseDate, Math.round(rand() * dateSpread));
    out.push(file(name, ftype, size, modified, thumb ? thumb(i) : null));
  }
  return out;
}

const KB = 1024, MB = 1024 * 1024;

// Texture/sprite thumbnails reuse the procedural sprite renderer.
function texThumb(palette, patterns) {
  return (i) => ({ kind: 'sprite', palette: shiftPalette(palette, i), pattern: patterns[i % patterns.length] });
}
function audioThumb() { return (i) => ({ kind: 'waveform', seed: 3 + i * 2 }); }

// ---- Per-pack tree generators -------------------------------------------
function syntyEnvironmentTree(a) {
  const r = rng(seedFromId(a.id));
  const d = a.imported;
  const pal = a.thumb.palette || ['#7d9266', '#b8a375', '#5a6b52', '#d4c28b'];
  return [
    dir('Assets', [
      dir('PolygonFantasyKingdom', [
        dir('Prefabs', [
          dir('Buildings', seq(r, 0, { prefix: 'SM_Bld', suffixes: ['Castle', 'House', 'Tower', 'Wall', 'Gate'], ext: '.prefab', ftype: 'prefab', count: 16, min: 6 * KB, max: 48 * KB, baseDate: d, dateSpread: 40 })),
          dir('Environment', seq(r, 0, { prefix: 'SM_Env', suffixes: ['Tree', 'Rock', 'Bush', 'Cliff', 'Path'], ext: '.prefab', ftype: 'prefab', count: 18, min: 5 * KB, max: 36 * KB, baseDate: d, dateSpread: 60 })),
          dir('Characters', seq(r, 0, { prefix: 'SM_Chr', suffixes: ['Knight', 'Peasant', 'King', 'Archer'], ext: '.prefab', ftype: 'prefab', count: 8, min: 8 * KB, max: 40 * KB, baseDate: d, dateSpread: 50 })),
          dir('Props', seq(r, 0, { prefix: 'SM_Prop', suffixes: ['Barrel', 'Crate', 'Banner', 'Torch', 'Cart', 'Sign'], ext: '.prefab', ftype: 'prefab', count: 22, min: 3 * KB, max: 22 * KB, baseDate: d, dateSpread: 70 })),
        ], true),
        dir('Meshes', seq(r, 0, { prefix: 'SM', suffixes: ['Bld_Castle', 'Env_Tree', 'Chr_Knight', 'Prop_Barrel'], ext: '.fbx', ftype: 'mesh', count: 28, min: 180 * KB, max: 3.4 * MB, baseDate: d, dateSpread: 80 })),
        dir('Materials', seq(r, 0, { prefix: 'PolygonFantasy_Mat', ext: '.mat', ftype: 'material', count: 14, min: 2 * KB, max: 12 * KB, baseDate: d, dateSpread: 30 })),
        dir('Textures', seq(r, 0, { prefix: 'PolygonFantasy_Texture', ext: '.png', ftype: 'texture', count: 12, min: 256 * KB, max: 8 * MB, baseDate: d, dateSpread: 50, thumb: texThumb(pal, ['landscape', 'tiles', 'abstract', 'gradient']) })),
        dir('Scenes', [
          file('Demo_Day.unity', 'scene', 1.2 * MB, dateOffset(d, 2)),
          file('Demo_Night.unity', 'scene', 1.4 * MB, dateOffset(d, 2)),
          file('Showcase.unity', 'scene', 2.1 * MB, dateOffset(d, 12)),
        ]),
        dir('Scripts', seq(r, 0, { prefix: 'Demo', suffixes: ['CameraController', 'CharacterMover', 'DayNight', 'SceneSwitcher'], ext: '.cs', ftype: 'script', count: 6, min: 1 * KB, max: 14 * KB, baseDate: d, dateSpread: 20 })),
        file('PolygonFantasyKingdom_Manual.pdf', 'doc', 4.2 * MB, dateOffset(d, 5)),
        file('README.md', 'doc', 4 * KB, dateOffset(d, 1)),
        file('CHANGELOG.txt', 'doc', 9 * KB, d),
        file('LICENSE.txt', 'doc', 2 * KB, dateOffset(d, 120)),
      ], true),
    ], true),
  ];
}

function sciFiUITree(a) {
  const r = rng(seedFromId(a.id));
  const d = a.imported;
  const pal = a.thumb.palette || ['#2d4a6b', '#58a7d4', '#0f1620'];
  return [
    dir('Assets', [
      dir('SciFiUIPack', [
        dir('Sprites', seq(r, 0, { prefix: 'UI', suffixes: ['Button', 'Panel', 'Frame', 'Icon', 'Bar', 'Slider'], ext: '.png', ftype: 'sprite', count: 26, min: 8 * KB, max: 420 * KB, baseDate: d, dateSpread: 40, thumb: texThumb(pal, ['ui', 'abstract', 'gradient']) })),
        dir('Prefabs', seq(r, 0, { prefix: 'Panel', suffixes: ['HUD', 'Menu', 'Inventory', 'Dialog'], ext: '.prefab', ftype: 'prefab', count: 10, min: 6 * KB, max: 34 * KB, baseDate: d, dateSpread: 30 })),
        dir('Fonts', [
          file('Orbitron-Regular.ttf', 'font', 42 * KB, dateOffset(d, 8), { kind: 'font', sample: 'Aa', family: 'Verdana' }),
          file('Orbitron-Bold.ttf', 'font', 44 * KB, dateOffset(d, 8), { kind: 'font', sample: 'Aa', family: 'Verdana' }),
          file('Rajdhani-Medium.ttf', 'font', 38 * KB, dateOffset(d, 8), { kind: 'font', sample: 'Rd', family: 'Verdana' }),
        ]),
        dir('Animations', seq(r, 0, { prefix: 'UI_Anim', suffixes: ['FadeIn', 'SlideUp', 'Pulse'], ext: '.anim', ftype: 'anim', count: 6, min: 3 * KB, max: 18 * KB, baseDate: d, dateSpread: 20 })),
        dir('Materials', seq(r, 0, { prefix: 'UI_Mat', ext: '.mat', ftype: 'material', count: 4, min: 2 * KB, max: 9 * KB, baseDate: d, dateSpread: 15 })),
        dir('Demo', [
          file('Demo_HUD.unity', 'scene', 820 * KB, dateOffset(d, 3)),
          file('Demo_Menu.unity', 'scene', 640 * KB, dateOffset(d, 3)),
        ]),
        file('Documentation.pdf', 'doc', 1.8 * MB, dateOffset(d, 4)),
        file('LICENSE.txt', 'doc', 2 * KB, dateOffset(d, 30)),
      ], true),
    ], true),
  ];
}

function toolTree(a, vendor, root) {
  const r = rng(seedFromId(a.id));
  const d = a.imported;
  return [
    dir('Assets', [
      dir(vendor, [
        dir(root, [
          dir('Runtime', seq(r, 0, { prefix: root, suffixes: ['Core', 'Tween', 'Sequence', 'Utils', 'Extensions'], ext: '.cs', ftype: 'script', count: 14, min: 2 * KB, max: 38 * KB, baseDate: d, dateSpread: 60 })),
          dir('Editor', seq(r, 0, { prefix: root + '_Editor', suffixes: ['Inspector', 'Window', 'Settings'], ext: '.cs', ftype: 'script', count: 8, min: 3 * KB, max: 26 * KB, baseDate: d, dateSpread: 40 })),
          dir('Shaders', seq(r, 0, { prefix: root, suffixes: ['Unlit', 'Standard'], ext: '.shader', ftype: 'shader', count: 4, min: 4 * KB, max: 16 * KB, baseDate: d, dateSpread: 30 })),
          file(root + '.dll', 'data', 340 * KB, dateOffset(d, 1)),
          file('API.pdf', 'doc', 1.2 * MB, dateOffset(d, 6)),
          file('readme.txt', 'doc', 6 * KB, d),
          file('LICENSE.md', 'doc', 3 * KB, dateOffset(d, 90)),
        ], true),
      ], true),
    ], true),
  ];
}

function vfxTree(a) {
  const r = rng(seedFromId(a.id));
  const d = a.imported;
  const pal = a.thumb.palette || ['#ff6a00', '#ffd447', '#2a0e00'];
  return [
    dir('Assets', [
      dir('FireVFXPack', [
        dir('Prefabs', seq(r, 0, { prefix: 'PS_Fire', suffixes: ['Torch', 'Campfire', 'Explosion', 'Embers'], ext: '.prefab', ftype: 'prefab', count: 10, min: 8 * KB, max: 30 * KB, baseDate: d, dateSpread: 30 })),
        dir('Materials', seq(r, 0, { prefix: 'M_Fire', ext: '.mat', ftype: 'material', count: 8, min: 2 * KB, max: 10 * KB, baseDate: d, dateSpread: 20 })),
        dir('Textures', seq(r, 0, { prefix: 'T_Fire', suffixes: ['Flame', 'Smoke', 'Spark', 'Noise'], ext: '.png', ftype: 'texture', count: 12, min: 128 * KB, max: 4 * MB, baseDate: d, dateSpread: 40, thumb: texThumb(pal, ['fire', 'gradient', 'abstract']) })),
        dir('Meshes', seq(r, 0, { prefix: 'SM_Fire', ext: '.fbx', ftype: 'mesh', count: 4, min: 120 * KB, max: 800 * KB, baseDate: d, dateSpread: 30 })),
        file('Demo.unity', 'scene', 540 * KB, dateOffset(d, 2)),
        file('README.txt', 'doc', 5 * KB, d),
        file('LICENSE.txt', 'doc', 2 * KB, dateOffset(d, 10)),
      ], true),
    ], true),
  ];
}

// Flat pack — multi-file assets that aren't full Unity packages.
function flatTree(a) {
  const r = rng(seedFromId(a.id));
  const d = a.imported;
  const n = Math.min(a.files, 40);
  const ftypeByType = { sprite: 'sprite', audio: 'audio', font: 'font', unitypackage: 'prefab' };
  const ft = ftypeByType[a.type] || 'data';
  const ext = a.ext || '.png';
  const baseName = a.name.replace(/[^a-z0-9]+/gi, '_').replace(/^_|_$/g, '');
  const thumb =
    a.type === 'sprite' ? texThumb(a.thumb.palette || ['#8b6a4f', '#c9a87c', '#4a3528'], ['portrait', 'character', 'abstract']) :
    a.type === 'audio' ? audioThumb() :
    a.type === 'font' ? (i) => ({ kind: 'font', sample: a.thumb.sample || 'Aa', family: a.thumb.family || 'Georgia' }) :
    null;
  const files = [];
  for (let i = 0; i < n; i++) {
    files.push(file(`${baseName}_${pad(i + 1)}${ext}`, ft, Math.round(a.size / a.files * (0.7 + r() * 0.6)), dateOffset(d, Math.round(r() * 12)), thumb ? thumb(i) : null));
  }
  return files;
}

// ---- Public: build a tree for any asset ---------------------------------
function buildPackTree(asset) {
  if (!asset) return [];
  if (asset.id === 'a01') return syntyEnvironmentTree(asset);
  if (asset.id === 'a05') return sciFiUITree(asset);
  if (asset.id === 'a08') return toolTree(asset, 'Demigiant', 'DOTween');
  if (asset.id === 'a13') return toolTree(asset, 'Unity', 'ProBuilder');
  if (asset.id === 'a16') return vfxTree(asset);
  return flatTree(asset);
}

// A pack is browsable when it holds more than one file.
function isBrowsable(asset) { return asset && asset.files > 1; }

// ---- Tree navigation / query helpers ------------------------------------
function nodesAtPath(tree, path) {
  let nodes = tree;
  for (const seg of path) {
    const found = nodes.find(n => n.type === 'folder' && n.name === seg);
    if (!found || !Array.isArray(found.children)) return [];
    nodes = found.children;
  }
  return nodes;
}

function countFiles(node) {
  if (node.type === 'file') return 1;
  if (!Array.isArray(node.children)) return 0;
  return node.children.reduce((s, c) => s + countFiles(c), 0);
}

function flattenFiles(tree, parent = []) {
  const out = [];
  for (const n of tree) {
    if (n.type === 'file') out.push({ ...n, dir: parent });
    else if (Array.isArray(n.children)) out.push(...flattenFiles(n.children, [...parent, n.name]));
  }
  return out;
}

function typeCounts(tree) {
  const counts = {};
  for (const f of flattenFiles(tree)) counts[f.ftype] = (counts[f.ftype] || 0) + 1;
  return counts;
}

function pathStr(dir, name) { return [...dir, name].join('/'); }

Object.assign(window, {
  FTYPES, FileGlyph, buildPackTree, isBrowsable,
  nodesAtPath, countFiles, flattenFiles, typeCounts, pathStr,
});
