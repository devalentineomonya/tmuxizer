// Package ui is the Bubble Tea interface: project picker, launch options
// and the resume prompt.
package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/devalentineomonya/tmuxizer/internal/git"
	"github.com/devalentineomonya/tmuxizer/internal/launcher"
	"github.com/devalentineomonya/tmuxizer/internal/project"
	"github.com/devalentineomonya/tmuxizer/internal/report"
	"github.com/devalentineomonya/tmuxizer/internal/stack"
	"github.com/devalentineomonya/tmuxizer/internal/store"
	"github.com/devalentineomonya/tmuxizer/internal/theme"
	"github.com/devalentineomonya/tmuxizer/internal/tools"
)

// Action is what the caller should do after the UI exits.
type Action int

const (
	// Quit leaves for the plain shell.
	Quit Action = iota
	// Launch opens a project's tmux session.
	Launch
	// Shell opens the user's shell inside a project directory.
	Shell
)

// Result is the UI's outcome.
type Result struct {
	Action Action
	Plan   launcher.Plan // set for Launch
	Dir    string        // set for Shell
}

// Deps is everything the UI needs from the outside world.
type Deps struct {
	Theme         theme.Theme
	Projects      []project.Project
	Rescan        func() []project.Project // run in the background; may be nil
	State         store.State
	Live          map[string]string // project path -> its running tmux session name
	Current       string            // project containing the working directory, "" if none
	Editors       []tools.Tool
	Agents        []tools.Tool
	Tracking      bool   // iwashere installed
	TitleTemplate string // configured iwashere session title template
	Orphans       []launcher.Orphan
	Dismiss       func(launcher.Orphan) error
	Close         func(project.Project) error // ends a running session
	Now           func() time.Time
}

type screen int

const (
	picker screen = iota
	options
	resume
	stats
)

// Model is the root Bubble Tea model.
type Model struct {
	deps   Deps
	th     theme.Theme
	screen screen
	width  int
	height int
	result Result
	done   bool // a screen has produced its result

	// picker
	all       []entry
	view      []entry // all filtered
	cursor    int
	offset    int
	filter    textinput.Model
	filtered  bool // filter input focused
	stacks    map[string]stack.Stack
	gits      map[string]git.Info
	gitFlight map[string]bool
	works     map[string]workInfo
	manifests map[string]manifestInfo
	scanning  bool
	confirm   string // project path awaiting a close confirmation

	// launch options
	launch launchForm

	// time screen
	statsPeriod report.Period
	stats       report.Report
	statsReady  bool

	// resume prompt
	orphans []launcher.Orphan
	orphan  int
	notice  string
}

type entry struct {
	project project.Project
	opened  time.Time
	session string // running tmux session name, "" when not running
	current bool
	match   []int // rune positions of the filter match in the name
}

func (e entry) live() bool { return e.session != "" }

type (
	scanMsg []project.Project
	gitMsg  struct {
		path string
		info git.Info
	}
	dismissMsg struct{ err error }
	closeMsg   struct {
		path string
		err  error
	}
)

// New builds the model.
func New(d Deps) *Model {
	if d.Now == nil {
		d.Now = time.Now
	}
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = d.Theme.Accent
	fi.Placeholder = "filter"

	m := &Model{
		deps:      d,
		th:        d.Theme,
		filter:    fi,
		stacks:    map[string]stack.Stack{},
		gits:      map[string]git.Info{},
		gitFlight: map[string]bool{},
		works:     map[string]workInfo{},
		manifests: map[string]manifestInfo{},
		orphans:   d.Orphans,
		scanning:  d.Rescan != nil,
	}
	m.setProjects(d.Projects)
	if len(m.orphans) > 0 {
		m.screen = resume
	}
	return m
}

// Run shows the UI and returns what the user chose.
func Run(d Deps) (Result, error) {
	final, err := tea.NewProgram(New(d), tea.WithAltScreen()).Run()
	if err != nil {
		return Result{}, err
	}
	return final.(*Model).result, nil
}

// Init starts the background rescan and the first git lookup.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.gitCmd()}
	if m.deps.Rescan != nil {
		rescan := m.deps.Rescan
		cmds = append(cmds, func() tea.Msg { return scanMsg(rescan()) })
	}
	return tea.Batch(cmds...)
}

// Update routes messages to the active screen.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.keepCursorVisible()
		return m, nil
	case scanMsg:
		m.scanning = false
		m.setProjects(msg)
		return m, m.gitCmd()
	case gitMsg:
		m.gits[msg.path] = msg.info
		return m, nil
	case statsMsg:
		if rep := report.Report(msg); rep.Period == m.statsPeriod {
			m.stats, m.statsReady = rep, true
		}
		return m, nil
	case closeMsg:
		if msg.err != nil {
			m.notice = "could not close session: " + msg.err.Error()
			return m, nil
		}
		delete(m.deps.Live, msg.path)
		m.setProjects(m.projects())
		return m, nil
	case dismissMsg:
		if msg.err != nil {
			m.notice = "could not end session: " + msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if keys := m.splitKeys(msg); keys != nil {
			return m.replay(keys)
		}
		switch m.screen {
		case resume:
			return m.updateResume(msg)
		case stats:
			return m.updateStats(msg)
		case options:
			return m.updateOptions(msg)
		default:
			return m.updatePicker(msg)
		}
	}
	return m, nil
}

// View renders the active screen.
func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.screen {
	case resume:
		return m.viewResume()
	case stats:
		return m.viewStats()
	case options:
		return m.viewOptions()
	default:
		return m.viewPicker()
	}
}

// splitKeys breaks a burst of typed characters ("jj", delivered as one
// message when input arrives faster than it is read) back into single keys.
// Pastes and the filter input, which want the whole text, are left alone.
func (m *Model) splitKeys(msg tea.KeyMsg) []tea.KeyMsg {
	if msg.Type != tea.KeyRunes || len(msg.Runes) < 2 || msg.Paste || (m.screen == picker && m.filtered) || (m.screen == options && m.launch.editing) {
		return nil
	}
	keys := make([]tea.KeyMsg, len(msg.Runes))
	for i, r := range msg.Runes {
		keys[i] = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
	}
	return keys
}

// replay feeds keys through Update one at a time, stopping once one quits.
func (m *Model) replay(keys []tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, k := range keys {
		_, cmd := m.Update(k)
		cmds = append(cmds, cmd)
		if m.done {
			break
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) finish(r Result) (tea.Model, tea.Cmd) {
	m.result, m.done = r, true
	return m, tea.Quit
}
