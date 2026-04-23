// Seed data for Damson prototype — game-dev asset mix.
// Exposed on window for other babel scripts.

const LICENCES = {
  'MIT': { spdx: 'MIT', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: true, shareAlike: false, opensource: true } },
  'CC-BY-4.0': { spdx: 'CC-BY-4.0', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: true, shareAlike: false, opensource: false } },
  'CC-BY-SA-4.0': { spdx: 'CC-BY-SA-4.0', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: true, shareAlike: true, opensource: false } },
  'CC0-1.0': { spdx: 'CC0-1.0', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: false, shareAlike: false, opensource: true } },
  'OFL-1.1': { spdx: 'OFL-1.1', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: true, shareAlike: true, opensource: true } },
  'Apache-2.0': { spdx: 'Apache-2.0', status: 'ok', attrs: { commercial: true, modification: true, redistribution: true, attribution: true, shareAlike: false, opensource: true } },
  'Unity-Asset-Store-EULA': { spdx: 'Unity-Asset-Store-EULA', status: 'warn', attrs: { commercial: true, modification: true, redistribution: false, attribution: false, shareAlike: false, opensource: false } },
  'CC-BY-NC-4.0': { spdx: 'CC-BY-NC-4.0', status: 'err', attrs: { commercial: false, modification: true, redistribution: true, attribution: true, shareAlike: false, opensource: false } },
  'Unknown': { spdx: '—', status: 'unk', attrs: {} },
};

const LICENCE_TEXT_MIT =
  'MIT License\n\nCopyright (c) 2024\n\nPermission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software…';

const LICENCE_TEXT_UNITY =
  'Unity Asset Store End User License Agreement\n\n1. Grant of License. Subject to your compliance with the terms herein, Publisher grants Licensee a non-exclusive, non-transferable license to use the Asset in up to one (1) commercial project…';

