package model

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type FlexTime struct {
	time.Time
}

func (ft *FlexTime) Scan(value interface{}) error {
	if value == nil {
		ft.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		ft.Time = v
		return nil
	case []byte:
		return ft.parseString(string(v))
	case string:
		return ft.parseString(v)
	default:
		return fmt.Errorf("FlexTime.Scan: unsupported type %T", value)
	}
}

func (ft *FlexTime) parseString(s string) error {
	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}

	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			ft.Time = t
			return nil
		}
	}
	return fmt.Errorf("FlexTime.Scan: cannot parse %q as time", s)
}

func (ft FlexTime) Value() (driver.Value, error) {
	if ft.Time.IsZero() {
		return nil, nil
	}
	return ft.Time.Format("2006-01-02 15:04:05"), nil
}
