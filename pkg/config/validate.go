package config

import (
	"errors"
	"fmt"
)

// validate checks that all registrar configs in cfg have the required fields set.
// Returns a joined error containing one message per missing field, in the form:
//
//	registrars.internetx.host: required
//
// The error is returned before any network connection is attempted, satisfying CFG-03.
func validate(cfg *Config) error {
	var errs []error
	for name, r := range cfg.Registrars {
		prefix := fmt.Sprintf("registrars.%s", name)
		if r.Host == "" {
			errs = append(errs, fmt.Errorf("%s.host: required", prefix))
		}
		if r.Port == 0 {
			errs = append(errs, fmt.Errorf("%s.port: required", prefix))
		}
		if r.Username == "" {
			errs = append(errs, fmt.Errorf("%s.username: required", prefix))
		}
		if r.Password == "" {
			errs = append(errs, fmt.Errorf("%s.password: required", prefix))
		}
	}
	// errors.Join requires Go 1.20+. go.mod declares go 1.25 so this is available.
	return errors.Join(errs...)
}
