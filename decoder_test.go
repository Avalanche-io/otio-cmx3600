// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package cmx3600

import (
	"strings"
	"testing"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio"
)

func TestDecoder_SimpleEDL(t *testing.T) {
	edl := `TITLE: Test Timeline
FCM: NON-DROP FRAME

001  AX       V     C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
* FROM CLIP NAME: Shot1

002  AX       V     C
     00:00:10:00 00:00:15:00 00:00:05:00 00:00:10:00
* FROM CLIP NAME: Shot2

003  AX       V     D    030
     00:00:20:00 00:00:25:00 00:00:10:00 00:00:15:00
* FROM CLIP NAME: Shot3
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if timeline == nil {
		t.Fatal("Decode() returned nil timeline")
	}

	// Check that we have a video track
	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()

	// We should have 3 clips and 1 transition
	// Shot1, Shot2, Transition, Shot3
	clipCount := 0
	transitionCount := 0
	for _, child := range children {
		if _, ok := child.(*gotio.Clip); ok {
			clipCount++
		}
		if _, ok := child.(*gotio.Transition); ok {
			transitionCount++
		}
	}

	if clipCount != 3 {
		t.Errorf("Expected 3 clips, got %d", clipCount)
	}

	if transitionCount != 1 {
		t.Errorf("Expected 1 transition, got %d", transitionCount)
	}

	// Check first clip
	firstClip := children[0].(*gotio.Clip)
	if firstClip.Name() != "Shot1" {
		t.Errorf("Expected first clip name 'Shot1', got '%s'", firstClip.Name())
	}

	// Check duration
	duration, err := firstClip.Duration()
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}
	expectedDuration := opentime.NewRationalTime(5*24, 24) // 5 seconds at 24fps
	if duration.Value() != expectedDuration.Value() {
		t.Errorf("Expected duration %v, got %v", expectedDuration, duration)
	}
}

func TestDecoder_AudioTrack(t *testing.T) {
	edl := `TITLE: Audio Test
FCM: NON-DROP FRAME

001  AUDIO1   A1    C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
* FROM CLIP NAME: AudioClip1

002  AUDIO2   A2    C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
* FROM CLIP NAME: AudioClip2
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	// Check that we have audio tracks
	audioTracks := timeline.AudioTracks()
	if len(audioTracks) != 2 {
		t.Fatalf("Expected 2 audio tracks, got %d", len(audioTracks))
	}

	// Collect all clip names from audio tracks (order may vary due to map iteration)
	clipNames := make(map[string]bool)
	for _, track := range audioTracks {
		children := track.Children()
		if len(children) != 1 {
			t.Errorf("Expected 1 child in audio track, got %d", len(children))
		}
		if clip, ok := children[0].(*gotio.Clip); ok {
			clipNames[clip.Name()] = true
		}
	}

	// Verify both clips are present
	if !clipNames["AudioClip1"] {
		t.Error("AudioClip1 not found in audio tracks")
	}
	if !clipNames["AudioClip2"] {
		t.Error("AudioClip2 not found in audio tracks")
	}
}

func TestDecoder_WithGaps(t *testing.T) {
	edl := `TITLE: Gap Test
FCM: NON-DROP FRAME

001  AX       V     C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
* FROM CLIP NAME: Shot1

002  AX       V     C
     00:00:10:00 00:00:15:00 00:00:10:00 00:00:15:00
* FROM CLIP NAME: Shot2
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()

	// Should have: Clip, Gap, Clip
	gapCount := 0
	clipCount := 0
	for _, child := range children {
		if _, ok := child.(*gotio.Gap); ok {
			gapCount++
		}
		if _, ok := child.(*gotio.Clip); ok {
			clipCount++
		}
	}

	if clipCount != 2 {
		t.Errorf("Expected 2 clips, got %d", clipCount)
	}

	if gapCount != 1 {
		t.Errorf("Expected 1 gap, got %d", gapCount)
	}

	// Check gap duration (5 seconds)
	for _, child := range children {
		if gap, ok := child.(*gotio.Gap); ok {
			duration, err := gap.Duration()
			if err != nil {
				t.Fatalf("Gap duration error = %v", err)
			}
			expectedGapDuration := opentime.NewRationalTime(5*24, 24) // 5 seconds at 24fps
			if duration.Value() != expectedGapDuration.Value() {
				t.Errorf("Expected gap duration %v, got %v", expectedGapDuration, duration)
			}
		}
	}
}

func TestDecoder_EmptyEDL(t *testing.T) {
	edl := `TITLE: Empty
FCM: NON-DROP FRAME
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if timeline == nil {
		t.Fatal("Decode() returned nil timeline")
	}

	// Should have no tracks
	tracks := timeline.Tracks()
	if len(tracks.Children()) != 0 {
		t.Errorf("Expected 0 tracks, got %d", len(tracks.Children()))
	}
}

