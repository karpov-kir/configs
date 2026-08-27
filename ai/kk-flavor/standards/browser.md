# Driving a Browser

Two MCP servers reach Chrome. `playwright` is the default; `chrome-devtools` is the exception you justify.

- **`playwright` unless the human's own session is the point.** It launches its own Chrome in a clean profile: signed in nowhere, sharing nothing with the browser they have open, and the same on the next run.
- **`chrome-devtools` when a clean profile cannot reach the page** — it sits behind the human's login, or in a tab they set up, or holds state you must not discard. It attaches to their running Chrome and drives every window of that profile.

Through `chrome-devtools` you act as the human, inside their real accounts. [live-systems.md](live-systems.md) binds every click there.
