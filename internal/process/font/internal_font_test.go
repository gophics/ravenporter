package font

import "testing"

func TestFontStepNames(t *testing.T) {
	if (&generateFontAtlasStep{}).Name() != "GenerateFontAtlas" {
		t.Fail()
	}
}
