# Driving a Browser

- **`playwright` unless the browser the human is using is the point.** It drives a Chrome of its own, which may already be signed in from an earlier run.
- **`chrome-devtools` when the page needs the session they are signed into right now** — in a tab they set up, or holding state you must not discard. It attaches to their running Chrome and drives every window of that profile.

**A `playwright` profile takes one browser at a time**, so a second session driving the same project cannot launch one. Read that as another session holding it rather than a fault, and coordinate instead of retrying.

A click can act as the human in either. [live-systems.md](live-systems.md) binds every click through `chrome-devtools`, and every click through `playwright` once its profile turns out to be signed in.
