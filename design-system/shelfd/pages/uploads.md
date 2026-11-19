# Page Override: Uploads (`/uploads`)

## Goals
Eliminate double headers, shrink dropzone chrome when reviewing staged items, and unify all colors/inputs with `ShelfdColors` and `ThemeData`.

## Structure
- **Single Header**: Mobile uses `ShelfdTopBar`; Desktop uses `ShelfdPageHeader` (`Uploads & Ingestion`) with the `Auto-add` switch integrated cleanly in the header actions.
- **Adaptive Dropzone**:
  - Empty queue: Full editorial dropzone with `Browse Files`, `Select Folder`, `Browse Storefronts`.
  - Non-empty queue: Compact horizontal drop strip with the same 3 actions + drag-and-drop support, bringing the staged queue and metadata inspector immediately above the fold.
- **Queue & Inspector**:
  - Queue toolbar shows `Queue (N)`, `Ready` / `Incomplete` status pills, `Clear Queue`, and `Approve All Ready (N)`.
  - Staged job cards and Metadata Inspector use `ShelfdSectionCard`, `ShelfdInlineBanner`, and theme-inherited `InputDecoration` (zero hardcoded hexes).
