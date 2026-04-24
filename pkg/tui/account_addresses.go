package tui


func (m Model) AddressesView(width int) string {
	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	activeStyle := m.theme.TextAccent().Bold(true)
	contentStyle := m.theme.TextBody().Width(width)

	lines := titleStyle.Render("Address") + "\n\n"
	if len(m.SavedAddresses) == 0 {
			lines += contentStyle.Render("No saved addresses")
	} else {
			for i, addr := range m.SavedAddresses {
					isSelected := m.account.addressListFocused && i == m.account.addressCursor
					label := addr.Name + "  " + addr.Street + ", " + addr.City
					if len(label) > width-4 {
							label = label[:width-4]
					}
					if m.account.addressDeleting != nil && *m.account.addressDeleting == i {
							lines += m.theme.TextError().Bold(true).Render("  deletes? (y/n)") + "\n"
					} else if isSelected {
							lines += activeStyle.Render("> "+label) + "\n"
					} else {
							lines += contentStyle.Render(" "+label) + "\n"
					}
			}
	}
	if m.account.addressListFocused {
			lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
	} else {
			lines += "\n" + m.theme.TextDim().Render("enter: manage")
	}
	return lines
}

