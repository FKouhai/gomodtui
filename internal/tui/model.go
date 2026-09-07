package tui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gomodtui/svc/pkggodev"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// pane represents focused pane
type pane int

const (
	paneSearch pane = iota
	paneList
	paneDetail
)

// Messages
type searchResultMsg struct {
	query  string
	result *pkggodev.PaginatedSearch
	err    error
}

type packageResultMsg struct {
	path string
	pkg  *pkggodev.Package
	err  error
}

type symbolsResultMsg struct {
	path    string
	symbols *pkggodev.PackageSymbols
	err     error
}

type searchTickMsg struct {
	query string
}

type symbolFilterTickMsg struct {
	filter string
}

// list item
type pkgItem struct {
	res pkggodev.SearchResult
}

func (i pkgItem) Title() string { return i.res.PackagePath }
func (i pkgItem) Description() string {
	if i.res.Synopsis != "" {
		return fmt.Sprintf("%s — %s", i.res.ModulePath, i.res.Synopsis)
	}
	return i.res.ModulePath
}
func (i pkgItem) FilterValue() string { return i.res.PackagePath + " " + i.res.ModulePath }

type Model struct {
	width, height int

	focus   pane
	history []pane

	input       textinput.Model
	symbolInput textinput.Model
	list        list.Model
	viewport    viewport.Model

	results  []pkggodev.SearchResult
	selected *pkggodev.Package
	cache    map[string]*pkggodev.Package
	rendered map[string]string // cached markdown render

	// symbols within selected package
	symbols            []pkggodev.Symbol
	filteredSymbols    []pkggodev.Symbol
	symbolCache        map[string][]pkggodev.Symbol // path -> symbols
	symbolFilterActive bool

	glamour *glamour.TermRenderer

	loading          bool
	symbolsLoading   bool
	errMsg           string
	query            string
	debounceID       int
	symbolDebounceID int
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "type a package to search"
	ti.Prompt = " > "
	ti.Focus()

	si := textinput.New()
	si.Placeholder = "filter symbols — s to focus, esc to clear"
	si.Prompt = " s> "

	delegate := list.NewDefaultDelegate()
	// style delegate slightly
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	vp := viewport.New()

	// glamour renderer with dark style (WithAutoStyle queries terminal via OSC and leaks chars into input)
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(80),
	)

	return Model{
		focus:       paneSearch,
		input:       ti,
		symbolInput: si,
		list:        l,
		viewport:    vp,
		cache:       make(map[string]*pkggodev.Package),
		rendered:    make(map[string]string),
		symbolCache: make(map[string][]pkggodev.Symbol),
		glamour:     renderer,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// layout: independent full-screen panes for readability (no split)
		// keeps docs easy to read even in tmux vertical splits
		searchH := 3
		// account for header + search + optional symbol filter + footer
		extra := 2 // footer/header
		if m.selected != nil && m.focus == paneDetail {
			extra += 3 // symbol filter bar
		}
		contentH := m.height - searchH - extra
		if contentH < 5 {
			contentH = 5
		}
		// detail and list each get full width independent pane
		m.list.SetSize(m.width-4, contentH)
		m.viewport.SetWidth(m.width - 4)
		m.viewport.SetHeight(contentH)
		if m.glamour != nil {
			if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(m.width-4)); err == nil {
				m.glamour = r
			}
		}
		return m, nil

	case symbolFilterTickMsg:
		// debounced symbol filter — stale check
		if strings.TrimSpace(m.symbolInput.Value()) != msg.filter {
			return m, nil
		}
		if m.selected == nil {
			return m, nil
		}
		if msg.filter == "" {
			m.filteredSymbols = m.symbols
		} else {
			m.filteredSymbols = filterSymbols(m.symbols, msg.filter)
		}
		rendered := m.renderPackageWithSymbols(m.selected, m.filteredSymbols, msg.filter)
		m.viewport.SetContent(rendered)
		m.rendered[m.selected.Path] = rendered
		if msg.filter != "" && len(m.filteredSymbols) > 0 {
			m.jumpToSymbol(m.filteredSymbols[0].Name)
		} else {
			m.viewport.GotoTop()
		}
		return m, nil

	case searchTickMsg:
		// only trigger if query still matches and not empty
		if msg.query != m.input.Value() {
			return m, nil
		}
		if strings.TrimSpace(msg.query) == "" {
			m.results = nil
			m.list.SetItems(nil)
			m.loading = false
			return m, nil
		}
		m.loading = true
		m.errMsg = ""
		return m, m.searchCmd(msg.query)

	case searchResultMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if msg.query != m.input.Value() {
			// stale result
			return m, nil
		}
		m.results = msg.result.Items
		items := make([]list.Item, len(m.results))
		for i, r := range m.results {
			items[i] = pkgItem{res: r}
		}
		m.list.SetItems(items)
		if len(items) > 0 {
			m.push(paneList)
			m.list.Select(0)
		}
		m.errMsg = ""
		return m, nil

	case packageResultMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.cache[msg.path] = msg.pkg
		m.selected = msg.pkg
		// reset symbols filter
		m.symbolInput.SetValue("")
		m.symbolFilterActive = false
		m.symbols = nil
		m.filteredSymbols = nil
		rendered := m.renderPackage(msg.pkg)
		m.rendered[msg.path] = rendered
		m.viewport.SetContent(rendered)
		m.viewport.GotoTop()
		m.push(paneDetail)
		// fetch symbols for this package (cached check inside cmd)
		if _, ok := m.symbolCache[msg.path]; ok {
			// use cached symbols
			m.symbols = m.symbolCache[msg.path]
			m.filteredSymbols = m.symbols
			// re-render with symbols
			rendered = m.renderPackageWithSymbols(msg.pkg, m.filteredSymbols, "")
			m.rendered[msg.path] = rendered
			m.viewport.SetContent(rendered)
			return m, nil
		}
		m.symbolsLoading = true
		return m, m.fetchSymbolsCmd(msg.path)

	case symbolsResultMsg:
		m.symbolsLoading = false
		if msg.err != nil {
			// non-fatal: keep package view, show error in status
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.symbolCache[msg.path] = msg.symbols.Symbols.Items
		// only apply if still viewing same package
		if m.selected != nil && m.selected.Path == msg.path {
			m.symbols = msg.symbols.Symbols.Items
			m.filteredSymbols = m.symbols
			filter := strings.TrimSpace(m.symbolInput.Value())
			if filter != "" {
				m.filteredSymbols = filterSymbols(m.symbols, filter)
			}
			rendered2 := m.renderPackageWithSymbols(m.selected, m.filteredSymbols, filter)
			m.rendered[msg.path] = rendered2
			m.viewport.SetContent(rendered2)
			if filter != "" && len(m.filteredSymbols) > 0 {
				m.jumpToSymbol(m.filteredSymbols[0].Name)
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		// Symbol filter input has priority: when active, letters like hjklbq must be typed, not handled as navigation
		if m.symbolFilterActive {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.symbolFilterActive = false
				m.symbolInput.Blur()
				m.symbolInput.SetValue("")
				if m.selected != nil {
					m.filteredSymbols = m.symbols
					rendered := m.renderPackageWithSymbols(m.selected, m.filteredSymbols, "")
					m.viewport.SetContent(rendered)
					m.viewport.GotoTop()
					m.rendered[m.selected.Path] = rendered
				}
				return m, nil
			case "enter":
				m.symbolFilterActive = false
				m.symbolInput.Blur()
				return m, nil
			}
			// all other keys (hjkl,b,q,s,/, etc.) go to symbol input — do not intercept
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.focus == paneDetail {
				m.pop()
				return m, nil
			}
			if m.focus == paneList {
				m.pop()
				m.input.Focus()
				return m, nil
			}
			// in search, esc clears
			m.input.SetValue("")
			return m, nil
		case "enter":
			if m.focus == paneSearch {
				// trigger immediate search
				q := strings.TrimSpace(m.input.Value())
				if q != "" {
					m.loading = true
					return m, m.searchCmd(q)
				}
				return m, nil
			}
			if m.focus == paneList {
				if sel, ok := m.list.SelectedItem().(pkgItem); ok {
					path := sel.res.PackagePath
					if cached, ok := m.cache[path]; ok {
						m.selected = cached
						// try symbols cache
						if syms, ok := m.symbolCache[path]; ok {
							m.symbols = syms
							m.filteredSymbols = syms
							filter := strings.TrimSpace(m.symbolInput.Value())
							if filter != "" {
								m.filteredSymbols = filterSymbols(m.symbols, filter)
							}
							r := m.renderPackageWithSymbols(cached, m.filteredSymbols, filter)
							m.viewport.SetContent(r)
							m.rendered[path] = r
						} else if r, ok := m.rendered[path]; ok {
							m.viewport.SetContent(r)
						} else {
							m.viewport.SetContent(m.renderPackage(cached))
						}
						m.viewport.GotoTop()
						m.push(paneDetail)
						// fetch symbols if not cached
						if _, ok := m.symbolCache[path]; !ok {
							m.symbolsLoading = true
							return m, m.fetchSymbolsCmd(path)
						}
						return m, nil
					}
					m.loading = true
					m.errMsg = ""
					return m, m.fetchPackageCmd(path)
				}
			}
		case "b":
			if m.focus == paneDetail {
				m.pop()
				return m, nil
			}
		case "/":
			if m.focus != paneSearch {
				m.push(paneSearch)
				m.input.Focus()
				return m, nil
			}
		case "s", "S":
			if m.focus == paneDetail && !m.symbolFilterActive {
				m.symbolFilterActive = true
				m.symbolInput.Focus()
				return m, nil
			}
		case "h":
			if m.focus == paneDetail {
				// h as back to list
				m.pop()
				return m, nil
			}
			if m.focus == paneList {
				m.pop()
				m.input.Focus()
				return m, nil
			}
		case "l":
			if m.focus == paneSearch && len(m.results) > 0 {
				m.push(paneList)
				return m, nil
			}
			if m.focus == paneList && m.selected != nil {
				m.push(paneDetail)
				return m, nil
			}
		}
	}

	// Route input to focused component
	// symbol filter input has priority when active in detail — debounced to avoid glamour lag
	if m.focus == paneDetail && m.symbolFilterActive {
		var cmd tea.Cmd
		prev := m.symbolInput.Value()
		m.symbolInput, cmd = m.symbolInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.symbolInput.Value() != prev {
			filter := strings.TrimSpace(m.symbolInput.Value())
			m.symbolDebounceID++
			// debounce heavy glamour render 120ms instead of per-keystroke
			cmds = append(cmds, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
				return symbolFilterTickMsg{filter: filter}
			}))
			// optimistic: keep status help updated without full render
			if m.selected != nil {
				if filter == "" {
					m.filteredSymbols = m.symbols
				} else {
					// fast filter for count only, defer full render to tick
					m.filteredSymbols = filterSymbols(m.symbols, filter)
				}
			}
		}
		return m, tea.Batch(cmds...)
	}

	switch m.focus {
	case paneSearch:
		var cmd tea.Cmd
		prev := m.input.Value()
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		if m.input.Value() != prev {
			// debounce
			m.debounceID++
			id := m.debounceID
			q := m.input.Value()
			// capture id via closure check in tick
			_ = id
			cmds = append(cmds, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
				return searchTickMsg{query: q}
			}))
		}
	case paneList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case paneDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
		// also allow list navigation when detail focused with mouse? no
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) push(p pane) {
	if m.focus == p {
		return
	}
	m.history = append(m.history, m.focus)
	m.focus = p
	if p == paneSearch {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *Model) pop() {
	if len(m.history) == 0 {
		// stay at search instead of quitting
		m.focus = paneSearch
		m.input.Focus()
		return
	}
	prev := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.focus = prev
	if prev == paneSearch {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("loading...")
		v.AltScreen = true
		return v
	}

	// styles
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	focusedBorder := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205")).Padding(0, 1)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// search box
	searchContent := m.input.View()
	searchBox := borderStyle.Render(searchContent)
	if m.focus == paneSearch {
		searchBox = focusedBorder.Render(searchContent)
	}

	// symbol filter bar (only when viewing a package)
	symbolBox := ""
	if m.selected != nil {
		symContent := m.symbolInput.View()
		boxStyle := borderStyle
		if m.symbolFilterActive {
			boxStyle = focusedBorder
		}
		// show count in placeholder when not active?
		symbolBox = boxStyle.Render(symContent)
		if m.symbolsLoading && !m.symbolFilterActive {
			symbolBox = boxStyle.Render("loading symbols...")
		}
	}

	// loading / error bar
	status := ""
	if m.loading {
		status = "  loading..."
	} else if m.errMsg != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  " + m.errMsg)
	} else if len(m.results) > 0 {
		status = fmt.Sprintf("  %d results", len(m.results))
	}

	// main content — independent full-screen panes (no side-by-side split)
	// This keeps docs readable in tmux vertical splits; use esc/b to go back
	var mainContent string
	switch m.focus {
	case paneSearch:
		if len(m.results) == 0 {
			placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("type to search • enter to search • q to quit")
			mainContent = placeholder
		} else {
			// search pane shows input already; main area placeholder remains
			mainContent = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("↑↓ to browse results • enter to view docs")
		}
	case paneList:
		box := focusedBorder.Render(m.list.View())
		mainContent = box
	case paneDetail:
		// detail gets full width independent pane for readable docs
		box := focusedBorder.Render(m.viewport.View())
		mainContent = box
	}

	header := titleStyle.Render(" gomodtui — pkg.go.dev browser ") + status
	helpText := " hjkl/↑↓ navigate • enter select • esc/b back • / search"
	if m.selected != nil {
		helpText += " • s filter symbols"
		if len(m.symbols) > 0 {
			helpText += fmt.Sprintf(" (%d)", len(m.symbols))
		}
		if m.symbolFilterActive {
			helpText += " • esc clear filter"
		}
	}
	helpText += " • q quit • mouse wheel scroll "
	help := helpStyle.Render(helpText)

	// include symbol filter bar when viewing package
	if symbolBox != "" && m.focus == paneDetail {
		content := lipgloss.JoinVertical(lipgloss.Left, header, searchBox, symbolBox, mainContent, help)
		v := tea.NewView(content)
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, searchBox, mainContent, help)
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := pkggodev.SearchWithContext(ctx, http.DefaultClient, query)
		return searchResultMsg{query: query, result: res, err: err}
	}
}

