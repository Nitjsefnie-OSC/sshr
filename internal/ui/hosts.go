package ui

import (
	"fmt"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	sshrapp "sshr.dev/internal/app"
)

// layoutHostsList renders hosts in the left Servers sidebar.
func layoutHostsList(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	if len(s.hosts) == 0 {
		empty := material.Caption(th, s.t("list.empty"))
		empty.Color = colMuted
		return empty.Layout(gtx)
	}
	return material.List(th, &s.list).Layout(gtx, len(s.hosts), func(gtx layout.Context, i int) layout.Dimensions {
		h := s.hosts[i]
		authLabel := ""
		switch {
		case h.Key != "":
			authLabel = s.t("auth.key.short") + ": " + filepath.Base(h.Key)
		case h.KeyMode == sshrapp.KeyVault:
			authLabel = s.t("auth.vault.short")
		case h.KeyMode == sshrapp.KeyOnePassword:
			authLabel = s.t("auth.opKey.short")
		case h.KeyMode == sshrapp.KeyAgent:
			authLabel = s.t("auth.agent.short")
		case h.PasswordMode == sshrapp.PasswordOnePassword:
			authLabel = s.t("auth.opPass.short")
		case h.PasswordMode == sshrapp.PasswordSaved:
			authLabel = s.t("auth.password.short")
		case h.PasswordMode == sshrapp.PasswordAsk:
			authLabel = s.t("auth.ask.short")
		}
		return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					t := material.Body1(th, h.Name)
					t.Font.Weight = 600
					t.Color = colFG
					t.MaxLines = 1
					return t.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					line := fmt.Sprintf("%s@%s:%d", h.User, h.Host, h.Port)
					sub := material.Caption(th, line)
					sub.Color = colMuted
					sub.MaxLines = 1
					return sub.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sub := material.Caption(th, authLabel)
					sub.Color = colMutedSoft
					sub.MaxLines = 1
					return sub.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if i >= len(s.hostEditBtns) {
								return layout.Dimensions{}
							}
							btn := material.Button(th, &s.hostEditBtns[i], s.t("btn.edit"))
							btn.TextSize = unit.Sp(11)
							btn.Inset = layout.Inset{
								Top: unit.Dp(5), Bottom: unit.Dp(5),
								Left: unit.Dp(8), Right: unit.Dp(8),
							}
							return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, btn.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if i >= len(s.hostDelBtns) {
								return layout.Dimensions{}
							}
							btn := material.Button(th, &s.hostDelBtns[i], s.t("btn.delete"))
							btn.Background = colErr
							btn.TextSize = unit.Sp(11)
							btn.Inset = layout.Inset{
								Top: unit.Dp(5), Bottom: unit.Dp(5),
								Left: unit.Dp(8), Right: unit.Dp(8),
							}
							return btn.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}

// layoutHostsMain is the right pane when Servers is active (list lives in sidebar).
func layoutHostsMain(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	msg := s.t("sidebar.hosts.main")
	if len(s.hosts) == 0 {
		msg = s.t("list.empty")
	}
	body := material.Body1(th, msg)
	body.Color = colMuted
	body.Alignment = text.Middle
	return layout.Center.Layout(gtx, body.Layout)
}

// layoutForm is the add/edit host fields used inside the form modal.
func layoutForm(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	children := []layout.FlexChild{
		field(th, s.t("field.name"), &s.nameEd),
		spacer(10),
		field(th, s.t("field.host"), &s.hostEd),
		spacer(10),
		field(th, s.t("field.user"), &s.userEd),
		spacer(10),
		field(th, s.t("field.port"), &s.portEd),
		spacer(12),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutAuthPicker(gtx, th, s)
		}),
	}
	if s.authKind == authAgent {
		children = append(children, spacer(10), field(th, s.t("field.agent"), &s.agentEd),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, s.t("field.agent.hint"))
				l.Color = colMuted
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
			}),
			spacer(10), field(th, s.t("field.agentKey"), &s.agentKeyEd),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, s.t("field.agentKey.hint"))
				l.Color = colMuted
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
			}))
	} else if s.authKind == authOP {
		children = append(children, spacer(10), field(th, s.t("field.keyRef"), &s.keyRefEd),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, s.t("field.keyRef.hint"))
				l.Color = colMuted
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
			}))
	} else if s.authKind == authVault {
		hint := s.t("field.keyPaste.hint")
		if s.form.vaultKeyPrint != "" {
			hint = s.t("field.keyPaste.keep", s.form.vaultKeyPrint)
		}
		children = append(children, spacer(10), multilineField(th, s.t("field.keyPaste"), &s.keyPasteEd),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, hint)
				l.Color = colMuted
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
			}),
			spacer(10), passwordField(th, s.t("field.keyPassphrase"), &s.keyPassEd))
	} else if s.authKind == authKey {
		children = append(children, spacer(10), field(th, s.t("field.key"), &s.keyEd))
	} else {
		children = append(children, spacer(10), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPassModePicker(gtx, th, s)
		}))
		if s.passMode == sshrapp.PasswordOnePassword {
			children = append(children, spacer(10), field(th, s.t("field.passRef"), &s.passRefEd))
		}
		if s.passMode != sshrapp.PasswordAsk {
			hintKey := "field.password.hint"
			if s.passMode == sshrapp.PasswordOnePassword {
				hintKey = "field.passRef.hint"
			} else if s.form.hasSavedPassword {
				hintKey = "field.password.keep"
			}
			children = append(children, spacer(10), passwordField(th, s.t("field.password"), &s.passEd),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					hint := material.Caption(th, s.t(hintKey))
					hint.Color = colMuted
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, hint.Layout)
				}))
		}
	}
	children = append(children, spacer(12), field(th, s.t("field.jump"), &s.jumpEd),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hint := material.Caption(th, s.t("field.jump.hint"))
			hint.Color = colMuted
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, hint.Layout)
		}),
		spacer(12),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutSessionPicker(gtx, th, s)
		}),
		spacer(12),
		multilineField(th, s.t("field.forwards"), &s.fwdEd),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hint := material.Caption(th, s.t("field.forwards.hint"))
			hint.Color = colMuted
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, hint.Layout)
		}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutSessionPicker chooses what connect opens on the server: a plain
