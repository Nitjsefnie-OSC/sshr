package ui

import (
	"errors"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	sshrapp "sshr.dev/internal/app"
	"sshr.dev/internal/clip"
)

type uploadResult struct {
	host   string
	remote string
	err    error
}

// startUpload sends the clipboard image to h in the background; the result
// comes back through s.uploads so state is only touched on the UI goroutine.
func (s *state) startUpload(h sshrapp.Host) {
	if s.uploading {
		return
	}
	s.uploading = true
	s.err = ""
	s.status = s.t("status.uploading", h.Name)
	if s.uploads == nil {
		s.uploads = make(chan uploadResult, 1)
	}
	go func() {
		res := uploadResult{host: h.Name}
		img, err := clip.ReadImage()
		if err == nil {
			// No terminal to ask for a password in: key or agent only.
			res.remote, err = s.store.UploadImage(&h, img.Data, img.Ext, true)
		}
		res.err = err
		s.uploads <- res
		s.win.Invalidate()
	}()
}

// finishUploads applies a finished upload: the server path goes to the
// clipboard, ready to paste into the remote session.
func (s *state) finishUploads(gtx layout.Context) {
	select {
	case res := <-s.uploads:
		s.uploading = false
		switch {
		case errors.Is(res.err, clip.ErrNoImage):
			s.err = s.t("err.noImage")
		case res.err != nil:
			s.err = res.err.Error()
		default:
			gtx.Execute(clipboard.WriteCmd{
				Type: "application/text",
				Data: io.NopCloser(strings.NewReader(res.remote)),
			})
			s.status = s.t("status.uploaded", res.host)
		}
	default:
	}
}
