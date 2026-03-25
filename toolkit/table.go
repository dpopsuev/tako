package toolkit

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Mode controls the output format.
type Mode int

const (
	ASCII    Mode = iota // Fixed-width terminal tables
	Markdown             // GitHub-flavoured Markdown tables
)

// ColumnAlign specifies the horizontal alignment for a column.
type ColumnAlign int

const (
	AlignDefault ColumnAlign = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// ColumnConfig controls per-column formatting.
type ColumnConfig struct {
	Number   int         // 1-based column index
	Align    ColumnAlign // horizontal alignment
	MaxWidth int         // truncate or wrap content beyond this width (0 = unlimited)
}

// TableBuilder is the project-owned table abstraction.
// Build a table once; render it as ASCII or Markdown via the Mode set at creation.
type TableBuilder interface {
	// Header sets the column headers.
	Header(cols ...string)
	// Row appends a data row. Values are converted to strings via fmt Sprint.
	Row(vals ...any)
	// Footer appends a footer row (e.g. totals).
	Footer(vals ...any)
	// Columns applies per-column configuration (alignment, max width).
	Columns(cfgs ...ColumnConfig)
	// String renders the table in the configured Mode.
	String() string
}

// NewTable returns a TableBuilder that renders in the given Mode.
func NewTable(m Mode) TableBuilder {
	w := table.NewWriter()

	switch m {
	case ASCII:
		w.SetStyle(table.StyleLight)
	case Markdown:
		// go-pretty's RenderMarkdown uses its own formatting; no style needed.
	}

	return &prettyAdapter{writer: w, mode: m}
}

// prettyAdapter wraps go-pretty/v6/table.Writer behind the TableBuilder interface.
type prettyAdapter struct {
	writer table.Writer
	mode   Mode
}

func (a *prettyAdapter) Header(cols ...string) {
	row := make(table.Row, len(cols))
	for i, c := range cols {
		row[i] = c
	}
	a.writer.AppendHeader(row)
}

func (a *prettyAdapter) Row(vals ...any) {
	row := make(table.Row, len(vals))
	copy(row, vals)
	a.writer.AppendRow(row)
}

func (a *prettyAdapter) Footer(vals ...any) {
	row := make(table.Row, len(vals))
	copy(row, vals)
	a.writer.AppendFooter(row)
}

func (a *prettyAdapter) Columns(cfgs ...ColumnConfig) {
	goCfgs := make([]table.ColumnConfig, len(cfgs))
	for i, c := range cfgs {
		goCfgs[i] = table.ColumnConfig{
			Number:   c.Number,
			Align:    toTextAlign(c.Align),
			WidthMax: c.MaxWidth,
		}
	}
	a.writer.SetColumnConfigs(goCfgs)
}

func (a *prettyAdapter) String() string {
	switch a.mode {
	case Markdown:
		return a.writer.RenderMarkdown()
	default:
		return a.writer.Render()
	}
}

func toTextAlign(a ColumnAlign) text.Align {
	switch a {
	case AlignLeft:
		return text.AlignLeft
	case AlignRight:
		return text.AlignRight
	case AlignCenter:
		return text.AlignCenter
	default:
		return text.AlignDefault
	}
}
