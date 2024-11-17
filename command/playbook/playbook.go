package playbook

import (
	"github.com/scylladb/gosible/command"
	"github.com/spf13/cobra"
)

func NewCommand(app *command.App) *cobra.Command {
	return NewPlayCommand(app)
}
