# Module: `pkg/utils`

**Package role:** Miscellaneous utilities: Dolt binary resolution (local vs PATH), git exclude management, network helpers, Git version validation, and Git worktree orchestration (redirect files, conflict checks, provisioning, lifecycle, Claude settings sync, init-time worktree setup).

> _Updated 2026-04-17 (Story 5.5, grava-4136)._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `dolt_resolver.go` | 44 | ResolveDoltBinary, LocalDoltBinDir, LocalDoltBinaryPath |
| `gitexclude.go` | 92 | WriteGitExclude |
| `net.go` | 118 | GetGlobalPortsFile, LoadUsedPorts, SaveUsedPort, AllocatePort, FindAvailablePort |
| `path.go` | 33 | FindScript |
| `schema.go` | 95 | CheckSchemaVersion, WriteSchemaVersion, ResolveGravaDir |
| `gitversion.go` | 56 | CheckGitVersion, ParseAndCheckGitVersion |
| `worktree.go` | ~330 | IsWorktree, ComputeRedirectPath, WriteRedirectFile, ResolveGravaDirWithRedirect, CheckWorktreeConflict, ProvisionWorktree, DeleteWorktree, LinkClaudeWorktree, IsWorktreeDirty, RemoveWorktreeOnly, IsInsideClaudeWorktree, SyncClaudeSettings, ConfigureGitUser |
| `worktree_init.go` | ~116 | EnsureWorktreeDir, EnsureWorktreeGitignore, SetWorktreeGitConfig, EnsureClaudeWorktreeSettings |

## Public API

```
const SchemaVersion = 8

// Dolt
func ResolveDoltBinary(projectRoot string) (string, error)
func LocalDoltBinDir(projectRoot string) string
func LocalDoltBinaryPath(projectRoot string) string

// Git exclude
func WriteGitExclude(repoRoot string) (migrated bool, err error)

// Network
func AllocatePort(projectPath string, startPort int) (int, error)
func FindAvailablePort(start int) int
func GetGlobalPortsFile() (string, error)
func LoadUsedPorts() (map[string]int, error)
func SaveUsedPort(projectPath string, port int) error

// Misc
func FindScript(name string) (string, error)
func CheckSchemaVersion(gravaDir string, expectedVersion int) error
func WriteSchemaVersion(gravaDir string, version int) error
func ResolveGravaDir() (string, error)

// Git version (Story 5.5 AC#4)
func CheckGitVersion() error
func ParseAndCheckGitVersion(versionStr string) error

// Worktree init (Story 5.5 AC#1–AC#3)
func EnsureWorktreeDir(repoRoot string) (bool, error)
func EnsureWorktreeGitignore(repoRoot string) (bool, error)
func SetWorktreeGitConfig(repoRoot string) error
func EnsureClaudeWorktreeSettings(repoRoot string) (bool, error)

// Worktree orchestration (Stories 5.1–5.5)
func IsWorktree(cwd string) bool
func ComputeRedirectPath(cwd string) (string, error)
func WriteRedirectFile(cwd string) (bool, error)
func ResolveGravaDirWithRedirect(cwd string) (string, error)
func CheckWorktreeConflict(cwd, issueID string) error
func ProvisionWorktree(cwd, issueID string) error
func DeleteWorktree(cwd, issueID string) error
func LinkClaudeWorktree(cwd, issueID string) error
func IsWorktreeDirty(cwd, issueID string) (bool, error)
func RemoveWorktreeOnly(cwd, issueID string) error
func IsInsideClaudeWorktree(cwd string) bool
func SyncClaudeSettings(mainRepoDir, worktreeDir string) error  // Story 5.5
func ConfigureGitUser(mainRepoDir, worktreeDir string) error    // Story 5.5
```

