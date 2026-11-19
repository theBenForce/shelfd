# Page Override: Utilities (`/utilities`)

## Goals
Provide a focused, uncluttered maintenance workspace constrained to `1040px` (`AppTokens.maxWorkspaceWidth`), removing redundant double-headers and oversized overview chrome.

## Structure
- **Consistent Header**: `ShelfdTopBar` on mobile, `ShelfdPageHeader` (`Utilities & Maintenance`) with `Scan for Duplicates` primary action on desktop.
- **Compact Status Strip**: Single streamlined summary bar for `Duplicate Books Finder` showing status badge (`Library Clean` / `N Groups Found`) and compact count pills (`Groups`, `Redundant Copies`).
- **Duplicate Group Cards**: Clean `ShelfdSectionCard` per group with responsive header (`Wrap`), `Options` secondary button, `Merge Group` primary button, and tactile candidate book cards with clear `PRIMARY` selection highlight.
- **Empty / Scanning States**: Standardized `ShelfdEmptyState`.
