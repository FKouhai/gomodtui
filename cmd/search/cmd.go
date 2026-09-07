package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/FKouhai/gomodtui/svc/pkggodev"

	"github.com/spf13/cobra"
)

// NewSearchCmd creates the search subcommand.
func NewSearchCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search packages on pkg.go.dev",
		Long:  `Search for packages matching the query via the pkg.go.dev API (GET /v1/search).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			result, err := pkggodev.SearchWithContext(ctx, http.DefaultClient, query)
			if err != nil {
				return err
			}

			if jsonOut {
				raw, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Found %d results (total: %d):\n", len(result.Items), result.Total)
			for _, item := range result.Items {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s) [%s] - %s\n", item.PackagePath, item.ModulePath, item.Version, item.Synopsis)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")

	return cmd
}