func TestDecoder_InvalidTimecode(t *testing.T) {
	edl := `TITLE: Invalid
FCM: NON-DROP FRAME

001  AX       V     C
     XX:XX:XX:XX 00:00:05:00 00:00:00:00 00:00:05:00
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	_, err := decoder.Decode()
	if err == nil {
		t.Error("Expected error for invalid timecode, got nil")
	}
}

func TestDecoder_SpeedEffects(t *testing.T) {
	edl := `TITLE: Speed Effects Test
FCM: NON-DROP FRAME

001  CLIP1    V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: SpeedClip
M2   CLIP1       047.6                01:00:04:05
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()
	if len(children) != 1 {
		t.Fatalf("Expected 1 clip, got %d", len(children))
	}

	clip := children[0].(*gotio.Clip)
	effects := clip.Effects()
	if len(effects) != 1 {
		t.Fatalf("Expected 1 effect, got %d", len(effects))
	}

	// Check if it's a LinearTimeWarp
	if ltw, ok := effects[0].(*gotio.LinearTimeWarp); ok {
		// Time scalar should be speed / rate = 47.6 / 24.0
		expectedScalar := 47.6 / 24.0
		if ltw.TimeScalar() != expectedScalar {
			t.Errorf("Expected time scalar %f, got %f", expectedScalar, ltw.TimeScalar())
		}
	} else {
		t.Errorf("Expected LinearTimeWarp effect, got %T", effects[0])
	}
}

func TestDecoder_WipeTransitions(t *testing.T) {
	edl := `TITLE: Wipe Test
FCM: NON-DROP FRAME

001  CLIP1    V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: Clip1

002  CLIP2    V     W001    030
     01:00:06:00 01:00:07:00 00:00:01:07 00:00:02:07
* FROM CLIP NAME: Clip2
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()

	// Should have: Clip1, Transition (wipe), Clip2
	transitionFound := false
	for _, child := range children {
		if transition, ok := child.(*gotio.Transition); ok {
			transitionFound = true
			if transition.TransitionType() != gotio.TransitionTypeCustom {
				t.Errorf("Expected Custom transition type, got %v", transition.TransitionType())
			}
			if transition.Name() != "W001" {
				t.Errorf("Expected transition name 'W001', got '%s'", transition.Name())
			}
		}
	}

	if !transitionFound {
		t.Error("Expected wipe transition not found")
	}
}

func TestDecoder_FreezeFrame(t *testing.T) {
	edl := `TITLE: Freeze Frame Test
FCM: NON-DROP FRAME

001  CLIP1    V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: FrozenClip FF
* FREEZE FRAME
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()
	if len(children) != 1 {
		t.Fatalf("Expected 1 clip, got %d", len(children))
	}

	clip := children[0].(*gotio.Clip)

	// Check clip name has FF suffix stripped
	if clip.Name() != "FrozenClip" {
		t.Errorf("Expected clip name 'FrozenClip' (FF stripped), got '%s'", clip.Name())
	}

	// Check for freeze frame effect
	effects := clip.Effects()
	freezeFrameFound := false
	for _, effect := range effects {
		if _, ok := effect.(*gotio.FreezeFrame); ok {
			freezeFrameFound = true
			break
		}
	}

	if !freezeFrameFound {
		t.Error("Expected FreezeFrame effect not found")
	}
}

