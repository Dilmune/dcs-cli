package workspace

import (
	"context"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	minimumWidth  = 48
	minimumHeight = 20
	queryLimit    = 100
)

type loadedMsg struct {
	generation int
	item       Item
	err        error
}
type identityMsg struct {
	name string
	err  error
}
type welcomeSavedMsg struct{ err error }
type frameState struct {
	item    Item
	cursor  int
	loading bool
	err     error
}
type searchResult struct {
	item Item
	rank int
}

type model struct {
	styles                                    styles
	root, current                             Item
	history                                   []frameState
	cursor, scroll, width, height, generation int
	searchCursor                              int
	query                                     []rune
	searching, help, welcome, loading         bool
	account, version, notice                  string
	err                                       error
	fetch                                     func(Request, int) (tea.Cmd, context.CancelFunc)
	cancelLoad                                context.CancelFunc
	identify                                  tea.Cmd
	remember                                  func() error
}

func newModel(opts Options) model {
	root := cleanItem(opts.Catalog)
	return model{styles: newStyles(opts.Output, opts.Theme, opts.NoColor), root: root, current: root,
		width: 100, height: 30, account: "Checking account...", version: Clean(opts.Version), welcome: opts.ShowWelcome, remember: opts.RememberWelcome}
}

func (m model) Init() tea.Cmd {
	return m.identify
}

func (m model) isHome() bool     { return len(m.history) == 0 && m.current.ID == m.root.ID }
func (m model) canRefresh() bool { return m.current.Request != nil }
func (m model) hasError() bool   { return m.err != nil }
func (m model) isTooSmall() bool { return m.width < minimumWidth || m.height < minimumHeight }

func (m model) results() []searchResult {
	items := m.current.Children
	if m.searching {
		items = flatten(m.root.Children)
		if !m.isHome() {
			items = append(items, flatten(m.current.Children)...)
		}
	}
	var results []searchResult
	seen := make(map[string]bool)
	query := strings.ToLower(string(m.query))
	for _, item := range items {
		if m.searching && item.ID != "" {
			if seen[item.ID] {
				continue
			}
			seen[item.ID] = true
		}
		text := strings.ToLower(item.Title + " " + item.Description + " " + item.Command)
		if !strings.Contains(text, query) {
			continue
		}
		rank := 2
		if strings.Contains(strings.ToLower(item.Command), query) {
			rank = 1
		}
		if strings.Contains(strings.ToLower(item.Title), query) {
			rank = 0
		}
		results = append(results, searchResult{item, rank})
	}
	if m.searching {
		sort.SliceStable(results, func(i, j int) bool { return results[i].rank < results[j].rank })
	}
	return results
}

func flatten(items []Item) []Item {
	var result []Item
	for _, item := range items {
		result = append(result, item)
		result = append(result, flatten(item.Children)...)
	}
	return result
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case identityMsg:
		m.account = msg.name
		if msg.err != nil {
			m.account = "Not verified · dcs login / dcs whoami"
		}
	case loadedMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.loading, m.err, m.cancelLoad = false, msg.err, nil
		if msg.err == nil {
			m.current = msg.item
			m.cursor = min(m.cursor, max(0, len(m.results())-1))
		}
	case welcomeSavedMsg:
		if msg.err != nil {
			m.notice = "Welcome preference could not be saved; it may appear again."
		}
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || (!m.searching && key == "q") {
		m.stopLoad()
		return m, tea.Quit
	}
	if m.help {
		switch key {
		case "esc", "?", "enter":
			m.help = false
			m.scroll = 0
		case "pgdown":
			m.scroll += 5
		case "pgup":
			m.scroll = max(0, m.scroll-5)
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes && msg.Paste && !m.searching {
		m.startSearch()
	}
	if m.searching {
		return m.searchKey(msg)
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
		var commands []tea.Cmd
		for _, r := range msg.Runes {
			next, command := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = next.(model)
			commands = append(commands, command)
		}
		return m, tea.Batch(commands...)
	}
	switch key {
	case "?":
		m.help = true
		m.scroll = 0
	case "/":
		m.startSearch()
	case "esc", "backspace":
		command := m.back()
		return m, command
	case "0":
		m.stopLoad()
		m.current = m.root
		m.history = nil
		m.cursor, m.scroll = 0, 0
		m.query = nil
		m.err = nil
	case "1", "2", "3", "4", "5", "6":
		i := int(key[0] - '1')
		if i < len(m.root.Children) {
			return m.open(m.root.Children[i])
		}
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
		m.scroll = 0
	case "down", "j":
		m.cursor = min(max(0, len(m.results())-1), m.cursor+1)
		m.scroll = 0
	case "pgdown":
		m.scroll += max(1, m.height-14)
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.height-14))
	case "r":
		if m.canRefresh() {
			command := m.load(*m.current.Request)
			return m, command
		}
	case "enter":
		results := m.results()
		if !m.loading && len(results) > 0 {
			return m.open(results[m.cursor].item)
		}
	}
	return m, nil
}

