package t0reference

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type RuleDefinition struct {
	RuleKind          string
	Name              string
	DefinitionVersion string
	ConditionJSON     string
	EntryJSON         string
	ManualRank        int
	MinSamples        int
	DeepRed           bool
	RedEntry          bool
}

type CompiledRule struct {
	Definition RuleDefinition
	RuleKey    string
	condition  any
	entry      any
}

type RuleHit struct {
	RuleKey       string
	Name          string
	ManualRank    int
	DeepRed       bool
	ResearchTier  string
	StrictWinRate float64
	SampleCount   int
	RedEntry      bool
}

func CompileRule(definition RuleDefinition) (CompiledRule, error) {
	if strings.TrimSpace(definition.DefinitionVersion) == "" {
		definition.DefinitionVersion = "v1"
	}
	condition, err := decodeRuleJSON(definition.ConditionJSON)
	if err != nil {
		return CompiledRule{}, fmt.Errorf("condition_json: %w", err)
	}
	if err := validateCondition(condition); err != nil {
		return CompiledRule{}, fmt.Errorf("condition_json: %w", err)
	}
	entry, err := decodeRuleJSON(definition.EntryJSON)
	if err != nil {
		return CompiledRule{}, fmt.Errorf("entry_json: %w", err)
	}
	if err := validateEntry(entry); err != nil {
		return CompiledRule{}, fmt.Errorf("entry_json: %w", err)
	}
	canonical, err := canonicalRuleJSON(condition, entry, definition.DefinitionVersion)
	if err != nil {
		return CompiledRule{}, err
	}
	hash := sha256.Sum256([]byte(canonical))
	definitionKey := "v1:" + hex.EncodeToString(hash[:])
	return CompiledRule{
		Definition: definition,
		RuleKey:    definitionKey,
		condition:  condition,
		entry:      entry,
	}, nil
}

func MatchReferenceRules(view PreT0View, entry EntryView, rules []CompiledRule) []RuleHit {
	hits := make([]RuleHit, 0, len(rules))
	for _, rule := range rules {
		if !evaluateCondition(rule.condition, view) || !evaluateEntry(rule.entry, entry) {
			continue
		}
		hits = append(hits, RuleHit{
			RuleKey:      rule.RuleKey,
			Name:         rule.Definition.Name,
			ManualRank:   rule.Definition.ManualRank,
			DeepRed:      rule.Definition.DeepRed,
			RedEntry:     rule.Definition.RedEntry,
			ResearchTier: "normal",
		})
	}
	return hits
}

func decodeRuleJSON(raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("must not be empty")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("must contain one JSON value")
	}
	return value, nil
}

func validateCondition(node any) error {
	m, ok := node.(map[string]any)
	if !ok {
		return errors.New("must be an object")
	}
	if sequence, ok := m["sequence"]; ok {
		items, ok := sequence.([]any)
		if !ok || len(items) != 3 {
			return errors.New("sequence must contain exactly three labels")
		}
		for _, item := range items {
			if label, ok := item.(string); !ok || label == "" {
				return errors.New("sequence labels must be non-empty strings")
			}
		}
		return nil
	}
	for _, key := range []string{"all", "any"} {
		if children, ok := m[key]; ok {
			items, ok := children.([]any)
			if !ok || len(items) == 0 {
				return fmt.Errorf("%s must contain conditions", key)
			}
			for _, child := range items {
				if err := validateCondition(child); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return validatePredicate(m, false)
}

func validateEntry(node any) error {
	m, ok := node.(map[string]any)
	if !ok {
		return errors.New("must be an object")
	}
	return validatePredicate(m, true)
}

func validatePredicate(m map[string]any, entry bool) error {
	field, ok := m["field"].(string)
	if !ok || field == "" {
		return errors.New("field is required")
	}
	if entry {
		if field != "entry_gap" {
			return fmt.Errorf("field %q is not allowed for entry", field)
		}
	} else if !isAllowedConditionField(field) {
		return fmt.Errorf("field %q is not allowed", field)
	}
	op, ok := m["op"].(string)
	if !ok || !isAllowedOperator(op) {
		return fmt.Errorf("operator %q is not allowed", op)
	}
	if op == "between" {
		if _, ok := numberValue(m["min"]); !ok {
			return errors.New("between requires numeric min")
		}
		if _, ok := numberValue(m["max"]); !ok {
			return errors.New("between requires numeric max")
		}
		if inclusive, exists := m["inclusive"]; exists {
			if _, ok := inclusive.(bool); !ok {
				return errors.New("inclusive must be boolean")
			}
		}
		return nil
	}
	if _, ok := m["value"]; !ok {
		return fmt.Errorf("operator %q requires value", op)
	}
	return nil
}

func isAllowedConditionField(field string) bool {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 {
		return false
	}
	if parts[0] != "t-3" && parts[0] != "t-2" && parts[0] != "t-1" {
		return false
	}
	switch parts[1] {
	case "close_type", "is_one_word", "amplitude", "open_ret", "close_ret", "body_ret", "open", "high", "low", "close", "volume", "amount_yi", "prev_close":
		return true
	case "close_vs_t-2_ret", "body_drop_vs_t-2_close", "close_gt_open", "is_bearish":
		return parts[0] == "t-1"
	default:
		return false
	}
}

func isAllowedOperator(op string) bool {
	switch op {
	case "eq", "lt", "lte", "gt", "gte", "between":
		return true
	default:
		return false
	}
}

func canonicalRuleJSON(condition, entry any, version string) (string, error) {
	payload := map[string]any{
		"condition": normalizeJSONValue(condition, true),
		"entry":     normalizeJSONValue(entry, false),
		"version":   version,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize rule: %w", err)
	}
	return string(encoded), nil
}

func normalizeJSONValue(value any, sortAll bool) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = normalizeJSONValue(child, sortAll)
		}
		if sortAll {
			if children, ok := out["all"].([]any); ok {
				sort.Slice(children, func(i, j int) bool {
					left, _ := json.Marshal(children[i])
					right, _ := json.Marshal(children[j])
					return string(left) < string(right)
				})
			}
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = normalizeJSONValue(child, sortAll)
		}
		return out
	case json.Number:
		if number, err := strconv.ParseFloat(string(typed), 64); err == nil {
			return number
		}
		return string(typed)
	default:
		return value
	}
}

