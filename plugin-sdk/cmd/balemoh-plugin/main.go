package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/author"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := command().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func command() *cobra.Command {
	root := &cobra.Command{Use: "balemoh-plugin", Short: "Build trusted Balemoh application plugins", SilenceUsage: true, SilenceErrors: true}
	var id, module, output string
	var backend bool
	init := &cobra.Command{Use: "init", Short: "Create a new independent plugin project", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return author.Init(output, id, module, backend) }}
	init.Flags().StringVar(&id, "id", "", "Dotted plugin identity")
	_ = init.MarkFlagRequired("id")
	init.Flags().StringVar(&module, "module", "", "Go module path")
	init.Flags().StringVar(&output, "output", ".", "New or empty output directory")
	init.Flags().BoolVar(&backend, "backend", false, "Include a backend query capability")
	var source, packageOutput, modfile string
	var targets []string
	pack := &cobra.Command{Use: "package", Short: "Build an immutable local package; never publishes", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		p, e := author.Package(c.Context(), source, packageOutput, modfile, targets)
		if e != nil {
			return e
		}
		digest, e := p.ContentDigest()
		if e != nil {
			return e
		}
		return json.NewEncoder(c.OutOrStdout()).Encode(map[string]string{"manifestDigest": p.Digest, "packageDigest": digest})
	}}
	pack.Flags().StringVar(&source, "source", ".", "Plugin source directory")
	pack.Flags().StringVar(&packageOutput, "output", "dist", "New package directory")
	pack.Flags().StringVar(&modfile, "modfile", "", "Explicit development modfile")
	pack.Flags().StringSliceVar(&targets, "target", []string{runtime.GOOS + "/" + runtime.GOARCH}, "Repeatable GOOS/GOARCH build target")
	var dir string
	check := &cobra.Command{Use: "check", Short: "Validate local package, Describe and child health", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return author.Check(c.Context(), dir) }}
	check.Flags().StringVar(&dir, "package", "dist", "Installed package directory")
	var dev author.DevOptions
	devCmd := &cobra.Command{Use: "dev", Short: "Run a real BFF with isolated loopback fixture data", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return author.Dev(c.Context(), dev, c.OutOrStdout()) }}
	devCmd.Flags().StringVar(&dev.Source, "source", ".", "Author source directory")
	devCmd.Flags().StringVar(&dev.Host, "host", "", "Actual compatible Balemoh UI executable")
	_ = devCmd.MarkFlagRequired("host")
	devCmd.Flags().StringVar(&dev.SDK, "sdk", "", "Explicit local SDK bootstrap while unpublished")
	devCmd.Flags().StringVar(&dev.Fixtures, "fixtures", "", "JSON operation results by preview state")
	devCmd.Flags().BoolVar(&dev.Watch, "watch", true, "Rebuild and restart on author file changes")
	root.AddCommand(init, pack, check, devCmd)
	return root
}
