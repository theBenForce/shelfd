# Flutter Widget & Test Safety Standards

## Invariants

1. **Package Version API Verification (Grep Before Coding)**:
   - Before calling any third-party package in `apps/app/pubspec.yaml` (such as `file_picker`, `flutter_riverpod`, `go_router`, `just_audio`), ALWAYS search existing call sites in `apps/app/lib/` (`grep -rn "<Symbol>" apps/app/lib/`) to confirm the exact major-version API used in this repository.
   - Example: `file_picker: ^12.2.0` uses the static `FilePicker.pickFiles(...)` API returning `List<XFile>` (with `await file.readAsBytes()`), NOT `FilePicker.platform.pickFiles(...)`.

2. **`Ahem` Test Font Horizontal Overflow Prevention**:
   - In `flutter test`, all text renders using Flutter's `Ahem` font, where every glyph is a wide `1em × 1em` square.
   - Any `Row` containing a title/label alongside icons, `Spacer()`, badges, or buttons MUST wrap its `Text` child in `Expanded` or `Flexible` (with `maxLines: 1, overflow: TextOverflow.ellipsis`). Un-flexed `Text` inside a `Row` will cause `RenderFlex overflowed by X pixels on the right` in widget tests.

3. **`ListTile` Material Ancestor Requirement**:
   - `ListTile`, `SwitchListTile`, and `CheckboxListTile` paint their ink splashes and background on the nearest `Material` ancestor.
   - Whenever placing a `ListTile`, `SwitchListTile`, or `CheckboxListTile` inside a `Container` or `DecoratedBox` that has a background `color` (e.g., `BoxDecoration(color: colors.containerLow)`), ALWAYS wrap the tile in `Material(type: MaterialType.transparency, child: ...)` to prevent Flutter's debug assertion (`The ListTile is wrapped in a DecoratedBox that has a background color`).

4. **`ListView` Viewport Culling & Scroll Order Preservation**:
   - `ListView` lazily builds only children visible within the active test viewport (typically `1200×900` in widget tests).
   - When adding a new card or section to an existing scrollable view (such as `SettingsView`), place it **below** existing tested sections (or ensure tests scroll via `tester.ensureVisible` / `tester.scrollUntilVisible`) so existing tests that assert on cards like `Server Connection` or `AI Indexing & Vectors` do not fail due to those cards being pushed off-screen.
   - Before calling `tester.tap(finder)` on an element inside a `SingleChildScrollView` or `ListView`, always call `await tester.ensureVisible(finder); await tester.pumpAndSettle();`.

5. **Riverpod `Notifier` Disposal Guards & Test Vectors**:
   - Any `Notifier` that schedules async work in `build()` (e.g., `Future.microtask(...)`) MUST track disposal via `ref.onDispose(() => _isDisposed = true)` and check `if (!_isDisposed)` before mutating `state`.
   - Any pure-Dart cryptographic, hashing, or encoding test vector (such as SHA-256 / PKCE) MUST be cross-checked with `python3 -c "import hashlib..."` before committing.

6. **Mandatory PR CI Gate**:
   - Every Pull Request MUST pass the GitHub Actions `CI` workflow (`pnpm lint`, `pnpm test`, `pnpm --filter @shelfd/app build:web`).
   - If the local environment does not have the `flutter` SDK installed, the agent MUST watch the GitHub Actions CI run (`gh pr checks <PR> --watch` or `gh run watch`) and resolve any failures before presenting the PR as ready to merge.
