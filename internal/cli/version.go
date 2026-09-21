package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print dbakit version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dbakit %s (commit: %s, built: %s)\n", Version, GitCommit, BuildDate)
	},
}

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "List the compiled-in diagnostic rules",
	Long: `Lists every rule in the catalog. Rules are compiled in for the MVP;
there is no custom rule DSL yet.`,
	Run: func(cmd *cobra.Command, args []string) {
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tGROUP\tTITLE")
		for _, r := range evaluator.Rules {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.ID, r.Group, r.Title)
		}
		tw.Flush()
	},
}