func TestDecoder_GeneratorReferences(t *testing.T) {
	tests := []struct {
		name          string
		reelName      string
		expectedKind  string
	}{
		{"BLACK", "BLACK", "black"},
		{"BL", "BL", "black"},
		{"BARS", "BARS", "SMPTEBars"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edl := `TITLE: Generator Test
FCM: NON-DROP FRAME

001  ` + tt.reelName + `       V     C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
`

			decoder := NewDecoder(strings.NewReader(edl))
			decoder.SetRate(24.0)

			timeline, err := decoder.Decode()
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			videoTracks := timeline.VideoTracks()
			if len(videoTracks) != 1 {
				t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
			}

			track := videoTracks[0]
			children := track.Children()
			if len(children) != 1 {
				t.Fatalf("Expected 1 clip, got %d", len(children))
			}

			clip := children[0].(*gotio.Clip)
			mediaRef := clip.MediaReference()

			if genRef, ok := mediaRef.(*gotio.GeneratorReference); ok {
				if genRef.GeneratorKind() != tt.expectedKind {
					t.Errorf("Expected generator kind '%s', got '%s'", tt.expectedKind, genRef.GeneratorKind())
				}
			} else {
				t.Errorf("Expected GeneratorReference, got %T", mediaRef)
			}
		})
	}
}

func TestDecoder_ASCCDL(t *testing.T) {
	edl := `TITLE: CDL Test
FCM: NON-DROP FRAME

001  AX       V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: ColorCorrected
* ASC_SOP (1.5 1.0 0.9) (0.1 -0.2 0.0) (1.0 1.1 0.95)
* ASC_SAT 0.9
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()
	if len(children) != 1 {
		t.Fatalf("Expected 1 clip, got %d", len(children))
	}

	clip := children[0].(*gotio.Clip)
	metadata := clip.Metadata()

	cdl, ok := metadata["cdl"]
	if !ok {
		t.Fatal("Expected CDL metadata not found")
	}

	cdlMap, ok := cdl.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected CDL to be a map, got %T", cdl)
	}

	// Check slope values
	slope := cdlMap["slope"].([3]float64)
	if slope[0] != 1.5 || slope[1] != 1.0 || slope[2] != 0.9 {
		t.Errorf("Expected slope [1.5 1.0 0.9], got %v", slope)
	}

	// Check offset values
	offset := cdlMap["offset"].([3]float64)
	if offset[0] != 0.1 || offset[1] != -0.2 || offset[2] != 0.0 {
		t.Errorf("Expected offset [0.1 -0.2 0.0], got %v", offset)
	}

	// Check power values
	power := cdlMap["power"].([3]float64)
	if power[0] != 1.0 || power[1] != 1.1 || power[2] != 0.95 {
		t.Errorf("Expected power [1.0 1.1 0.95], got %v", power)
	}

	// Check saturation
	saturation := cdlMap["saturation"].(float64)
	if saturation != 0.9 {
		t.Errorf("Expected saturation 0.9, got %f", saturation)
	}
}

func TestDecoder_Markers(t *testing.T) {
	edl := `TITLE: Marker Test
FCM: NON-DROP FRAME

001  AX       V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: MarkedClip
* LOC: 01:00:04:10 RED This is a marker
* LOC: 01:00:05:00 BLUE Another marker
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()
	if len(children) != 1 {
		t.Fatalf("Expected 1 clip, got %d", len(children))
	}

	clip := children[0].(*gotio.Clip)
	markers := clip.Markers()

	if len(markers) != 2 {
		t.Fatalf("Expected 2 markers, got %d", len(markers))
	}

	// Check first marker
	if markers[0].Comment() != "This is a marker" {
		t.Errorf("Expected marker comment 'This is a marker', got '%s'", markers[0].Comment())
	}
	if string(markers[0].Color()) != "RED" {
		t.Errorf("Expected marker color 'RED', got '%s'", markers[0].Color())
	}

	// Check second marker
	if markers[1].Comment() != "Another marker" {
		t.Errorf("Expected marker comment 'Another marker', got '%s'", markers[1].Comment())
	}
	if string(markers[1].Color()) != "BLUE" {
		t.Errorf("Expected marker color 'BLUE', got '%s'", markers[1].Color())
	}
}

