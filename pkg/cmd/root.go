package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	cmdgraph "github.com/hoangtrungnguyen/grava/pkg/cmd/graph"
	"github.com/hoangtrungnguyen/grava/pkg/cmd/issues"
	"github.com/hoangtrungnguyen/grava/pkg/cmd/maintenance"
	synccmd "github.com/hoangtrungnguyen/grava/pkg/cmd/sync"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	gravaerrors "github.com/hoangtrungnguyen/grava/pkg/errors"
	gravelog "github.com/hoangtrungnguyen/grava/pkg/log"
	"github.com/hoangtrungnguyen/grava/pkg/notify"
	"github.com/hoangtrungnguyen/grava/pkg/utils"
)

var (
	cfgFile    string
	dbURL      string
	actor      string
	agentModel string
	Store      dolt.Store
	outputJSON bool
	Version    = "v0.0.1"
	// Notifier is the package-level alert notifier. Default: ConsoleNotifier.
	// Commands call Notifier.Send(...) — never instantiate directly in command code.
	// Tests inject a mock: cmd.Notifier = &mock.MockNotifier{}
	Notifier notify.Notifier = notify.NewConsoleNotifier()

	// deps is the shared dependency struct passed to sub-package AddCommands.
	// Pointer fields allow sub-package commands to read current values set by PersistentPreRunE.
	deps = &cmddeps.Deps{
		Store:      &Store,
		Actor:      &actor,
		AgentModel: &agentModel,
		OutputJSON: &outputJSON,
		Notifier:   &Notifier,
	}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grava",
	Short: "Grava is a distributed issue tracker CLI",
	Long: `Grava is a distributed issue tracker built on top of Dolt.
It allows you to manage issues, tasks, and bugs directly from your terminal,
leveraging the power of a version-controlled database.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Step 1: Initialise zerolog (GRAVA_LOG_LEVEL env var, default: warn)
		logLevel := os.Getenv("GRAVA_LOG_LEVEL")
		if logLevel == "" {
			logLevel = "warn"
		}
		gravelog.Init(logLevel, outputJSON)

		gravelog.Logger.Debug().Str("command", cmd.Name()).Msg("Starting Grava command")

		// Step 2: Skip DB init for commands that don't need it
		if cmd.Name() == "help" || cmd.Name() == "init" || cmd.Name() == "version" ||
			cmd.Name() == "start" || cmd.Name() == "stop" {
			return nil
		}

		// If Store is already injected (e.g. tests), use it
		if Store != nil {
			return nil
		}

		// Step 3: Resolve .grava/ directory (simple resolution; full ADR-004 chain in Story 1.3)
		gravaDir, err := utils.ResolveGravaDir()
		if err != nil {
			return gravaerrors.New("NOT_INITIALIZED", "grava is not initialized in this directory; run 'grava init' first", err)
		}

		// Step 4: Check schema version — replaces migrate.Run() in PersistentPreRunE (ADR-FM6)
		if gravaDir != "" {
			if err := utils.CheckSchemaVersion(gravaDir, utils.SchemaVersion); err != nil {
				return err
			}
		}

		// Step 5: Resolve DB URL (flag → viper → env → default)
		if dbURL == "" {
			dbURL = viper.GetString("db_url")
		}
		if dbURL == "" {
			dbURL = "root@tcp(127.0.0.1:3306)/grava?parseTime=true"
		}

		// Sync actor/model from viper (handles env vars and config)
		if actor == "unknown" {
			actor = viper.GetString("actor")
		}
		if agentModel == "" {
			agentModel = viper.GetString("agent_model")
		}

		// Step 6: Connect to Dolt
		Store, err = dolt.NewClient(dbURL)
		if err != nil {
			return gravaerrors.New("DB_UNREACHABLE", "failed to connect to database", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		gravelog.Logger.Debug().Str("command", cmd.Name()).Msg("Grava command completed")

		if Store != nil {
			err := Store.Close()
			Store = nil
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Suppress cobra's default error printing — we handle it below so that
	// --json mode always emits a structured JSON error envelope instead of plain text.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	err := rootCmd.Execute()
	if err != nil {
		if outputJSON {
			writeJSONError(rootCmd, err) //nolint:errcheck
		} else {
			_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

// SetVersion sets the version string for the CLI.
func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.grava.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "Dolt database connection string")
	rootCmd.PersistentFlags().StringVar(&actor, "actor", "unknown", "User or agent identity (env: GRAVA_ACTOR)")
	rootCmd.PersistentFlags().StringVar(&agentModel, "agent-model", "", "AI model identifier (env: GRAVA_AGENT_MODEL)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	// Bind flags to viper for ENV var support
	viper.BindPFlag("actor", rootCmd.PersistentFlags().Lookup("actor"))             //nolint:errcheck
	viper.BindPFlag("agent_model", rootCmd.PersistentFlags().Lookup("agent-model")) //nolint:errcheck

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// Register commands from sub-packages.
	issues.AddCommands(rootCmd, deps)
	cmdgraph.AddCommands(rootCmd, deps)
	maintenance.AddCommands(rootCmd, deps)
	synccmd.AddCommands(rootCmd, deps)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".grava")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		_, _ = fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
