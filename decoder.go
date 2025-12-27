// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package cmx3600

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio/opentimelineio"
)

// Decoder reads CMX 3600 EDL format and produces an OpenTimelineIO Timeline.
type Decoder struct {
	r                      io.Reader
	rate                   float64
	ignoreTimecodeMismatch bool
	fcmMode                string // "DROP FRAME" or "NON-DROP FRAME"
}

// NewDecoder creates a new EDL decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:    r,
		rate: 24.0, // Default frame rate
	}
}

// SetRate sets the frame rate for timecode interpretation.
func (d *Decoder) SetRate(rate float64) {
	d.rate = rate
}

// SetIgnoreTimecodeMismatch sets whether to ignore timecode mismatches.
// When true, the decoder will infer correct record timecode from source timecode
// and adjacent cuts, which helps handle common EDL errors.
func (d *Decoder) SetIgnoreTimecodeMismatch(ignore bool) {
	d.ignoreTimecodeMismatch = ignore
}

// eventLineRegex matches an EDL event line.
// Format: EVENT# REEL TRACK EDIT_TYPE [TRANSITION_DURATION]
var eventLineRegex = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(V|A\d?|AA)\s+(C|D|W\d{3}|KB|K)\s*(\d+)?`)

// timecodeLineRegex matches a timecode line.
// Format: SOURCE_IN SOURCE_OUT RECORD_IN RECORD_OUT
var timecodeLineRegex = regexp.MustCompile(`^\s*(\d{2}:\d{2}:\d{2}[;:]\d{2})\s+(\d{2}:\d{2}:\d{2}[;:]\d{2})\s+(\d{2}:\d{2}:\d{2}[;:]\d{2})\s+(\d{2}:\d{2}:\d{2}[;:]\d{2})`)

// speedEffectRegex matches an M2 motion effect line.
// Format: M2 REEL SPEED TIMECODE
var speedEffectRegex = regexp.MustCompile(`^M2\s+(?P<name>\S+)\s+(?P<speed>-?[0-9.]+)\s+(?P<tc>\d{2}:\d{2}:\d{2}:\d{2})`)

// markerRegex matches a locator/marker line.
// Format: * LOC: TIMECODE COLOR COMMENT
var markerRegex = regexp.MustCompile(`^\*\s*LOC:\s+(\d{2}:\d{2}:\d{2}:\d{2})\s+(\w*)(\s+|$)(.*)`)

// ascSOPRegex matches ASC_SOP (slope, offset, power) values.
var ascSOPRegex = regexp.MustCompile(`ASC_SOP\s*\(\s*([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)\s*\)\s*\(\s*([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)\s*\)\s*\(\s*([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)[,\s]+([-+]?[\d.]+)\s*\)`)

// ascSATRegex matches ASC_SAT (saturation) value.
var ascSATRegex = regexp.MustCompile(`ASC_SAT\s+([-+]?[\d.]+)`)

// Decode reads the EDL and returns an OpenTimelineIO Timeline.
func (d *Decoder) Decode() (*opentimelineio.Timeline, error) {
	events, err := d.parseEvents()
	if err != nil {
		return nil, err
	}

	return d.eventsToTimeline(events)
}

// parseEvents reads all events from the EDL.
func (d *Decoder) parseEvents() ([]EDLEvent, error) {
	scanner := bufio.NewScanner(d.r)
	var events []EDLEvent
	var currentEvent *EDLEvent
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip blank lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for title line
		if strings.HasPrefix(strings.TrimSpace(line), "TITLE:") {
			continue
		}

		// Check for FCM (frame count mode) line
		if strings.HasPrefix(strings.TrimSpace(line), "FCM:") {
			// Parse FCM mode (DROP FRAME or NON-DROP FRAME)
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			if len(parts) == 2 {
				d.fcmMode = strings.TrimSpace(parts[1])
			}
			continue
		}

		// Try to match event line
		if matches := eventLineRegex.FindStringSubmatch(line); matches != nil {
			// Save previous event if exists
			if currentEvent != nil {
				events = append(events, *currentEvent)
			}

			// Parse event number
			eventNum, _ := strconv.Atoi(matches[1])

			// Parse transition duration if present
			transitionDuration := 0
			if matches[5] != "" {
				transitionDuration, _ = strconv.Atoi(matches[5])
			}

			// Extract edit type and wipe code
			editTypeStr := matches[4]
			editType := EditType(editTypeStr)
			wipeCode := ""
			if len(editTypeStr) == 4 && editTypeStr[0] == 'W' {
				// This is a wipe code (W###)
				editType = EditTypeWipe
				wipeCode = editTypeStr
			}

			currentEvent = &EDLEvent{
				EventNumber:        eventNum,
				ReelName:           matches[2],
				TrackType:          TrackType(matches[3]),
				EditType:           editType,
				TransitionDuration: transitionDuration,
				WipeCode:           wipeCode,
			}

			// The next line should be timecodes
			if scanner.Scan() {
				lineNum++
				tcLine := scanner.Text()
				if tcMatches := timecodeLineRegex.FindStringSubmatch(tcLine); tcMatches != nil {
					currentEvent.SourceIn = tcMatches[1]
					currentEvent.SourceOut = tcMatches[2]
					currentEvent.RecordIn = tcMatches[3]
					currentEvent.RecordOut = tcMatches[4]
				} else {
					return nil, &ParseError{
						Line:    lineNum,
						Message: "expected timecode line after event",
					}
				}
			}
			continue
		}

		// Check for M2 speed effect lines
		if strings.HasPrefix(strings.TrimSpace(line), "M2") {
			if currentEvent != nil && speedEffectRegex.MatchString(line) {
				matches := speedEffectRegex.FindStringSubmatch(line)
				if len(matches) == 4 {
					speed, _ := strconv.ParseFloat(matches[2], 64)
					currentEvent.SpeedEffect = &SpeedEffect{
						Name:     matches[1],
						Speed:    speed,
						Timecode: matches[3],
					}
				}
			}
			continue
		}

		// Check for comment lines
		if currentEvent != nil {
			trimmed := strings.TrimSpace(line)

			// FROM CLIP NAME: indicates the clip name
			// Handle both "*FROM CLIP NAME:" and "* FROM CLIP NAME:"
			if strings.HasPrefix(trimmed, "*FROM CLIP NAME:") {
				currentEvent.ClipName = strings.TrimSpace(strings.TrimPrefix(trimmed, "*FROM CLIP NAME:"))
			} else if strings.HasPrefix(trimmed, "* FROM CLIP NAME:") {
				currentEvent.ClipName = strings.TrimSpace(strings.TrimPrefix(trimmed, "* FROM CLIP NAME:"))
			} else if strings.HasPrefix(trimmed, "*FROM CLIP:") {
				// FROM CLIP: for Avid style - file path
				currentEvent.FilePath = strings.TrimSpace(strings.TrimPrefix(trimmed, "*FROM CLIP:"))
			} else if strings.HasPrefix(trimmed, "* FROM CLIP:") {
				currentEvent.FilePath = strings.TrimSpace(strings.TrimPrefix(trimmed, "* FROM CLIP:"))
			} else if strings.HasPrefix(trimmed, "*FROM FILE:") {
				// FROM FILE: for Nucoda style - file path
				currentEvent.FilePath = strings.TrimSpace(strings.TrimPrefix(trimmed, "*FROM FILE:"))
			} else if strings.HasPrefix(trimmed, "* FROM FILE:") {
				currentEvent.FilePath = strings.TrimSpace(strings.TrimPrefix(trimmed, "* FROM FILE:"))
			} else if strings.HasPrefix(trimmed, "* FREEZE FRAME") || strings.HasSuffix(trimmed, " FF") {
				// Freeze frame detection
				currentEvent.FreezeFrame = true
			} else if markerRegex.MatchString(trimmed) {
				// Locator/marker
				matches := markerRegex.FindStringSubmatch(trimmed)
				if len(matches) == 5 {
					marker := Marker{
						Timecode: matches[1],
						Color:    matches[2],
						Comment:  strings.TrimSpace(matches[4]),
					}
					currentEvent.Markers = append(currentEvent.Markers, marker)
				}
			} else if ascSOPRegex.MatchString(trimmed) {
				// ASC_SOP color correction
				matches := ascSOPRegex.FindStringSubmatch(trimmed)
				if len(matches) == 10 {
					if currentEvent.ASCCDL == nil {
						currentEvent.ASCCDL = &ASCCDL{}
					}
					for i := 0; i < 3; i++ {
						currentEvent.ASCCDL.Slope[i], _ = strconv.ParseFloat(matches[1+i], 64)
						currentEvent.ASCCDL.Offset[i], _ = strconv.ParseFloat(matches[4+i], 64)
						currentEvent.ASCCDL.Power[i], _ = strconv.ParseFloat(matches[7+i], 64)
					}
				}
			} else if ascSATRegex.MatchString(trimmed) {
				// ASC_SAT saturation
				matches := ascSATRegex.FindStringSubmatch(trimmed)
				if len(matches) == 2 {
					if currentEvent.ASCCDL == nil {
						currentEvent.ASCCDL = &ASCCDL{}
					}
					currentEvent.ASCCDL.Saturation, _ = strconv.ParseFloat(matches[1], 64)
				}
			} else if strings.HasPrefix(trimmed, "*") {
				// Other comments
				if currentEvent.Comment != "" {
					currentEvent.Comment += "\n"
				}
				currentEvent.Comment += trimmed
			}
		}
	}

	// Save last event
	if currentEvent != nil {
		events = append(events, *currentEvent)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// eventsToTimeline converts parsed events to an OpenTimelineIO Timeline.
func (d *Decoder) eventsToTimeline(events []EDLEvent) (*opentimelineio.Timeline, error) {
	timeline := opentimelineio.NewTimeline("", nil, nil)
	tracks := timeline.Tracks()

	// Group events by track type
	trackMap := make(map[TrackType][]EDLEvent)
	for _, event := range events {
		trackMap[event.TrackType] = append(trackMap[event.TrackType], event)
	}

	// Create tracks
	for trackType, trackEvents := range trackMap {
		track, err := d.createTrack(trackType, trackEvents)
		if err != nil {
			return nil, err
		}
		if err := tracks.AppendChild(track); err != nil {
			return nil, err
		}
	}

	return timeline, nil
}

// createTrack creates a track from a list of events.
func (d *Decoder) createTrack(trackType TrackType, events []EDLEvent) (*opentimelineio.Track, error) {
	kind := opentimelineio.TrackKindVideo
	if trackType.IsAudioTrack() {
		kind = opentimelineio.TrackKindAudio
	}

	track := opentimelineio.NewTrack(string(trackType), nil, kind, nil, nil)

	// Sort events by event number (should already be sorted)
	// For now, assume they are in order

	var lastRecordOut opentime.RationalTime

	for _, event := range events {
		// Parse timecodes
		sourceIn, err := opentime.FromTimecode(event.SourceIn, d.rate)
		if err != nil {
			return nil, fmt.Errorf("invalid source in timecode '%s': %w", event.SourceIn, err)
		}

		sourceOut, err := opentime.FromTimecode(event.SourceOut, d.rate)
		if err != nil {
			return nil, fmt.Errorf("invalid source out timecode '%s': %w", event.SourceOut, err)
		}

		recordIn, err := opentime.FromTimecode(event.RecordIn, d.rate)
		if err != nil {
			return nil, fmt.Errorf("invalid record in timecode '%s': %w", event.RecordIn, err)
		}

		recordOut, err := opentime.FromTimecode(event.RecordOut, d.rate)
		if err != nil {
			return nil, fmt.Errorf("invalid record out timecode '%s': %w", event.RecordOut, err)
		}

		// Check for gaps in the timeline
		if lastRecordOut.IsValidTime() {
			gap := recordIn.Sub(lastRecordOut)
			if gap.Value() > 0.5 { // Allow for rounding errors
				// Insert a gap
				gapDuration := gap
				gapItem := opentimelineio.NewGapWithDuration(gapDuration)
				if err := track.AppendChild(gapItem); err != nil {
					return nil, err
				}
			}
		}

		// Source range
		sourceDuration := opentime.DurationFromStartEndTime(sourceIn, sourceOut)
		sourceRange := opentime.NewTimeRange(sourceIn, sourceDuration)

		// Create media reference based on reel name
		var mediaRef opentimelineio.MediaReference

		// Check for generator references (BLACK, BL, BARS)
		reelUpper := strings.ToUpper(event.ReelName)
		if reelUpper == "BLACK" || reelUpper == "BL" {
			genRef := opentimelineio.NewGeneratorReference(
				"black",
				"black",
				nil,
				&sourceRange,
				nil,
			)
			mediaRef = genRef
		} else if reelUpper == "BARS" {
			genRef := opentimelineio.NewGeneratorReference(
				"SMPTEBars",
				"SMPTEBars",
				nil,
				&sourceRange,
				nil,
			)
			mediaRef = genRef
		} else {
			// Use file path from comment if available, otherwise use reel name
			targetURL := event.ReelName
			if event.FilePath != "" {
				targetURL = event.FilePath
			}
			mediaRef = opentimelineio.NewExternalReference(
				targetURL,
				targetURL,
				&sourceRange,
				nil,
			)
		}

		// Use clip name from comment if available, otherwise use reel name
		clipName := event.ClipName
		if clipName == "" {
			clipName = event.ReelName
		}

		// Strip " FF" suffix if freeze frame detected
		if event.FreezeFrame && strings.HasSuffix(clipName, " FF") {
			clipName = clipName[:len(clipName)-3]
		}

		// Create metadata for CDL and other info
		metadata := make(map[string]interface{})
		if event.ASCCDL != nil {
			metadata["cdl"] = map[string]interface{}{
				"slope":      event.ASCCDL.Slope,
				"offset":     event.ASCCDL.Offset,
				"power":      event.ASCCDL.Power,
				"saturation": event.ASCCDL.Saturation,
			}
		}
		if event.WipeCode != "" {
			metadata["wipe_code"] = event.WipeCode
		}

		// Build effects list
		var effects []opentimelineio.Effect

		// Add speed effects
		if event.SpeedEffect != nil {
			// Create LinearTimeWarp effect
			timeScalar := event.SpeedEffect.Speed / d.rate
			effect := opentimelineio.NewLinearTimeWarp(
				"",
				"LinearTimeWarp",
				timeScalar,
				nil,
			)
			effects = append(effects, effect)
		}

		// Add freeze frame effect
		if event.FreezeFrame {
			effect := opentimelineio.NewFreezeFrame("", nil)
			effects = append(effects, effect)
		}

		// Build markers list
		var markers []*opentimelineio.Marker
		for _, marker := range event.Markers {
			markerTC, err := opentime.FromTimecode(marker.Timecode, d.rate)
			if err != nil {
				continue // Skip invalid marker timecodes
			}
			markerRange := opentime.NewTimeRange(markerTC, opentime.NewRationalTime(0, d.rate))

			markerMeta := make(map[string]interface{})
			if marker.Color != "" {
				markerMeta["color"] = marker.Color
			}

			// Convert color string to MarkerColor
			markerColor := opentimelineio.MarkerColor(marker.Color)

			otioMarker := opentimelineio.NewMarker(
				marker.Comment,
				markerRange,
				markerColor,
				marker.Comment,
				markerMeta,
			)
			markers = append(markers, otioMarker)
		}

		// Create clip
		clip := opentimelineio.NewClip(
			clipName,
			mediaRef,
			&sourceRange,
			metadata,
			effects,
			markers,
			"",
			nil,
		)

		// Handle transitions
		if (event.EditType == EditTypeDissolve || event.EditType == EditTypeWipe) && event.TransitionDuration > 0 {
			// Create a transition
			transitionDuration := opentime.NewRationalTime(float64(event.TransitionDuration), d.rate)
			transitionType := opentimelineio.TransitionTypeSMPTEDissolve
			transitionName := ""
			if event.EditType == EditTypeWipe {
				// For wipes, use custom transition type and include wipe code in name
				transitionType = opentimelineio.TransitionTypeCustom
				if event.WipeCode != "" {
					transitionName = event.WipeCode
				} else {
					transitionName = "SMPTE_Wipe"
				}
			}
			transition := opentimelineio.NewTransition(
				transitionName,
				transitionType,
				opentime.NewRationalTime(0, d.rate),
				transitionDuration,
				nil,
			)
			if err := track.AppendChild(transition); err != nil {
				return nil, err
			}
		}

		if err := track.AppendChild(clip); err != nil {
			return nil, err
		}

		lastRecordOut = recordOut
	}

	return track, nil
}
