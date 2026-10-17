package ui

import (
	"errors"
	"strings"

	"gioui.org/layout"

	sshrapp "sshr.dev/internal/app"
)

type importResult struct {
	items []sshrapp.ImportItem
	err   error
}

// startImport reads ~/.ssh/config in the background, ssh -G runs once per
// host there
func (s *state) startImport() {
	if s.importing {
		return
	}
	s.importing = true
	s.err = ""
	s.status = s.t("status.importing")
	if s.imports == nil {
		s.imports = make(chan importResult, 1)
	}
	go func() {
		items, err := s.store.ImportSSHConfig(false)
		s.imports <- importResult{items, err}
		s.win.Invalidate()
	}()
}

// finishImport shows how many hosts came in and what was skipped
func (s *state) finishImport(gtx layout.Context) {
	select {
	case res := <-s.imports:
		s.importing = false
		if res.err != nil {
			s.err = res.err.Error()
			return
		}
		added, skipped := 0, 0
		var notes []string
		for _, it := range res.items {
			if it.Host != nil {
				added++
			} else {
				skipped++
			}
			for _, n := range it.Notes {
				notes = append(notes, it.Alias+": "+n)
			}
		}
		s.status = s.t("status.imported", added, skipped)
		if len(notes) > 0 {
			s.err = strings.Join(notes, ", ")
		}
		s.reload()
	default:
	}
}

// setExport installs or removes the export to ~/.ssh/config, it reads only
// local files and runs in the frame
func (s *state) setExport(on bool) {
	s.err = ""
	if !on {
		if err := s.store.UninstallSSHConfig(); err != nil {
			s.err = err.Error()
			s.exportBox.Value = s.store.SSHConfigExported()
			return
		}
		s.status = s.t("status.exportRemoved")
		return
	}
	items, err := s.store.InstallSSHConfig()
	var skipped []string
	for _, it := range items {
		if it.Skipped != "" {
			skipped = append(skipped, it.Name+": "+it.Skipped)
		}
	}
	s.exportBox.Value = s.store.SSHConfigExported()
	if err != nil && !errors.Is(err, sshrapp.ErrSSHConfigLink) {
		s.err = err.Error()
		return
	}
	s.status = s.t("status.exported", len(items)-len(skipped), len(skipped))
	if err != nil {
		// the file is written, ~/.ssh/config is a link and needs the line by hand
		skipped = append([]string{strings.TrimPrefix(err.Error(), sshrapp.ErrSSHConfigLink.Error()+": ")}, skipped...)
	}
	s.err = strings.Join(skipped, ", ")
}