func TestDecoder_StyleVariants(t *testing.T) {
	tests := []struct {
		name         string
		edl          string
		expectedPath string
	}{
		{
			name: "Avid style",
			edl: `TITLE: Avid Test
FCM: NON-DROP FRAME

001  CLIP1    V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: TestClip
* FROM CLIP: S:\path\to\clip.mov
`,
			expectedPath: `S:\path\to\clip.mov`,
		},
		{
			name: "Nucoda style",
			edl: `TITLE: Nucoda Test
FCM: NON-DROP FRAME

001  CLIP1    V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: TestClip
* FROM FILE: S:\path\to\clip.exr
`,
			expectedPath: `S:\path\to\clip.exr`,
		},
		{
			name: "Premiere style",
			edl: `TITLE: Premiere Test
FCM: NON-DROP FRAME

001  AX       V     C
     01:00:04:05 01:00:05:12 00:00:00:00 00:00:01:07
* FROM CLIP NAME: clip.mov
`,
			expectedPath: `AX`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(strings.NewReader(tt.edl))
			decoder.SetRate(24.0)

			timeline, err := decoder.Decode()
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			videoTracks := timeline.VideoTracks()
			if len(videoTracks) != 1 {
				t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
			}

			track := videoTracks[0]
			children := track.Children()
			if len(children) != 1 {
				t.Fatalf("Expected 1 clip, got %d", len(children))
			}

			clip := children[0].(*gotio.Clip)
			mediaRef := clip.MediaReference()

			if extRef, ok := mediaRef.(*gotio.ExternalReference); ok {
				if extRef.TargetURL() != tt.expectedPath {
					t.Errorf("Expected target URL '%s', got '%s'", tt.expectedPath, extRef.TargetURL())
				}
			} else {
				t.Errorf("Expected ExternalReference, got %T", mediaRef)
			}
		})
	}
}

