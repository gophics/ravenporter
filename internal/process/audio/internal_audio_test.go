package audio

import "testing"

func TestAudioStepNames(t *testing.T) {
	if (&mixdownAudioStep{}).Name() != "MixdownAudio" {
		t.Fail()
	}
	if (&resampleAudioStep{}).Name() != "ResampleAudio" {
		t.Fail()
	}
	if (&normalizeAudioStep{}).Name() != "NormalizeAudio" {
		t.Fail()
	}
	if (&trimAudioStep{}).Name() != "TrimAudio" {
		t.Fail()
	}
}
