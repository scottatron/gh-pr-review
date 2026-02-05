package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gh-pr-review/internal/gh"
	"gh-pr-review/internal/github"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

type tuiModel struct {
	allThreads []reviewThread
	threads    []reviewThread
	index      int
	width      int
	height     int
	ready      bool
	viewport   viewport.Model

	owner  string
	name   string
	pr     int
	status string

	contentCache  map[string]map[int]string
	rendererCache map[int]*glamour.TermRenderer
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { printTUIUsage(fs.Output()) }
	var repo string
	var pr int
	var status string
	var host string
	fs.StringVar(&repo, "repo", "", "owner/name (defaults to gh repo view)")
	fs.IntVar(&pr, "pr", 0, "PR number")
	fs.StringVar(&status, "status", "all", "all|resolved|unresolved|resolved-no-reply")
	fs.StringVar(&host, "host", gh.DefaultHost(), "GitHub host")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "resolved" && status != "unresolved" && status != "resolved-no-reply" {
		return fmt.Errorf("invalid --status %q", status)
	}

	ctx := context.Background()
	if pr <= 0 {
		derived, err := gh.CurrentPrNumber(ctx)
		if err != nil {
			return fmt.Errorf("--pr is required (and could not be derived from current checkout): %w", err)
		}
		pr = derived
	}

	owner, name, err := resolveRepo(ctx, repo)
	if err != nil {
		return err
	}
	token, err := gh.AuthToken(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to get gh auth token: %w", err)
	}
	client := github.NewClient(github.GraphQLEndpoint(host), token)

	threads, err := fetchAllThreads(ctx, client, owner, name, pr)
	if err != nil {
		return err
	}
	filtered := filterThreads(threads, status)

	model := newTUIModel(owner, name, pr, status, filtered)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func newTUIModel(owner, name string, pr int, status string, threads []reviewThread) *tuiModel {
	return &tuiModel{
		allThreads:    threads,
		threads:       threads,
		index:         0,
		owner:         owner,
		name:          name,
		pr:            pr,
		status:        status,
		contentCache:  map[string]map[int]string{},
		rendererCache: map[int]*glamour.TermRenderer{},
	}
}

func (m *tuiModel) Init() tea.Cmd {
	if width, height, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		header := m.headerLines()
		footer := m.footerLines()
		viewportHeight := height - header - footer
		if viewportHeight < 1 {
			viewportHeight = 1
		}
		m.width = width
		m.height = height
		m.viewport = viewport.New(width, viewportHeight)
		m.viewport.SetContent(m.threadContent())
		m.ready = true
	}
	return nil
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.ready && msg.Width == m.width && msg.Height == m.height {
			return m, nil
		}
		m.width = msg.Width
		m.height = msg.Height
		header := m.headerLines()
		footer := m.footerLines()
		viewportHeight := msg.Height - header - footer
		if viewportHeight < 1 {
			viewportHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}
		m.viewport.SetContent(m.threadContent())
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "f":
			m.cycleFilter()
			return m, nil
		case "j":
			m.nextThread()
			return m, nil
		case "k":
			m.prevThread()
			return m, nil
		case "g":
			m.firstThread()
			return m, nil
		case "G":
			m.lastThread()
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tuiModel) View() string {
	if !m.ready {
		return "loading..."
	}
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m *tuiModel) headerLines() int {
	return 2
}

func (m *tuiModel) footerLines() int {
	return 1
}

func (m *tuiModel) headerView() string {
	styler := newStyler(os.Stdout)
	repo := fmt.Sprintf("%s/%s", m.owner, m.name)
	threadLine := "No threads"
	if len(m.threads) > 0 {
		current := m.threads[m.index]
		status := "unresolved"
		if current.IsResolved {
			status = "resolved"
		}
		threadLine = fmt.Sprintf(
			"%s %d/%d  %s%s",
			styler.label("Thread"),
			m.index+1,
			len(m.threads),
			styler.status(status),
			styler.dim(formatLineInfo(current)),
		)
	}
	return strings.Join([]string{
		fmt.Sprintf("%s %s  %s #%d  %s %d (filter: %s)",
			styler.label("Repo:"),
			repo,
			styler.label("PR:"),
			m.pr,
			styler.label("Threads:"),
			len(m.threads),
			m.status,
		),
		threadLine,
	}, "\n")
}

func (m *tuiModel) footerView() string {
	styler := newStyler(os.Stdout)
	return fmt.Sprintf(
		"%s next/prev  %s first/last  %s filter  %s scroll  %s quit",
		styler.label("j/k"),
		styler.label("g/G"),
		styler.label("f"),
		styler.label("up/down"),
		styler.label("q"),
	)
}

