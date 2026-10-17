//go:build !windows

package desktop

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"sshr.dev/assets"
)

// icnsTypes maps the icns element types to the PNG size they hold, the
// @2x types are the retina variants of the size half as large
var icnsTypes = []struct {
	typ  string
	size int
}{
	{"icp4", 16}, {"icp5", 32}, {"ic11", 32}, {"ic12", 64}, {"ic07", 128},
	{"ic13", 256}, {"ic08", 256}, {"ic14", 512}, {"ic09", 512},
}

// icns builds the bundle icon from the PNGs, an icns file is "icns", the
// total length and then elements of type, length and PNG data
func icns() ([]byte, error) {
	var body bytes.Buffer
	for _, t := range icnsTypes {
		png, err := assets.Icons.ReadFile(fmt.Sprintf("icons/sshr-%d.png", t.size))
		if err != nil {
			return nil, err
		}
		body.WriteString(t.typ)
		binary.Write(&body, binary.BigEndian, uint32(8+len(png)))
		body.Write(png)
	}
	var out bytes.Buffer
	out.WriteString("icns")
	binary.Write(&out, binary.BigEndian, uint32(8+body.Len()))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
