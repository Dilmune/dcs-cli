package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/ui"
)

func testModel() model {
	root := Item{ID: "home", Title: "Your workspace", Children: []Item{
		{ID: "servers", Title: "Servers", Children: []Item{{ID: "live", Title: "Your servers", Request: &Request{Kind: Servers}}}},
		{ID: "sites", Title: "Sites & deploys", Body: "Sites"},
		{ID: "db", Title: "Databases", Children: []Item{{ID: "schema", Title: "dcs db schema", Command: "dcs db schema <name>", Body: "Inspect schema"}}},
		{ID: "storage", Title: "Storage", Children: []Item{{ID: "delete", Title: "dcs storage delete", Command: "dcs storage delete <key>", Body: "Deletes a file"}}},
		{ID: "access", Title: "Access", Body: "Keys"},
		{ID: "operations", Title: "Operations", Body: "Logs"},
	}}
	m := newModel(Options{Output: io.Discard, Catalog: root, NoColor: true, Mode: ui.ModeLight, RememberWelcome: func() error { return nil }})
	m.identify = func() tea.Msg { return identityMsg{name: "Test account"} }
	m.fetch = func(req Request, generation int) (tea.Cmd, context.CancelFunc) {
		return func() tea.Msg {
			return loadedMsg{generation: generation, item: Item{ID: "live", Title: "Your servers", Request: &req, Children: []Item{{ID: "test", Title: "Fixture server", Body: "Details"}}}}
		}, func() {}
	}
	return m
}

// settle runs a command, expanding batches, and returns every message it
// produced; ticks are waited out so the caller sees them as the program would.
func settle(t *testing.T, command tea.Cmd) []tea.Msg {
	t.Helper()
	require.NotNil(t, command)
	msg := command()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var messages []tea.Msg
	for _, sub := range batch {
		if sub != nil {
			messages = append(messages, settle(t, sub)...)
		}
	}
	return messages
}

func loadedFrom(t *testing.T, command tea.Cmd) loadedMsg {
	t.Helper()
	for _, msg := range settle(t, command) {
		if loaded, ok := msg.(loadedMsg); ok {
			return loaded
		}
	}
	t.Fatal("command produced no loadedMsg")
	return loadedMsg{}
}

func press(m model, key tea.KeyType, runes ...rune) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: key, Runes: runes})
	return next.(model), cmd
}

func TestWorkspaceNavigationSearchAndInertReferences(t *testing.T) {
	m := testModel()
	var command tea.Cmd
	m, command = press(m, tea.KeyRunes, '3')
	assert.Equal(t, "Databases", m.current.Title)
	assert.Nil(t, command)
	m, _ = press(m, tea.KeyRunes, []rune("/delete")...)
	require.True(t, m.searching)
	require.Len(t, m.results(), 1)
	m, command = press(m, tea.KeyEnter)
	assert.Equal(t, "dcs storage delete", m.current.Title)
	assert.Nil(t, command, "opening a mutating command must never execute it")
	assert.Contains(t, m.View(), "NOT EXECUTED")
	m, _ = press(m, tea.KeyEsc)
	assert.Equal(t, "Databases", m.current.Title)
	assert.NotPanics(t, func() { m.View() })
	m, _ = press(m, tea.KeyRunes, '0')
	assert.True(t, m.isHome())
	m, _ = press(m, tea.KeyRunes, '6')
	assert.Equal(t, "Operations", m.current.Title)
}

func TestWorkspaceSearchRestoresCursorAndPasteIsNeverAnAction(t *testing.T) {
	m := testModel()
	m.cursor = 5
	m, _ = press(m, tea.KeyRunes, '/')
	m, _ = press(m, tea.KeyRunes, []rune("schema")...)
	m, _ = press(m, tea.KeyEnter)
	m, _ = press(m, tea.KeyEsc)
	assert.Equal(t, 5, m.cursor)
	assert.NotPanics(t, func() { m.View() })
	next, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q\rdelete"), Paste: true})
	m = next.(model)
	assert.Nil(t, command)
	assert.True(t, m.searching)
	assert.Equal(t, "qdelete", string(m.query))
	m, _ = press(m, tea.KeyCtrlU)
	assert.Empty(t, m.query)
	m, _ = press(m, tea.KeyRunes, []rune("no such command")...)
	m, command = press(m, tea.KeyEnter)
	assert.Nil(t, command)
	assert.Contains(t, m.View(), "No matches")
	m, _ = press(m, tea.KeyEsc)
	assert.Equal(t, 5, m.cursor)
}

