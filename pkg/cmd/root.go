package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hoangtrungnguyen/grava/pkg/devlog"
	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/migrate"
)

var (
	cfgFile      string
	dbURL        string
	actor        string
	agentModel   string
	Store        dolt.Store
	outputJSON   bool
	Version      = "v0.0.1"
	enableDevLog bool
	logFilePath  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grava",
	Short: "Grava is a distributed issue tracker CLI",
	Long: `Grava is a distributed issue tracker built on top of Dolt.
It allows you to manage issues, tasks, and bugs directly from your terminal,
leveraging the power of a version-controlled database.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize dev logger
		useDevLog := enableDevLog || viper.GetBool("enable_dev_log")
		logPath := logFilePath
		if logPath == "" {
			logPath = viper.GetString("log_file_path")
		}
		if err := devlog.Init(useDevLog, logPath); err != nil {
			return fmt.Errorf("failed to initialize development log: %w", err)
		}

		devlog.Printf("Starting Grava command: %s", cmd.Name())

		// Initialize DB connection if not help command or init command
		if cmd.Name() == "help" || cmd.Name() == "init" || cmd.Name() == "start" || cmd.Name() == "stop" {
			return nil
		}

		// If Store is already injected (e.g. tests), use it
		if Store != nil {
			return nil
		}

		var err error
		// Use flag value or config value
		if dbURL == "" {
			dbURL = viper.GetString("db_url")
		}

		if dbURL == "" {
			// Default DSN for local Dolt
			// The database name exposed by `dolt sql-server` inside a repo is `dolt`
			dbURL = "root@tcp(127.0.0.1:3306)/dolt?parseTime=true"
		}

		// Sync flags with viper (handles env vars and config)
		if actor == "unknown" {
			actor = viper.GetString("actor")
		}
		if agentModel == "" {
			agentModel = viper.GetString("agent_model")
		}

		Store, err = dolt.NewClient(dbURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}

		// Run pending migrations
		devlog.Println("Running migrations...")
		if err := migrate.Run(Store.DB()); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		devlog.Println("Migrations complete.")

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		var errStore error
		if Store != nil {
			errStore = Store.Close()
			Store = nil
		}

		devlog.Printf("Grava command completed: %s", cmd.Name())
		errLog := devlog.Close()

		if errStore != nil {
			return errStore
		}
		return errLog
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// SetVersion sets the version string for the CLI
func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.grava.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "Dolt database connection string")
	rootCmd.PersistentFlags().StringVar(&actor, "actor", "unknown", "User or agent identity (env: GRAVA_ACTOR)")
	rootCmd.PersistentFlags().StringVar(&agentModel, "agent-model", "", "AI model identifier (env: GRAVA_AGENT_MODEL)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&enableDevLog, "enable-dev-log", false, "Enable the development log")
	rootCmd.PersistentFlags().StringVar(&logFilePath, "log-file-path", "", "Path to the development log file (default .grava/dev.log)")

	// Bind flags to viper for ENV var support
	viper.BindPFlag("actor", rootCmd.PersistentFlags().Lookup("actor"))                   //nolint:errcheck
	viper.BindPFlag("agent_model", rootCmd.PersistentFlags().Lookup("agent-model"))       //nolint:errcheck
	viper.BindPFlag("enable_dev_log", rootCmd.PersistentFlags().Lookup("enable-dev-log")) //nolint:errcheck
	viper.BindPFlag("log_file_path", rootCmd.PersistentFlags().Lookup("log-file-path"))   //nolint:errcheck

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".grava" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".grava")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		_, _ = fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
