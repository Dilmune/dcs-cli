# CLI design standard

This is the visual and interaction law for every terminal surface the CLI
prints: plain commands, `dcs ui`, errors, prompts. It is set once here and
never re-decided per command. A deviation is a defect, the same as a bug.

Emotional target: calm, dev-native. Influence: macOS Settings and Apple's
receipt emails. Typographic, aligned, one accent for one job, no boxes.

## Tokens

One theme package, `internal/ui/theme.go`, owns every color. `internal/workspace`
imports it and defines nothing of its own except the logo palette. OKLCH only,
converted to sRGB hex at the terminal boundary. No hex literal appears in any
other file.

| Token     | Light               | Dim                  | Dark                 | Job |
|-----------|---------------------|----------------------|----------------------|-----|
| accent    | oklch(.47 .16 35)   | oklch(.435 .17 30)   | oklch(.70 .16 35)    | selection marker, selected label, `$` prompt, section title |
| muted     | oklch(.50 .012 260) | oklch(.44 .015 50)   | oklch(.60 .01 260)   | labels, descriptions, hints, table headers |
| divider   | oklch(.90 .008 80)  | oklch(.84 .025 75)   | oklch(.34 .008 260)  | rules, the pane separator, table header rule |
| success   | oklch(.55 .15 150)  | same                 | oklch(.75 .16 150)   | `●` and word for healthy states |
| warning   | oklch(.60 .14 80)   | same                 | oklch(.80 .15 85)    | `◐` and word for in-progress states |
| danger    | oklch(.55 .20 27)   | same                 | oklch(.68 .19 25)    | `✕` and word for failed states, error prefix |

Body text is never styled. It uses the terminal's own foreground so it is
readable on any background. Bold is the only emphasis for titles.

Mode resolution runs once at process start: `--theme` if given, else
background detection, else dark. `NO_COLOR`, `--no-color`, a non-TTY stdout
or `--quiet` produce the same layout with no escape sequences. Layout never
depends on color.

## Layout

- Left margin 2 for plain output, 3 inside the workspace canvas.
- One blank line before a section title, one after. Never two.
- Section titles: bold, terminal foreground. The accent is not a heading color.
- Key-value blocks: label muted, right-aligned in a 12-cell column, two
  spaces, value in terminal foreground. IDs, IPs, timestamps, sizes and counts
  carry no styling. An empty value hides the row; a bare label is never printed.
- Tables: headers uppercase muted, one rule in the divider token, cells in
  terminal foreground, three-cell gutter. Columns with a known shape (status,
  region, provider, IPv4, dates) are reserved first; only free-text columns
  shrink, and the ellipsis is `…`, one cell.
- Status cells and status fields always show glyph and word:
  `● active`, `◐ deploying`, `○ off`, `✕ failed`, `· deleted`. Color is
  reinforcement, never the only signal.
- No boxes, borders or banners in command output. The welcome cube in
  `dcs ui --welcome` and the login banner are the only artwork, drawn once.

## dcs status

Prints the workspace overview, static: brand line, account line, then the
six areas with their live counts in the same typography as `dcs ui`. The
JSON shape is unchanged.

## Help

`dcs --help` groups commands into the workspace's six areas with Cobra
command groups, in this order: Servers, Sites & deploys, Databases, Storage,
Access, Operations. `completion`, `docs`, `help` and `version` sit last under
Operations. Descriptions stay as they are.

## Motion and timing

- Any loading indicator appears only after 150ms. A fast read never flashes.
- Spinner frame interval 80ms.
- Cursor movement and view changes have no animation.
- Nothing blinks.

## Account

When the API returns no display name, the name field falls back to the
email and the separate email row is omitted.

## Verification

A change to any of this ships only with:

1. Ghostty screenshots in dark, light and dim at 80 and 120 columns for
   `status`, `servers list`, `servers info`, `whoami`, `--help` and the
   workspace overview, servers list and one detail.
2. `--no-color` output read as text: same layout, zero escape sequences.
3. `--json` output for every changed command byte-identical to the previous
   release.
4. Pass signal: every surface uses exactly the tokens above and nothing else.
   A grep for `#[0-9a-fA-F]{6}` outside `theme.go` and the logo returns nothing.

Two scripts produce this evidence. `scripts/verify-terminal.sh <dcs-binary>
<out-dir>` writes the item 1 screenshots (macOS, Ghostty, tmux). It quits
Ghostty after every capture, so run it from another terminal such as
Terminal.app with Ghostty closed. `scripts/verify-output.sh <new-binary>
[<baseline-binary>]` checks items 2 to 4 and prints PASS or FAIL per check;
it runs only read-only commands.
