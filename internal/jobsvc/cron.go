package jobsvc

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const maxNextRunLookaheadMinutes = 60 * 24 * 366 * 5

type cronField struct {
	any    bool
	values map[int]struct{}
}

func (f cronField) matches(v int) bool {
	if f.any {
		return true
	}
	_, ok := f.values[v]
	return ok
}

type cronSchedule struct {
	minute     cronField
	hour       cronField
	dayOfMonth cronField
	month      cronField
	dayOfWeek  cronField
}

func (s *cronSchedule) matches(t time.Time) bool {
	minuteMatch := s.minute.matches(t.Minute())
	hourMatch := s.hour.matches(t.Hour())
	monthMatch := s.month.matches(int(t.Month()))
	domMatch := s.dayOfMonth.matches(t.Day())
	dowMatch := s.dayOfWeek.matches(int(t.Weekday()))

	dayMatch := false
	if !s.dayOfMonth.any && !s.dayOfWeek.any {
		// Standard cron semantics: when both DOM and DOW are restricted,
		// either field matching is enough.
		dayMatch = domMatch || dowMatch
	} else {
		// If one is wildcard, the other controls matching.
		dayMatch = domMatch && dowMatch
	}

	return minuteMatch && hourMatch && monthMatch && dayMatch
}

// ValidateCronExpr validates a 5-field cron expression.
func ValidateCronExpr(expr string) error {
	_, err := parseCron(expr)
	return err
}

// NextRun computes the next run time after from, rounded to minute granularity.
func NextRun(expr string, from time.Time) (time.Time, error) {
	s, err := parseCron(expr)
	if err != nil {
		return time.Time{}, err
	}

	candidate := from.In(from.Location()).Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < maxNextRunLookaheadMinutes; i++ {
		if s.matches(candidate) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("unable to find next run for %q within lookahead window", expr)
}

func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minute, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}
	dayOfMonth, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}
	dayOfWeek, err := parseCronField(fields[4], 0, 6, true)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	return &cronSchedule{
		minute:     minute,
		hour:       hour,
		dayOfMonth: dayOfMonth,
		month:      month,
		dayOfWeek:  dayOfWeek,
	}, nil
}

func parseCronField(raw string, minVal, maxVal int, allowSevenAsSunday bool) (cronField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cronField{}, fmt.Errorf("empty field")
	}
	if raw == "*" {
		return cronField{any: true}, nil
	}

	field := cronField{values: make(map[int]struct{})}
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return cronField{}, fmt.Errorf("empty list item")
		}
		if err := addCronPart(field.values, p, minVal, maxVal, allowSevenAsSunday); err != nil {
			return cronField{}, err
		}
	}
	if len(field.values) == 0 {
		return cronField{}, fmt.Errorf("no values parsed")
	}
	return field, nil
}

func addCronPart(out map[int]struct{}, part string, minVal, maxVal int, allowSevenAsSunday bool) error {
	base := part
	step := 1

	if strings.Contains(part, "/") {
		parts := strings.Split(part, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid step expression %q", part)
		}
		base = strings.TrimSpace(parts[0])
		stepValue, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || stepValue <= 0 {
			return fmt.Errorf("invalid step value in %q", part)
		}
		step = stepValue
	}

	var start, end int
	switch {
	case base == "*":
		start, end = minVal, maxVal
	case strings.Contains(base, "-"):
		rangeParts := strings.Split(base, "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("invalid range %q", base)
		}
		var err error
		start, err = parseCronValue(strings.TrimSpace(rangeParts[0]), minVal, maxVal, allowSevenAsSunday)
		if err != nil {
			return err
		}
		end, err = parseCronValue(strings.TrimSpace(rangeParts[1]), minVal, maxVal, allowSevenAsSunday)
		if err != nil {
			return err
		}
		if end < start {
			return fmt.Errorf("range end before start in %q", base)
		}
	default:
		value, err := parseCronValue(base, minVal, maxVal, allowSevenAsSunday)
		if err != nil {
			return err
		}
		start, end = value, value
	}

	for v := start; v <= end; v += step {
		normalized := v
		if allowSevenAsSunday && normalized == 7 {
			normalized = 0
		}
		if normalized < minVal || normalized > maxVal {
			return fmt.Errorf("value %d out of range [%d,%d]", normalized, minVal, maxVal)
		}
		out[normalized] = struct{}{}
	}

	return nil
}

func parseCronValue(raw string, minVal, maxVal int, allowSevenAsSunday bool) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", raw)
	}
	if allowSevenAsSunday && value == 7 {
		value = 0
	}
	if value < minVal || value > maxVal {
		return 0, fmt.Errorf("value %d out of range [%d,%d]", value, minVal, maxVal)
	}
	return value, nil
}
