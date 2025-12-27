// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

// Package cmx3600 provides support for reading and writing CMX 3600 EDL (Edit Decision List) files.
// The CMX 3600 format is a text-based interchange format used in video editing.
package cmx3600

import (
	"fmt"
	"strings"
)

// EditType represents the type of edit in an EDL.
type EditType string

const (
	// EditTypeCut represents a cut (instantaneous transition).
	EditTypeCut EditType = "C"
	// EditTypeDissolve represents a dissolve/cross-fade.
	EditTypeDissolve EditType = "D"
	// EditTypeWipe represents a wipe transition.
	EditTypeWipe EditType = "W"
	// EditTypeKeyBackground represents a key with background.
	EditTypeKeyBackground EditType = "KB"
	// EditTypeKey represents a key (overlay).
	EditTypeKey EditType = "K"
)

// TrackType represents the type of track in an EDL.
type TrackType string

const (
	// TrackTypeVideo represents a video track.
	TrackTypeVideo TrackType = "V"
	// TrackTypeAudio represents an audio track.
	TrackTypeAudio TrackType = "A"
	// TrackTypeAudio1 represents audio track 1.
	TrackTypeAudio1 TrackType = "A1"
	// TrackTypeAudio2 represents audio track 2.
	TrackTypeAudio2 TrackType = "A2"
	// TrackTypeAudio3 represents audio track 3.
	TrackTypeAudio3 TrackType = "A3"
	// TrackTypeAudio4 represents audio track 4.
	TrackTypeAudio4 TrackType = "A4"
)

// IsVideoTrack returns true if the track type is video.
func (t TrackType) IsVideoTrack() bool {
	return t == TrackTypeVideo
}

// IsAudioTrack returns true if the track type is audio.
func (t TrackType) IsAudioTrack() bool {
	return t == TrackTypeAudio ||
		t == TrackTypeAudio1 ||
		t == TrackTypeAudio2 ||
		t == TrackTypeAudio3 ||
		t == TrackTypeAudio4
}

// EDLEvent represents a single edit event in an EDL.
type EDLEvent struct {
	EventNumber        int       // Event number (line number in EDL)
	ReelName           string    // Source reel/tape name
	TrackType          TrackType // Track type (V, A, A1, A2, etc.)
	EditType           EditType  // Edit type (C, D, W, etc.)
	SourceIn           string    // Source in timecode (HH:MM:SS:FF)
	SourceOut          string    // Source out timecode (HH:MM:SS:FF)
	RecordIn           string    // Record in timecode (HH:MM:SS:FF)
	RecordOut          string    // Record out timecode (HH:MM:SS:FF)
	Comment            string    // Optional comment line(s)
	ClipName           string    // Clip name from comment
	TransitionDuration int       // Transition duration in frames (for dissolves/wipes)
	WipeCode           string    // Wipe code (e.g., W001, W002)
	SpeedEffect        *SpeedEffect // M2 motion effect
	FreezeFrame        bool      // Freeze frame detected
	FilePath           string    // File path from FROM CLIP/FROM FILE comment
	Markers            []Marker  // Locators/markers
	ASCCDL             *ASCCDL   // ASC CDL color correction
}

// SpeedEffect represents an M2 motion effect.
type SpeedEffect struct {
	Name     string  // Effect name/reel
	Speed    float64 // Speed multiplier (frames per second)
	Timecode string  // Source timecode
}

// Marker represents a locator or marker in an EDL.
type Marker struct {
	Timecode string // Marker timecode
	Color    string // Marker color
	Comment  string // Marker comment
}

// ASCCDL represents ASC Color Decision List metadata.
type ASCCDL struct {
	Slope      [3]float64 // RGB slope values
	Offset     [3]float64 // RGB offset values
	Power      [3]float64 // RGB power values
	Saturation float64    // Saturation value
}

// OutputStyle represents the style/flavor of EDL output.
type OutputStyle string

const (
	// OutputStyleAvid represents Avid Media Composer style EDL.
	OutputStyleAvid OutputStyle = "avid"
	// OutputStyleNucoda represents Nucoda style EDL.
	OutputStyleNucoda OutputStyle = "nucoda"
	// OutputStylePremiere represents Adobe Premiere Pro style EDL.
	OutputStylePremiere OutputStyle = "premiere"
)

// DefaultReelNameLength is the default maximum length for reel names.
const DefaultReelNameLength = 8

// SanitizeReelName ensures a reel name conforms to EDL requirements.
// Reel names should be alphanumeric and not exceed the specified length.
// If maxLength is 0 or negative, no length limit is applied.
func SanitizeReelName(name string, maxLength int) string {
	// Replace spaces and special characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Truncate to max length if maxLength is positive
	if maxLength > 0 && len(name) > maxLength {
		name = name[:maxLength]
	}

	// Ensure not empty
	if name == "" {
		name = "AX"
	}

	return name
}

// ParseError represents an error that occurred during EDL parsing.
type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// EncodeError represents an error that occurred during EDL encoding.
type EncodeError struct {
	Message string
}

func (e *EncodeError) Error() string {
	return fmt.Sprintf("encode error: %s", e.Message)
}