func (m *tuiModel) nextThread() {
	if len(m.threads) == 0 {
		return
	}
	if m.index < len(m.threads)-1 {
		m.index++
		m.viewport.SetContent(m.threadContent())
		m.viewport.GotoTop()
	}
}

func (m *tuiModel) prevThread() {
	if len(m.threads) == 0 {
		return
	}
	if m.index > 0 {
		m.index--
		m.viewport.SetContent(m.threadContent())
		m.viewport.GotoTop()
	}
}

func (m *tuiModel) firstThread() {
	if len(m.threads) == 0 {
		return
	}
	if m.index != 0 {
		m.index = 0
		m.viewport.SetContent(m.threadContent())
		m.viewport.GotoTop()
	}
}

func (m *tuiModel) lastThread() {
	if len(m.threads) == 0 {
		return
	}
	last := len(m.threads) - 1
	if m.index != last {
		m.index = last
		m.viewport.SetContent(m.threadContent())
		m.viewport.GotoTop()
	}
}

func (m *tuiModel) cycleFilter() {
	next := "all"
	switch m.status {
	case "all":
		next = "unresolved"
	case "unresolved":
		next = "resolved"
	case "resolved":
		next = "resolved-no-reply"
	case "resolved-no-reply":
		next = "all"
	}
	m.status = next
	m.threads = filterThreads(m.allThreads, m.status)
	if len(m.threads) == 0 {
		m.index = 0
		m.viewport.SetContent(m.threadContent())
		m.viewport.GotoTop()
		return
	}
	if m.index >= len(m.threads) {
		m.index = len(m.threads) - 1
	}
	m.viewport.SetContent(m.threadContent())
	m.viewport.GotoTop()
}

func (m *tuiModel) threadContent() string {
	if len(m.threads) == 0 {
		return "no review threads found"
	}
	thread := m.threads[m.index]
	width := m.viewport.Width
	if width <= 0 {
		width = 120
	}
	if cached := m.cachedContent(thread.ID, width); cached != "" {
		return cached
	}
	metaStyler := newStyler(os.Stdout)
	bodyStyler := newStyler(os.Stdout)
	renderer := m.rendererForWidth(width)

	var b strings.Builder
	for i, c := range thread.Comments.Nodes {
		author := c.Author.Login
		if author == "" {
			author = "unknown"
		}
		b.WriteString(fmt.Sprintf("%s %s — %s\n", metaStyler.bullet(), metaStyler.author(author), metaStyler.dim(c.CreatedAt)))
		if c.URL != "" {
			b.WriteString(fmt.Sprintf("  %s\n", metaStyler.dim(c.URL)))
		}
		b.WriteString("\n")
		for _, line := range formatCommentBodyWithRenderer(c.Body, "  ", width, bodyStyler, renderer) {
			b.WriteString(line)
			b.WriteString("\n")
		}
		if i < len(thread.Comments.Nodes)-1 {
			b.WriteString("\n")
		}
	}
	content := b.String()
	m.storeContent(thread.ID, width, content)
	return content
}

func printTUIUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gh-pr-review tui [--pr <number>] [--repo owner/name] [--status all|resolved|unresolved|resolved-no-reply] [--host host]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --pr <number>   PR number (defaults to current branch PR if available)")
	fmt.Fprintln(w, "  --repo <owner/name>   Repository (defaults to gh repo view)")
	fmt.Fprintln(w, "  --status <value>   all|resolved|unresolved|resolved-no-reply")
	fmt.Fprintln(w, "  --host <host>   GitHub host")
}

func formatCommentBodyWithRenderer(body, indent string, width int, styler styler, renderer *glamour.TermRenderer) []string {
	if styler.enabled && renderer != nil {
		rendered, err := renderer.Render(body)
		if err == nil {
			return indentRendered(rendered, indent)
		}
	}
	return wrapPlainText(body, indent, width)
}

func (m *tuiModel) rendererForWidth(width int) *glamour.TermRenderer {
	if width < 20 {
		width = 20
	}
	if cached := m.rendererCache[width]; cached != nil {
		return cached
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-2),
	)
	if err != nil {
		return nil
	}
	m.rendererCache[width] = renderer
	return renderer
}

func (m *tuiModel) cachedContent(threadID string, width int) string {
	if threadID == "" {
		return ""
	}
	if perThread, ok := m.contentCache[threadID]; ok {
		if content, ok := perThread[width]; ok {
			return content
		}
	}
	return ""
}

func (m *tuiModel) storeContent(threadID string, width int, content string) {
	if threadID == "" {
		return
	}
	perThread := m.contentCache[threadID]
	if perThread == nil {
		perThread = map[int]string{}
		m.contentCache[threadID] = perThread
	}
	perThread[width] = content
}
