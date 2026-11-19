# Design System: Shelfd — Tactile Scriptorium

> Grounded in `ui-ux-pro-max` (Minimalism & Swiss grid discipline, editorial typography, accessible contrast, progressive disclosure, mobile navigation limits) and tailored to Shelfd's warm archival reading identity.

## 1. Visual Direction & Principles
- **Identity**: *Tactile Scriptorium* — warm, calm, book-first personal library and e-reader.
- **Hierarchy**: Editorial serif headings (`Newsreader`) paired with clean humanist sans UI (`Work Sans`) and tabular monospace (`JetBrains Mono`) for paths, percentages, and counts.
- **Surface Discipline**: Flat, tonal paper-layer surfaces with 1px hairline borders (`subtleBorder` / `border`) and minimal soft shadows. No loud gradients, no emoji icons, no competing primary CTAs.
- **Progressive Disclosure**: Secondary/destructive actions and rarely used forms (such as password changes) stay tucked behind clean disclosure controls so primary workflows remain immediately scannable.
- **Navigation Hygiene**:
  - **Desktop**: Collapsible `Library` section + `Semantic Search` under *Browse*; `Uploads`, `Utilities`, `Settings` under *Manage*; compact status footer with a subtle tonal `Rescan Library` utility action.
  - **Mobile**: Maximum **5** bottom navigation destinations (`Library`, `Search`, `Uploads`, `Utilities`, `Settings`) so labels and touch targets (≥48dp) never crowd on 375px screens.

---

## 2. Color Tokens (3 Cohesive Palettes)

All `ThemeData` surfaces (`scaffoldBackgroundColor`, `colorScheme`, `appBarTheme`, `inputDecorationTheme`, `cardTheme`, `dialogTheme`, `chipTheme`) MUST derive directly from the active `ShelfdColors` palette — zero legacy/hardcoded hex drift.

| Semantic Token | Bone (Warm Parchment) | Sepia (Archival Vellum) | Dark (Warm Walnut) |
| :--- | :--- | :--- | :--- |
| `background` | `#FBF9F4` | `#F4ECD8` | `#161412` |
| `surface` | `#FFFFFF` | `#EDE3CB` | `#1E1B18` |
| `containerLow` | `#F5F3EE` | `#EFE6D1` | `#23201C` |
| `container` | `#EFECE6` | `#E6DBC2` | `#292521` |
| `containerHigh` | `#E9E6DF` | `#DCD0B4` | `#322D28` |
| `ink` | `#1E1B18` | `#5B4636` | `#EDE7DF` |
| `secondaryInk` | `#53433C` | `#6E5644` | `#BDB3A7` |
| `mutedInk` | `#756860` | `#866D59` | `#8C8277` |
| `border` | `#E3DDD3` | `#E0D4BA` | `#332E29` |
| `subtleBorder` | `#EDE8DF` | `#E8DEC6` | `#282420` |
| `accentPrimary` | `#8E4A26` (Burnt Sienna) | `#8E4A26` | `#D98A5F` |
| `accentContainer` | `#F7ECE6` | `#DCD0B4` | `#3A261B` |
| `accentSecondary` | `#3A5A40` (Library Green) | `#3A5A40` | `#81B28A` |
| `accentSecondaryContainer` | `#BCE0C1` | `#BCE0C1` | `#1E3122` |
| `errorBg` / `errorText` | `#FEF2F2` / `#B91C1C` | `#FEF2F2` / `#B91C1C` | `#2D1618` / `#FCA5A5` |

---

## 3. Typography & Spacing Scale
- **Spacing**: Strict 4/8px rhythm (`4, 8, 12, 16, 20, 24, 32, 48, 64`).
- **Radii**: `radiusSm = 4`, `radiusMd = 8`, `radiusLg = 12`, `radiusPill = 999`.
- **Content Measure**:
  - Reading & Settings forms: `maxWidth = 720px` (`AppTokens.maxReadingWidth`)
  - Search & Utilities workspaces: `maxWidth = 1040px` (`AppTokens.maxWorkspaceWidth`)
  - Library & Uploads split-pane: `maxWidth = 1800px` (`AppTokens.maxLibraryWidth`)
- **Touch Targets**: All interactive controls ≥ 48px (`AppTokens.minTouchTarget`).

---

## 4. Bottom-Up UI Primitives (`apps/app/lib/ui/core/primitives.dart`)
1. **`ShelfdPageHeader`**: Responsive desktop page header (serif title, muted subtitle, optional trailing widget). Mobile uses `ShelfdTopBar` consistently across every view.
2. **`ShelfdSectionCard`**: Flat editorial card with optional icon, serif title, status badge, trailing action, and consistent 20px padding.
3. **`ShelfdInlineBanner`**: Unified semantic alert banner (`error`, `success`, `warning`, `info`) with icon, message, and optional dismiss action.
4. **`ShelfdEmptyState`**: Centered empty/clean state with circular tinted icon badge, serif heading, muted description, and optional action button.
5. **`ShelfdSecondaryButton` & `ShelfdDangerButton`**: Standardized outlined secondary and destructive buttons matching `PrimaryButton` metrics and typography.