func (m model) searchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.stopSearch()
	case "up":
		m.cursor = max(0, m.cursor-1)
	case "down":
		m.cursor = min(max(0, len(m.results())-1), m.cursor+1)
	case "enter":
		results := m.results()
		if len(results) > 0 {
			item := results[min(m.cursor, len(results)-1)].item
			m.stopSearch()
			return m.open(item)
		}
	case "backspace", "ctrl+h":
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
		}
		m.cursor = 0
	case "ctrl+u":
		m.query = nil
		m.cursor = 0
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			for _, r := range msg.Runes {
				if unicode.IsPrint(r) && len(m.query) < queryLimit {
					m.query = append(m.query, r)
				}
			}
			m.cursor = 0
		}
	}
	return m, nil
}

func (m model) open(item Item) (tea.Model, tea.Cmd) {
	if !item.CanOpen() {
		return m, nil
	}
	previous := frameState{m.current, m.cursor, m.loading, m.err}
	remember := m.acknowledgeWelcome()
	m.stopLoad()
	m.history = append(append([]frameState(nil), m.history...), previous)
	m.current, m.cursor, m.scroll, m.query, m.err = item, 0, 0, nil, nil
	if item.Request != nil {
		command := m.load(*item.Request)
		return m, tea.Batch(command, remember)
	}
	return m, remember
}

func (m *model) acknowledgeWelcome() tea.Cmd {
	if !m.welcome {
		return nil
	}
	m.welcome = false
	remember := m.remember
	return func() tea.Msg { return welcomeSavedMsg{remember()} }
}

func (m *model) load(req Request) tea.Cmd {
	m.stopLoad()
	m.loading, m.err = true, nil
	command, cancel := m.fetch(req, m.generation)
	m.cancelLoad = cancel
	return command
}

func (m *model) stopLoad() {
	if m.cancelLoad != nil {
		m.cancelLoad()
		m.cancelLoad = nil
	}
	m.generation++
	m.loading = false
}

func (m *model) back() tea.Cmd {
	m.stopLoad()
	m.scroll, m.err = 0, nil
	if len(m.history) > 0 {
		previous := m.history[len(m.history)-1]
		m.history = m.history[:len(m.history)-1]
		m.current, m.cursor, m.query, m.err = previous.item, previous.cursor, nil, previous.err
		m.cursor = min(m.cursor, max(0, len(m.results())-1))
		if previous.loading && m.canRefresh() {
			return m.load(*m.current.Request)
		}
	}
	return nil
}

func (m *model) startSearch() {
	m.searchCursor = m.cursor
	m.searching, m.query, m.cursor, m.scroll = true, nil, 0, 0
}

func (m *model) stopSearch() {
	m.searching, m.query, m.scroll = false, nil, 0
	m.cursor = min(m.searchCursor, max(0, len(m.results())-1))
}
