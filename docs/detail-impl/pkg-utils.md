# Module: `pkg/utils`

**Package role:** Miscellaneous utilities: Dolt binary resolution (local vs PATH), git exclude management, and network helpers.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `dolt_resolver.go` | 44 | ResolveDoltBinary,LocalDoltBinDir LocalDoltBinaryPath |
| `dolt_resolver_test.go` | 50 | TestResolveDoltBinary_LocalExists,TestResolveDoltBinary_FallsBackToSystem TestResolveDoltBinary_NeitherFound |
| `gitexclude.go` | 92 | WriteGitExclude |
| `gitexclude_test.go` | 174 | TestWriteGitExclude_NoGitDir,TestWriteGitExclude_AddsEntryToNewFile TestWriteGitExclude_Idempotent,TestWriteGitExclude_AppendsToExistingFile TestWriteGitExclude_MigratesGitignore |
| `net.go` | 118 | GetGlobalPortsFile,LoadUsedPorts SaveUsedPort,AllocatePort FindAvailablePort |
| `net_test.go` | 40 | TestAllocatePort |
| `path.go` | 33 | FindScript |
| `schema.go` | 95 | CheckSchemaVersion,WriteSchemaVersion ResolveGravaDir |
| `schema_test.go` | 148 | TestCheckSchemaVersion_Match,TestCheckSchemaVersion_MatchWithNewline TestCheckSchemaVersion_Mismatch,TestCheckSchemaVersion_FileMissing TestCheckSchemaVersion_CorruptFile |

## Public API

```
const SchemaVersion = 8
func AllocatePort(projectPath string, startPort int) (int, error)
func CheckSchemaVersion(gravaDir string, expectedVersion int) error
func FindAvailablePort(start int) int
func FindScript(name string) (string, error)
func GetGlobalPortsFile() (string, error)
func LoadUsedPorts() (map[string]int, error)
func LocalDoltBinDir(projectRoot string) string
func LocalDoltBinaryPath(projectRoot string) string
func ResolveDoltBinary(projectRoot string) (string, error)
func ResolveGravaDir() (string, error)
func SaveUsedPort(projectPath string, port int) error
func WriteGitExclude(repoRoot string) (migrated bool, err error)
func WriteSchemaVersion(gravaDir string, version int) error
```

