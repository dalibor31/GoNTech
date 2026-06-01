package handler

import (
	"fmt"
	"strconv"
)

// parseID parsira string ID iz URL parametra u int64
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ntech: parseID: neispravan ID %q: %w", s, err)
	}
	return id, nil
}
