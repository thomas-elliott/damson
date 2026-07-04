# file-browser

## ADDED Requirements

### Requirement: Design-system file-browser components ship with stories

The file-browser UI SHALL be built from components that live in the Damson design
system and compose its existing primitives, tokens, and composition frames rather
than re-rolling styles. Each new component SHALL ship a Storybook story that acts
as its visual and accessibility contract.

#### Scenario: Each component renders in Storybook
- WHEN the file-browser components are built (tree sidebar, browser toolbar/breadcrumb, folder card, file card, file thumbnail/placeholder, file-type filter, file detail panel)
- THEN each has a Storybook story that renders it against the design-system tokens
- AND the story passes the project's Storybook a11y checks

#### Scenario: Components reuse existing system surfaces
- WHEN a file-browser component needs chrome that already exists (detail panel, breadcrumb, filter sidebar, segmented control, search input, section)
- THEN it composes the existing design-system component rather than duplicating its markup or styles

#### Scenario: New icons are added through the icon surface
- WHEN the browser needs icons not yet present (FolderOpen, Cube, Code, Scene, Sphere, Play, CornerArrow)
- THEN they are added through the existing `Glyph`/lucide surface and available to design-system consumers

### Requirement: Entering and exiting the scoped browser

An asset SHALL be browsable when its current version holds more than one file. A
browsable asset SHALL expose a "Browse files" action from the asset grid toolbar
and from the asset detail panel. Activating it SHALL enter a full-page scoped
browser for that asset; leaving it SHALL return to the library view.

#### Scenario: Browse action shown only for browsable assets
- WHEN an asset's current version has more than one file
- THEN the "Browse files" action is shown on the grid toolbar and the detail panel
- AND for a single-file asset the action is not shown

#### Scenario: Entering the browser
- WHEN the user activates "Browse files" for a browsable asset
- THEN the library view is replaced by the scoped browser for that asset
- AND the browser scope is the asset's current version manifest

#### Scenario: Exiting the browser
- WHEN the user activates the breadcrumb "Library" segment, the scope back-control, or navigates via the nav rail
- THEN the browser closes and the library view is restored
- AND browser state (path, selection, open file) is cleared

### Requirement: File tree navigation

The browser SHALL present the asset's files as a navigable directory tree built
from each file's relative path. The user SHALL be able to navigate into folders,
navigate up the hierarchy via the breadcrumb, and jump to any folder from the tree
sidebar. The current path SHALL be reflected in both the breadcrumb and the tree.

#### Scenario: Tree built from relative paths
- WHEN the browser opens for an asset version
- THEN the file manifest is grouped into a folder tree by splitting each file's relative path
- AND folders display the count of files they contain (recursively)

#### Scenario: Navigating into a folder
- WHEN the user opens a folder (tree, folder card, or list row)
- THEN the main view shows that folder's immediate folders and files
- AND the breadcrumb appends the folder segment

#### Scenario: Navigating up via breadcrumb
- WHEN the user activates a breadcrumb segment above the current folder
- THEN the browser navigates to that ancestor folder

#### Scenario: Keyboard navigation
- WHEN the user presses Escape in the browser
- THEN the browser steps up one level in the hierarchy (closing the open file first if one is open, then stepping up folders, then exiting)
- AND arrow keys move between files when a file is open

### Requirement: Grid and list views

The main browser area SHALL render the current folder's contents as either a grid
of cards or a list/table, switchable by the user, with the choice persisted within
the session.

#### Scenario: Switching views
- WHEN the user toggles the grid/list control in the toolbar
- THEN the current folder contents re-render in the chosen layout
- AND folders and files are both represented in each layout

#### Scenario: Empty folder
- WHEN the current folder contains no items
- THEN an empty-state message is shown

### Requirement: File search and type filtering

The browser SHALL let the user search files within the current asset by name and
filter by file type. Active search or filters SHALL switch the main view into a
flat results mode that shows matching files across the whole pack with their
folder path, and SHALL be clearable.

#### Scenario: Searching within the pack
- WHEN the user types a query in the file search
- THEN the main view shows files across the whole pack whose name matches, each annotated with its folder path
- AND the breadcrumb reflects the search/results state

#### Scenario: Filtering by file type
- WHEN the user selects one or more file types in the sidebar type filter
- THEN only files of those types are shown, as active filter chips in the toolbar
- AND the type filter lists each present type with its file count

#### Scenario: Clearing results mode
- WHEN the user clears the search and removes all type filters
- THEN the browser returns to normal folder navigation at the current path

### Requirement: File-type taxonomy derived from extension

Each file SHALL be classified into a display type (e.g. prefab, mesh, texture,
material, scene, script, shader, animation, audio, font, sprite, doc, data) from
its file extension, since the stored `detected_type` is empty for
package-extracted files. The classification SHALL drive the file icon, the
file-type filter grouping, and the preview style. The stored `detected_type` MAY
be used when present (e.g. loose uploads) but SHALL NOT be required.

#### Scenario: Type derived from extension
- WHEN a file has a recognised extension (e.g. `.prefab`, `.fbx`, `.png`, `.mat`, `.unity`, `.cs`)
- THEN it is classified into the corresponding display type and shown with that type's icon

#### Scenario: Unrecognised extension
- WHEN a file's extension is not recognised
- THEN it falls back to a generic file type and icon

### Requirement: File detail and preview

Opening a file SHALL show a detail panel — reusing the library detail-panel chrome
— with the file's name, type, size, full path, and actions to download the file.
Image-type files SHALL preview inline from the file-serving endpoint; all other
types SHALL show a type-styled placeholder. The panel SHALL allow stepping to the
previous/next file in the current listing and closing.

#### Scenario: Opening a file
- WHEN the user opens a file (file card, list row, or tree leaf)
- THEN a detail panel shows the file's name, derived type, size, and full path within the asset
- AND a download action is available

#### Scenario: Image preview
- WHEN the open file is an image type (texture/sprite)
- THEN its content is previewed inline from `GET /api/files/:objectKey`

#### Scenario: Non-image placeholder
- WHEN the open file is not an image type
- THEN a type-styled placeholder (icon plus extension) is shown instead of a rendered preview

#### Scenario: Stepping between files
- WHEN the detail panel is open and the user steps to previous/next
- THEN the adjacent file in the current listing becomes the open file
- AND the position indicator (n / total) updates

#### Scenario: Downloading a file
- WHEN the user activates download on a file
- THEN the file is fetched from `GET /api/files/:objectKey` for that file's object key

### Requirement: Create a subset from a file selection

The browser SHALL let the user select one or more files (without opening them) and
create a subset from the selection. Creating the subset SHALL call the existing
asset-version subset endpoint, producing a new version whose files are the
selection with blobs reused. Selection state SHALL be visible and clearable.

#### Scenario: Selecting files
- WHEN the user shift/meta/ctrl-clicks files, or drags a selection
- THEN those files are marked selected and a selection bar shows the count with clear and subset actions

#### Scenario: Creating a subset
- WHEN the user creates a subset from the current selection
- THEN `POST /assets/:id/versions/:vid/subset` is called with the selected files
- AND a confirmation is shown on success

#### Scenario: Clearing the selection
- WHEN the user clears the selection
- THEN no files are marked selected and the selection bar is hidden

## REMOVED Requirements

### Requirement: Flat-list subset picker dialog

**Reason:** The scoped file browser becomes the single file-selection surface for
creating subsets; the flat-list `SubsetVersionDialog` is superseded.
**Migration:** Subset creation moves into the browser's file selection +
"Create subset" action (see "Create a subset from a file selection"). The subset
backend endpoint is unchanged; only the picking UI is replaced.
