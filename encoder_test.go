// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package cmx3600

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio"
)

func TestEncoder_SimpleTimeline(t *testing.T) {
	// Create a simple timeline with one video track and two clips
	timeline := gotio.NewTimeline("Test Timeline", nil, nil)
	track := gotio.NewTrack("V", nil, gotio.TrackKindVideo, nil, nil)

	// Create first clip
	sourceRange1 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24), // 5 seconds
	)
	mediaRef1 := gotio.NewExternalReference("Clip1", "Clip1", &sourceRange1, nil)
	clip1 := gotio.NewClip("Clip1", mediaRef1, &sourceRange1, nil, nil, nil, "", nil)

	// Create second clip
	sourceRange2 := opentime.NewTimeRange(
		opentime.NewRationalTime(240, 24), // 10 seconds in
		opentime.NewRationalTime(120, 24), // 5 seconds duration
	)
	mediaRef2 := gotio.NewExternalReference("Clip2", "Clip2", &sourceRange2, nil)
	clip2 := gotio.NewClip("Clip2", mediaRef2, &sourceRange2, nil, nil, nil, "", nil)

	track.AppendChild(clip1)
	track.AppendChild(clip2)
	timeline.Tracks().AppendChild(track)

	// Encode to EDL
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	output := buf.String()

	// Check that output contains expected elements
	if !strings.Contains(output, "TITLE: Test Timeline") {
		t.Error("Output missing title")
	}

	if !strings.Contains(output, "FCM: NON-DROP FRAME") {
		t.Error("Output missing FCM line")
	}

	if !strings.Contains(output, "001") {
		t.Error("Output missing event 001")
	}

	if !strings.Contains(output, "002") {
		t.Error("Output missing event 002")
	}

	if !strings.Contains(output, "FROM CLIP NAME: Clip1") {
		t.Error("Output missing Clip1 name")
	}

	if !strings.Contains(output, "FROM CLIP NAME: Clip2") {
		t.Error("Output missing Clip2 name")
	}
}

func TestEncoder_WithTransition(t *testing.T) {
	// Create timeline with dissolve transition
	timeline := gotio.NewTimeline("Transition Test", nil, nil)
	track := gotio.NewTrack("V", nil, gotio.TrackKindVideo, nil, nil)

	// Create first clip
	sourceRange1 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef1 := gotio.NewExternalReference("Shot1", "Shot1", &sourceRange1, nil)
	clip1 := gotio.NewClip("Shot1", mediaRef1, &sourceRange1, nil, nil, nil, "", nil)

	// Create transition (30 frames dissolve)
	transition := gotio.NewTransition(
		"",
		"SMPTE_Dissolve",
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(30, 24),
		nil,
	)

	// Create second clip
	sourceRange2 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef2 := gotio.NewExternalReference("Shot2", "Shot2", &sourceRange2, nil)
	clip2 := gotio.NewClip("Shot2", mediaRef2, &sourceRange2, nil, nil, nil, "", nil)

	track.AppendChild(clip1)
	track.AppendChild(transition)
	track.AppendChild(clip2)
	timeline.Tracks().AppendChild(track)

	// Encode to EDL
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	output := buf.String()

	// Check for dissolve with duration
	if !strings.Contains(output, "D") {
		t.Error("Output missing dissolve edit type")
	}

	if !strings.Contains(output, "030") {
		t.Error("Output missing transition duration")
	}
}

func TestEncoder_AudioTracks(t *testing.T) {
	// Create timeline with audio tracks
	timeline := gotio.NewTimeline("Audio Test", nil, nil)

	// Audio track 1
	track1 := gotio.NewTrack("A1", nil, gotio.TrackKindAudio, nil, nil)
	sourceRange1 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef1 := gotio.NewExternalReference("Audio1", "Audio1", &sourceRange1, nil)
	clip1 := gotio.NewClip("Audio1", mediaRef1, &sourceRange1, nil, nil, nil, "", nil)
	track1.AppendChild(clip1)

	// Audio track 2
	track2 := gotio.NewTrack("A2", nil, gotio.TrackKindAudio, nil, nil)
	sourceRange2 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef2 := gotio.NewExternalReference("Audio2", "Audio2", &sourceRange2, nil)
	clip2 := gotio.NewClip("Audio2", mediaRef2, &sourceRange2, nil, nil, nil, "", nil)
	track2.AppendChild(clip2)

	timeline.Tracks().AppendChild(track1)
	timeline.Tracks().AppendChild(track2)

	// Encode to EDL
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	output := buf.String()

	// Check for audio track types
	if !strings.Contains(output, "A1") {
		t.Error("Output missing A1 track type")
	}

	if !strings.Contains(output, "A2") {
		t.Error("Output missing A2 track type")
	}
}

