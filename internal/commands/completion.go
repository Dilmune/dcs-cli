package commands

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for dcs.

To load completions:

Bash:
  $ source <(dcs completion bash)
  # To load completions for each session, execute once:
  $ dcs completion bash > /etc/bash_completion.d/dcs

Zsh:
  $ source <(dcs completion zsh)
  # To load completions for each session, execute once:
  $ dcs completion zsh > "${fpath[1]}/_dcs"

Fish:
  $ dcs completion fish | source
  # To load completions for each session, execute once:
  $ dcs completion fish > ~/.config/fish/completions/dcs.fish

PowerShell:
  PS> dcs completion powershell | Out-String | Invoke-Expression
`,
		Example:   "  dcs completion bash\n  dcs completion zsh\n  source <(dcs completion bash)",
		Args:      cobra.ExactValidArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
