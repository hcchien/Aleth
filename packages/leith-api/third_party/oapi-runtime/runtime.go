package runtime

import (
	"fmt"
	"net/url"
	"strconv"
)

type BindQueryParameterOptions struct {
	Type   string
	Format string
}

// BindQueryParameterWithOptions supports the subset required by this project:
// optional integer query parameters bound to *int destinations.
func BindQueryParameterWithOptions(_ string, _ bool, required bool, paramName string, query url.Values, dest interface{}, _ BindQueryParameterOptions) error {
	values, ok := query[paramName]
	if !ok || len(values) == 0 || values[0] == "" {
		if required {
			return fmt.Errorf("missing query parameter: %s", paramName)
		}
		return nil
	}

	switch p := dest.(type) {
	case **int:
		v, err := strconv.Atoi(values[0])
		if err != nil {
			return fmt.Errorf("invalid integer for %s: %w", paramName, err)
		}
		*p = &v
		return nil
	default:
		return fmt.Errorf("unsupported destination type for %s", paramName)
	}
}