func TestWorkspaceSearchDoesNotDuplicateTheCurrentCommandArea(t *testing.T) {
	m := testModel()
	m, _ = press(m, tea.KeyRunes, '3')
	m, _ = press(m, tea.KeyRunes, []rune("/schema")...)
	require.Len(t, m.results(), 1)
	assert.Equal(t, "schema", m.results()[0].item.ID)
}

func TestWorkspaceSearchPreservesTypedAndPastedSpaces(t *testing.T) {
	for _, query := range []string{"buckets", "storage buckets", "servers delete", "Next page", "  two  words  "} {
		for _, pasted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/pasted=%t", query, pasted), func(t *testing.T) {
				m := testModel()
				m.root.Children = []Item{{ID: "target", Title: query, Command: query, Body: "Reference"}}
				m.current = m.root
				if pasted {
					next, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(query), Paste: true})
					m = next.(model)
					require.Nil(t, command)
				} else {
					m, _ = press(m, tea.KeyRunes, '/')
					for _, r := range query {
						key := tea.KeyRunes
						if r == ' ' {
							key = tea.KeySpace
						}
						var command tea.Cmd
						m, command = press(m, key, r)
						require.Nil(t, command)
					}
				}
				require.True(t, m.searching)
				require.Equal(t, query, string(m.query))
				require.Len(t, m.results(), 1)
				assert.Equal(t, "target", m.results()[0].item.ID)
				m, command := press(m, tea.KeyEnter)
				assert.Equal(t, "target", m.current.ID)
				assert.Nil(t, command, "a search result must remain an inert command reference")
			})
		}
	}
}

func TestWorkspaceSearchSpaceLimitAndEditing(t *testing.T) {
	m := testModel()
	m.cursor = 5
	m, _ = press(m, tea.KeyRunes, '/')
	m, _ = press(m, tea.KeyRunes, []rune(strings.Repeat("界", queryLimit-1))...)
	m.cursor = 3
	m, _ = press(m, tea.KeySpace, ' ')
	require.Equal(t, strings.Repeat("界", queryLimit-1)+" ", string(m.query))
	assert.Zero(t, m.cursor)
	m, _ = press(m, tea.KeySpace, ' ')
	m, _ = press(m, tea.KeyRunes, 'x')
	require.Len(t, m.query, queryLimit)
	m, _ = press(m, tea.KeyBackspace)
	require.Equal(t, strings.Repeat("界", queryLimit-1), string(m.query))
	m, _ = press(m, tea.KeySpace, ' ')
	m, _ = press(m, tea.KeyCtrlH)
	require.Len(t, m.query, queryLimit-1)
	m, _ = press(m, tea.KeyCtrlU)
	assert.Empty(t, m.query)
	m, _ = press(m, tea.KeySpace, ' ')
	require.Equal(t, " ", string(m.query))
	m, _ = press(m, tea.KeyEsc)
	assert.False(t, m.searching)
	assert.Empty(t, m.query)
	assert.Equal(t, 5, m.cursor)
}

func TestWorkspaceLoadsRefreshesAndCancelsStaleResults(t *testing.T) {
	m := testModel()
	m, _ = press(m, tea.KeyRunes, '1')
	m, command := press(m, tea.KeyEnter)
	require.True(t, m.pending)
	require.NotNil(t, command)
	oldMessage := loadedFrom(t, command)
	canceled := false
	m.cancelLoad = func() { canceled = true }
	m, _ = press(m, tea.KeyEsc)
	assert.True(t, canceled)
	next, _ := m.Update(oldMessage)
	m = next.(model)
	assert.Equal(t, "Servers", m.current.Title)
	m, command = press(m, tea.KeyEnter)
	next, _ = m.Update(loadedFrom(t, command))
	m = next.(model)
	assert.False(t, m.pending)
	assert.Equal(t, "Fixture server", m.current.Children[0].Title)
	m, command = press(m, tea.KeyRunes, 'r')
	assert.True(t, m.pending, "refresh must return the modified model, not its old value")
	require.NotNil(t, command)
	m, _ = press(m, tea.KeyRunes, '4')
	m, command = press(m, tea.KeyEsc)
	assert.True(t, m.pending, "returning to a pending view restarts its canceled read")
	require.NotNil(t, command)
}

