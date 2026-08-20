package runner

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
)

// decodeParams decodes a flat map[string]any into dst (a typed request struct).
//
// WeaklyTypedInput is mandatory (Pitfall 1): YAML scenario authors write
// params: {period: "1"} (string) or params: {period: 1} (integer). Without
// WeaklyTypedInput, mapstructure returns an error on the string form.
//
// TagName "mapstructure" tells the decoder to use `mapstructure:"field_name"`
// struct tags (already present on all registrar request structs) for field
// matching, enabling snake_case YAML keys (e.g., auth_info) to map to
// Go exported fields (e.g., AuthInfo).
func decodeParams(params map[string]any, dst any) error {
	cfg := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           dst,
		TagName:          "mapstructure",
	}
	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return fmt.Errorf("params decoder init: %w", err)
	}
	if err := dec.Decode(params); err != nil {
		return fmt.Errorf("decode params: %w", err)
	}
	return nil
}
