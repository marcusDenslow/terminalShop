
package tui

func (m Model) SSHKeysView(width int) string {
	titleSTyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	contentStyle := m.theme.TextBody().Width(width)
	activeStyle := m.theme.TextAccent().Bold(true)


	lines := titleSTyle.Render("SSH keys") + "\n\n"
	if !m.SSHKeysLoaded {
		lines += contentStyle.Render("Loading ssh keys...")
	} else if len(m.SSHKeys) == 0 {
		lines += contentStyle.Render("No saved ssh keys")
	} else {
		for i, key := range m.SSHKeys {
			isSelected := m.account.sshKeyListFocused && i == m.account.sshKeyCursor
			label := key.Fingerprint
			if key.Comment != "" {
				label += " " + key.Comment
			} 
			if len(label) > width-4	{
				label += label[:width-4]
			} 
			if m.account.sshKeyDeleting != nil && *m.account.sshKeyDeleting == i {
				lines += m.theme.TextError().Bold(true).Render(" delete? (y/n)") + "\n"
			} else if isSelected {
				lines += activeStyle.Render("> "+label) + "\n"
			} else {
				lines += contentStyle.Render(" "+label) + "\n"
			}
		}
	}
	if m.account.sshKeyListFocused {
		lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
	} else {
		lines += "\n" + m.theme.TextDim().Render("enter: manage")
	}

	return lines
}
