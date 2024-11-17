package playbook

import (
	"fmt"
	"github.com/scylladb/gosible/command"
	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/executor"
	"github.com/scylladb/gosible/inventory"
	"github.com/scylladb/gosible/modules"
	defaultModules "github.com/scylladb/gosible/modules/default"
	"github.com/scylladb/gosible/playbook"
	defaultPlugins "github.com/scylladb/gosible/plugins/default"
	"github.com/scylladb/gosible/utils/display"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/types"
	varsPkg "github.com/scylladb/gosible/vars"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strings"
)

func NewPlayCommand(app *command.App) *cobra.Command {
	c := &playCmd{App: app}

	cmd := &cobra.Command{
		Use:     "play",
		Short:   "Play a playbook",
		Example: "gosible play -i inventory.txt -e ansible_ssh_private_key_file=key.pem -e custom_config=42 playbook.yml",
		Args:    cobra.ExactArgs(1),
		RunE:    c.run,
	}

	c.register(cmd)
	return cmd
}

type playCmd struct {
	*command.App

	inventoryFile string   // -i, --inventory
	extraVars     []string // -e, --extra-vars
	cmdLineData
}

type cmdLineData struct {
	askBecomePassword      bool   // -K, --ask-become-pass
	becomePasswordFile     string // --become-password-file, --become-pass-file
	askConnectionPassword  bool   // -k, --ask-pass
	connectionPasswordFile string // --connection-password-file, --conn-pass-file
	become                 bool   // --become, -b
	becomeMethod           string // --become-method,
	becomeUser             string // --become-user
}

func (c *playCmd) register(cmd *cobra.Command) {
	// TODO inventory should have a default value

	cmd.Flags().StringVarP(&c.inventoryFile, "inventory", "i", "", "specify inventory host path or comma separated host list")
	cobra.CheckErr(cmd.MarkFlagFilename("inventory", "txt"))
	cobra.CheckErr(cmd.MarkFlagRequired("inventory"))

	c.addBecomeOptions(cmd)
	c.addConnectionPasswordPrompt(cmd)

	cmd.Flags().StringSliceVarP(&c.extraVars, "extra-vars", "e", nil, "set additional variables as key=value or YAML/JSON, if filename prepend with @")
}

func (c *playCmd) addBecomeOptions(cmd *cobra.Command) {
	settings := config.Manager().Settings
	cmd.Flags().BoolVarP(&c.become, "become", "b", settings.DEFAULT_BECOME, "run operations with become (does not imply password prompting)")
	cmd.Flags().StringVar(&c.becomeMethod, "become-method", settings.DEFAULT_BECOME_METHOD, fmt.Sprintf("privilege escalation method to use (default=%s)\n, use `ansible-doc -t become -l` to list valid choices.", settings.DEFAULT_BECOME_METHOD))
	cmd.Flags().StringVar(&c.becomeUser, "become-user", "", fmt.Sprintf("run operations as this user (default=%s)", settings.DEFAULT_BECOME_USER))
	c.addBecomePromptOptions(cmd)
}

func (c *playCmd) addBecomePromptOptions(cmd *cobra.Command) {
	defaultAsk := config.Manager().Settings.DEFAULT_BECOME_ASK_PASS
	passwordFile := config.Manager().Settings.BECOME_PASSWORD_FILE
	cmd.Flags().BoolVarP(&c.askBecomePassword, "ask-become-pass", "K", defaultAsk, "ask for privilege escalation password")
	// TODO implement password file handling.
	// cmd.Flags().StringVarP(&c.becomePasswordFile, "become-password-file", "-become-pass-file", passwordFile, "Become password file")

	if c.askBecomePassword != defaultAsk && c.becomePasswordFile != passwordFile {
		cobra.CheckErr("Become password file and asking for password during become shouldn't be set at the same time!")
	}
	c.becomePasswordFile = pathUtils.UnfrackPath(c.becomePasswordFile, pathUtils.UnfrackOptions{})
}

