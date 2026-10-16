package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/helper/sqlident"
	"muara.bi/core/pkg/customerror"
)

// ParseRLSRuleSpecs decodes the project_members.rls_rules JSONB payload. An
// empty or absent payload means "no row restriction", which is valid.
func ParseRLSRuleSpecs(raw json.RawMessage) ([]entity.RLSRuleSpec, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var specs []entity.RLSRuleSpec
	if err := json.Unmarshal([]byte(trimmed), &specs); err != nil {
		return nil, customerror.NewBadRequestError("stored rls rules are malformed", err)
	}
	return specs, nil
}

// ResolveRLSRules binds stored rule specs against the caller's user attributes.
//
// A spec may pin a literal Value (set by an administrator) or name an attribute
// key through ValueFromUser. When the referenced attribute is missing or not a
// scalar the resolution fails closed with 403: silently dropping the rule would
// widen the result set to every row the user should not see.
func ResolveRLSRules(specs []entity.RLSRuleSpec, attributes map[string]any) ([]duckdb.RLSRule, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	rules := make([]duckdb.RLSRule, 0, len(specs))
	for _, spec := range specs {
		column := strings.TrimSpace(spec.Column)
		if !sqlident.IsQualified(column) {
			return nil, customerror.NewBadRequestError(
				fmt.Sprintf("rls rule column %q is not a valid identifier", spec.Column), nil)
		}

		value, err := resolveRLSValue(spec, attributes)
		if err != nil {
			return nil, err
		}

		rules = append(rules, duckdb.RLSRule{
			Column:   column,
			Operator: strings.TrimSpace(spec.Operator),
			Value:    value,
		})
	}
	return rules, nil
}

func resolveRLSValue(spec entity.RLSRuleSpec, attributes map[string]any) (any, error) {
	attributeKey := strings.TrimSpace(spec.ValueFromUser)
	if attributeKey == "" {
		if spec.Value == nil {
			return nil, customerror.NewBadRequestError(
				fmt.Sprintf("rls rule for column %q has neither value nor value_from_user", spec.Column), nil)
		}
		return spec.Value, nil
	}

	raw, ok := attributes[attributeKey]
	if !ok || raw == nil {
		return nil, customerror.NewForbiddenError(
			fmt.Sprintf("user attribute %q required by row-level security is not set", attributeKey))
	}
	if !isScalar(raw) {
		return nil, customerror.NewForbiddenError(
			fmt.Sprintf("user attribute %q must be a scalar value to satisfy row-level security", attributeKey))
	}
	return raw, nil
}

func isScalar(v any) bool {
	switch v.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64,
		json.Number:
		return true
	default:
		return false
	}
}
