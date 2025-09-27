package speech

import (
	"fmt"
	"reflect"
	"testing"
)

func TestNormalizeVoiceAlias(t *testing.T) {
	cases := []struct {
		alias  string
		expect string
	}{
		{alias: "hogwarts-young-hero", expect: "zh_male_junlangnanyou_emo_v2_mars_bigtts"},
		{alias: "zh_male_junlangnanyou_emo_v2_mars_bigtts", expect: "zh_male_junlangnanyou_emo_v2_mars_bigtts"},
		{alias: "", expect: ""},
	}

	for _, tc := range cases {
		if got := NormalizeVoiceAlias(tc.alias); got != tc.expect {
			t.Fatalf("NormalizeVoiceAlias(%s) = %s, want %s", tc.alias, got, tc.expect)
		}
	}
}

func TestResolveTTSResourceCandidates(t *testing.T) {
	tests := []struct {
		name  string
		voice string
		want  []string
	}{
		{
			name:  "default voice",
			voice: "",
			want:  []string{"volc.service_type.10029", "seed-tts-2.0"},
		},
		{
			name:  "mega clone voice",
			voice: "S_clone_speaker",
			want:  []string{"volc.megatts.default"},
		},
		{
			name:  "bigtts voice",
			voice: "zh_female_vv_uranus_bigtts",
			want:  []string{"seed-tts-2.0", "volc.service_type.10029"},
		},
		{
			name:  "legacy 1.0 voice",
			voice: "zh_male_organizer",
			want:  []string{"volc.service_type.10029", "seed-tts-2.0"},
		},
	}

	for _, tt := range tests {
		got := resolveTTSResourceCandidates(tt.voice)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: resolveTTSResourceCandidates(%q) = %v, want %v", tt.name, tt.voice, got, tt.want)
		}
	}
}

func TestResolveTTSSpeakerCandidates(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		fallback string
		want     []string
	}{
		{
			name:     "request and fallback",
			request:  "persona-voice",
			fallback: "zh_female_vv_uranus_bigtts",
			want:     []string{"persona-voice", "zh_female_vv_uranus_bigtts"},
		},
		{
			name:     "request empty",
			request:  "",
			fallback: "zh_male_M392_conversation_wvae_bigtts",
			want:     []string{"zh_male_M392_conversation_wvae_bigtts"},
		},
		{
			name:     "duplicates ignored",
			request:  "ZH_voice",
			fallback: "zh_voice",
			want:     []string{"ZH_voice"},
		},
		{
			name:     "persona alias",
			request:  "hogwarts-young-hero",
			fallback: "zh_male_M392_conversation_wvae_bigtts",
			want:     []string{"zh_male_junlangnanyou_emo_v2_mars_bigtts", "zh_male_M392_conversation_wvae_bigtts"},
		},
	}

	for _, tt := range tests {
		got := resolveTTSSpeakerCandidates(tt.request, tt.fallback)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: resolveTTSSpeakerCandidates(%q, %q) = %v, want %v", tt.name, tt.request, tt.fallback, got, tt.want)
		}
	}
}

func TestIsResourceMismatchError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("some other error"),
			want: false,
		},
		{
			name: "mismatch substring",
			err:  fmt.Errorf("TTS error: {\"error\":\"resource ID is mismatched with speaker related resource\"}"),
			want: true,
		},
	}

	for _, tc := range cases {
		if got := isResourceMismatchError(tc.err); got != tc.want {
			t.Errorf("%s: isResourceMismatchError(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}
