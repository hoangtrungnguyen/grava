package merge

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
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
		b, err := MarshalSorted(item)
		if err != nil {
			return "", false, err
		}
		sb.Write(b)
		sb.WriteString("\n")
	}

	return sb.String(), hasAnyConflict, nil
}

// MarshalSorted encodes v as JSON with map keys sorted alphabetically at every
// level. This makes the output byte-for-byte stable across runs, ensuring that
// the same logical merge always produces the same git object hash.
func MarshalSorted(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf.Write(keyBytes)
			buf.WriteByte(':')
			valBytes, err := MarshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			buf.Write(valBytes)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil

	case []interface{}:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, elem := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			b, err := MarshalSorted(elem)
			if err != nil {
				return nil, err
			}
			buf.Write(b)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil

	default:
		return json.Marshal(v)
	}
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

// ConflictEntry describes one unresolvable collision found during a merge.
type ConflictEntry struct {
	// ID is a short hash derived from issue_id + field for deduplication.
	ID         string          `json:"id"`
	IssueID    string          `json:"issue_id"`
	Field      string          `json:"field"`      // empty string for whole-issue conflicts
	Local      json.RawMessage `json:"local"`      // value on the current branch
	Remote     json.RawMessage `json:"remote"`     // value on the other branch
	DetectedAt time.Time       `json:"detected_at"`
	Resolved   bool            `json:"resolved"`
}

// conflictID produces a short identifier for a conflict from its issue+field.
func conflictID(issueID, field string) string {
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s", issueID, field)
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}

// ExtractConflicts parses a merged JSONL string produced by ProcessMerge and
// returns one ConflictEntry per unresolvable collision. It detects two
// patterns left by mergeObjects:
//
//  1. Field-level conflict: the field value is an object with "_conflict":true.
//  2. Issue-level delete conflict: the top-level issue object has "_conflict":true.
func ExtractConflicts(mergedJSONL string, detectedAt time.Time) ([]ConflictEntry, error) {
	var entries []ConflictEntry

	for _, line := range strings.Split(strings.TrimSpace(mergedJSONL), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, fmt.Errorf("failed to parse merged line: %w", err)
		}

		issueID, _ := obj["id"].(string)

		// Check for issue-level delete conflict.
		if topConflict, ok := obj["_conflict"].(bool); ok && topConflict {
			local, _ := json.Marshal(obj["local"])
			remote, _ := json.Marshal(obj["remote"])
			entries = append(entries, ConflictEntry{
				ID:         conflictID(issueID, ""),
				IssueID:    issueID,
				Field:      "",
				Local:      json.RawMessage(local),
				Remote:     json.RawMessage(remote),
				DetectedAt: detectedAt,
			})
			continue
		}

		// Check each field for a field-level conflict marker.
		for field, val := range obj {
			if field == "id" {
				continue
			}
			valMap, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			if isConflict, _ := valMap["_conflict"].(bool); !isConflict {
				continue
			}
			local, _ := json.Marshal(valMap["local"])
			remote, _ := json.Marshal(valMap["remote"])
			entries = append(entries, ConflictEntry{
				ID:         conflictID(issueID, field),
				IssueID:    issueID,
				Field:      field,
				Local:      json.RawMessage(local),
				Remote:     json.RawMessage(remote),
				DetectedAt: detectedAt,
			})
		}
	}

	// Sort by issue_id then field for deterministic output.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IssueID != entries[j].IssueID {
			return entries[i].IssueID < entries[j].IssueID
		}
		return entries[i].Field < entries[j].Field
	})

	return entries, nil
}