func (m Model) fetchPackageCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		opts := &pkggodev.PackageOptions{Doc: "md", Imports: true}
		pkg, err := pkggodev.GetPackageWithContext(ctx, http.DefaultClient, path, opts)
		return packageResultMsg{path: path, pkg: pkg, err: err}
	}
}

func (m Model) fetchSymbolsCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		opts := &pkggodev.SymbolsOptions{Limit: 500}
		syms, err := pkggodev.GetSymbolsWithContext(ctx, http.DefaultClient, path, opts)
		return symbolsResultMsg{path: path, symbols: syms, err: err}
	}
}

func filterSymbols(syms []pkggodev.Symbol, filter string) []pkggodev.Symbol {
	if filter == "" {
		return syms
	}
	lower := strings.ToLower(filter)
	var out []pkggodev.Symbol
	for _, s := range syms {
		if strings.Contains(strings.ToLower(s.Name), lower) ||
			strings.Contains(strings.ToLower(s.Synopsis), lower) ||
			strings.Contains(strings.ToLower(s.Kind), lower) ||
			strings.Contains(strings.ToLower(s.Parent), lower) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) jumpToSymbol(name string) {
	// search rendered viewport content for symbol name and scroll there
	// content is the glamour-rendered markdown with ANSI codes, still contains plain name
	content := m.viewport.View()
	// viewport.View() returns styled view, but we want the raw content lines
	// Instead use the stored rendered string from cache if available
	if m.selected != nil {
		if r, ok := m.rendered[m.selected.Path]; ok {
			content = r
		}
	}
	lines := strings.Split(content, "\n")
	lowerName := strings.ToLower(name)
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerName) {
			// center a bit
			offset := i - 2
			if offset < 0 {
				offset = 0
			}
			m.viewport.SetYOffset(offset)
			break
		}
	}
}

