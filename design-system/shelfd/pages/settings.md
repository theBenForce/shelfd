# Page Override: Settings (`/settings`)

## Goals
Eliminate sprawl, duplicate controls, and awkward card ordering.

## Structure (Top to Bottom, `maxWidth: 720px`)
1. **Appearance**: Compact `ShelfdSectionCard` with `Bone | Sepia | Dark` segmented selector.
2. **Plugins & Storefronts**: Clean list of installed plugins with status badge, enable switch, and concise action row (`Configure`, `Logs`, `Browse Catalog`). Header has `Rescan` and `Install .zip`.
3. **Server & Indexing**: Host URL, live connection indicator, inline AI Indexing progress bar (when available), and a compact `Rescan Library` action.
4. **Account & Security**:
   - Current user badge + inline `Default Username` field & `Save`.
   - **Change Password** tucked behind a clean disclosure toggle (collapsed by default, expands inline on tap) so 3 password inputs do not dominate the page.
   - `Disconnect / Sign Out` placed cleanly at the bottom of Account & Security.

## Removed Redundancy
- Removed the redundant "Uploads & Ingestion" navigation card (Uploads is a top-level destination and owns the `Auto-add` toggle).
- Removed the redundant "Library Utilities" navigation card (Utilities is a top-level destination).
