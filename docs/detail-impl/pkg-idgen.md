# Module: `pkg/idgen`

**Package role:** Hierarchical issue ID generation. Base IDs use SHA-256 of timestamp+random. Child IDs use the DB-backed child_counters table for atomicity.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `generator.go` | 68 | IDGenerator,StandardGenerator NewStandardGenerator |
| `generator_test.go` | 65 | TestStandardGenerator_GenerateBaseID,TestStandardGenerator_GenerateChildID |

## Public API

```
type IDGenerator interface{ ... }
type StandardGenerator struct{ ... }
    func NewStandardGenerator(store dolt.Store) *StandardGenerator
```

