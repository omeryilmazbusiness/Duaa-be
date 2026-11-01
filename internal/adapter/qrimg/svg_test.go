package qrimg_test

import (
	"bytes"
	"testing"

	"github.com/dua-os/dua-os-be/internal/adapter/qrimg"
)

func TestSVGEncodesURL(t *testing.T) {
	raw, err := qrimg.SVG("http://localhost:3000/q/abcdefghijabcdefghijab")
	if err != nil || !qrimg.IsSVG(raw) || !bytes.Contains(raw, []byte("<rect")) {
		t.Fatalf("svg=%s err=%v", raw, err)
	}
}
