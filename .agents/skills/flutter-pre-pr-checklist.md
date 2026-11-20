---
name: flutter-pre-pr-checklist
description: Pre-PR verification checklist and layout safety guardrails for Flutter changes in apps/app. Use this whenever creating or modifying Flutter widgets, providers, or tests.
---

# Flutter Pre-PR Verification & Layout Safety Skill

Use this checklist whenever modifying Dart/Flutter files under `apps/app/` before opening or updating a Pull Request.

## 1. Package API Verification
- [ ] Grep `apps/app/lib/` and `apps/app/test/` for existing usage of any package from `apps/app/pubspec.yaml` before writing new calls:
  - `file_picker: ^12.2.0`: Use `final result = await FilePicker.pickFiles(...);` and `await result.first.readAsBytes();` (never `FilePicker.platform`).
  - `flutter_riverpod: ^3.4.3`: Override repository providers with `.overrideWithValue(mockRepo)` and `NotifierProvider`s with `.overrideWith(MockNotifier.new)`.

## 2. Widget Layout & `Ahem` Test Font Audit
- [ ] **Every `Row` with text**: Wrap `Text` children in `Expanded` or `Flexible` so Flutter's wide `Ahem` test font does not trigger `RenderFlex overflowed` assertions.
- [ ] **Every `ListTile` / `SwitchListTile` / `CheckboxListTile` inside a colored `Container`**: Wrap the tile in `Material(type: MaterialType.transparency, child: ...)` so Flutter's debug `DecoratedBox` assertion does not fire.
- [ ] **Scrollable views (`ListView` / `SingleChildScrollView`)**:
  - Do not insert tall new cards above existing cards in `SettingsView` or `BookDetailView` if existing tests expect those cards in the initial `1200×900` viewport without scrolling.
  - In widget tests, always call `await tester.ensureVisible(finder); await tester.pumpAndSettle();` before `await tester.tap(finder);` on items inside scrollable containers.

## 3. Riverpod & Async Safety
- [ ] Guard async methods in `Notifier`s with a `_isDisposed` flag set in `ref.onDispose(() { _isDisposed = true; })` so tests disposing `ProviderScope` early do not throw post-dispose state errors.
- [ ] Avoid watching authenticated/linked `FutureProvider`s when the parent account state is unlinked.

## 4. UI Screenshots (No Backend Required)
- [ ] When capturing screenshots for UI changes, write Flutter golden/screenshot tests under `apps/app/test/` using mock repositories and Riverpod `.overrideWithValue(...)` / `.overrideWith(...)` — do not spin up the Go backend or database just for UI screenshots.
- [ ] Use a Flutter golden/screenshot library such as `golden_screenshot` (or `alchemist`) with `matchesGoldenFile(...)` and real font/asset loading, and generate screenshots via `flutter test --update-goldens`.

## 5. Test Execution & CI Gate
- [ ] Run backend tests: `cd apps/server && go test -tags fts5 ./...`
- [ ] Run Flutter analyzer and tests locally if `flutter` is installed (`flutter analyze && flutter test && flutter build web`).
- [ ] After pushing the branch and opening/updating a PR, ALWAYS verify GitHub Actions CI passes (`gh pr checks --watch` or `gh run watch --exit-status`) before asking the user to merge.