func TestWorkspaceLoadingIndicatorWaitsForSlowReadsOnly(t *testing.T) {
	m := testModel()
	m, _ = press(m, tea.KeyRunes, '1')
	// The tick's timer starts inside load(), so the clock must start before Enter.
	started := time.Now()
	m, command := press(m, tea.KeyEnter)
	require.True(t, m.pending)
	assert.False(t, m.loading, "a read that just started has nothing to show yet")
	assert.NotContains(t, m.View(), "Loading...")

	messages := settle(t, command)
	var loaded loadedMsg
	var tick loadingTickMsg
	for _, msg := range messages {
		switch typed := msg.(type) {
		case loadedMsg:
			loaded = typed
		case loadingTickMsg:
			tick = typed
		}
	}
	require.NotZero(t, loaded.item.ID)
	require.Equal(t, m.generation, tick.generation)
	assert.GreaterOrEqual(t, time.Since(started), loadingDelay, "the indicator tick fires no earlier than 150ms")

	fast, _ := m.Update(loaded)
	fast, _ = fast.(model).Update(tick)
	fastModel := fast.(model)
	assert.False(t, fastModel.pending)
	assert.False(t, fastModel.loading, "a tick landing after the read finished must not light the indicator")
	assert.NotContains(t, fastModel.View(), "Loading...")
	assert.Equal(t, "Fixture server", fastModel.current.Children[0].Title)

	slow, _ := m.Update(tick)
	slowModel := slow.(model)
	assert.True(t, slowModel.loading, "a read still pending when the tick fires shows the indicator")
	assert.Contains(t, slowModel.View(), "Loading...")
	assert.Contains(t, slowModel.footer(), "esc cancel request")
	slow, _ = slowModel.Update(loaded)
	slowModel = slow.(model)
	assert.False(t, slowModel.loading)
	assert.NotContains(t, slowModel.View(), "Loading...")

	canceledModel, _ := press(m, tea.KeyEsc)
	canceled, _ := canceledModel.Update(tick)
	canceledModel = canceled.(model)
	assert.False(t, canceledModel.pending)
	assert.False(t, canceledModel.loading, "a tick from a canceled read must not light the indicator")
	assert.NotContains(t, canceledModel.View(), "Loading...")
}

func TestWorkspaceFirstRunOpensDirectlyAndRemembersOnlyOnNavigation(t *testing.T) {
	m := testModel()
	m.welcome = true
	remembered := 0
	m.remember = func() error { remembered++; return nil }
	require.NotNil(t, m.Init())
	next, _ := m.Update(m.Init()())
	m = next.(model)
	assert.Equal(t, "Test account", m.account)
	for _, title := range []string{"Servers", "Sites & deploys", "Databases", "Storage", "Access", "Operations"} {
		assert.Contains(t, m.View(), title, "all six areas must be visible before any keypress")
	}
	assert.Equal(t, 1, strings.Count(m.View(), "Dilmune Cloud"))
	assert.Equal(t, 1, strings.Count(m.View(), "Read-only"))
	assert.NotContains(t, m.View(), "Open workspace")
	_, command := press(m, tea.KeyRunes, 'q')
	assert.IsType(t, tea.QuitMsg{}, command())
	assert.Zero(t, remembered)
	m, command = press(m, tea.KeyDown)
	assert.Equal(t, 1, m.cursor)
	assert.Nil(t, command, "moving the selection must not write preferences")
	m, _ = press(m, tea.KeyRunes, '?')
	assert.True(t, m.help)
	m, _ = press(m, tea.KeyEsc)
	m, command = press(m, tea.KeyEnter)
	assert.False(t, m.welcome)
	assert.Equal(t, "Sites & deploys", m.current.Title, "Enter opens the selected area, not another welcome screen")
	require.NotNil(t, command)
	next, _ = m.Update(command())
	m = next.(model)
	assert.Equal(t, 1, remembered)
	assert.Equal(t, "Test account", m.account)
	next, _ = m.Update(welcomeSavedMsg{err: errors.New("readonly disk")})
	assert.Contains(t, next.(model).View(), "Welcome preference could not be saved")
}

