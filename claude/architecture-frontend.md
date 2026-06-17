# Frontend Architecture

Frontend / UI flavor of [architecture.md](~/.claude/architecture.md) — read it first for the shared core. This file is the frontend's flavor of each shared section, plus a few UI-only ones; it restates no principle and never references its sibling doc.

## Folder layout

**Browser extension** — multiple entrypoints:

```
src/
  entrypoints/
    popup/         # Popup.tsx (boundary) + dependencies.ts (composition root) + scoped components
    options/
    editor/
    content/       # Content.tsx + content-only logic
    background/
  features/        # slices promoted because 2+ entrypoints use them
  shared/
    ui/            # reusable components (design system)
    lib/           # framework-agnostic helpers
  infra/           # adapters: clipboard, storage, messaging, configured API client
```

**SPA** — a single entrypoint; the folder is singular `entrypoint/`, with `pages/` alongside:

```
src/
  entrypoint/      # the one mount: shell, router, providers, composition root
  pages/           # route boundaries — one per route
  features/
  shared/{ui,lib}
  infra/           # configured API client, query client, storage adapters
```

Grow a second surface (an embed, an extension) and `entrypoint/` pluralizes to `entrypoints/<surface>/` — the extension layout above, each surface holding its own boundary and pages.

## Entrypoints — mount surfaces

A frontend entrypoint is a **mount surface**: where a UI tree attaches to the platform.

- SPA → one entrypoint (the app mount): router + providers + shell.
- Browser extension → several: popup, options, a content script, the background worker — each its own bundle and mount.

Each entrypoint owns its **composition root** (constructs adapters, injects them — e.g. a `dependencies.ts` per surface), its provider/shell setup, and app-wide cross-cutting (error boundary, theme, auth guard). This is why an entrypoint is a mount surface, not a page: an extension has no pages but several clear entrypoints.

## Layering

The inward chain is `entrypoint → page → feature → core/ports`.

Each layer is earned:

- **page** — the route boundary (maps URL/params in, composes features, renders, no business logic). Only when the entrypoint routes; a single-view surface (popup, options, overlay) uses its mount root as the boundary instead.
- **feature** — only when logic is worth a reusable, testable unit: 2+ pages/entrypoints share it (see Promotion), or it orchestrates a multi-step flow (optimistic update + rollback, coordinating several ports). Else the boundary reaches `core`/ports directly.

**Import direction**: import only from layers below — entrypoint → page → feature → shared/infra. Never sibling-to-sibling.

## Validation

Form input → validate at the boundary (page/component) with a schema before it leaves for the API. Server semantic rejections (409/422) → map onto the offending field, not a generic toast. Payloads you read → parse through the same schema; trust no response.

## External tech

Edges the UI drives — e.g. the API, browser storage, the clock, `window`/router, platform messaging — each behind a port, implemented by an adapter at the edge. The composition root injects it; components and core never call `fetch` / `chrome.*` / `localStorage` directly. The feature/core names the port (`ScreenshotStore`, `Clipboard`); the adapter binds the tech (`ChromeStorage`).

## State

The UI holds long-lived client state — two kinds, kept apart:

- **Server cache** — data mirrored from the API (staleness, refetch, invalidation) → a data-fetching library (TanStack Query, a Solid resource) behind a port, not hand-rolled in components.
- **UI state** — local, ephemeral (open/closed, selection, draft) → component-local signals, lifted only as far as use demands.

## Promotion — where a component lives

Start scoped, promote on reuse — the CLAUDE.md duplication rule (tolerate 1–2 sites, extract on the 3rd) for UI:

1. **In its entrypoint** — used by one surface, lives there (e.g. `popup/ScreenshotCard.tsx`).
2. **→ `features/`** — a 2nd entrypoint needs the same slice; promote logic + components + API access together.
3. **→ `shared/ui`** — generic and broadly reused, no feature logic.

So `features/` holds the **promoted** slices: with many entrypoints, a slice goes top-level only once 2+ share it.

## Rendering

Components are the thin boundary: props in, events out, no business logic. Derivations and decisions live in the feature/core (diff / format / transform). A component branching on a business rule is a missing feature function.

## Living example — snipping-tool extension

| Concept | Location |
|---|---|
| entrypoints (mount surfaces) | `extension/entrypoints/{popup,options,editor,content,background}` |
| per-entrypoint composition root | `popup/dependencies.tsx`, `content/dependencies.ts` |
| boundary (no router → root component) | `Popup.tsx`, `Editor.tsx`, `Content.tsx` |
| scoped, not yet promoted | `popup/ScreenshotCard.tsx`, `content/overlayTools/` |
| adapters behind ports | `extension/chrome/*` (ChromeClipboard, ChromeStorage, ChromeMessaging) |
| promoted features + shared ui | `core/*` (editor, overlay, components, apiClient) |

This project lumps features + shared + infra into one `core/` — coarser than the split above. Fine per "match the existing project," but new work uses the finer split.

## Tempted to couple? Do this instead

| Temptation | Instead |
|---|---|
| Component calls `fetch` / `chrome.*` / `localStorage` | Port + adapter at the edge; inject via the composition root |
| Business rule inside a component (`if balance > …`) | Push to a feature function / pure core; render the result |
| Server data in a hand-rolled signal with manual refetch | A data-fetching library behind a port |
| Component imported across features | Promote to `features/` or `shared/ui` |
