# `serve`

`probaci serve` opens a web UI on this machine: the repositories a run covers, a form to start a run
with the stages you pick, its output as it happens, and every earlier run. It is an option beside
`run` and `tui`, not a replacement; a bare `probaci` still prints help.

## Starting it

```sh
probaci serve                  # the current repository, 127.0.0.1:7844, opens a browser
probaci serve ../api ../web    # several repositories, as for run
probaci serve --no-open        # print the address only
probaci serve --read-only      # look at runs, start none
```

It prints an address like this:

```text
probaci web UI: http://127.0.0.1:7844/?token=…
press Ctrl-C to stop
```

The token in the address signs the browser in, once. The browser swaps it for a session cookie and
drops it from the address bar. Without it every page answers 401.

If port 7844 is taken, `serve` uses the next free port and says which, trying up to twenty. `--port`
pins a port and fails when that port is taken.

A browser opens only for a person at a terminal: never when `CI` is set, when
`PROBACI_NO_BROWSER=1`, when stdout is not a terminal, or on Linux and the BSDs with neither
`DISPLAY` nor `WAYLAND_DISPLAY`.

## Flags

| Flag | What it does |
|---|---|
| `--addr HOST[:PORT]` | The address to listen on (default `127.0.0.1`). A port here pins it, like `--port`. |
| `--port N` | The port to listen on (default 7844). Given, it is pinned. |
| `--allow-remote` | Allow a non-loopback `--addr`. Plain HTTP: prefer `ssh -L 7844:127.0.0.1:7844 <host>`. |
| `--allow-host NAME` | With `--allow-remote`, an extra `Host` header to accept, such as a proxy's name. Repeatable. |
| `--read-only` | Serve no route that starts or cancels a run. |
| `--no-open` | Do not open a browser. |

The global `--repos`, `-C`, `--config`, `--no-docker`, `--backend`, `--pull`, `--platform` and
`--full-image` apply to every run started from the UI, as they do to `run`.

## Pages

- **Overview** lists the repositories with the languages detected in each, says whether containers
  can run, and holds the run form: every enabled stage, ticked, and how many repositories run at
  once when there is more than one. It also lists the five newest runs.
- **Runs** lists every run, newest first, with the stages it ran and its result.
- **A run** shows each repository's stages with their result, summary, time and output, and the
  run's streamed output below. While it runs the page updates as each stage starts and finishes,
  and **Cancel run** stops it. **Report (JSON)** downloads the results.
- **Settings** shows the address, the mode, where runs are kept, the configuration files read and
  the enabled stages.

One run goes at a time: starting a second while one is in progress points you to the first.
Every run builds probaci afresh, so it reads the configuration again and writes its own log file,
exactly as `probaci run` does.

## Where runs are kept

Each finished run is saved as one JSON file in `$XDG_STATE_HOME/probaci/runs`, else
`~/.local/state/probaci/runs` (`%LocalAppData%\probaci\runs` on Windows), and the Runs page lists
the newest hundred. A run keeps its first 5,000 lines of output; the run's log file has them all.
A run the server stopped before it finished is listed as canceled.

## Security

The server is built for one person on one machine, and holds to the same rules as the other
Simtabi web UIs:

- It listens on loopback unless told otherwise, and answers only to the `Host` it printed, which
  stops a DNS-rebinding page reaching it.
- The sign-in token is 32 random bytes, and the session cookie is `HttpOnly` and `SameSite=Strict`.
- Every request that changes something must come from the server's own pages (`Origin`, or
  `Sec-Fetch-Site: same-origin`) and carry the session's CSRF token.
- Every response sends a Content-Security-Policy allowing nothing but this server, and nothing is
  inline. The page fetches nothing from the network: Tabler's CSS, scripts and icons are embedded,
  and their licenses are on the Third-party licenses page and in `third-party-notices.md`.
- Stage output is shown after probaci's redactor has removed the secret values it knows.

`/healthz` answers `probaci` to a request carrying the token in an `X-Probaci-Token` header, and
401 to anything else.

---

[← Docs index](../../README.md#documentation)