func TestEncoder_ReelNameSanitization(t *testing.T) {
	// Create timeline with long reel name
	timeline := gotio.NewTimeline("Reel Test", nil, nil)
	track := gotio.NewTrack("V", nil, gotio.TrackKindVideo, nil, nil)

	sourceRange := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef := gotio.NewExternalReference(
		"VeryLongReelNameThatExceedsLimit",
		"VeryLongReelNameThatExceedsLimit",
		&sourceRange,
		nil,
	)
	clip := gotio.NewClip(
		"Clip",
		mediaRef,
		&sourceRange,
		nil, nil, nil, "", nil,
	)

	track.AppendChild(clip)
	timeline.Tracks().AppendChild(track)

	// Encode with default reel name length (8)
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)
	encoder.SetReelNameLength(8)

	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	output := buf.String()

	// Check that reel name is truncated to 8 characters
	if strings.Contains(output, "VeryLongReelNameThatExceedsLimit") {
		t.Error("Reel name should be truncated")
	}

	// Should contain truncated version
	if !strings.Contains(output, "VeryLong") {
		t.Error("Output missing truncated reel name")
	}
}

func TestEncoder_EmptyTimeline(t *testing.T) {
	timeline := gotio.NewTimeline("Empty", nil, nil)

	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	output := buf.String()

	// Should still have header
	if !strings.Contains(output, "TITLE: Empty") {
		t.Error("Output missing title")
	}

	if !strings.Contains(output, "FCM: NON-DROP FRAME") {
		t.Error("Output missing FCM line")
	}

	// Should not have any events
	if strings.Contains(output, "001") {
		t.Error("Empty timeline should not have events")
	}
}

func TestEncoder_MultipleVideoTracks(t *testing.T) {
	// EDL doesn't support multiple video tracks - should error
	timeline := gotio.NewTimeline("Multi Video", nil, nil)

	track1 := gotio.NewTrack("V1", nil, gotio.TrackKindVideo, nil, nil)
	track2 := gotio.NewTrack("V2", nil, gotio.TrackKindVideo, nil, nil)

	timeline.Tracks().AppendChild(track1)
	timeline.Tracks().AppendChild(track2)

	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(timeline)
	if err == nil {
		t.Error("Expected error for multiple video tracks")
	}

	if _, ok := err.(*EncodeError); !ok {
		t.Error("Expected EncodeError type")
	}
}

func TestEncoder_RoundTrip(t *testing.T) {
	// Create a timeline, encode it, decode it, and verify
	originalTimeline := gotio.NewTimeline("Round Trip Test", nil, nil)
	track := gotio.NewTrack("V", nil, gotio.TrackKindVideo, nil, nil)

	sourceRange := opentime.NewTimeRange(
		opentime.NewRationalTime(0, 24),
		opentime.NewRationalTime(120, 24),
	)
	mediaRef := gotio.NewExternalReference("TestClip", "TestClip", &sourceRange, nil)
	clip := gotio.NewClip("TestClip", mediaRef, &sourceRange, nil, nil, nil, "", nil)

	track.AppendChild(clip)
	originalTimeline.Tracks().AppendChild(track)

	// Encode
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	encoder.SetRate(24.0)

	err := encoder.Encode(originalTimeline)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Decode
	decoder := NewDecoder(strings.NewReader(buf.String()))
	decoder.SetRate(24.0)

	decodedTimeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	// Verify
	videoTracks := decodedTimeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	children := videoTracks[0].Children()
	if len(children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(children))
	}

	clip, ok := children[0].(*gotio.Clip)
	if !ok {
		t.Fatal("Child is not a clip")
	}

	if clip.Name() != "TestClip" {
		t.Errorf("Expected clip name 'TestClip', got '%s'", clip.Name())
	}

	duration, err := clip.Duration()
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}

	expectedDuration := opentime.NewRationalTime(120, 24)
	if duration.Value() != expectedDuration.Value() {
		t.Errorf("Expected duration %v, got %v", expectedDuration, duration)
	}
}
