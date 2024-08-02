package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/machbase/neo-server/mods/pkgs"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(NewCmd().ExecuteContext(context.Background()))
}

func NewCmd() *cobra.Command {
	cobra.EnableCommandSorting = false

	rootCmd := &cobra.Command{
		Use:           "neopkg [command] [flags] [args]",
		Short:         "neopkg is a package manager for machbase-neo",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Print(cmd.UsageString())
		},
	}
	rootCmd.PersistentFlags().StringP("meta-dir", "m", "", "`<MetaDir>` path to cache the package roster")
	rootCmd.PersistentFlags().StringP("dist-dir", "d", "", "`<DistDir>` path to packages installed")
	rootCmd.MarkPersistentFlagRequired("meta-dir")
	rootCmd.MarkPersistentFlagRequired("dist-dir")

	syncCmd := &cobra.Command{
		Use:   "sync [flags]",
		Short: "Sync package index",
		RunE:  doSync,
	}

	searchCmd := &cobra.Command{
		Use:   "search [flags] <package name>",
		Short: "Search package info",
		RunE:  doSearch,
	}
	searchCmd.Args = cobra.ExactArgs(1)

	installCmd := &cobra.Command{
		Use:   "install [flags] <package name>",
		Short: "Install a package",
		RunE:  doInstall,
	}
	installCmd.Args = cobra.ExactArgs(1)
	installCmd.PersistentFlags().StringP("version", "v", "latest", "`<Version>` of the package to install")

	buildCmd := &cobra.Command{
		Use:   "build [flags] <path to package.yml>",
		Short: "Build a package",
		RunE:  doBuild,
	}
	buildCmd.Args = cobra.ExactArgs(1)
	buildCmd.PersistentFlags().StringP("version", "v", "latest", "`<Version>` of the package to build")

	rootCmd.AddCommand(
		syncCmd,
		searchCmd,
		installCmd,
		buildCmd,
	)
	return rootCmd
}

func doSearch(cmd *cobra.Command, args []string) error {
	metaDir, err := cmd.Flags().GetString("meta-dir")
	if err != nil {
		return err
	}
	distDir, err := cmd.Flags().GetString("dist-dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(metaDir, distDir)
	if err != nil {
		return err
	}
	name := args[0]
	result, err := mgr.Search(name, false)
	if err != nil {
		return err
	}
	if result.ExactMatch != nil {
		print(result.ExactMatch)
	} else {
		fmt.Printf("Package %q not found\n", args[0])
		if len(result.Possibles) > 0 {
			fmt.Println("\nWhat your are looking for might be:")
			for _, s := range result.Possibles {
				if s.Github != nil {
					fmt.Printf("  %s   https://github.com/%s/%s installed: %s\n",
						s.Name, s.Github.Organization, s.Github.Name, s.InstalledVersion)
				}
			}
		}
	}
	return nil
}

func doSync(cmd *cobra.Command, args []string) error {
	metaDir, err := cmd.Flags().GetString("meta-dir")
	if err != nil {
		return err
	}
	distDir, err := cmd.Flags().GetString("dist-dir")
	if err != nil {
		return err
	}
	mgr, err := pkgs.NewPkgManager(metaDir, distDir)
	if err != nil {
		return err
	}
	err = mgr.Sync()
	if err != nil {
		return err
	}
	return nil
}

func doInstall(cmd *cobra.Command, args []string) error {
	metaDir, err := cmd.Flags().GetString("meta-dir")
	if err != nil {
		return err
	}
	distDir, err := cmd.Flags().GetString("dist-dir")
	if err != nil {
		return err
	}
	version, err := cmd.Flags().GetString("version")
	if err != nil {
		return err
	}
	if version == "" {
		version = "latest"
	}
	mgr, err := pkgs.NewPkgManager(metaDir, distDir)
	if err != nil {
		return err
	}

	cache, err := mgr.Install(args[0], os.Stdout)
	if err != nil {
		return err
	}
	fmt.Println("Installed to", cache.InstalledPath, cache.InstalledVersion)
	return err
}

func doBuild(cmd *cobra.Command, args []string) error {
	metaDir, err := cmd.Flags().GetString("meta-dir")
	if err != nil {
		return err
	}
	distDir, err := cmd.Flags().GetString("dist-dir")
	if err != nil {
		return err
	}
	name := args[0]
	pkgName, pkgVersion, found := strings.Cut(name, ":")
	if !found {
		pkgVersion = "latest"
	}

	mgr, err := pkgs.NewPkgManager(metaDir, distDir)
	if err != nil {
		return err
	}
	if err := mgr.Build(pkgName, pkgVersion); err != nil {
		return err
	}
	cmd.Print("Building successful\n")
	return nil
}

func print(nr *pkgs.PackageCache) {
	fmt.Println("Package             ", nr.Name)
	fmt.Println("Github              ", nr.Github)
	fmt.Println("Latest Release      ", nr.LatestRelease)
	fmt.Println("Latest Release Tag  ", nr.LatestReleaseTag)
	fmt.Println("Published At        ", nr.PublishedAt)
	fmt.Println("Url                 ", nr.Url)
	fmt.Println("StripComponents     ", nr.StripComponents)
	fmt.Println("Cached At           ", nr.CachedAt)
	fmt.Println("Installed Version   ", nr.InstalledVersion)
	fmt.Println("Installed Path      ", nr.InstalledPath)
}