func (c *playCmd) addConnectionPasswordPrompt(cmd *cobra.Command) {
	defaultAsk := config.Manager().Settings.DEFAULT_ASK_PASS
	passwordFile := config.Manager().Settings.CONNECTION_PASSWORD_FILE
	cmd.Flags().BoolVarP(&c.askConnectionPassword, "ask-pass", "k", defaultAsk, "ask for connection password")
	// TODO implement password file handling.
	// cmd.Flags().StringVarP(&c.connectionPasswordFile, "connection-password-file", "-conn-pass-file", passwordFile, "Connection password file")

	if c.askConnectionPassword != defaultAsk && c.connectionPasswordFile != passwordFile {
		cobra.CheckErr("Connection password file and asking for password during connection shouldn't be set at the same time!")
	}
	c.connectionPasswordFile = pathUtils.UnfrackPath(c.connectionPasswordFile, pathUtils.UnfrackOptions{})
}

func (c *playCmd) run(_ *cobra.Command, args []string) error {
	// TODO config should be parsed for all commands

	mgr := config.Manager()
	if err := mgr.TryLoadConfigFile(""); err != nil {
		display.Fatal(display.ErrorOptions{}, "could not load config file: %s", err)
	}

	defaultPlugins.Register()
	mods := modules.NewRegistry()
	defaultModules.Register(mods)
	defaultModules.RegisterPython(mods)

	display.Banner(display.BannerOptions{Color: "magenta"}, "%s %s", "gosible", "hello!")
	display.Display(display.Options{Color: "cyan"}, "play called with %v", args)

	inventoryData, err := inventory.Parse(c.inventoryFile)
	if err != nil {
		display.Error(display.ErrorOptions{}, "error parsing inventory: %v", err)
		return err
	}

	varsManager := varsPkg.MakeManager(inventoryData)
	if err := varsManager.SetExtraVars(c.extraVars); err != nil {
		display.Error(display.ErrorOptions{}, "error setting extra vars: %v", err)
		return err
	}

	pbook, err := playbook.Parse(args[0], mods)
	if err != nil {
		display.Error(display.ErrorOptions{}, "error parsing playbook: %v", err)
		return err
	}

	// TODO handle CLI config options?

	display.Display(display.Options{Color: "cyan"}, "Running playbook")
	pass, err := c.askPasswords()
	if err != nil {
		display.Error(display.ErrorOptions{}, "error collecting passwords: %v", err)
		return err
	}

	if err = executor.ExecutePlaybook(pbook, inventoryData, varsManager, pass); err != nil {
		display.Fatal(display.ErrorOptions{}, "error executing playbook: %v", err)
	}

	return nil
}
func (c *playCmd) askPasswords() (pass types.Passwords, err error) {
	settings := config.Manager().Settings
	becomePromptMethod := "BECOME"
	if !settings.AGNOSTIC_BECOME_PROMPT {
		becomePromptMethod = strings.ToUpper(c.becomeMethod)
	}
	becomePrompt := fmt.Sprintf("%s password: ", becomePromptMethod)
	if c.askConnectionPassword {
		pass.Ssh, err = getSecret("SSH password: ")
		becomePrompt = fmt.Sprintf("%s password[defaults to SSH password]: ", becomePromptMethod)
	} else if c.connectionPasswordFile != "" {
		// TODO figure out when to ignore it and read the password
	}
	if err != nil {
		return
	}
	if c.askBecomePassword {
		pass.Become, err = getSecret(becomePrompt)
	} else if c.becomePasswordFile != "" {
		// TODO figure out when to ignore it and read the password
	}
	if err != nil {
		return
	}
	return
}

func getSecret(s string) ([]byte, error) {
	fmt.Println(s)
	return terminal.ReadPassword(int(os.Stdin.Fd()))
}
