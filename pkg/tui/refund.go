package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
)

// Refund composer layout constants.
const (
	refundMinTextareaRows = 5
	refundMaxTextareaRows = 10
	// chrome above the textarea inside the modal (title, label, padding, footer).
	refundModalChrome   = 12
	refundModalMaxWidth = 58
	refundModalMinWidth = 24
	// horizontal chrome the modal subtracts from m.widthContent.
	refundModalHGutter = 6
	// inner padding around the textarea inside the modal body.
	refundTextareaHPad = 4
)

// Nerd-font glyphs used in the refund composer. Requires a Nerd Font in the
// SSH client terminal; renders as tofu otherwise.
const (
	refundIconUndo    = "\uf0e2" // nf-fa-undo
	refundIconTag     = "\uf02b" // nf-oct-tag
	refundIconComment = "\uf41f" // nf-oct-comment
	refundIconKey     = "\uf11c" // nf-fa-keyboard_o
)

// refundFocus identifies which field in the refund composer has keyboard focus.
type refundFocus int

const (
	refundFocusReason refundFocus = iota
	refundFocusMessage
	refundFocusCount
)

type refundState struct {
	open       bool
	orderID    uint
	gen        uint64 // bumped each open so late async results from a prior session are ignored
	reason     string
	form       *huh.Form
	textarea   textarea.Model
	focusIdx   refundFocus
	submitting bool
	err        string
	reasonErr  string
	messageErr string
}

type refundRequestMsg struct {
	OrderID uint
	Gen     uint64
	Err     error
}

// OpenRefundComposer opens the refund request composer for an order.
func (m Model) OpenRefundComposer(orderID uint) (Model, tea.Cmd) {
	state := refundState{
		open:    true,
		orderID: orderID,
		gen:     m.refund.gen + 1,
	}
	state.textarea = m.newRefundTextarea()
	state.form = m.buildRefundForm(&state)
	m.refund = state
	return m, m.refund.form.Init()
}

// RefundUpdate handles all input and async results while the refund composer is open.
func (m Model) RefundUpdate(msg tea.Msg) (Model, tea.Cmd) {
	state := &m.refund
	if !state.open {
		return m, nil
	}

	switch msg := msg.(type) {
	case refundRequestMsg:
		if msg.Gen != state.gen || msg.OrderID != state.orderID {
			return m, nil
		}
		state.submitting = false
		if msg.Err != nil {
			state.err = fmt.Sprintf("failed to send refund request: %v", msg.Err)
			return m, nil
		}
		gen := state.gen
		m.refund = refundState{gen: gen}
		m.notice = &VisibleNotice{message: "Refund request sent"}
		return m, nil

	case tea.WindowSizeMsg:
		state.form = m.buildRefundForm(state)
		m.resizeRefundTextarea(state)
		return m, nil

	case tea.KeyMsg:
		if state.submitting {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			gen := state.gen
			m.refund = refundState{gen: gen}
			return m, nil
		case "ctrl+s":
			return m, m.submitRefundRequest(state)
		case "tab", "shift+tab":
			return m, m.setRefundFocus(state, (state.focusIdx+1)%refundFocusCount)
		}
	}

	if state.focusIdx == refundFocusReason {
		state.err = ""
		state.reasonErr = ""
		form, cmd := state.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			state.form = f
		}
		if state.form.State == huh.StateCompleted {
			state.form = m.buildRefundForm(state)
			return m, m.setRefundFocus(state, refundFocusMessage)
		}
		return m, cmd
	}

	if _, ok := msg.(tea.KeyMsg); ok {
		state.err = ""
		state.messageErr = ""
	}
	ta, cmd := state.textarea.Update(msg)
	state.textarea = ta
	return m, cmd
}

func (m Model) buildRefundForm(state *refundState) *huh.Form {
	options := make([]huh.Option[string], 0, len(models.RefundRequestReasons))
	for _, reason := range models.RefundRequestReasons {
		options = append(options, huh.NewOption(reason, reason))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Reason").
				Key("reason").
				Options(options...).
				Value(&state.reason).
				Height(len(models.RefundRequestReasons) + 1),
		),
	).
		WithShowErrors(false).
		WithShowHelp(false).
		WithTheme(m.theme.Form()).
		WithLayout(huh.LayoutStack).
		WithWidth(m.refundContentWidth())
}

func (m Model) newRefundTextarea() textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = "Describe the issue"
	ta.ShowLineNumbers = true
	ta.CharLimit = 65536
	ta.DynamicHeight = false
	ta.SetHeight(m.refundTextareaHeight())
	ta.SetWidth(m.refundTextareaWidth())

	base := lipgloss.NewStyle()
	// Cursor-line uses a darker grey than theme.Border() so the active line
	// reads as a subtle wash, not a bright bar competing with the highlight.
	cursorLineBG := lipgloss.Color("236")
	ta.SetStyles(textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base,
			Text:             base.Foreground(m.theme.Accent()),
			LineNumber:       base.Foreground(m.theme.Dim()),
			CursorLine:       base.Background(cursorLineBG).Foreground(m.theme.Accent()),
			CursorLineNumber: base.Foreground(m.theme.Body()),
			Placeholder:      base.Foreground(m.theme.Dim()),
			EndOfBuffer:      base.Foreground(m.theme.Dim()),
		},
		Blurred: textarea.StyleState{
			Base:             base,
			Text:             base.Foreground(m.theme.Body()),
			LineNumber:       base.Foreground(m.theme.Dim()),
			CursorLine:       base.Foreground(m.theme.Background()),
			CursorLineNumber: base.Foreground(m.theme.Dim()),
			Placeholder:      base.Foreground(m.theme.Dim()),
			EndOfBuffer:      base.Foreground(m.theme.Dim()),
		},
		Cursor: textarea.CursorStyle{
			Color: m.theme.Highlight(),
			Shape: tea.CursorBlock,
			Blink: true,
		},
	})
	return ta
}

