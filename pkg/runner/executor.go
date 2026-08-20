package runner

import (
	"context"
	"fmt"

	"github.com/riftcell/epp-test/pkg/registrar"
)

// opTable maps snake_case op names (as written in YAML scenario files) to
// executor functions. Each executor decodes the step params into the typed
// request struct and calls the matching Registrar method.
//
// wait_for_poll is dispatched specially in runner.go and is NOT in this table
// to keep the timing/polling logic self-contained in the runner.
var opTable = map[string]func(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error){
	"check_domain":    execCheckDomain,
	"create_domain":   execCreateDomain,
	"info_domain":     execInfoDomain,
	"update_domain":   execUpdateDomain,
	"delete_domain":   execDeleteDomain,
	"renew_domain":    execRenewDomain,
	"transfer_domain": execTransferDomain,
	"check_contact":   execCheckContact,
	"create_contact":  execCreateContact,
	"info_contact":    execInfoContact,
	"update_contact":  execUpdateContact,
	"delete_contact":  execDeleteContact,
	"check_host":      execCheckHost,
	"create_host":     execCreateHost,
	"info_host":       execInfoHost,
	"update_host":     execUpdateHost,
	"delete_host":     execDeleteHost,
	"poll":            execPollRead,
	"poll_ack":        execPollAck,
	"login":           execLogin,
	"logout":          execLogout,
	"ping":            execPing,
}

// anyStrings converts a []any (from YAML) to []string.
// Elements that are already strings are passed through; non-strings are
// formatted with %v to avoid silent data loss.
func anyStrings(v []any) []string {
	out := make([]string, len(v))
	for i, elem := range v {
		if s, ok := elem.(string); ok {
			out[i] = s
		} else {
			out[i] = fmt.Sprintf("%v", elem)
		}
	}
	return out
}

// stringParam reads a string param by key. Returns "" if missing or not a string.
func stringParam(params map[string]any, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string) //nolint:errcheck // type assertion ok-idiom; non-string returns ""
	return s
}

// intParam reads an int param by key using weak coercion (same as decodeParams).
func intParam(params map[string]any, key string) int {
	v, ok := params[key]
	if !ok {
		return 0
	}
	switch vt := v.(type) {
	case int:
		return vt
	case int64:
		return int(vt)
	case float64:
		return int(vt)
	case string:
		var n int
		_, _ = fmt.Sscan(vt, &n) //nolint:errcheck // Sscan parse failure leaves n=0, which is the correct fallback
		return n
	}
	return 0
}

// ---- Domain operations ----

func execCheckDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var names []string
	if v, ok := params["names"]; ok {
		switch vt := v.(type) {
		case []any:
			names = anyStrings(vt)
		case []string:
			names = vt
		case string:
			names = []string{vt}
		}
	}
	return reg.CheckDomain(ctx, names...) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execCreateDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.DomainCreateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.CreateDomain(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execInfoDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return reg.InfoDomain(ctx, stringParam(params, "name")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execUpdateDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.DomainUpdateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.UpdateDomain(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execDeleteDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return nil, reg.DeleteDomain(ctx, stringParam(params, "name")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execRenewDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	name := stringParam(params, "name")
	years := intParam(params, "years")
	return reg.RenewDomain(ctx, name, years) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execTransferDomain(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.DomainTransferRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.TransferDomain(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

// ---- Contact operations ----

func execCheckContact(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var ids []string
	if v, ok := params["ids"]; ok {
		switch vt := v.(type) {
		case []any:
			ids = anyStrings(vt)
		case []string:
			ids = vt
		case string:
			ids = []string{vt}
		}
	}
	return reg.CheckContact(ctx, ids...) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execCreateContact(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.ContactCreateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.CreateContact(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execInfoContact(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return reg.InfoContact(ctx, stringParam(params, "id")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execUpdateContact(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.ContactUpdateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.UpdateContact(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execDeleteContact(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return nil, reg.DeleteContact(ctx, stringParam(params, "id")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

// ---- Host operations ----

func execCheckHost(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var names []string
	if v, ok := params["names"]; ok {
		switch vt := v.(type) {
		case []any:
			names = anyStrings(vt)
		case []string:
			names = vt
		case string:
			names = []string{vt}
		}
	}
	return reg.CheckHost(ctx, names...) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execCreateHost(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.HostCreateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.CreateHost(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execInfoHost(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return reg.InfoHost(ctx, stringParam(params, "name")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execUpdateHost(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	var req registrar.HostUpdateRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	return reg.UpdateHost(ctx, req) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execDeleteHost(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	return nil, reg.DeleteHost(ctx, stringParam(params, "name")) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

// ---- Poll operations ----

func execPollRead(ctx context.Context, reg registrar.Registrar, _ map[string]any) (any, error) {
	return reg.PollRead(ctx) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execPollAck(ctx context.Context, reg registrar.Registrar, params map[string]any) (any, error) {
	// Accept either "id" or "msg_id" as the message ID param key.
	msgID := stringParam(params, "id")
	if msgID == "" {
		msgID = stringParam(params, "msg_id")
	}
	return nil, reg.PollAck(ctx, msgID) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

// ---- Session operations ----

func execLogin(ctx context.Context, reg registrar.Registrar, _ map[string]any) (any, error) {
	return nil, reg.Login(ctx) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execLogout(ctx context.Context, reg registrar.Registrar, _ map[string]any) (any, error) {
	return nil, reg.Logout(ctx) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}

func execPing(ctx context.Context, reg registrar.Registrar, _ map[string]any) (any, error) {
	return nil, reg.Ping(ctx) //nolint:wrapcheck // op name supplied by YAML step; caller has full context
}
