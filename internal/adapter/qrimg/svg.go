package qrimg

import (
	"bytes"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// SVG encodes content as a print-ready QR. No third-party API, no pixels on disk.
func SVG(content string) ([]byte, error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	code.DisableBorder = false
	bits := code.Bitmap()
	if len(bits) == 0 {
		return nil, fmt.Errorf("empty qr")
	}
	n := len(bits)
	var b bytes.Buffer
	b.Grow(n*n + 128)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	b.WriteString(`<rect width="100%" height="100%" fill="#fff"/>`)
	for y, row := range bits {
		for x, on := range row {
			if on {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="1" height="1" fill="#111"/>`, x, y)
			}
		}
	}
	b.WriteString(`</svg>`)
	return b.Bytes(), nil
}

func IsSVG(raw []byte) bool {
	return strings.Contains(string(raw[:min(64, len(raw))]), "<svg")
}
