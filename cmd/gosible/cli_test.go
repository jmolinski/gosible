package main

import (
	"github.com/scylladb/gosible/command"
	"github.com/scylladb/gosible/command/playbook"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"testing"
)

type cmd []string

func (c cmd) String() string {
	return strings.Join(c, " ")
}

func TestCorrectCli(t *testing.T) {
	cases := []cmd{
		{"gosible", "play", "playbook.yml", "-i", "inventory.txt"},
	}
	mockNewPlaybookCommand()
	defer fixNewPlaybookCommand()
	for _, testCase := range cases {
		os.Args = testCase
		err := execGosible()
		if err != nil {
			t.Error("On", testCase, "Unexpected error", err)
		}
	}
}

func TestWrongCli(t *testing.T) {
	cases := [][]string{
		{"gosible", "play"},
		{"gosible", "play", "playbook.yml"},
		{"gosible", "play", "playbook.yml", "i"},
		{"gosible", "play", "i", "inventory.txt"},
	}
	mockNewPlaybookCommand()
	defer fixNewPlaybookCommand()
	for _, testCase := range cases {
		os.Args = testCase
		err := execGosible()
		if err == nil {
			t.Error("Expected error on", testCase)
		}
	}
}

func fixNewPlaybookCommand() {
	newPlaybookCommand = playbook.NewCommand
}

func mockNewPlaybookCommand() {
	newPlaybookCommand = func(app *command.App) *cobra.Command {
		playbookCmd := playbook.NewPlayCommand(app)
		playbookCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return nil
		}
		return playbookCmd
	}
}
