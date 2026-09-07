// Package bounds contains small, reusable value-bound helpers.
package bounds

func String(value string, limit int) string {
	if limit < 1 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
