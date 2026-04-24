package tui

import (
	"fmt"
)

func (m Model) CardsView(width int) string {
	titleSTyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	contentStyle := m.theme.TextBody().Width(width)
	activeStyle := m.theme.TextAccent().Bold(true)


	lines := titleSTyle.Render("Cards") + "\n\n"
	if m.SavedCardsIsEmpty() {
		lines += contentStyle.Render("No saved cards")
	} else {
		for i, card := range m.SavedCards {
			isSelected := m.account.cardListFocused && i == m.account.cardCursor
			label := fmt.Sprintf("**** **** **** %s  %s  exp %02d/%02d", card.Last4, card.Brand, card.ExpMonth, card.ExpYear%100)
			if m.account.cardDeleting != nil && *m.account.cardDeleting == i {
				lines += m.theme.TextError().Bold(true).Render(" delete? (y/s)") + "\n"
			} else if isSelected {
				lines += activeStyle.Render("> "+label) + "\n"
			} else {
				lines += contentStyle.Render(" "+label) + "\n"
			}
		}
	}
	if m.account.cardListFocused {
		lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
	} else {
		lines += "\n" + m.theme.TextDim().Render("enter: manage")
	}

	return lines
}
