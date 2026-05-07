# Module: `pkg/validation`

**Package role:** Input validators for issue fields. Case-insensitive. Validates type, status, priority, and date ranges before any DB write.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `validation.go` | 87 | ValidateIssueType,ValidateStatus ValidatePriority,ValidateDateRange |
| `validation_test.go` | 110 | TestValidateIssueType,TestValidateStatus TestValidatePriority,TestValidateDateRange |

## Public API

```
var AllowedIssueTypes = map[string]bool{ ... } ...
func ValidateDateRange(fromStr, toStr string) (time.Time, time.Time, error)
func ValidateIssueType(t string) error
func ValidatePriority(p string) (int, error)
func ValidateStatus(s string) error
```

