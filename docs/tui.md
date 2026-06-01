# Terminal UI

The terminal UI gives the same source-group model as the browser UI without
leaving the terminal, so backend, CLI, browser, journal, HTTP, and SDK records
can still be viewed side by side or merged while you stay in a shell.

```bash
devlogbus tui
devlogbus tui --endpoint prod-box:7422 --replay-per-source 500
```

The default replay window is 1000 records per source.

## Views

Merged view is one chronological stream across included sources.

By-source view renders source or source-group panes. Browser Tap `sourceGroup`
values become parent panes. Press `enter` on a grouped pane to drill into child
sources, then `esc` or `backspace` to return to the parent source list.

## Keyboard Reference

| Key | Action |
| --- | --- |
| `?` | Open help |
| `q`, `ctrl+c` | Quit |
| `/` | Start search |
| `m` | Toggle merged/by-source view |
| `a` | Cycle tiled, vertical, horizontal layouts |
| `tab`, `shift+tab` | Move between included source panes |
| `[`, `]`, `left`, `right`, `h` | Move across source chips |
| `enter` | Drill into focused source group in by-source view |
| `esc`, `backspace` | Leave group drilldown or clear search |
| `up`, `down`, `k`, `j` | Move selected record |
| `pgup`, `pgdown` | Page records |
| `home`, `g` | Jump to first record |
| `end`, `G` | Jump to latest record |
| `1` | Toggle DEBUG |
| `2` | Toggle INFO |
| `3` | Toggle WARN |
| `4` | Toggle ERROR |
| `s` | Include or exclude focused source/group |
| `p` | Pause active merged stream, source, or group |
| `b` | Toggle follow-bottom |
| `d` | Toggle inline details |
| `c` | Clear visible records for the active pane |
| `x` | Queue expunge confirmation |
| `+`, `-` | Change tiled pane width |

## Search

Search filters records by time, level, source, source group, message, and
attributes. Press `/`, type the search text, then press `enter` or `esc` to
leave input mode. Press `esc` again to clear the search.

## Clear And Expunge

`c` clears the active UI pane only.

`x` asks for confirmation before expunging records from the daemon replay
buffer. In grouped panes, expunge deletes replay records for each child source
in the group.
