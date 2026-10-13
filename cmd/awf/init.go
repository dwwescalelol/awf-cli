package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a project store in the working directory",
		Long: `Create a project store, a .awf directory, in the working directory.

Init refuses inside an existing project store, so a store never holds another.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := initStore()
			if err != nil {
				return err
			}
			fmt.Println(formatInit(s.Dir()))
			return nil
		},
	}
}

func formatInit(dir string) string { return dir + "\ncreated" }

func initStore() (*store.Store, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return store.Init(dir)
}
