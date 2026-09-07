package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gomodtui/svc/pkggodev"

	"github.com/spf13/cobra"
)

// NewPackageCmd creates the package subcommand with alias "get".
func NewPackageCmd() *cobra.Command {
	var opts pkggodev.PackageOptions
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "package <import-path>",
		Aliases: []string{"get"},
		Short:   "Get package documentation from pkg.go.dev",
		Long:    `Retrieve package documentation via the pkg.go.dev API (GET /v1/package/{path}).`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			importPath := args[0]
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			result, err := pkggodev.GetPackageWithContext(ctx, http.DefaultClient, importPath, &opts)
			if err != nil {
				return err
			}

			if jsonOut {
				raw, _ := json.MarshalIndent(result, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Package: %s\n", result.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Module: %s\n", result.ModulePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", result.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s (latest: %v)\n", result.Version, result.IsLatest)
			fmt.Fprintf(cmd.OutOrStdout(), "Synopsis: %s\n", result.Synopsis)
			if result.Docs != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDocs:\n%s\n", result.Docs)
			}
			if len(result.Imports) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nImports (%d):\n", len(result.Imports))
				for _, imp := range result.Imports {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", imp)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Module, "module", "", "module path for disambiguation (?module)")
	cmd.Flags().StringVar(&opts.Version, "version", "", "version (?version)")
	cmd.Flags().StringVar(&opts.GOOS, "goos", "", "GOOS (?goos)")
	cmd.Flags().StringVar(&opts.GOARCH, "goarch", "", "GOARCH (?goarch)")
	cmd.Flags().StringVar(&opts.Doc, "doc", "", "doc format text|html|md (?doc)")
	cmd.Flags().BoolVar(&opts.Imports, "imports", false, "include imports (?imports=true)")
	cmd.Flags().BoolVar(&opts.Examples, "examples", false, "include examples (?examples=true)")
	cmd.Flags().BoolVar(&opts.Licenses, "licenses", false, "include licenses (?licenses=true)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")

	return cmd
}
