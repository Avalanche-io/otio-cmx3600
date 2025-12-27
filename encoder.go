// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package cmx3600

import (
	"fmt"
	"io"
	"strings"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio/opentimelineio"
)

// Encoder writes OpenTimelineIO Timeline to CMX 3600 EDL format.
type Encoder struct {
	w             io.Writer
	style         OutputStyle
	reelNameLen   int
	rate          float64
}

// NewEncoder creates a new EDL encoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:           w,
		style:       OutputStyleAvid,
		reelNameLen: DefaultReelNameLength,
		rate:        24.0, // Default frame rate
	}
}

// SetStyle sets the output style (avid, nucoda, premiere).
func (e *Encoder) SetStyle(style OutputStyle) {
	e.style = style
}

// SetReelNameLength sets the maximum length for reel names.
// Use 0 or negative for unlimited length.
func (e *Encoder) SetReelNameLength(length int) {
	e.reelNameLen = length
}

// SetRate sets the frame rate for timecode generation.
func (e *Encoder) SetRate(rate float64) {
	e.rate = rate
}

// Encode writes the Timeline to EDL format.
func (e *Encoder) Encode(t *opentimelineio.Timeline) error {
	if t == nil {
		return &EncodeError{Message: "timeline is nil"}
	}

	// Write header
	if err := e.writeHeader(t); err != nil {
		return err
	}

	// Get video tracks (EDL supports only one video track)
	videoTracks := t.VideoTracks()
	if len(videoTracks) > 1 {
		return &EncodeError{Message: "EDL format supports only one video track"}
	}

	// Get audio tracks
	audioTracks := t.AudioTracks()

	eventNumber := 1

	// Write video track events
	if len(videoTracks) > 0 {
		track := videoTracks[0]
		var err error
		eventNumber, err = e.writeTrackEvents(track, TrackTypeVideo, eventNumber)
		if err != nil {
			return err
		}
	}

	// Write audio track events
	for i, track := range audioTracks {
		trackType := TrackTypeAudio
		if i == 0 {
			trackType = TrackTypeAudio1
		} else if i == 1 {
			trackType = TrackTypeAudio2
		} else if i == 2 {
			trackType = TrackTypeAudio3
		} else if i == 3 {
			trackType = TrackTypeAudio4
		}

		var err error
		eventNumber, err = e.writeTrackEvents(track, trackType, eventNumber)
		if err != nil {
			return err
		}
	}

	return nil
}

// writeHeader writes the EDL header.
func (e *Encoder) writeHeader(t *opentimelineio.Timeline) error {
	title := t.Name()
	if title == "" {
		title = "Timeline"
	}

	_, err := fmt.Fprintf(e.w, "TITLE: %s\n", title)
	if err != nil {
		return err
	}

	// Write FCM (Frame Count Mode) - NON-DROP FRAME by default
	_, err = fmt.Fprintf(e.w, "FCM: NON-DROP FRAME\n\n")
	return err
}

