package branding

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEmbeddedIcon(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(PNG))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatal("application icon must be a 256px square")
	}
	_, _, _, corner := img.At(0, 0).RGBA()
	_, _, _, tile := img.At(64, 64).RGBA()
	if corner != 0 || tile != 65535 {
		t.Fatal("icon must retain transparent margins and opaque tiles")
	}
}
