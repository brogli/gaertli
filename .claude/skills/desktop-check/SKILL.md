---
name: desktop-check
description: Use when checking nebegit in the real app rather than in tests - a manual-check item, a layout at some width, native windows (diff window, second window, restore after quit), keyboard behaviour, or a bug the mock e2e cannot show. Covers the Linux desktop app (WebKitGTK under WSLg) driven with xdotool, and server mode in a browser.
---

# Desktop check

Claude runs every manual check it can. The owner keeps only the file picker, the Windows host (the Windows build, WSL from Windows), their own editor and desktop, and taste calls.

## Pick the cheapest level

1. The real-backend e2e suite (`task e2e:real`, Playwright against Wails server mode with the real core) covers most flows. Server mode serves the real app to a browser and finds far more than the mock e2e. Events sent before the page's websocket connects are lost.
2. The desktop app, only for what server mode cannot do: native windows (the diff window and its keys, a second window, switching in place, restore after quit) and WebKitGTK looks (overlay scrollbars, font metrics, narrow layouts).

## Desktop app

Only when `pgrep -x nebegit` prints nothing: the app is single-instance, and the owner's may be running.

1. `nix develop --command task build`.
2. `export SP=<a fresh scratch dir>`; `bash mkrepos.sh $SP/t` builds scratch repos: a bare `remote.git`, `work` (branches, a tag, a merge, a conflict pair, CRLF, binary and no-newline files) and an `other` clone.
3. In `nix shell nixpkgs#xdotool nixpkgs#xorg.xwd nixpkgs#xorg.xwininfo nixpkgs#imagemagick nixpkgs#xclip`: `./g start $SP/t/work`, then `click`/`key`/`shot` (run `g` without arguments for usage). Screenshots land in `$SP/shots/`; read them to judge.

`g start` uses `$SP/cfg` as the config dir, never the owner's settings, and unsets Claude's `GIT_EDITOR=true`. Set the scratch config's editor to a harmless command before any "Open in …": a real one once launched the owner's Windows editor through WSL interop.

## WSLg traps

- Weston ignores raise, focus and close requests: minimize other windows first (`xdotool windowminimize <id>`); a new window can open off-screen (`g use <id>; g move`); close a window with a WM_DELETE_WINDOW client message (python-xlib).
- Keys reach a WSLg window only while it has keyboard focus on the Windows side. While the owner works in another Windows app, keys go nowhere (clicks still work): check keyboard behaviour in Chromium instead, against server mode or `vite --mode e2e`.
- A GUI started from a tool call must not inherit the tool's stdout, or the call hangs until the app exits (`g start` redirects every stream).
- Never `pkill -f <pattern>` with the pattern in the same command line: it kills the calling shell. `g stop` uses `pkill -x`.

## What to look for

- WebKitGTK differs from Chromium: overlay scrollbars cover a sideways-scrolling element's last row and swallow clicks there; flex items keep their content width, so toolbars and filter bars that pass the Chromium e2e can overlap. Check narrow widths on the desktop before calling a layout done.
- CodeMirror renders only visible lines: jump (next change) before clicking a far line.
- A bug found gets a failing test first, then the fix.