func TestDecoder_FCMHeader(t *testing.T) {
	tests := []struct {
		name        string
		fcmHeader   string
		expectedFCM string
	}{
		{"Drop frame", "DROP FRAME", "DROP FRAME"},
		{"Non-drop frame", "NON-DROP FRAME", "NON-DROP FRAME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edl := `TITLE: FCM Test
FCM: ` + tt.fcmHeader + `

001  AX       V     C
     00:00:00:00 00:00:05:00 00:00:00:00 00:00:05:00
`

			decoder := NewDecoder(strings.NewReader(edl))
			decoder.SetRate(24.0)

			_, err := decoder.Decode()
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			if decoder.fcmMode != tt.expectedFCM {
				t.Errorf("Expected FCM mode '%s', got '%s'", tt.expectedFCM, decoder.fcmMode)
			}
		})
	}
}

func TestDecoder_ComprehensiveFeatures(t *testing.T) {
	// This test validates all new features together
	edl := `TITLE:   Comprehensive_Features_Test
FCM: NON-DROP FRAME
001  BLACK    V     C
     00:00:00:00 00:00:02:00 00:00:00:00 00:00:02:00
* FROM CLIP NAME: BlackLeader
002  ZZ100_50 V     C
     01:00:04:05 01:00:05:12 00:00:02:00 00:00:03:07
* FROM CLIP NAME:  Speed_Clip
* FROM CLIP: S:\path\to\ZZ100_501.speed.0001.exr
M2   ZZ100_50       047.6                01:00:04:05
* ASC_SOP (1.5 1.0 0.9) (0.1 -0.2 0.0) (1.0 1.1 0.95)
* ASC_SAT 0.9
* LOC: 01:00:04:10 RED Start marker
003  BARS     V     C
     00:00:00:00 00:00:03:00 00:00:03:07 00:00:06:07
* FROM CLIP NAME: ColorBars
004  ZZ100_51 V     W001    030
     01:00:06:13 01:00:08:15 00:00:06:07 00:00:08:09
* FROM CLIP NAME:  Wipe_Clip
* FROM FILE: S:\path\to\ZZ100_502A.wipe.0101.exr
005  ZZ100_52 V     C
     01:00:10:00 01:00:12:00 00:00:08:09 00:00:10:09
* FROM CLIP NAME: FrozenClip FF
* FREEZE FRAME
`

	decoder := NewDecoder(strings.NewReader(edl))
	decoder.SetRate(24.0)

	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	track := videoTracks[0]
	children := track.Children()

	// Should have 5 clips and 1 transition
	clipCount := 0
	transitionCount := 0
	generatorCount := 0
	speedEffectCount := 0
	freezeFrameCount := 0
	cdlCount := 0
	markerCount := 0
	wipeCount := 0

	for _, child := range children {
		if clip, ok := child.(*gotio.Clip); ok {
			clipCount++

			// Check for generator references
			if genRef, ok := clip.MediaReference().(*gotio.GeneratorReference); ok {
				generatorCount++
				t.Logf("Generator: %s - %s", clip.Name(), genRef.GeneratorKind())
			}

			// Check for speed effects
			for _, effect := range clip.Effects() {
				if _, ok := effect.(*gotio.LinearTimeWarp); ok {
					speedEffectCount++
				}
				if _, ok := effect.(*gotio.FreezeFrame); ok {
					freezeFrameCount++
				}
			}

			// Check for CDL metadata
			if _, hasCDL := clip.Metadata()["cdl"]; hasCDL {
				cdlCount++
			}

			// Check for markers
			markerCount += len(clip.Markers())
		}

		if transition, ok := child.(*gotio.Transition); ok {
			transitionCount++
			if transition.TransitionType() == gotio.TransitionTypeCustom {
				wipeCount++
			}
		}
	}

	if clipCount != 5 {
		t.Errorf("Expected 5 clips, got %d", clipCount)
	}

	if transitionCount != 1 {
		t.Errorf("Expected 1 transition, got %d", transitionCount)
	}

	if generatorCount != 2 {
		t.Errorf("Expected 2 generator references (BLACK, BARS), got %d", generatorCount)
	}

	if speedEffectCount != 1 {
		t.Errorf("Expected 1 speed effect, got %d", speedEffectCount)
	}

	if freezeFrameCount != 1 {
		t.Errorf("Expected 1 freeze frame, got %d", freezeFrameCount)
	}

	if cdlCount != 1 {
		t.Errorf("Expected 1 CDL metadata, got %d", cdlCount)
	}

	if markerCount != 1 {
		t.Errorf("Expected 1 marker, got %d", markerCount)
	}

	if wipeCount != 1 {
		t.Errorf("Expected 1 wipe transition, got %d", wipeCount)
	}

	t.Log("Successfully validated all comprehensive features!")
}

func TestSanitizeReelName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		want      string
	}{
		{
			name:      "simple name",
			input:     "MyClip",
			maxLength: 8,
			want:      "MyClip",
		},
		{
			name:      "name with spaces",
			input:     "My Clip",
			maxLength: 8,
			want:      "My_Clip",
		},
		{
			name:      "long name",
			input:     "VeryLongClipName",
			maxLength: 8,
			want:      "VeryLong",
		},
		{
			name:      "special characters",
			input:     "Clip-123!",
			maxLength: 8,
			want:      "Clip_123",
		},
		{
			name:      "empty name",
			input:     "",
			maxLength: 8,
			want:      "AX",
		},
		{
			name:      "unlimited length",
			input:     "VeryLongClipName",
			maxLength: 0,
			want:      "VeryLongClipName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeReelName(tt.input, tt.maxLength)
			if got != tt.want {
				t.Errorf("SanitizeReelName() = %v, want %v", got, tt.want)
			}
		})
	}
}