// shell, or a tmux/zellij session that survives a dropped connection
func layoutSessionPicker(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	choice := func(btn *widget.Clickable, label, kind string) layout.FlexChild {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return authChoiceBtn(gtx, th, btn, label, s.session == kind)
		})
	}
	gap := layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(th, s.t("field.session"))
			l.Color = colMuted
			return l.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				choice(&s.sessionOffBtn, s.t("session.off"), ""),
				gap,
				choice(&s.sessionTmuxBtn, "tmux", sshrapp.SessionTmux),
				gap,
				choice(&s.sessionZellijBtn, "zellij", sshrapp.SessionZellij),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hint := material.Caption(th, s.t("field.session.hint"))
			hint.Color = colMuted
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, hint.Layout)
		}),
	)
}

// layoutPassModePicker chooses between a password saved encrypted in sshr
// and one typed on every connect
func layoutPassModePicker(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return authChoiceBtn(gtx, th, &s.passSavedBtn, s.t("auth.pass.saved"), s.passMode != sshrapp.PasswordAsk && s.passMode != sshrapp.PasswordOnePassword)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return authChoiceBtn(gtx, th, &s.passOPBtn, "1Password", s.passMode == sshrapp.PasswordOnePassword)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return authChoiceBtn(gtx, th, &s.passAskBtn, s.t("auth.pass.ask"), s.passMode == sshrapp.PasswordAsk)
		}),
	)
}

func layoutAuthPicker(gtx layout.Context, th *material.Theme, s *state) layout.Dimensions {
	choice := func(btn *widget.Clickable, label string, kind authKind) layout.FlexChild {
		return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return authChoiceBtn(gtx, th, btn, label, s.authKind == kind)
		})
	}
	gap := layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(th, s.t("field.auth"))
			l.Color = colMuted
			return l.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		// two rows, five buttons in one row are too narrow for their labels
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				choice(&s.authPassBtn, s.t("auth.password"), authPassword), gap,
				choice(&s.authKeyBtn, s.t("auth.key"), authKey), gap,
				choice(&s.authVaultBtn, s.t("auth.vault"), authVault),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				choice(&s.authOPBtn, s.t("auth.opKey"), authOP), gap,
				choice(&s.authAgentBtn, s.t("auth.agent"), authAgent), gap,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
			)
		}),
	)
}

func authChoiceBtn(gtx layout.Context, th *material.Theme, btn *widget.Clickable, label string, active bool) layout.Dimensions {
	b := material.Button(th, btn, label)
	b.TextSize = unit.Sp(13)
	b.Inset = layout.Inset{
		Top: unit.Dp(8), Bottom: unit.Dp(8),
		Left: unit.Dp(10), Right: unit.Dp(10),
	}
	if active {
		b.Background = colAccent
	} else {
		b.Background = colMuted
	}
	return b.Layout(gtx)
}

func field(th *material.Theme, label string, ed *widget.Editor) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, label)
				l.Color = colMuted
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				ed.Submit = false
				ed.SingleLine = true
				return material.Editor(th, ed, label).Layout(gtx)
			}),
		)
	})
}

// multilineField is field for text with one entry per line
func multilineField(th *material.Theme, label string, ed *widget.Editor) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, label)
				l.Color = colMuted
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				ed.Submit = false
				ed.SingleLine = false
				return material.Editor(th, ed, label).Layout(gtx)
			}),
		)
	})
}

func passwordField(th *material.Theme, label string, ed *widget.Editor) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, label)
				l.Color = colMuted
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				ed.Submit = false
				ed.SingleLine = true
				ed.Mask = '•'
				return material.Editor(th, ed, label).Layout(gtx)
			}),
		)
	})
}

func spacer(h unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Height: h}.Layout)
}
