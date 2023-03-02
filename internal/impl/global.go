// Copyright 2022 Jetpack Technologies Inc and contributors. All rights reserved.
// Use of this source code is governed by the license in the LICENSE file.

package impl

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/devbox/internal/boxcli/usererr"
	"go.jetpack.io/devbox/internal/nix"
	"go.jetpack.io/devbox/internal/planner/plansdk"
	"go.jetpack.io/devbox/internal/ux"
	"go.jetpack.io/devbox/internal/xdg"
)

var warningNotInPath = usererr.NewWarning(`the devbox global profile is not in your $PATH.

Add the following line to your shell's rcfile (e.g., ~/.bashrc or ~/.zshrc)
and restart your shell to fix this:

	eval "$(devbox global shellenv)"
`)

// In the future we will support multiple global profiles
const currentGlobalProfile = "default"

func (d *Devbox) AddGlobal(pkgs ...string) error {
	// validate all packages exist. Don't install anything if any are missing
	for _, pkg := range pkgs {
		if !nix.FlakesPkgExists(plansdk.DefaultNixpkgsCommit, pkg) {
			return nix.ErrPackageNotFound
		}
	}
	var added []string
	profilePath, err := GlobalNixProfilePath()
	if err != nil {
		return err
	}

	total := len(pkgs)
	for idx, pkg := range pkgs {
		stepNum := idx + 1
		stepMsg := fmt.Sprintf("[%d/%d] %s", stepNum, total, pkg)
		if err := nix.ProfileInstall(&nix.ProfileInstallArgs{
			CustomStepMessage: stepMsg,
			NixpkgsCommit:     d.cfg.Nixpkgs.Commit,
			Package:           pkg,
			ProfilePath:       profilePath,
			Writer:            d.writer,
		}); err != nil {
			fmt.Fprintf(d.writer, "Error installing %s: %s", pkg, err)
		} else {
			fmt.Fprintf(d.writer, "%s is now installed\n", pkg)
			added = append(added, pkg)
		}
	}
	d.cfg.RawPackages = lo.Uniq(append(d.cfg.RawPackages, added...))
	if err := d.saveCfg(); err != nil {
		return err
	}
	return ensureGlobalProfileInPath()
}

func (d *Devbox) RemoveGlobal(pkgs ...string) error {
	if _, missing := lo.Difference(d.cfg.RawPackages, pkgs); len(missing) > 0 {
		ux.Fwarning(
			d.writer,
			"the following packages were not found in your global devbox.json: %s\n",
			strings.Join(missing, ", "),
		)
	}
	var removed []string
	profilePath, err := GlobalNixProfilePath()
	if err != nil {
		return err
	}
	for _, pkg := range lo.Intersect(d.cfg.RawPackages, pkgs) {
		if err := nix.ProfileRemove(profilePath, plansdk.DefaultNixpkgsCommit, pkg); err != nil {
			fmt.Fprintf(d.writer, "Error removing %s: %s", pkg, err)
		} else {
			fmt.Fprintf(d.writer, "%s was removed\n", pkg)
			removed = append(removed, pkg)
		}
	}
	d.cfg.RawPackages, _ = lo.Difference(d.cfg.RawPackages, removed)
	return d.saveCfg()
}

func (d *Devbox) PullGlobal(path string) error {
	u, err := url.Parse(path)
	if err == nil && u.Scheme != "" {
		return d.pullGlobalFromURL(u)
	}
	return d.pullGlobalFromPath(path)
}

func (d *Devbox) PrintGlobalList() error {
	for _, p := range d.cfg.RawPackages {
		fmt.Fprintf(d.writer, "* %s\n", p)
	}
	return nil
}

func (d *Devbox) pullGlobalFromURL(u *url.URL) error {
	fmt.Fprintf(d.writer, "Pulling global config from %s\n", u)
	cfg, err := readConfigFromURL(u)
	if err != nil {
		return err
	}
	return d.addFromPull(cfg)
}

func (d *Devbox) pullGlobalFromPath(path string) error {
	fmt.Fprintf(d.writer, "Pulling global config from %s\n", path)
	cfg, err := readConfig(path)
	if err != nil {
		return err
	}
	return d.addFromPull(cfg)
}

func (d *Devbox) addFromPull(pullCfg *Config) error {
	if pullCfg.Nixpkgs.Commit != plansdk.DefaultNixpkgsCommit {
		// TODO: For now show this warning, but we do plan to allow packages from
		// multiple commits in the future
		ux.Fwarning(d.writer, "nixpkgs commit mismatch. Using local one by default\n")
	}

	diff, _ := lo.Difference(pullCfg.RawPackages, d.cfg.RawPackages)
	if len(diff) == 0 {
		fmt.Fprint(d.writer, "No new packages to install\n")
		return nil
	}
	fmt.Fprintf(
		d.writer,
		"Installing the following packages: %s\n",
		strings.Join(diff, ", "),
	)
	return d.AddGlobal(diff...)
}

func GlobalDataPath() (string, error) {
	path := xdg.DataSubpath(filepath.Join("devbox/global", currentGlobalProfile))
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", errors.WithStack(err)
	}
	return path, nil
}

func GlobalNixProfilePath() (string, error) {
	path, err := GlobalDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "profile"), nil
}

func globalBinPath() (string, error) {
	nixProfilePath, err := GlobalNixProfilePath()
	if err != nil {
		return "", err
	}
	currentPath := xdg.DataSubpath("devbox/global/current")
	// For now default is always current. In the future we will support multiple
	// and allow user to switch.
	if err := os.Symlink(nixProfilePath, currentPath); err != nil && !os.IsExist(err) {
		return "", errors.WithStack(err)
	}
	return filepath.Join(currentPath, "bin"), nil
}

// GenerateShellEnv generates shell commands that configure the user's shell
// environment to work with Devbox packages. Most notably, it adds Devbox
// packages to the user's PATH. The commands are intended to be evaluated in
// the user's shell rcfile. For example:
//
//	echo 'eval "$(devbox global shellenv)"' >> ~/.zshrc
func GenerateShellEnv() string {
	// If the user has "eval $(devbox global shellenv)" in their shell's
	// rcfile, then running "devbox shell" will cause these commands to be
	// evaluated twice (once by the "parent" shell and once by the devbox
	// shell). Prevent this by making the eval a no-op when we're already
	// inside a devbox shell.
	if IsDevboxShellEnabled() {
		return ""
	}
	return `export PATH="${XDG_DATA_HOME:-$HOME/.local/share}/devbox/global/current/bin${PATH+:$PATH}";`
}

// Checks if the global profile is in the path
func ensureGlobalProfileInPath() error {
	binPath, err := globalBinPath()
	if err != nil {
		return err
	}
	if !strings.Contains(os.Getenv("PATH"), binPath) {
		return warningNotInPath
	}
	return nil
}