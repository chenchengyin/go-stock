package flutter_api

import "encoding/json"

// Add subgroup labels only to already-confirmed base admissions, using their
// frozen opening quote. Never refetch prices or rewrite the immutable snapshot.
func appendOptimizedConfirmedRedHits(response map[string]interface{}, snapshot sharedGobRedConfirmation) {
	results, _ := response["results"].([]interface{})
	needsHistory := false
	for _, raw := range results {
		data, _ := json.Marshal(raw)
		var result T0SelectionResult
		if json.Unmarshal(data, &result) != nil {
			continue
		}
		for _, hit := range result.RedRuleHits {
			if hit.Key == "gob_zt_t1_bear_open_close_body_4combos_v1" && hit.Version == 1 {
				needsHistory = true
			}
		}
	}
	if !needsHistory {
		return
	}
	shared, err := loadMainboardSharedGob()
	if err != nil {
		return
	}
	for _, raw := range results {
		object, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		data, _ := json.Marshal(raw)
		var result T0SelectionResult
		if json.Unmarshal(data, &result) != nil {
			continue
		}
		var base T0RedRuleHit
		existing := map[string]bool{}
		for _, hit := range result.RedRuleHits {
			existing[hit.Key] = true
			if hit.Key == "gob_zt_t1_bear_open_close_body_4combos_v1" && hit.Version == 1 {
				base = hit
			}
		}
		if base.EntryOpen <= 0 {
			continue
		}
		code := t0ShortCodeFromResultCode(result.StockCode)
		hist := histBarsBeforeTradeDate(shared.Daily[code], snapshot.Date)
		if len(hist) == 0 || hist[len(hist)-1].Date != snapshot.HistoryDate {
			continue
		}
		prev := snapshot.Quotes[code].PrevClose
		if prev <= 0 {
			prev = result.PrevClose
		}
		var added []T0RedRuleHit
		for _, rule := range optimizedGobRedRules() {
			if existing[rule.Key] || !rule.MatchHistory(hist) || !rule.MatchOpen(base.EntryOpen, prev) {
				continue
			}
			added = append(added, T0RedRuleHit{Key: rule.Key, Version: rule.Version, Name: rule.Name, Description: rule.Description, ExitHorizon: gobRedRuleExitHorizon(rule), EntrySource: base.EntrySource, EntryOpen: base.EntryOpen, StatsStatus: "pending"})
		}
		if len(added) == 0 {
			continue
		}
		enriched := enrichGobRedRuleStats([]T0SelectionResult{{RedRuleHits: added}})[0].RedRuleHits
		hits, _ := object["红策独立形态命中"].([]interface{})
		names, _ := object["命中条件"].([]interface{})
		for _, hit := range enriched {
			encoded, _ := json.Marshal(hit)
			var mapped map[string]interface{}
			json.Unmarshal(encoded, &mapped)
			hits = append(hits, mapped)
			names = append(names, hit.Name)
		}
		object["红策独立形态命中"], object["命中条件"] = hits, names
	}
}