// writeTrackEvents writes all events for a track.
func (e *Encoder) writeTrackEvents(track *opentimelineio.Track, trackType TrackType, startEventNum int) (int, error) {
	eventNumber := startEventNum
	recordTime := opentime.NewRationalTime(0, e.rate)

	children := track.Children()
	for i := 0; i < len(children); i++ {
		child := children[i]

		// Handle gaps
		if gap, ok := child.(*opentimelineio.Gap); ok {
			duration, err := gap.Duration()
			if err != nil {
				return eventNumber, err
			}
			recordTime = recordTime.Add(duration)
			continue
		}

		// Handle clips
		clip, ok := child.(*opentimelineio.Clip)
		if !ok {
			// Skip non-clip, non-gap items
			continue
		}

		// Get clip duration and source range
		duration, err := clip.Duration()
		if err != nil {
			return eventNumber, err
		}

		sourceRange := clip.SourceRange()
		if sourceRange == nil {
			// Use available range if no source range
			ar, err := clip.AvailableRange()
			if err != nil {
				return eventNumber, err
			}
			sourceRange = &ar
		}

		sourceIn := sourceRange.StartTime()
		sourceOut := sourceIn.Add(duration)
		recordIn := recordTime
		recordOut := recordTime.Add(duration)

		// Get reel name from media reference
		reelName := "AX"
		if mediaRef := clip.MediaReference(); mediaRef != nil {
			reelName = mediaRef.Name()
			if reelName == "" {
				if extRef, ok := mediaRef.(*opentimelineio.ExternalReference); ok {
					reelName = extRef.TargetURL()
				}
			}
		}
		reelName = SanitizeReelName(reelName, e.reelNameLen)

		// Determine edit type
		editType := EditTypeCut
		transitionDuration := 0

		// Check if next child is a transition
		if i+1 < len(children) {
			if transition, ok := children[i+1].(*opentimelineio.Transition); ok {
				transitionType := transition.TransitionType()
				if transitionType == "SMPTE_Dissolve" {
					editType = EditTypeDissolve
					transDur := transition.OutOffset()
					transitionDuration = int(transDur.Value())
				}
				// Skip the transition in the next iteration
				i++
			}
		}

		// Write the event
		if err := e.writeEvent(EDLEvent{
			EventNumber:        eventNumber,
			ReelName:           reelName,
			TrackType:          trackType,
			EditType:           editType,
			SourceIn:           e.formatTimecode(sourceIn),
			SourceOut:          e.formatTimecode(sourceOut),
			RecordIn:           e.formatTimecode(recordIn),
			RecordOut:          e.formatTimecode(recordOut),
			ClipName:           clip.Name(),
			TransitionDuration: transitionDuration,
		}); err != nil {
			return eventNumber, err
		}

		eventNumber++
		recordTime = recordOut
	}

	return eventNumber, nil
}

// writeEvent writes a single EDL event.
func (e *Encoder) writeEvent(event EDLEvent) error {
	// Write event line
	eventLine := fmt.Sprintf("%03d  %-8s %s    %-2s",
		event.EventNumber,
		event.ReelName,
		event.TrackType,
		event.EditType,
	)

	// Add transition duration if applicable
	if event.EditType == EditTypeDissolve && event.TransitionDuration > 0 {
		eventLine += fmt.Sprintf("   %03d", event.TransitionDuration)
	}

	_, err := fmt.Fprintf(e.w, "%s\n", eventLine)
	if err != nil {
		return err
	}

	// Write timecode line
	timecodeLine := fmt.Sprintf("     %s %s %s %s",
		event.SourceIn,
		event.SourceOut,
		event.RecordIn,
		event.RecordOut,
	)

	_, err = fmt.Fprintf(e.w, "%s\n", timecodeLine)
	if err != nil {
		return err
	}

	// Write clip name comment if present
	if event.ClipName != "" {
		_, err = fmt.Fprintf(e.w, "* FROM CLIP NAME: %s\n", event.ClipName)
		if err != nil {
			return err
		}
	}

	// Add blank line between events for readability
	_, err = fmt.Fprintf(e.w, "\n")
	return err
}

// formatTimecode formats a RationalTime as a timecode string.
func (e *Encoder) formatTimecode(t opentime.RationalTime) string {
	// Rescale to the encoder's rate
	rescaled := t.RescaledTo(e.rate)

	// Convert to timecode
	tc, err := rescaled.ToTimecode(e.rate, opentime.InferFromRate)
	if err != nil {
		// Fallback to 00:00:00:00
		return "00:00:00:00"
	}

	// EDL uses colon separator (not semicolon) for non-drop frame
	// Replace semicolon with colon if not drop frame
	if !isDropFrameRate(e.rate) {
		tc = strings.ReplaceAll(tc, ";", ":")
	}

	return tc
}

// isDropFrameRate determines if a rate uses drop frame timecode.
func isDropFrameRate(rate float64) bool {
	// 29.97 and 59.94 use drop frame
	return (rate > 29.96 && rate < 29.98) || (rate > 59.93 && rate < 59.95)
}
