package merge

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ProcessMerge executes a 3-way merge on JSONL files
// returns the merged JSONL string, a boolean indicating if there were conflicts, and an error if parsing failed.
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
			// New in C or O (or both)
			if hasC && hasO {
				// Added in both. Compare recursively
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

		// Existed in Ancestor
		if !hasC && !hasO {
			// Deleted in both
			continue
		}

		if !hasC && hasO {
			// Deleted in Local, kept in Remote
			if reflect.DeepEqual(a, o) {
				// Remote didn't change it, Local deleted it -> Delete wins.
				continue
			}
			// Remote modified it, Local deleted it -> Conflict!
			// Output a conflict marker for the whole object
			conflictObj := map[string]interface{}{
				"id":        id,
				"_conflict": true,
				"local":     nil,
				"remote":    o,
			}
			mergedList = append(mergedList, conflictObj)
			hasAnyConflict = true
			continue
		}

		if hasC && !hasO {
			// Deleted in Remote, kept in Local
			if reflect.DeepEqual(a, c) {
				// Local didn't change it, Remote deleted it -> Delete wins.
				continue
			}
			// Local modified it, Remote deleted it -> Conflict!
			conflictObj := map[string]interface{}{
				"id":        id,
				"_conflict": true,
				"local":     c,
				"remote":    nil,
			}
			mergedList = append(mergedList, conflictObj)
			hasAnyConflict = true
			continue
		}

		// Modified in C and/or O
		mergedItem, conflict := mergeObjects(a, c, o)
		mergedItem["id"] = id
		mergedList = append(mergedList, mergedItem)
		if conflict {
			hasAnyConflict = true
		}
	}

	// Serialize back to JSONL
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
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, err
		}
		if id, ok := obj["id"].(string); ok {
			result[id] = obj
		}
	}
	return result, nil
}

func mergeObjects(ancestor, current, other map[string]interface{}) (map[string]interface{}, bool) {
	if ancestor == nil {
		ancestor = make(map[string]interface{})
	}
	result := make(map[string]interface{})
	hasConflict := false

	// Get all keys
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

		if !cChanged && !oChanged {
			if hasA {
				result[k] = aVal
			}
			continue
		}

		if cChanged && !oChanged {
			if hasC {
				result[k] = cVal
			}
			continue
		}

		if !cChanged && oChanged {
			if hasO {
				result[k] = oVal
			}
			continue
		}

		// Both changed
		if reflect.DeepEqual(cVal, oVal) {
			if hasC {
				result[k] = cVal
			}
			continue
		}

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

	return result, hasConflict
}
