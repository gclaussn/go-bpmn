package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var iso8601DurationDateRegexp = regexp.MustCompile(`^P(\d+Y)?(\d+M)?(\d+W)?(\d+D)?$`)
var iso8601DurationTimeRegexp = regexp.MustCompile(`^(\d+H)?(\d+M)?(\d+S)?$`)

func NewISO8601Duration(v string) (ISO8601Duration, error) {
	if v == "" {
		return ISO8601Duration(v), nil
	}

	s := strings.Split(v, "T")

	var valid bool
	if len(s) == 1 && len(s[0]) >= 3 { // e.g. P1D
		valid = iso8601DurationDateRegexp.MatchString(s[0])
	} else if len(s) == 2 {
		if len(s[0]) == 1 && s[0][0] == 'P' { // e.g. PT1S -> P
			valid = true
		} else { // e.g. P1DT1S -> P1D
			valid = iso8601DurationDateRegexp.MatchString(s[0])
		}

		if len(s[1]) >= 2 { // e.g. PT1S -> 1S
			valid = valid && iso8601DurationTimeRegexp.MatchString(s[1])
		} else {
			valid = false
		}
	}

	if !valid {
		return "", fmt.Errorf("failed to parse ISO 8601 duration %s", v)
	}

	return ISO8601Duration(v), nil
}

// ISO8601Duration is a duration in ISO 8601 format.
// The zero value has a duration of 0 seconds.
//
// see https://en.wikipedia.org/wiki/ISO_8601#Durations
type ISO8601Duration string

func (d ISO8601Duration) Calculate(t time.Time) time.Time {
	if d.IsZero() {
		return t
	}

	v := string(d)

	var isTime bool
	var l, n int
	for i, r := range v {
		if unicode.IsLetter(r) && l != 0 {
			n, _ = strconv.Atoi(v[i-l : i])
			l = 0
		} else if unicode.IsDigit(r) {
			l++
		}

		switch r {
		case 'Y':
			t = t.AddDate(n, 0, 0)
		case 'M':
			if isTime {
				t = t.Add(time.Duration(n) * time.Minute)
			} else {
				t = t.AddDate(0, n, 0)
			}
		case 'W':
			t = t.AddDate(0, 0, n*7)
		case 'D':
			t = t.AddDate(0, 0, n)
		case 'T':
			isTime = true
		case 'H':
			t = t.Add(time.Duration(n) * time.Hour)
		case 'S':
			t = t.Add(time.Duration(n) * time.Second)
		}
	}

	return t
}

func (d ISO8601Duration) IsZero() bool {
	return d == ""
}

func (d ISO8601Duration) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", d.String())), nil
}

func (d ISO8601Duration) String() string {
	return string(d)
}

func (d *ISO8601Duration) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) < 2 {
		return fmt.Errorf("invalid ISO 8601 duration data %s", s)
	}

	// do not use NewISO8601Duration, since validation is done in a separate step - see http/server/request.go
	*d = ISO8601Duration(s[1 : len(s)-1])
	return nil
}