func (m Model) resizeRefundTextarea(state *refundState) {
	state.textarea.SetHeight(m.refundTextareaHeight())
	state.textarea.SetWidth(m.refundTextareaWidth())
}

func (m Model) setRefundFocus(state *refundState, focusIdx refundFocus) tea.Cmd {
	state.focusIdx = focusIdx
	if focusIdx == refundFocusReason {
		state.textarea.Blur()
		state.form = m.buildRefundForm(state)
		return state.form.Init()
	}
	return state.textarea.Focus()
}

func (m Model) submitRefundRequest(state *refundState) tea.Cmd {
	state.err = ""
	state.reasonErr = ""
	state.messageErr = ""

	reason := strings.TrimSpace(state.reason)
	message := strings.TrimSpace(state.textarea.Value())

	if reason == "" {
		state.reasonErr = "select a refund reason"
		return nil
	}

	if reason == models.RefundRequestReasonOther && message == "" {
		state.messageErr = "message is required when reason is other"
		return nil
	}

	state.submitting = true
	return m.createRefundRequestCmd(state.orderID, state.gen, reason, message)
}

func (m Model) createRefundRequestCmd(orderID uint, gen uint64, reason string, message string) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return refundRequestMsg{OrderID: orderID, Gen: gen, Err: fmt.Errorf("not authenticated")}
		}
		err := m.APIClient.CreateRefundRequest(orderID, api.RefundRequest{
			Reason:  reason,
			Message: message,
		})
		return refundRequestMsg{OrderID: orderID, Gen: gen, Err: err}
	}
}

// RenderRefundOverlay renders the refund composer as a centered modal.
// The modal has no border or background fill — only padding for breathing
// room. lipgloss.Place handles centering against the container.
func (m Model) RenderRefundOverlay() string {
	state := &m.refund
	contentWidth := m.refundContentWidth()

	titleText := fmt.Sprintf("%s  Refund request — Order #%d", refundIconUndo, state.orderID)
	title := m.theme.TextAccent().
		Bold(true).
		Width(contentWidth).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.theme.Highlight()).
		Render(titleText)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		m.renderRefundReason(state),
		"",
		m.renderRefundTextarea(state),
		"",
		m.renderRefundFooter(state),
	)

	modal := lipgloss.NewStyle().
		Padding(1, 2).
		Render(body)

	return lipgloss.Place(m.widthContainer, m.heightContainer, lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderRefundReason(state *refundState) string {
	if state.focusIdx == 0 && state.form != nil {
		out := state.form.View()
		if state.reasonErr != "" {
			out += "\n" + m.theme.TextError().Render(state.reasonErr)
		}
		return out
	}

	value := state.reason
	if value == "" {
		value = "Select a reason"
	}

	label := m.theme.TextLabel().Render(refundIconTag + "  Reason")
	selected := m.theme.TextAccent().
		Border(lipgloss.HiddenBorder()).
		PaddingLeft(1).
		Width(m.refundContentWidth()).
		Render(value)

	out := lipgloss.JoinVertical(lipgloss.Left, label, selected)
	if state.reasonErr != "" {
		out += "\n" + m.theme.TextError().Render(state.reasonErr)
	}
	return out
}

func (m Model) renderRefundTextarea(state *refundState) string {
	label := "Message (optional)"
	placeholder := "Describe the issue"
	if state.reason == models.RefundRequestReasonOther {
		label = "Message (required)"
		placeholder = "Describe the issue..."
	}
	state.textarea.Placeholder = placeholder

	border := m.theme.Border()
	if state.focusIdx == refundFocusMessage {
		border = m.theme.Highlight()
	}

	wrapper := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(border)

	contentWidth := m.refundContentWidth()
	state.textarea.SetWidth(contentWidth - wrapper.GetHorizontalFrameSize())
	state.textarea.SetHeight(m.refundTextareaHeight())

	textareaBox := wrapper.Render(state.textarea.View())

	counter := m.theme.TextDim().
		Width(contentWidth).
		Align(lipgloss.Right).
		Render(fmt.Sprintf("%d chars", state.textarea.Length()))

	out := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.TextLabel().Render(refundIconComment+"  "+label),
		textareaBox,
		counter,
	)
	if state.messageErr != "" {
		out += "\n" + m.theme.TextError().Render(state.messageErr)
	}
	return out
}

func (m Model) renderRefundFooter(state *refundState) string {
	if state.submitting {
		return m.theme.TextAccent().Render("sending refund request...")
	}
	if state.err != "" {
		return m.theme.TextError().Render(state.err)
	}
	key := m.theme.TextHighlight().Bold(true)
	sep := m.theme.TextDim().Render("  ·  ")
	dim := m.theme.TextDim()
	return dim.Render(refundIconKey+"  ") +
		key.Render("tab") + dim.Render(" next") + sep +
		key.Render("shift+tab") + dim.Render(" prev") + sep +
		key.Render("ctrl+s") + dim.Render(" submit") + sep +
		key.Render("esc") + dim.Render(" cancel")
}

func (m Model) refundContentWidth() int {
	width := min(m.widthContent-refundModalHGutter, refundModalMaxWidth)
	if width < refundModalMinWidth {
		width = max(1, m.widthContent-refundTextareaHPad)
	}
	return width
}

func (m Model) refundTextareaWidth() int {
	return max(1, m.refundContentWidth()-refundTextareaHPad)
}

func (m Model) refundTextareaHeight() int {
	return max(refundMinTextareaRows, min(refundMaxTextareaRows, m.heightContainer-refundModalChrome))
}