// Deterministic asset fixtures
const ASSETS = [
  {
    id: 'a01', name: 'POLYGON - Fantasy Kingdom', type: 'unitypackage', ext: '.unitypackage',
    source: 'Unity Asset Store', licence: 'Unity-Asset-Store-EULA',
    tags: ['environment', '3d', 'low-poly', 'fantasy'],
    size: 684_221_440, imported: '2026-03-12', version: '1.12.3',
    files: 1284, projects: ['Grove', 'Ashfall'], collections: ['Environments'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#7d9266', '#b8a375', '#5a6b52', '#d4c28b'], pattern: 'landscape' },
    versions: [
      { v: '1.12.3', date: '2026-03-12', files: 1284, notes: 'Added winter biome variants', current: true },
      { v: '1.11.0', date: '2026-01-04', files: 1210, notes: 'Unity 2023.3 compatibility' },
      { v: '1.10.2', date: '2025-09-18', files: 1187, notes: 'Bugfix: LOD0 meshes' },
      { v: '1.10.0', date: '2025-07-22', files: 1180, notes: 'New: buildings pack' },
    ],
    fileTree: [
      { name: 'Assets', type: 'folder', open: true, children: [
        { name: 'POLYGON_Fantasy', type: 'folder', open: true, children: [
          { name: 'Prefabs', type: 'folder', children: 422 },
          { name: 'Materials', type: 'folder', children: 184 },
          { name: 'Textures', type: 'folder', children: 312 },
          { name: 'Meshes', type: 'folder', children: 304 },
          { name: 'Demo.unity', type: 'file', size: '1.2 MB' },
          { name: 'README.md', type: 'file', size: '4 KB' },
          { name: 'LICENSE.txt', type: 'file', size: '2 KB' },
        ]},
      ]},
    ],
  },
  {
    id: 'a02', name: 'hero_run_cycle', type: 'sprite', ext: '.png',
    source: 'itch.io', licence: 'CC-BY-4.0',
    tags: ['character', '2d', 'animation'],
    size: 48_200, imported: '2026-04-18', version: '—',
    files: 1, projects: ['Ashfall'], collections: ['Character sheets'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#c85a4c', '#2b2f3a', '#f0d37a'], pattern: 'character' },
  },
  {
    id: 'a03', name: 'ambient_forest_loop', type: 'audio', ext: '.ogg',
    source: 'freesound.org', licence: 'CC0-1.0',
    tags: ['audio', 'ambient', 'loop'],
    size: 2_845_000, imported: '2026-04-11', version: '—',
    files: 1, projects: ['Grove'], collections: ['Ambience'],
    needsReview: false,
    thumb: { kind: 'waveform', seed: 7 },
  },
  {
    id: 'a04', name: 'Alagard', type: 'font', ext: '.ttf',
    source: 'Pixel Sagas', licence: 'OFL-1.1',
    tags: ['font', 'pixel', 'display'],
    size: 32_480, imported: '2026-02-01', version: '1.0',
    files: 2, projects: ['Ashfall', 'Grove'], collections: ['Pixel fonts'],
    needsReview: false,
    thumb: { kind: 'font', sample: 'Ag', family: 'Georgia' },
  },
  {
    id: 'a05', name: 'Sci-Fi UI Pack Vol. 3', type: 'unitypackage', ext: '.unitypackage',
    source: 'Unity Asset Store', licence: 'Unity-Asset-Store-EULA',
    tags: ['ui', 'sci-fi', '2d'],
    size: 142_000_000, imported: '2026-04-20', version: '3.2.1',
    files: 412, projects: [], collections: ['UI'],
    needsReview: true,
    thumb: { kind: 'sprite', palette: ['#2d4a6b', '#58a7d4', '#0f1620'], pattern: 'ui' },
  },
  {
    id: 'a06', name: 'metal_impact_01', type: 'audio', ext: '.wav',
    source: 'freesound.org', licence: 'CC-BY-4.0',
    tags: ['audio', 'sfx', 'impact'],
    size: 186_000, imported: '2026-04-15', version: '—',
    files: 1, projects: ['Ashfall'], collections: ['SFX'],
    needsReview: false,
    thumb: { kind: 'waveform', seed: 3 },
  },
  {
    id: 'a07', name: 'tileset_dungeon_16x', type: 'sprite', ext: '.png',
    source: 'OpenGameArt', licence: 'CC-BY-SA-4.0',
    tags: ['tileset', 'dungeon', '2d', 'pixel'],
    size: 98_400, imported: '2026-03-28', version: '—',
    files: 1, projects: ['Ashfall'], collections: ['Tilesets'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#3a3147', '#6b5477', '#9b8aa8', '#d4cce0'], pattern: 'tiles' },
  },
  {
    id: 'a08', name: 'DOTween Pro', type: 'unitypackage', ext: '.unitypackage',
    source: 'Unity Asset Store', licence: 'Unity-Asset-Store-EULA',
    tags: ['tool', 'animation'],
    size: 4_800_000, imported: '2026-01-15', version: '1.0.381',
    files: 238, projects: ['Grove', 'Ashfall'], collections: ['Tools'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#f5a623', '#2b2b2b'], pattern: 'abstract' },
  },
  {
    id: 'a09', name: 'VenusRising', type: 'font', ext: '.otf',
    source: 'Google Fonts', licence: 'OFL-1.1',
    tags: ['font', 'display', 'serif'],
    size: 184_200, imported: '2026-03-02', version: '2.1',
    files: 4, projects: [], collections: ['Display fonts'],
    needsReview: false,
    thumb: { kind: 'font', sample: 'Vr', family: 'Playfair Display, Georgia' },
  },
  {
    id: 'a10', name: 'banner_splash_hero', type: 'sprite', ext: '.psd',
    source: 'Internal', licence: 'Unknown',
    tags: ['marketing', '2d'],
    size: 18_400_000, imported: '2026-04-21', version: '—',
    files: 1, projects: [], collections: ['Marketing'],
    needsReview: true,
    thumb: { kind: 'sprite', palette: ['#2a1d3d', '#d84b7d', '#ffb070'], pattern: 'gradient' },
  },
  {
    id: 'a11', name: 'footstep_grass', type: 'audio', ext: '.wav',
    source: 'Zapsplat', licence: 'CC-BY-NC-4.0',
    tags: ['audio', 'sfx', 'foley'],
    size: 92_000, imported: '2026-04-02', version: '—',
    files: 1, projects: [], collections: ['SFX'],
    needsReview: false,
    thumb: { kind: 'waveform', seed: 5 },
  },
  {
    id: 'a12', name: 'character_portraits_hd', type: 'sprite', ext: '.png',
    source: 'Artstation', licence: 'CC-BY-4.0',
    tags: ['character', 'portrait', '2d'],
    size: 7_800_000, imported: '2026-03-19', version: '—',
    files: 12, projects: ['Grove'], collections: ['Character sheets'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#8b6a4f', '#c9a87c', '#4a3528'], pattern: 'portrait' },
  },
  {
    id: 'a13', name: 'ProBuilder', type: 'unitypackage', ext: '.unitypackage',
    source: 'Unity Registry', licence: 'Apache-2.0',
    tags: ['tool', '3d'],
    size: 18_400_000, imported: '2025-11-04', version: '5.1.1',
    files: 94, projects: ['Grove'], collections: ['Tools'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#4a5568', '#9ab0c9'], pattern: 'abstract' },
  },
  {
    id: 'a14', name: 'bgm_menu_theme', type: 'audio', ext: '.mp3',
    source: 'Incompetech', licence: 'CC-BY-4.0',
    tags: ['audio', 'music', 'menu'],
    size: 4_200_000, imported: '2026-02-18', version: '—',
    files: 1, projects: ['Grove'], collections: ['Music'],
    needsReview: false,
    thumb: { kind: 'waveform', seed: 11 },
  },
  {
    id: 'a15', name: 'VT323', type: 'font', ext: '.ttf',
    source: 'Google Fonts', licence: 'OFL-1.1',
    tags: ['font', 'mono', 'pixel'],
    size: 64_800, imported: '2026-01-22', version: '1.3',
    files: 1, projects: ['Ashfall'], collections: ['Pixel fonts'],
    needsReview: false,
    thumb: { kind: 'font', sample: 'VT', family: 'VT323, Courier New' },
  },
  {
    id: 'a16', name: 'particle_fx_fire_pack', type: 'unitypackage', ext: '.unitypackage',
    source: 'itch.io', licence: 'MIT',
    tags: ['vfx', 'particles', '3d'],
    size: 22_400_000, imported: '2026-04-22', version: '2.0.0',
    files: 86, projects: [], collections: [],
    needsReview: true,
    thumb: { kind: 'sprite', palette: ['#ff6a00', '#ffd447', '#2a0e00'], pattern: 'fire' },
  },
  {
    id: 'a17', name: 'ui_cursor_set', type: 'sprite', ext: '.png',
    source: 'Kenney.nl', licence: 'CC0-1.0',
    tags: ['ui', 'cursor', '2d'],
    size: 24_000, imported: '2025-12-12', version: '—',
    files: 1, projects: ['Ashfall', 'Grove'], collections: ['UI'],
    needsReview: false,
    thumb: { kind: 'sprite', palette: ['#ffffff', '#1a1a1a'], pattern: 'icons' },
  },
  {
    id: 'a18', name: 'synth_lead_pack', type: 'audio', ext: '.wav',
    source: 'Splice', licence: 'Unknown',
    tags: ['audio', 'music', 'synth'],
    size: 12_800_000, imported: '2026-04-19', version: '—',
    files: 24, projects: [], collections: [],
    needsReview: true,
    thumb: { kind: 'waveform', seed: 13 },
  },
];

// Populate standard version + licence text + tree for assets that don't define them.
ASSETS.forEach(a => {
  if (!a.versions) {
    a.versions = [{ v: a.version || '1.0', date: a.imported, files: a.files, notes: 'Initial import', current: true }];
  }
  if (!a.licenceText) {
    a.licenceText = a.licence === 'Unity-Asset-Store-EULA' ? LICENCE_TEXT_UNITY : LICENCE_TEXT_MIT;
  }
  if (!a.fileTree) {
    a.fileTree = [{ name: a.name + a.ext, type: 'file', size: formatSize(a.size) }];
  }
});

// Import queue — in-flight items (not yet in library, or newly landed with review flag).
const QUEUE_SEED = [
  { id: 'q01', name: 'Medieval_Weapons_HD.unitypackage', type: 'unitypackage', size: 412_000_000, imported: '2 min ago', progress: 100, detectedLicence: 'Unity-Asset-Store-EULA', detectedType: 'Unity Package', needsFields: ['Tags', 'Project'] },
  { id: 'q02', name: 'dungeon_ambience_vol2.zip', type: 'audio', size: 38_400_000, imported: '5 min ago', progress: 100, detectedLicence: 'Unknown', detectedType: 'Audio (24 files)', needsFields: ['Licence', 'Tags'] },
  { id: 'q03', name: 'boss_theme_final.wav', type: 'audio', size: 18_200_000, imported: 'uploading…', progress: 64, detectedLicence: null, detectedType: 'Audio', needsFields: [] },
];

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
}

function formatDate(iso) {
  if (!iso || !iso.includes('-')) return iso;
  const d = new Date(iso);
  const now = new Date('2026-04-23');
  const days = Math.floor((now - d) / 86400000);
  if (days < 1) return 'today';
  if (days < 2) return 'yesterday';
  if (days < 7) return days + 'd ago';
  if (days < 30) return Math.floor(days / 7) + 'w ago';
  if (days < 365) return Math.floor(days / 30) + 'mo ago';
  return Math.floor(days / 365) + 'y ago';
}

// Tag tree for sidebar
const TAG_TREE = [
  { name: 'environment', count: 1, children: [
    { name: 'fantasy', count: 1 }, { name: 'dungeon', count: 1 },
  ]},
  { name: 'character', count: 3, children: [
    { name: 'portrait', count: 1 },
  ]},
  { name: 'audio', count: 6, children: [
    { name: 'sfx', count: 3 }, { name: 'music', count: 2 }, { name: 'ambient', count: 1 },
  ]},
  { name: 'ui', count: 3 },
  { name: 'font', count: 3, children: [
    { name: 'pixel', count: 2 }, { name: 'display', count: 2 },
  ]},
  { name: 'tool', count: 2 },
  { name: 'vfx', count: 1 },
  { name: '2d', count: 7 },
  { name: '3d', count: 3 },
];

const PROJECTS = [
  { id: 'grove', name: 'Grove', colour: 'oklch(0.7 0.12 155)', count: 5 },
  { id: 'ashfall', name: 'Ashfall', colour: 'oklch(0.68 0.17 28)', count: 6 },
  { id: 'prologue', name: 'Prologue (new)', colour: 'oklch(0.7 0.14 244)', count: 0 },
];

const COLLECTIONS = [
  { id: 'env', name: 'Environments', count: 1 },
  { id: 'chars', name: 'Character sheets', count: 2 },
  { id: 'sfx', name: 'SFX', count: 2 },
  { id: 'music', name: 'Music', count: 1 },
  { id: 'amb', name: 'Ambience', count: 1 },
  { id: 'tiles', name: 'Tilesets', count: 1 },
  { id: 'ui', name: 'UI', count: 2 },
  { id: 'tools', name: 'Tools', count: 2 },
  { id: 'pf', name: 'Pixel fonts', count: 2 },
  { id: 'df', name: 'Display fonts', count: 1 },
  { id: 'mkt', name: 'Marketing', count: 1 },
];

const LICENCE_FILTERS = [
  { id: 'ok', label: 'Compliant', status: 'ok', count: 12 },
  { id: 'warn', label: 'Needs review', status: 'warn', count: 3 },
  { id: 'err', label: 'Violation', status: 'err', count: 1 },
  { id: 'unk', label: 'Unknown', status: 'unk', count: 2 },
];

const FILETYPE_FILTERS = [
  { id: 'unitypackage', label: 'Unity packages', count: 4 },
  { id: 'sprite', label: 'Sprites & images', count: 6 },
  { id: 'audio', label: 'Audio', count: 5 },
  { id: 'font', label: 'Fonts', count: 3 },
];

const SOURCES = [
  { id: 'uas', label: 'Unity Asset Store', count: 4 },
  { id: 'itch', label: 'itch.io', count: 2 },
  { id: 'fs', label: 'freesound.org', count: 2 },
  { id: 'oga', label: 'OpenGameArt', count: 1 },
  { id: 'gf', label: 'Google Fonts', count: 2 },
  { id: 'kenney', label: 'Kenney.nl', count: 1 },
  { id: 'misc', label: 'Other', count: 6 },
];

Object.assign(window, {
  ASSETS, LICENCES, LICENCE_TEXT_MIT, LICENCE_TEXT_UNITY,
  QUEUE_SEED, TAG_TREE, PROJECTS, COLLECTIONS,
  LICENCE_FILTERS, FILETYPE_FILTERS, SOURCES,
  formatSize, formatDate,
});
