# Stitch UI Designer Manual

This technical manual instructs the `@designer` persona on using the **Stitch MCP Server** to design, prototype, and refine all user interfaces for **Shelfd** (`apps/app`).

---

## Invariants

1. **Design Before Code**:
   * No UI screen or component in `apps/app/` shall be implemented without first generating and verifying its visual prototype using Stitch MCP.
2. **Mobile-First Responsive Layouts**:
   * All screens must be designed mobile-first:
     * **Mobile (320px – 640px)**: 1-column layout, touch targets >= 48x48px, sticky navigation, compact reader canvas.
     * **Tablet (641px – 1024px)**: 2-column bento grids, 32px padding, responsive typography.
     * **Desktop (1025px+)**: 12-column bento grids, centered reading canvas (max 750px line width for reading ergonomics), multi-pane inspector/drawers.
3. **E-Reading Visual Ergonomics**:
   * Reading canvas contrast ratio must exceed 9.5:1 for body copy.
   * Provide explicit themes:
     * **Warm Editorial / Bone**: `#FBFBFA` background, `#111111` ink, `#444748` body text.
     * **Sepia**: `#F4ECD8` background, `#5B4636` ink.
     * **Dark / OLED**: `#121212` background, `#E0E0E0` ink.
4. **Token & Primitive Extraction**:
   * Every Stitch screen must yield clear design tokens (colors, font scales, spacing, border radii) and component specifications for `@reader` to build in Flutter.

---

## Stitch MCP Tooling Workflow

### 1. Project Management
Before generating screens, ensure the project exists or inspect existing screens:
```json
{
  "ServerName": "stitch",
  "ToolName": "list_projects",
  "Arguments": {}
}
```

### 2. Screen Generation (`generate_screen_from_text`)
When generating screens, provide detailed, context-rich prompts specifying typography, layout structure, color tokens, and components:
* Always specify `deviceType`: `"MOBILE"` or `"DESKTOP"`.
* Specify `modelId`: `"GEMINI_3_FLASH"` or `"GEMINI_3_1_PRO"`.
* Include semantic constraints: high contrast, distraction-free reading canvas, bento grid layout for discovery.

### 3. Screen Iteration & Variants
* Use `generate_variants` to explore layout and theme alternatives (e.g. comparing Warm Bone vs Sepia reading modes).
* Use `edit_screens` for targeted adjustments to existing designs.

### 4. Handoff to `@reader`
Document the generated screen URLs, layout hierarchy, and design tokens so `@reader` can implement corresponding Flutter widgets (`apps/app/lib/...`).