func TestWorkspaceFirstRunSearchDoesNotNeedAnExtraEnter(t *testing.T) {
	m := testModel()
	m.welcome = true
	remembered := 0
	m.remember = func() error { remembered++; return nil }
	m.fetch = func(Request, int) (tea.Cmd, context.CancelFunc) {
		t.Fatal("opening a command reference must not fetch a resource")
		return nil, nil
	}
	m, command := press(m, tea.KeyRunes, []rune("/schema")...)
	require.True(t, m.searching)
	assert.Nil(t, command)
	assert.Zero(t, remembered)
	m, command = press(m, tea.KeyEnter)
	assert.Equal(t, "dcs db schema", m.current.Title)
	assert.False(t, m.welcome)
	require.NotNil(t, command)
	assert.IsType(t, welcomeSavedMsg{}, command(), "the only side effect is the local intro marker")
	assert.Equal(t, 1, remembered)
	m, _ = press(m, tea.KeyEsc)
	assert.True(t, m.isHome())
	assert.False(t, m.welcome, "returning home must not replay the introduction")
}

func TestWorkspaceLayoutBoundsAndWelcomeControls(t *testing.T) {
	for _, theme := range []ui.Mode{ui.ModeLight, ui.ModeDim, ui.ModeDark} {
		for _, size := range [][2]int{{20, 10}, {48, 20}, {60, 24}, {79, 24}, {80, 20}, {80, 21}, {80, 24}, {90, 28}, {110, 32}, {160, 48}} {
			for _, mode := range []string{"home", "welcome", "help", "search", "detail", "error", "loading"} {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", theme, size[0], size[1], mode), func(t *testing.T) {
					m := testModel()
					m.styles = newStyles(io.Discard, theme, true)
					m.width, m.height = size[0], size[1]
					switch mode {
					case "welcome":
						m.welcome = true
					case "help":
						m.help = true
					case "search":
						m.startSearch()
					case "detail":
						m.current = Item{Title: "Details", Body: strings.Repeat("long detail ", 100)}
					case "error":
						m.err = errors.New("\x1b[31mFailure\x1b[0m")
					case "loading":
						m.loading = true
					}
					view := m.View()
					assert.NotContains(t, view, "\x1b")
					lines := strings.Split(view, "\n")
					assert.Len(t, lines, size[1])
					for _, line := range lines {
						assert.LessOrEqual(t, ansi.StringWidth(line), size[0])
					}
					if m.isTooSmall() {
						assert.Contains(t, view, "Ctrl+C")
						return
					}
					assert.LessOrEqual(t, ansi.StringWidth(m.footer()), size[0]-6, "the keyboard footer must fit without truncation")
					if m.welcome {
						assert.Contains(t, view, "Operations")
						assert.Contains(t, view, "q quit")
						assert.NotContains(t, view, "PgDn", "first-run header and all six areas must fit")
					}
				})
			}
		}
	}
}

func TestWorkspaceSanitizesUntrustedText(t *testing.T) {
	item := cleanItem(Item{Title: "\x1b[31mHello\x1b[0m\u202e", Description: "one\r\ntwo", Body: "first\nsecond\x07", Fields: []Field{{Label: "IP", Value: "\x1b]0;evil\x07safe"}}})
	assert.Equal(t, "Hello", item.Title)
	assert.Equal(t, "onetwo", item.Description)
	assert.Equal(t, "first\nsecond", item.Body)
	assert.Equal(t, "safe", item.Fields[0].Value)
	assert.Equal(t, `'a'\''$(touch nope)'`, Quote("a'$(touch nope)"))
}

func TestWorkspaceHelpAndLongDetailsScroll(t *testing.T) {
	m := testModel()
	m, _ = press(m, tea.KeyRunes, '?')
	assert.True(t, m.help)
	m, _ = press(m, tea.KeyPgDown)
	assert.Positive(t, m.scroll)
	m, _ = press(m, tea.KeyEsc)
	assert.False(t, m.help)
	m.current = Item{Title: "Long", Body: strings.Repeat("line\n", 100)}
	assert.Contains(t, m.View(), "PgDn")
	m, _ = press(m, tea.KeyPgDown)
	assert.Positive(t, m.scroll)
	m, _ = press(m, tea.KeyPgUp)
	assert.Zero(t, m.scroll)
}

