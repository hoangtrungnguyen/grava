package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/doltinstall"
	"github.com/hoangtrungnguyen/grava/pkg/migrate"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Grava repository",
	Long: `Initialize a new Grava repository in the current directory.
This command creates a .grava directory and a default configuration file.
If Dolt is not installed locally or on the system PATH, it will be
automatically downloaded to .grava/bin/dolt (no sudo required).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// 1. Resolve or install Dolt
		doltBin, err := utils.ResolveDoltBinary(cwd)
		if err != nil {
			// Dolt not found locally or on PATH — download it
			if !outputJSON {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "📥 Dolt not found. Downloading to .grava/bin/dolt...")
			}
			binDir := utils.LocalDoltBinDir(cwd)
			if installErr := doltinstall.InstallDolt(binDir); installErr != nil {
				return fmt.Errorf("failed to install dolt: %w", installErr)
			}
			doltBin, err = utils.ResolveDoltBinary(cwd)
			if err != nil {
				return fmt.Errorf("dolt install appeared to succeed but binary not found: %w", err)
			}
			if !outputJSON {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt installed to .grava/bin/dolt")
			}
		}
		if !outputJSON {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Dolt is ready.")
		}

		// 2. Create .grava directory
		gravaDir := ".grava"
		if err := os.MkdirAll(gravaDir, 0755); err != nil {
			return fmt.Errorf("failed to create .grava directory: %w", err)
		}

		// 2a. Add .grava/ to .git/info/exclude (ADR-H5)
		if migrated, excludeErr := utils.WriteGitExclude(cwd); excludeErr != nil {
			// Non-fatal: warn but continue
			if !outputJSON {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to update .git/info/exclude: %v\n", excludeErr)
			}
		} else if migrated && !outputJSON {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Migrated .grava/ from .gitignore to .git/info/exclude")
		}

		// 3. Initialize Dolt Repo in .grava/dolt
		doltRepoDir := filepath.Join(gravaDir, "dolt")
		if err := os.MkdirAll(doltRepoDir, 0755); err != nil {
			return fmt.Errorf("failed to create dolt directory: %w", err)
		}

		if _, err := os.Stat(filepath.Join(doltRepoDir, ".dolt")); os.IsNotExist(err) {
			if !outputJSON {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "📦 Initializing Dolt database...")
			}
			initDoltCmd := exec.Command(doltBin, "init")
			initDoltCmd.Dir = doltRepoDir
			if err := initDoltCmd.Run(); err != nil {
				return fmt.Errorf("failed to initialize dolt: %w", err)
			}
		}

		// 4. Find Available Port
		port, err := utils.AllocatePort(cwd, 3306)
		if err != nil {
			return err
		}
		if !outputJSON && port != 3306 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Port 3306 is busy, using port %d\n", port)
		}

		// 5. Start Dolt Server in background
		if !outputJSON {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting Dolt server on port %d...\n", port)
		}

		serverCmd := exec.Command(doltBin, "sql-server", "--port", fmt.Sprintf("%d", port), "--host", "0.0.0.0")
		serverCmd.Dir = doltRepoDir

		// Redirect output to log file
		logFile, err := os.OpenFile(filepath.Join(gravaDir, "dolt.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			serverCmd.Stdout = logFile
			serverCmd.Stderr = logFile
		}

		if err := serverCmd.Start(); err != nil {
			return fmt.Errorf("failed to start dolt server: %w", err)
		}

		// 6. Wait for server ready, run migrations, write schema_version
		initDBURL := fmt.Sprintf("root@tcp(127.0.0.1:%d)/dolt?parseTime=true", port)
		var initStore *dolt.Client
		for i := 0; i < 10; i++ {
			initStore, err = dolt.NewClient(initDBURL)
			if err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if err != nil {
			return fmt.Errorf("dolt server did not become ready: %w", err)
		}
		defer initStore.Close() //nolint:errcheck

		if !outputJSON {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "📦 Running database migrations...")
		}
		if err := migrate.Run(initStore.DB()); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		if err := utils.WriteSchemaVersion(gravaDir, utils.SchemaVersion); err != nil {
			return fmt.Errorf("failed to write schema_version: %w", err)
		}
		if !outputJSON {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Database migrations complete.")
		}

		// 7. Create default config
		configFile := ".grava.yaml"
		dbURL := fmt.Sprintf("root@tcp(127.0.0.1:%d)/dolt?parseTime=true", port)
		viper.Set("db_url", dbURL)

		if err := viper.WriteConfigAs(configFile); err != nil {
			if os.IsExist(err) || err.Error() == "file already exists" {
				_ = os.Remove(configFile)
				_ = viper.WriteConfigAs(configFile)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ℹ️  Note: %v\n", err)
			}
		}

		if !outputJSON {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ Created configuration in .grava.yaml")
		}

		if outputJSON {
			resp := map[string]interface{}{
				"status": "initialized",
				"port":   port,
				"db_url": dbURL,
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "🎉 Grava initialized successfully and server is running!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
