// Copyright 2024 Jetify Inc. and contributors. All rights reserved.
// Use of this source code is governed by the license in the LICENSE file.

package devbox

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"go.jetpack.io/devbox/internal/devbox/envpath"
	"go.jetpack.io/devbox/internal/envir"
)

const devboxSetPrefix = "__DEVBOX_SET_"

// exportify formats vars as a line-separated string of shell export statements.
// Each line is of the form `export key="value";` with any special characters in
// value escaped. This means that the shell will always interpret values as
// literal strings; no variable expansion or command substitution will take
// place.
func exportify(vars map[string]string) string {
	keys := make([]string, len(vars))
	i := 0
	for k := range vars {
		keys[i] = k
		i++
	}
	slices.Sort(keys) // for reproducibility

	strb := strings.Builder{}
	for _, k := range keys {
		strb.WriteString("export ")
		strb.WriteString(k)
		strb.WriteString(`="`)
		for _, r := range vars[k] {
			switch r {
			// Special characters inside double quotes:
			// https://pubs.opengroup.org/onlinepubs/009604499/utilities/xcu_chap02.html#tag_02_02_03
			case '$', '`', '"', '\\', '\n':
				strb.WriteRune('\\')
			}
			strb.WriteRune(r)
		}
		strb.WriteString("\";\n")
	}
	return strings.TrimSpace(strb.String())
}

func exportifyElvish(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := bytes.NewBuffer(nil)
	for _, k := range keys {
		fmt.Fprintf(buf, "set-env %s \"", k)
		for _, r := range vars[k] {
			switch r {
			// TODO: escape special characters
			case '"':
				fmt.Fprint(buf, "\\")
			}
			fmt.Fprintf(buf, "%c", r)
		}
		fmt.Fprint(buf, "\"\n")
	}
	return strings.TrimSpace(buf.String())
}

// addEnvIfNotPreviouslySetByDevbox adds the key-value pairs from new to existing,
// but only if the key was not previously set by devbox
// Caveat, this won't mark the values as set by devbox automatically. Instead,
// you need to call markEnvAsSetByDevbox when you are done setting variables.
// This is so you can add variables from multiple sources (e.g. plugin, devbox.json)
// that may build on each other (e.g. PATH=$PATH:...)
func addEnvIfNotPreviouslySetByDevbox(existing, new map[string]string) {
	for k, v := range new {
		if _, alreadySet := existing[devboxSetPrefix+k]; !alreadySet {
			existing[k] = v
		}
	}
}

func markEnvsAsSetByDevbox(envs ...map[string]string) {
	for _, env := range envs {
		for key := range env {
			env[devboxSetPrefix+key] = "1"
		}
	}
}

// IsEnvEnabled checks if the devbox environment is enabled.
// This allows us to differentiate between global and
// individual project shells.
func (d *Devbox) IsEnvEnabled() bool {
	fakeEnv := map[string]string{}
	// the Stack is initialized in the fakeEnv, from the state in the real os.Environ
	pathStack := envpath.Stack(fakeEnv, envir.PairsToMap(os.Environ()))
	return pathStack.Has(d.ProjectDirHash())
}

func (d *Devbox) SkipInitHookEnvName() string {
	return "__DEVBOX_SKIP_INIT_HOOK_" + d.ProjectDirHash()
}