func TestWorkspaceLogoPixelGeometry(t *testing.T) {
	rows := logoRows(logoWidth)
	require.Len(t, rows, logoHeight)
	assert.Equal(t, 32, logoWidth)
	assert.Equal(t, 17, logoHeight)
	var pixels strings.Builder
	accents := 0
	for _, row := range rows {
		for _, half := range []string{row.upper, row.lower} {
			require.Len(t, half, logoWidth)
			pixels.WriteString(half)
			for _, pixel := range half {
				assert.Contains(t, " cas", string(pixel))
				if pixel == 'a' {
					accents++
				}
			}
		}
	}
	assert.Positive(t, accents)
	assert.Equal(t, "7db17bf6de599e3d0125c0391c16d94e8aac4e890eaed4d32257936e1cfa21b2",
		fmt.Sprintf("%x", sha256.Sum256([]byte(pixels.String()))), "keep the approved artwork, not an approximation")
	require.Len(t, asciiLogoRows(logoWidth), logoHeight)
	for _, row := range asciiLogoRows(logoWidth) {
		assert.Equal(t, logoWidth, len(row), "plain logo must reserve the same header space")
	}
	assert.Equal(t, "6f365721b20614243890928feb92c09805434f2aaf78f278a133d65dc706f696",
		fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(asciiLogoRows(logoWidth), "\n")))))
}

func TestWorkspaceLogoColorProfilesAndLayout(t *testing.T) {
	for _, theme := range []ui.Mode{ui.ModeLight, ui.ModeDim, ui.ModeDark} {
		for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii} {
			t.Run(fmt.Sprintf("%s/%d", theme, profile), func(t *testing.T) {
				r := lipgloss.NewRenderer(io.Discard)
				r.SetColorProfile(profile)
				r.SetHasDarkBackground(theme.HasDarkBackground())
				m := testModel()
				m.styles = workspaceStyles(r, theme)
				m.welcome = true
				logo := strings.Join(m.renderLogo(logoWidth), "\n")
				if profile == termenv.Ascii {
					assert.NotContains(t, logo, "\x1b")
					for _, glyph := range logo {
						assert.Less(t, glyph, rune(128))
					}
				} else {
					assert.Contains(t, logo, "\x1b[")
					assert.NotContains(t, logo, "█", "solid cells must use background fills to avoid font gaps")
					require.Len(t, m.renderLogo(logoWidth), logoHeight)
					for _, line := range m.renderLogo(logoWidth) {
						assert.Equal(t, logoWidth, ansi.StringWidth(line))
					}
				}
				if profile == termenv.TrueColor {
					assert.Contains(t, logo, "\x1b[38;2;", "must emit RGB, not just bold styling")
					assert.Contains(t, logo, "48;2;255;255;215", "cream must match the approved palette")
					assert.Contains(t, logo, "48;2;255;215;175", "shade must match the approved palette")
					assert.Contains(t, logo, "48;2;215;135;95", "terracotta must match the approved palette")
				}
				if profile == termenv.ANSI256 {
					assert.Contains(t, logo, "\x1b[38;5;", "must emit palette colors")
					assert.Contains(t, logo, "48;5;230", "cream must not quantize to gray")
					assert.Contains(t, logo, "48;5;223", "shade must not quantize to gray")
					assert.Contains(t, logo, "48;5;173", "terracotta must match the approved palette")
				}
				assert.NotEqual(t, m.styles.logoAccent.GetForeground(), m.styles.logoFace.GetForeground())
				for _, size := range logoSizes() {
					lines := m.renderLogo(size.width)
					require.Len(t, lines, size.height)
					for _, line := range lines {
						assert.Equal(t, size.width, ansi.StringWidth(line))
						if profile == termenv.Ascii {
							assert.NotContains(t, line, "\x1b")
							for _, glyph := range line {
								assert.Less(t, glyph, rune(128))
							}
						}
					}
				}
				for _, size := range [][2]int{{48, 20}, {79, 24}, {80, 20}, {80, 21}, {80, 24}, {110, 32}} {
					m.width, m.height = size[0], size[1]
					view := m.View()
					assert.Contains(t, ansi.Strip(view), "Operations")
					assert.Contains(t, ansi.Strip(view), "q quit")
					assert.NotContains(t, view, "PgDn", "the first frame must fit without scrolling")
					for _, line := range strings.Split(view, "\n") {
						assert.LessOrEqual(t, ansi.StringWidth(line), size[0])
					}
				}
			})
		}
	}
}

