package merge

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ProcessMerge executes a 3-way merge on JSONL files.
// Returns the merged JSONL string, a boolean indicating conflicts, and any parse error.
func ProcessMerge(ancestor, current, other string) (string, bool, error) {
	ancestorMap, err := parseJSONL(ancestor)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse ancestor: %w", err)
	}
	currentMap, err := parseJSONL(current)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse current: %w", err)
	}
	otherMap, err := parseJSONL(other)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse other: %w", err)
	}

	mergedList := []map[string]interface{}{}
	hasAnyConflict := false

	// Gather all unique IDs
	allIDs := make(map[string]bool)
	for id := range ancestorMap {
		allIDs[id] = true
	}
	for id := range currentMap {
		allIDs[id] = true
	}
	for id := range otherMap {
		allIDs[id] = true
	}

	// Sort IDs for deterministic output
	var sortedIDs []string
	for id := range allIDs {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	for _, id := range sortedIDs {
		a, hasA := ancestorMap[id]
		c, hasC := currentMap[id]
		o, hasO := otherMap[id]

		if !hasA {
			// New in current or other (or both)
			if hasC && hasO {
				mergedItem, conflict := mergeObjects(nil, c, o)
				mergedItem["id"] = id
				mergedList = append(mergedList, mergedItem)
				if conflict {
					hasAnyConflict = true
				}
			} else if hasC {
				mergedList = append(mergedList, c)
			} else if hasO {
				mergedList = append(mergedList, o)
			}
			continue
		}

		// Issue existed in ancestor
		if !hasC && !hasO {
			continue // deleted in both
		}

		if !hasC && hasO {
			// Deleted in current, kept in other
			if reflect.DeepEqual(a, o) {
				continue // other didn't change it; delete wins
			}
			// Other modified it; conflict
			mergedList = append(mergedList, map[string]interface{}{
				"id":        id,
				"_conflict": true,
				"local":     nil,
				"remote":    o,
			})
			hasAnyConflict = true
			continue
		}

		if hasC && !hasO {
			// Deleted in other, kept in current
			if reflect.DeepEqual(a, c) {
				continue // current didn't change it; delete wins
			}
			// Current modified it; conflict
			mergedList = append(mergedList, map[string]interface{}{
				"id":        id,
				"_conflict": true,
				"local":     c,
				"remote":    nil,
			})
			hasAnyConflict = true
			continue
		}

		// Modified in current and/or other
		mergedItem, conflict := mergeObjects(a, c, o)
		mergedItem["id"] = id
		mergedList = append(mergedList, mergedItem)
		if conflict {
			hasAnyConflict = true
		}
	}

	var sb strings.Builder
	for _, item := range mergedList {
		b, err := json.Marshal(item)
		if err != nil {
			return "", false, err
		}
		sb.Write(b)
		sb.WriteString("\n")
	}

	return sb.String(), hasAnyConflict, nil
}

func parseJSONL(content string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	if content == "" {
		return result, nil
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, err
		}
		id, ok := obj["id"].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("JSONL line has no 'id' field: %s", line)
		}
		result[id] = obj
	}
	return result, nil
}

func mergeObjects(ancestor, current, other map[string]interface{}) (map[string]interface{}, bool) {
	if ancestor == nil {
		ancestor = make(map[string]interface{})
	}
	result := make(map[string]interface{})
	hasConflict := false

	allKeys := make(map[string]bool)
	for k := range ancestor {
		allKeys[k] = true
	}
	for k := range current {
		allKeys[k] = true
	}
	for k := range other {
		allKeys[k] = true
	}

	for k := range allKeys {
		if k == "id" {
			continue
		}

		aVal, hasA := ancestor[k]
		cVal, hasC := current[k]
		oVal, hasO := other[k]

		cChanged := hasC != hasA || !reflect.DeepEqual(aVal, cVal)
		oChanged := hasO != hasA || !reflect.DeepEqual(aVal, oVal)

		switch {
		case !cChanged && !oChanged:
			if hasA {
				result[k] = aVal
			}
		case cChanged && !oChanged:
			if hasC {
				result[k] = cVal
			}
		case !cChanged && oChanged:
			if hasO {
				result[k] = oVal
			}
		case reflect.DeepEqual(cVal, oVal):
			if hasC {
				result[k] = cVal
			}
		default:
			// True field-level conflict
			hasConflict = true
			conflictMarker := map[string]interface{}{
				"_conflict": true,
			}
			if hasC {
				conflictMarker["local"] = cVal
			} else {
				conflictMarker["local"] = nil
			}
			if hasO {
				conflictMarker["remote"] = oVal
			} else {
				conflictMarker["remote"] = nil
			}
			result[k] = conflictMarker
		}
	}

	return result, hasConflict
}
