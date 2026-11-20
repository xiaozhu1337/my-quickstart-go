package nativeui

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// Page keeps user actions and worker completions on the same render path.
type Page struct {
	Scope   *core.Scope
	Refresh func()
}

func (p Page) Action(fn func()) func() {
	return func() { fn(); p.Refresh() }
}

func (p Page) Button(label string, fn func(), disabled ...bool) core.Node {
	return ui.Button{Text: label, Variant: ui.ButtonSecondary, Disabled: func() bool { return len(disabled) > 0 && disabled[0] }, OnPress: p.Action(fn)}
}

func (p Page) Primary(label string, fn func(), disabled ...bool) core.Node {
	return ui.Button{Text: label, Variant: ui.ButtonPrimary, Disabled: func() bool { return len(disabled) > 0 && disabled[0] }, OnPress: p.Action(fn)}
}

// Panel gives a related set of controls a shared surface and inset.
func (p Page) Panel(nodes ...core.Node) core.View {
	v := Column(nodes...)
	v.Style.FlexGrow = 0
	v.Style.Padding = core.All(14)
	v.Style.Gap = 10
	v.Style.Background = p.Scope.Palette().Surface
	v.Style.BorderWidth = 1
	v.Style.BorderColor = p.Scope.Palette().Border
	v.Style.CornerRadius = 10
	return v
}

func Heading(value string) core.Node {
	return core.Text{Content: value, Style: core.Style{FontSize: 18, FontWeight: core.FontWeightSemibold}}
}

func (p Page) Muted(value string) core.Node {
	return core.Text{Content: value, Style: core.Style{Color: p.Scope.Palette().TextMuted}}
}

// Spread keeps the context on the left and its actions on the right.
func Spread(left, right core.Node) core.View {
	v := Row(left, right)
	v.Style.JustifyContent = core.JustifySpaceBetween
	return v
}

func (p Page) Editor(title string, fields ...core.Node) core.Node {
	content := Column(append([]core.Node{Heading(title)}, fields...)...)
	content.Style.FlexGrow = 0
	return core.Scroll{Style: core.Style{FlexShrink: 1, MaxHeight: core.Pt(p.Scope.Env().Window().Height - 80)}, Children: []core.Node{content}}
}

func Text(value string) core.Node { return core.Text{Content: value} }

func Row(nodes ...core.Node) core.View {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignCenter}, Children: nodes}
}

func Column(nodes ...core.Node) core.View {
	return core.View{Style: core.Style{Direction: core.Column, Gap: 8, FlexGrow: 1}, Children: nodes}
}

func Form(nodes ...core.Node) core.Node {
	return core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{Column(nodes...)}}
}

func Input(label string, value *string, multi bool) core.Node {
	in := ui.Input{Label: label, DefaultValue: *value, Multiline: multi, OnChange: func(s string) { *value = s }}
	if multi {
		in.Style.Height = core.Pt(140)
	}
	return in
}

func Field(label string, value *string, multi bool) core.Node {
	field := Column(Text(label), Input(label, value, multi))
	field.Style.FlexGrow = 0
	return field
}

func Select(label, value string, options []string, change func(string)) core.Node {
	var items []ui.Option[string]
	for _, opt := range options {
		items = append(items, ui.Option[string]{Value: opt, Label: opt})
	}
	return ui.Select[string]{Label: label, Options: items, Value: func() string { return value }, OnChange: change}
}

// Search updates only the results subtree, preserving the native field and caret.
func Search(cx *core.Scope, label string, value *string, results func(string) core.Node) core.Node {
	query := core.NewSignal(cx, *value)
	return Column(ui.Input{Label: label, Placeholder: label, DefaultValue: *value, OnChange: func(v string) { *value = v; query.Set(v) }}, core.Dynamic(func(*core.Scope) core.Node { return results(query.Get()) }))
}

func List(count, columns int, size core.Points, key func(int) any, row func(int) core.Node) core.List {
	return core.List{Count: func() int { return count }, Key: key, NumColumns: columns, Gap: 6, ColumnGap: 6, EstimatedSize: size, Style: core.Style{FlexGrow: 1},
		Row: func(_ *core.Scope, r core.ListRow) core.Node {
			return core.Dynamic(func(*core.Scope) core.Node { return row(r.Index()) })
		}}
}

func (p Page) Directory(label string, value *string, fail func(error)) core.Node {
	return ui.Button{Text: label, Variant: ui.ButtonSecondary, OnPress: func() {
		p.Scope.PickDirectories(core.PickDirectoriesOptions{Title: label, Limit: 1}, func(d []core.Directory, err error) {
			if err != nil {
				fail(err)
			} else if len(d) > 0 {
				*value = d[0].Path
			}
			p.Refresh()
		})
	}}
}