func evaluateCondition(node any, view PreT0View) bool {
	m, ok := node.(map[string]any)
	if !ok {
		return false
	}
	if sequence, ok := m["sequence"].([]any); ok {
		for i, offset := range []int{-3, -2, -1} {
			bar, exists := view.Bars[offset]
			label, labelOK := sequence[i].(string)
			if !exists || !labelOK || bar.CloseType != label {
				return false
			}
		}
		return true
	}
	if children, ok := m["all"].([]any); ok {
		for _, child := range children {
			if !evaluateCondition(child, view) {
				return false
			}
		}
		return true
	}
	if children, ok := m["any"].([]any); ok {
		for _, child := range children {
			if evaluateCondition(child, view) {
				return true
			}
		}
		return false
	}
	return evaluatePredicate(m, func(field string) (any, bool) {
		return fieldValue(field, view)
	})
}

func evaluateEntry(node any, entry EntryView) bool {
	return evaluatePredicate(node.(map[string]any), func(field string) (any, bool) {
		if field == "entry_gap" {
			return entry.Gap, true
		}
		return nil, false
	})
}

func evaluatePredicate(node map[string]any, lookup func(string) (any, bool)) bool {
	field, ok := node["field"].(string)
	if !ok {
		return false
	}
	actual, ok := lookup(field)
	if !ok {
		return false
	}
	op, ok := node["op"].(string)
	if !ok {
		return false
	}
	if op == "between" {
		actualNumber, ok := numberValue(actual)
		if !ok {
			return false
		}
		min, minOK := numberValue(node["min"])
		max, maxOK := numberValue(node["max"])
		if !minOK || !maxOK {
			return false
		}
		inclusive, _ := node["inclusive"].(bool)
		if inclusive {
			return actualNumber >= min && actualNumber <= max
		}
		return actualNumber > min && actualNumber < max
	}
	return compareValue(actual, node["value"], op)
}

func compareValue(actual, expected any, op string) bool {
	actualNumber, actualOK := numberValue(actual)
	expectedNumber, expectedOK := numberValue(expected)
	if actualOK && expectedOK {
		switch op {
		case "eq":
			return actualNumber == expectedNumber
		case "lt":
			return actualNumber < expectedNumber
		case "lte":
			return actualNumber <= expectedNumber
		case "gt":
			return actualNumber > expectedNumber
		case "gte":
			return actualNumber >= expectedNumber
		}
	}
	if op != "eq" {
		return false
	}
	return fmt.Sprint(actual) == fmt.Sprint(expected)
}

func fieldValue(field string, view PreT0View) (any, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 {
		return nil, false
	}
	offset, ok := map[string]int{"t-3": -3, "t-2": -2, "t-1": -1}[parts[0]]
	if !ok {
		return nil, false
	}
	bar, ok := view.Bars[offset]
	if !ok {
		return nil, false
	}
	if offset == -1 {
		previous, previousOK := view.Bars[-2]
		if previousOK {
			switch parts[1] {
			case "close_vs_t-2_ret":
				if previous.Close <= 0 || bar.Close <= 0 {
					return nil, false
				}
				return percentChange(previous.Close, bar.Close), true
			case "body_drop_vs_t-2_close":
				if previous.Close <= 0 {
					return nil, false
				}
				return percentFrom(previous.Close, bar.Open-bar.Close), true
			case "close_gt_open":
				return bar.Close > bar.Open, true
			case "is_bearish":
				return bar.Close < bar.Open, true
			}
		}
	}
	switch parts[1] {
	case "close_type":
		return bar.CloseType, true
	case "is_one_word":
		return bar.IsOneWord, true
	case "amplitude":
		return bar.Amplitude, true
	case "open_ret":
		return bar.OpenRet, true
	case "close_ret":
		return bar.CloseRet, true
	case "body_ret":
		return bar.BodyRet, true
	case "open":
		return bar.Open, true
	case "high":
		return bar.High, true
	case "low":
		return bar.Low, true
	case "close":
		return bar.Close, true
	case "volume":
		return bar.Volume, true
	case "amount_yi":
		return bar.AmountYi, true
	case "prev_close":
		return bar.PrevClose, true
	default:
		return nil, false
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		n, err := typed.Float64()
		return n, err == nil
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}
