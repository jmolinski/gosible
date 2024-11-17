package command

import (
	"context"
	"github.com/spf13/cobra"
)

type App struct {
	context.Context

	// Global flags.
	ConfigFile string // -c, --config
	Format     string // -f, --format

	// Common resources.
	Config []byte
}

func (app *App) Register(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&app.ConfigFile, "config", "c", "", "Configuration file")
	cobra.CheckErr(cmd.MarkPersistentFlagFilename("config", "yml", "yaml"))

	cmd.PersistentFlags().StringVarP(&app.Format, "format", "f", "json", "Format type of output")

}