func TestWorkspaceWelcomeReflowsWithoutLosingMenuOrSelection(t *testing.T) {
	m := testModel()
	m.welcome = true
	m.version = "3.7.1"
	m.account = strings.Repeat("Account name ", 12)
	for i := range m.root.Children {
		m.root.Children[i].Description = "Reference: commands for this area"
	}
	m.cursor = 5
	for _, size := range []struct {
		width, height int
		logoWidth     int
		headerAbove   bool
	}{{80, 24, 16, false}, {79, 24, 16, false}, {80, 20, 16, false}, {48, 20, 16, true},
		{60, 24, 16, true}, {64, 24, 16, false}, {80, 21, 16, false}, {86, 51, 16, false},
		{90, 28, 16, false}, {100, 30, 18, false}, {110, 32, 20, false}, {48, 20, 16, true},
		{110, 20, 16, false}, {110, 26, 18, false}, {110, 28, 20, false}, {240, 80, 20, false}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			next, command := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			m = next.(model)
			assert.Nil(t, command, "resizing must not write the introduction preference")
			assert.True(t, m.welcome)
			assert.Equal(t, 5, m.cursor)
			view := m.View()
			lines := strings.Split(view, "\n")
			assert.Len(t, lines, size.height)
			for _, item := range m.root.Children {
				assert.Contains(t, view, item.Title)
			}
			assert.Contains(t, view, "› 6  Operations")
			assert.Contains(t, view, "q quit")
			assert.NotContains(t, view, "PgDn")
			assert.Equal(t, 1, strings.Count(view, "Dilmune Cloud"))
			assert.Equal(t, 1, strings.Count(view, "Read-only"))
			layout := m.responsiveWelcome(size.width-6, size.height-4)
			assert.Equal(t, size.logoWidth, layout.logo.width)
			assert.Equal(t, size.headerAbove, layout.headerAbove)
			start := 1
			if size.headerAbove {
				start += welcomeHeaderHeight
			}
			for i, row := range asciiLogoRows(size.logoWidth) {
				assert.Equal(t, row, lines[i+start][3:3+size.logoWidth], "the whole selected variant must fit beside the menu")
			}
		})
	}
}

func TestWorkspaceWelcomeYieldsToFocusedViewsAndNotices(t *testing.T) {
	m := testModel()
	m.welcome = true
	m.width, m.height = 110, 27
	assert.Equal(t, 20, m.responsiveWelcome(m.width-6, m.height-4).logo.width)
	m.notice = "A local preference could not be saved"
	view := m.View()
	assert.Contains(t, view, m.notice)
	assert.Contains(t, view, "Operations")
	assert.Contains(t, view, "@@@@", "a notice should select a smaller mark, not remove it")
	assert.Equal(t, 18, m.responsiveWelcome(m.width-6, m.height-5).logo.width)
	m.height = 30
	assert.Contains(t, m.View(), "@@@@")
	assert.Contains(t, m.View(), m.notice)
	assert.Equal(t, 20, m.responsiveWelcome(m.width-6, m.height-5).logo.width)
	m, _ = press(m, tea.KeyRunes, '?')
	assert.Contains(t, m.View(), "Keyboard shortcuts")
	assert.NotContains(t, m.View(), "@@@@")
	m, _ = press(m, tea.KeyEsc)
	assert.Contains(t, m.View(), "@@@@")
	m, _ = press(m, tea.KeyRunes, '/')
	assert.Contains(t, m.View(), "Find a command or resource")
	assert.NotContains(t, m.View(), "@@@@")
	m, _ = press(m, tea.KeyEsc)
	m, command := press(m, tea.KeyEnter)
	assert.Equal(t, "Servers", m.current.Title)
	assert.False(t, m.welcome)
	assert.NotContains(t, m.View(), "@@@@")
	require.NotNil(t, command)
	assert.IsType(t, welcomeSavedMsg{}, command())
	m, _ = press(m, tea.KeyEsc)
	assert.True(t, m.isHome())
	assert.NotContains(t, m.View(), "@@@@", "opening an area ends the introduction")
}

