// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package timeutil

import (
	"fmt"
	"testing"
	"time"
)

func mustParseTime(t string) time.Time {
	ref, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", t)
	if err != nil {
		panic(fmt.Sprintf("parsing time %s failed: %s", t, err))
	}
	return ref
}

func TestPeriodicParse(t *testing.T) {
	tests := []struct {
		start    string
		duration string
		err      bool
	}{
		{ // Valid start time
			start:    "14:00",
			duration: "1h",
			err:      false,
		},
		{ // Start hour out of range
			start:    "25:00",
			duration: "1h",
			err:      true,
		},
		{ // Start minute out of range
			start:    "23:61",
			duration: "1h",
			err:      true,
		},
		{ // Bad time of day
			start:    "14",
			duration: "1h",
			err:      true,
		},
		{ // Bad day of week
			start:    "foo 14:00",
			duration: "1h",
			err:      true,
		},
		{ // Bad duration
			start:    "sat 14:00",
			duration: "1j",
			err:      true,
		},
		{ // Negative duration
			start:    "sat 14:00",
			duration: "-1h",
			err:      true,
		},
		{ // Too many start fields
			start:    "sat 14:00 1234",
			duration: "1h",
			err:      true,
		},
		{ // Specified just a day
			start:    "sat",
			duration: "1h",
			err:      true,
		},
	}

	for i, tt := range tests {
		_, err := ParsePeriodic(tt.start, tt.duration)
		if err != nil && tt.err == false {
			t.Errorf("#%d: unexpected error: %q", i, err)
			continue
		}
		if err == nil && tt.err == true {
			t.Errorf("#%d: expected error and did not get it", i)
			continue
		}
	}
}

func TestToStart(t *testing.T) {
	tests := []struct {
		start    string
		duration string
		time     string
		toStart  time.Duration
	}{
		{ // Daily window in 15 minutes.
			start:    "00:15",
			duration: "10s",
			time:     "Thu May 21 00:00:00 PDT 2015",
			toStart:  15 * time.Minute,
		},
		{ // Daily window now.
			start:    "00:00",
			duration: "1h",
			time:     "Thu May 21 00:00:00 PDT 2015",
			toStart:  0,
		},
		{ // Daily window started 1 second ago.
			start:    "00:00",
			duration: "1h",
			time:     "Thu May 21 00:00:01 PDT 2015",
			toStart:  -1 * time.Second,
		},
		{ // Daily window started 10 hours ago but closing edge.
			start:    "02:33",
			duration: "10h",
			time:     "Thu May 21 12:33:00 PDT 2015",
			toStart:  -10 * time.Hour,
		},
		{ // Daily window started 10 hours ago but _just_ closed, expect next.
			start:    "02:33",
			duration: "10h",
			time:     "Thu May 21 12:33:01 PDT 2015",
			toStart:  13*time.Hour + 59*time.Minute + 59*time.Second,
		},
		{ // Daily window started last night but extends into now on the next day.
			start:    "23:05",
			duration: "11h",
			time:     "Thu May 21 09:33:01 PDT 2015",
			toStart:  -(10*time.Hour + 28*time.Minute + 1*time.Second),
		},
		{ // Weekly window started in 15 minutes.
			start:    "Thu 00:15",
			duration: "10s",
			time:     "Thu May 21 00:00:00 PDT 2015",
			toStart:  15 * time.Minute,
		},
		{ // Weekly window now.
			start:    "Thu 00:00",
			duration: "1h",
			time:     "Thu May 21 00:00:00 PDT 2015",
			toStart:  0,
		},
		{ // Weekly window started 1 second ago.
			start:    "Sun 00:00",
			duration: "1h",
			time:     "Sun May 17 00:00:01 PDT 2015",
			toStart:  -1 * time.Second,
		},
		{ // Weekly window started 10 hours ago but closing edge.
			start:    "Sun 02:33",
			duration: "10h",
			time:     "Sun May 17 12:33:00 PDT 2015",
			toStart:  -10 * time.Hour,
		},
		{ // Weekly window started last night in previous week but extends into now on the next day.
			start:    "Sat 23:05",
			duration: "11h",
			time:     "Sun May 17 09:33:01 PDT 2015",
			toStart:  -(10*time.Hour + 28*time.Minute + 1*time.Second),
		},
		{ // Weekly window where next period begins next month
			start:    "Mon 9:00",
			duration: "1h",
			time:     "Sat May 30 23:00:00 PDT 2015",
			toStart:  34 * time.Hour,
		},
		{ // Weekly window where the period started on the last day of the previous month
			start:    "Sun 23:00",
			duration: "4h",
			time:     "Mon Jun 1 01:00:00 PDT 2015",
			toStart:  -(2 * time.Hour),
		},
	}

	for i, tt := range tests {
		p, err := ParsePeriodic(tt.start, tt.duration)
		if err != nil {
			t.Errorf("#%d: periodic parse failed: %v", i, err)
			continue
		}
		ref := mustParseTime(tt.time)
		if dts := p.DurationToStart(ref); dts != tt.toStart {
			t.Errorf("#%d: got %v, want %v", i, dts, tt.toStart)
		}
	}
}
