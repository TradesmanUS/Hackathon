package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	_ DateTime = datetime{}

	reDuration = regexp.MustCompile(`(?i)(\d+)([a-z]+)`)

	timeDay   = time.Hour * 24
	timeWeek  = timeDay * 7
	timeMonth = timeDay * 30
	timeYear  = timeDay * 365
)

type DateTime interface {
	Value
	AsDateTime() (time.Time, error)
}

type Duration interface {
	Value
	AsDuration() (time.Duration, error)
}

type datetime time.Time

func (v datetime) String() string                 { return fmt.Sprint(time.Time(v)) }
func (v datetime) AsDateTime() (time.Time, error) { return time.Time(v), nil }

var (
	opGetDate = operator{"getDate", func(s State) error { return s.Data().Push(datetime(time.Now())) }}
	opDTadd   = binaryOp2("dt+", AsDateTime, AsDuration, func(x time.Time, y time.Duration) Value { return datetime(x.Add(y)) })
	opDTsub   = binaryOp2("dt-", AsDateTime, AsDateTime, func(x time.Time, y time.Time) Value { return Number[time.Duration]{x.Sub(y)} })
	opDTgt    = binaryOp("dt>", AsDateTime, func(x, y time.Time) Value { return boolean(x.After(y)) })
	opDTge    = binaryOp("dt≥", AsDateTime, func(x, y time.Time) Value { return boolean(!x.Before(y)) })
	opDTlt    = binaryOp("dt<", AsDateTime, func(x, y time.Time) Value { return boolean(x.Before(y)) })
	opDTle    = binaryOp("dt≤", AsDateTime, func(x, y time.Time) Value { return boolean(!x.After(y)) })
	opDTeq    = binaryOp("dt=", AsDateTime, func(x, y time.Time) Value { return boolean(x.Equal(y)) })

	opNewDate = unaryOpErr("newDate", AsString, func(x string) (Value, error) {
		t, err := parseDateTime(x)
		return datetime(t), err
	})
)

func resolveDateTimeOperator(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "newdate":
		return opNewDate, true
	case "getdate":
		return opGetDate, true
	case "plusdate":
		return opDTadd, true
	case "minusdate":
		return opDTsub, true
	case "dategt":
		return opDTgt, true
	case "datege":
		return opDTge, true
	case "datelt":
		return opDTlt, true
	case "datele":
		return opDTle, true
	case "dateeq":
		return opDTeq, true
	}
	return nil, false
}

func parseDateTime(s string) (time.Time, error) {
	return time.Parse(time.DateOnly, s)
}

func formatDuration(v time.Duration) string {
	s := new(strings.Builder)

	if u := v / timeYear; u > 0 {
		fmt.Fprintf(s, "%dy", u)
		v -= u * timeYear
	}

	if u := v / timeDay; u > 0 {
		fmt.Fprintf(s, "%dd", u)
		v -= u * timeDay
	}

	if v == 0 {
		return s.String()
	}

	fmt.Fprint(s, v)
	return s.String()
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.ToLower(s)
	i := reDuration.FindAllStringSubmatchIndex(s, -1)
	if i == nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}

	var v time.Duration
	var pos int
	for _, i := range i {
		if !isSpace(s[pos:i[0]]) {
			return 0, fmt.Errorf("invalid duration component %q", s[pos:i[0]])
		}
		pos = i[1]

		n, err := strconv.ParseInt(s[i[2]:i[3]], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration component %q: %v", s[i[2]:i[3]], err)
		}

		switch s[i[4]:i[5]] {
		case "ns", "nanosecond", "nanoseconds":
			v += time.Duration(n) * time.Nanosecond
		case "us", "microsecond", "microseconds",
			"µs", // U+00B5 = micro symbol
			"μs": // U+03BC = Greek letter mu
			v += time.Duration(n) * time.Microsecond
		case "ms", "millisecond", "milliseconds":
			v += time.Duration(n) * time.Millisecond
		case "s", "sec", "second", "seconds":
			v += time.Duration(n) * time.Second
		case "m", "min", "minute", "minutes":
			v += time.Duration(n) * time.Minute
		case "h", "hr", "hour", "hours":
			v += time.Duration(n) * time.Hour
		case "d", "day", "days":
			v += time.Duration(n) * timeDay
		case "w", "wk", "week", "weeks":
			v += time.Duration(n) * timeWeek
		case "mo", "month", "months":
			v += time.Duration(n) * timeMonth
		case "y", "year", "years":
			v += time.Duration(n) * timeYear
		default:
			return 0, fmt.Errorf("invalid duration component %q", s[i[4]:i[5]])
		}
	}
	if !isSpace(s[pos:]) {
		return 0, fmt.Errorf("invalid duration component %q", s[pos:])
	}

	return v, nil
}

func isSpace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