func TestWorkspaceLogoHonorsNoColor(t *testing.T) {
	m := testModel()
	m.styles = newStyles(io.Discard, ui.ModeDark, true)
	assert.True(t, m.styles.asciiLogo)
	assert.NotContains(t, strings.Join(m.renderLogo(logoWidth), "\n"), "\x1b")
}

func TestWorkspaceResponsiveLogoGeometry(t *testing.T) {
	hashes := map[int]string{
		16: "34e8a22338012092a411571de780a37db64efbc9c127c5bf0b852f6b7a0fd0ef",
		18: "784dbd3ae43aa9f82dbab71535bcf0a0ccfa900e54ca10fe20e6817f979cb035",
		20: "55fdb75cadc65528431b25402ac61567830e964b76e33fab4f06b62804446916",
		24: "5f8dfd88f47fdbc14b4d4d4e38d540bb33c7587782b93d512d1dca4dba753f96",
		28: "c292fa2bb25aeffec13d0023bb4ae91cd5ec172d05e9111b206d94cbdc9cfd2a",
		32: "7db17bf6de599e3d0125c0391c16d94e8aac4e890eaed4d32257936e1cfa21b2",
		36: "ab17e2a1fadee7cbcfc0eec5c0b97e2b5bcdd3de5b36b403d26759bb9282e70e",
		40: "2abe02a27addfc08e4b059387057c410a9350c2cfbfeba81128150ee45b3e746",
		44: "1765035cade09a2a34d3b4a7bb1d869267dedb6550714c619912730fea3496e6",
		48: "6d89c53d784c11aef37254095f0e4f1a26008d92acefb255fac9781d66ed62eb",
	}
	require.Len(t, logoSizes(), len(hashes))
	for _, size := range logoSizes() {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			rows := logoRows(size.width)
			require.Len(t, rows, size.height)
			var pixels strings.Builder
			for _, row := range rows {
				for _, half := range []string{row.upper, row.lower} {
					require.Len(t, half, size.width)
					pixels.WriteString(half)
					for _, pixel := range half {
						assert.Contains(t, " cas", string(pixel))
					}
				}
			}
			assert.Equal(t, hashes[size.width], fmt.Sprintf("%x", sha256.Sum256([]byte(pixels.String()))))
			for _, pixel := range "cas" {
				assert.Contains(t, pixels.String(), string(pixel), "each size retains every part of the palette")
			}
		})
	}
}

func TestWorkspaceLogoGrowthAndMenuBounds(t *testing.T) {
	m := testModel()
	m.welcome = true
	for _, notice := range []string{"", "Local preference could not be saved"} {
		m.notice = notice
		for _, height := range []int{20, 21, 22, 24, 28, 30, 40, 51} {
			previous := 0
			for width := minimumWidth; width <= 180; width++ {
				m.width, m.height = width, height
				capacity := height - 4
				if notice != "" {
					capacity--
				}
				layout := m.responsiveWelcome(width-6, capacity)
				require.True(t, layout.hasLogo(), "logo must remain visible at %dx%d", width, height)
				assert.LessOrEqual(t, layout.logo.width, 20, "larger terminals must give space to commands, not branding")
				assert.LessOrEqual(t, layout.logo.height, 11)
				assert.GreaterOrEqual(t, layout.logo.width, previous, "a wider window must never select a smaller mark")
				previous = layout.logo.width
				minimumMenuWidth := minimumWelcomeMenuWidth
				if layout.headerAbove {
					minimumMenuWidth = minimumCompactMenuWidth
				}
				assert.LessOrEqual(t, layout.logo.width+layout.gap+minimumMenuWidth, width-6)
				view := m.View()
				assert.NotContains(t, view, "PgDn")
				assert.Contains(t, view, "q quit")
				assert.Contains(t, view, notice)
				for _, item := range m.root.Children {
					assert.Contains(t, view, item.Title)
				}
				lines := strings.Split(view, "\n")
				require.Len(t, lines, height)
				for _, line := range lines {
					assert.LessOrEqual(t, ansi.StringWidth(line), width)
				}
			}
		}
	}
	for width := minimumWidth; width <= 180; width++ {
		previous := 0
		for height := minimumHeight; height <= 60; height++ {
			layout := m.responsiveWelcome(width-6, height-5)
			assert.GreaterOrEqual(t, layout.logo.width, previous, "a taller window must never select a smaller mark")
			previous = layout.logo.width
		}
	}
}
