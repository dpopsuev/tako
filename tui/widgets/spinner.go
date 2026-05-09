package widgets

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type SpinnerStyle int

const (
	SpinnerGeometric SpinnerStyle = iota
	SpinnerKaomoji
)

type phaseSpinners struct {
	thinking  spinner.Spinner
	executing spinner.Spinner
	reading   spinner.Spinner
	done      spinner.Spinner
	err       spinner.Spinner
}

var geometricSpinners = phaseSpinners{
	thinking:  spinner.Spinner{Frames: []string{"◐", "◓", "◑", "◒"}, FPS: time.Second / 8},
	executing: spinner.Spinner{Frames: []string{"▰▱▱▱", "▰▰▱▱", "▰▰▰▱", "▰▰▰▰", "▱▰▰▰", "▱▱▰▰", "▱▱▱▰", "▱▱▱▱"}, FPS: time.Second / 6},
	reading:   spinner.Spinner{Frames: []string{"◇", "◈", "◆", "◈"}, FPS: time.Second / 4},
	done:      spinner.Spinner{Frames: []string{"◆"}, FPS: time.Second},
	err:       spinner.Spinner{Frames: []string{"◢", "◣", "◤", "◥"}, FPS: time.Second / 10},
}

var kaomojiSpinners = phaseSpinners{
	thinking:  spinner.Spinner{Frames: []string{"(°_°)", "(°.°)", "(°-°)", "(°.°)"}, FPS: time.Second / 3},
	executing: spinner.Spinner{Frames: []string{"(•̀ᴗ•́)و", "(•̀ᴗ•́)و✧", "(•̀ᴗ•́)و✧✧"}, FPS: time.Second / 4},
	reading:   spinner.Spinner{Frames: []string{"(◉_◉)", "(◎_◎)", "(◉_◉)", "(◎_◎)"}, FPS: time.Second / 3},
	done:      spinner.Spinner{Frames: []string{"(◕‿◕)"}, FPS: time.Second},
	err:       spinner.Spinner{Frames: []string{"(×_×)", "(×_×)!", "(×_×)!!"}, FPS: time.Second / 3},
}

func spinnerSet(style SpinnerStyle) phaseSpinners {
	if style == SpinnerKaomoji {
		return kaomojiSpinners
	}
	return geometricSpinners
}

func SpinnerForPhase(phase string, style SpinnerStyle) spinner.Spinner {
	set := spinnerSet(style)
	switch phase {
	case "executing", "implement":
		return set.executing
	case "reading", "assessment", "knowledge":
		return set.reading
	case "done", "sealed":
		return set.done
	case "error":
		return set.err
	default:
		return set.thinking
	}
}

type PhaseSpinner struct {
	model   spinner.Model
	phase   string
	style   SpinnerStyle
	active  bool
}

func NewPhaseSpinner(style SpinnerStyle) PhaseSpinner {
	s := spinner.New()
	s.Spinner = SpinnerForPhase("idle", style)
	return PhaseSpinner{model: s, style: style, phase: "idle"}
}

func (ps PhaseSpinner) Update(msg tea.Msg) (PhaseSpinner, tea.Cmd) {
	switch msg := msg.(type) {
	case PhaseChangeMsg:
		ps.phase = msg.Phase
		ps.active = true
		ps.model.Spinner = SpinnerForPhase(msg.Phase, ps.style)
		return ps, ps.model.Tick
	case AgentDoneMsg:
		ps.active = false
		ps.model.Spinner = SpinnerForPhase("done", ps.style)
		return ps, nil
	case ErrorMsg:
		ps.active = true
		ps.model.Spinner = SpinnerForPhase("error", ps.style)
		return ps, ps.model.Tick
	case spinner.TickMsg:
		if !ps.active {
			return ps, nil
		}
		var cmd tea.Cmd
		ps.model, cmd = ps.model.Update(msg)
		return ps, cmd
	}
	return ps, nil
}

func (ps PhaseSpinner) View() string {
	if !ps.active {
		return ""
	}
	return ps.model.View()
}

func (ps PhaseSpinner) Active() bool {
	return ps.active
}
