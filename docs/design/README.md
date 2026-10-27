# Design mock-up

Frontend mock-up (#18 onwards). Live canvas (private): https://claude.ai/artifact/NG1s4yndHBJBWWjLfD4dsr

These are the canvas source files. The `.dc.html` files need the canvas runtime (`support.js`),
so open the canvas link to view them; they are kept here as the reference for the React port.

| File | View |
| --- | --- |
| `Main.dc.html` | Login (`error` tweak for the failed OAuth banner) |
| `Leaderboard.dc.html` | Clasificación (`viewState`: ready, loading, no-season, error) |
| `LeaderboardWide.dc.html` | Clasificación at 1280 px |
| `Activities.dc.html` | Mis actividades (`viewState`: ready, loading, empty, error; `lastPage`) |
| `Rules.dc.html` | Cómo se puntúa |
| `States.dc.html` | Loading, empty and error states side by side |
| `styles.css` | Tokens, light/dark themes, one section per component |
| `canvas.json` | Canvas layout |

Porting notes:
- Markup uses only classes from `styles.css` (BEM, one section per React component).
- Formatting helpers (`formatDuration`, `formatDistance`, `formatDay`, `groupByDay`, `zeroReason`)
  are in the `<script>` of `Activities.dc.html`.
- Breakpoints are container queries on `.app`; in the SPA `.app` fills the window.
- Links point to the other artboards; in the SPA: Strava button -> `/auth/strava` (full page load),
  logout -> `POST /auth/logout`.
- The `[data-theme]` blocks in `styles.css` are review-only overrides; drop them in the SPA.
