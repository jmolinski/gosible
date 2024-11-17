package main

import (
	"github.com/spf13/cobra"

	"github.com/scylladb/gosible/command"
	"github.com/scylladb/gosible/command/playbook"
)

var newPlaybookCommand = playbook.NewCommand

func NewCommand(app *command.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gosible",
		Short: "A drop-in replacement for Ansible",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newPlaybookCommand(app))

	app.Register(cmd)

	return cmd
}