func (m Model) renderPackage(pkg *pkggodev.Package) string {
	return m.renderPackageWithSymbols(pkg, nil, "")
}

func (m Model) renderPackageWithSymbols(pkg *pkggodev.Package, syms []pkggodev.Symbol, filter string) string {
	if pkg == nil {
		return "no package"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", pkg.Path))
	sb.WriteString(fmt.Sprintf("**Module:** %s  \n**Version:** %s  latest=%v\n\n", pkg.ModulePath, pkg.Version, pkg.IsLatest))
	if pkg.Synopsis != "" {
		sb.WriteString(pkg.Synopsis + "\n\n")
	}
	if pkg.Docs != "" {
		sb.WriteString(pkg.Docs + "\n\n")
	} else {
		sb.WriteString("_no docs_\n\n")
	}
	if len(pkg.Imports) > 0 {
		sb.WriteString("### Imports\n\n")
		for _, imp := range pkg.Imports {
			sb.WriteString("- " + imp + "\n")
		}
		sb.WriteString("\n")
	}
	// Symbols section
	if m.symbolsLoading && len(syms) == 0 {
		sb.WriteString("### Symbols\n\n_loading symbols..._\n\n")
	} else if len(syms) > 0 || len(m.symbols) > 0 {
		display := syms
		if display == nil {
			display = m.filteredSymbols
			if display == nil {
				display = m.symbols
			}
		}
		if filter != "" {
			sb.WriteString(fmt.Sprintf("### Symbols (%d/%d) — filter: `%s`\n\n", len(display), len(m.symbols), filter))
		} else {
			sb.WriteString(fmt.Sprintf("### Symbols (%d)\n\n", len(m.symbols)))
		}
		if len(display) == 0 {
			sb.WriteString("_no symbols match filter_\n\n")
		} else {
			// group by kind
			groups := map[string][]pkggodev.Symbol{}
			order := []string{"Constant", "Variable", "Function", "Type", "Field", "Method"}
			for _, s := range display {
				groups[s.Kind] = append(groups[s.Kind], s)
			}
			// render in order, then any remaining kinds
			seen := map[string]bool{}
			for _, kind := range order {
				symsKind, ok := groups[kind]
				if !ok || len(symsKind) == 0 {
					continue
				}
				seen[kind] = true
				sb.WriteString(fmt.Sprintf("#### %ss\n\n", kind))
				for _, s := range symsKind {
					sb.WriteString(fmt.Sprintf("- `%s` — %s\n", s.Name, s.Synopsis))
				}
				sb.WriteString("\n")
			}
			for kind, list := range groups {
				if seen[kind] {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %ss\n\n", kind))
				for _, s := range list {
					sb.WriteString(fmt.Sprintf("- `%s` — %s\n", s.Name, s.Synopsis))
				}
				sb.WriteString("\n")
			}
		}
	} else if m.selected != nil && m.selected.Path == pkg.Path && !m.symbolsLoading {
		sb.WriteString("### Symbols\n\n_no symbols_\n\n")
	}
	md := sb.String()
	if m.glamour == nil {
		return md
	}
	out, err := m.glamour.Render(md)
	if err != nil {
		return md
	}
	return out
}
