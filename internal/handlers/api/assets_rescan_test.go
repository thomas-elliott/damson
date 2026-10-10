package api

import (
	"testing"

	"github.com/thomas-elliott/damson/internal/models"
)

// Rescan picks its cover source from the same decoder set an import renders
// with, so a loose tier-2 image is a candidate, ahead of audio.
func TestPickThumbnailCandidate_TakesEveryImageTheDecodersRead(t *testing.T) {
	for _, name := range []string{"art.psd", "sky.tga", "light.exr", "env.hdr"} {
		got := pickThumbnailCandidate([]models.AssetFile{{RelativePath: "theme.wav"}, {RelativePath: name}})
		if got == nil || got.RelativePath != name {
			t.Errorf("%s: picked %+v, want the image", name, got)
		}
	}
}
