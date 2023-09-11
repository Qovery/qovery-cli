package utils

import "time"

func ToIso8601(v *time.Time) *string {
	if v == nil {
		return nil
	}

	x := v.Format("2006-01-02T15:04:05.000Z")
	return &x
}
