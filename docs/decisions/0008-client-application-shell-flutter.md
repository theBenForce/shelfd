# Client Application Shell — Flutter for Mobile & Desktop

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

`shelfd` needs a polished, dedicated reading application (`Shelf`) that works seamlessly across mobile devices (phones and tablets) and desktop computers (macOS, Linux, Windows), with offline caching capabilities and rich typography controls. Which framework should be chosen for the client application?

## Decision Drivers

* Cross-platform support from a single codebase (iOS, Android, macOS, Linux, Windows, Web).
* High-performance rendering engine with smooth scrolling and custom layout control.
* Mature typography, pagination, and reader rendering capabilities.
* Strong developer tooling and declarative UI architecture.

## Considered Options

* **Flutter**
* **React Native / Expo**
* **Tauri + Web Frontend (Vue/Svelte)**
* **Native Swift/Kotlin (Separate Apps)**

## Decision Outcome

Chosen option: **Flutter**, because it provides a consistent, pixel-level rendering engine across mobile and desktop, ideal for custom reader typography, cover grids, and pagination.

### Positive Consequences

* Single shared codebase in `apps/app/` across mobile and desktop.
* Fine-grained control over font rendering, line spacing, margins, and dark/light/sepia themes.
* Direct compilation to native arm64/x86 binaries with smooth 60/120fps UI performance.
* Clean integration with Turborepo via wrapper `package.json` scripts (`flutter build`, `flutter test`).

### Negative Consequences

* Higher baseline app bundle size compared to native Swift/Kotlin apps.
* EPUB rendering requires either a webview-based reader or custom Dart layout engines.

## Pros and Cons of the Options

### Flutter

* Good, because identical rendering and UI feel across iOS, Android, macOS, Linux, and Windows.
* Good, because declarative UI with robust state management (Riverpod/Bloc).
* Good, because strong canvas rendering primitives for custom reading views.
* Bad, because larger binary size (~15–25MB minimum).

### React Native / Expo

* Good, because large community and JavaScript ecosystem.
* Bad, because desktop support (React Native macOS/Windows) is significantly less mature than Flutter.

### Tauri + Web

* Good, because tiny desktop footprint and web technologies.
* Bad, because mobile support in Tauri v2 is still stabilizing compared to Flutter's mature mobile platform.

### Native Swift & Kotlin

* Good, because smallest app sizes and 100% native platform integration.
* Bad, because requires writing and maintaining two completely separate codebases for the client.
