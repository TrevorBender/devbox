package shenv

import (
	"fmt"

	"go.jetpack.io/devbox/internal/shenv"
)

type elvish struct{}

// Elvish adds support for the elvish shell as a host
var Elvish Shell = elvish{}

// Dump implements Shell.
func (elvish) Dump(env shenv.Env) (out string) {
	for k, v := range env {
		out += fmt.Sprintf("set-env %s %s", k, v)
	}
	return
}

// Export implements Shell.
func (elvish) Export(e shenv.ShellExport) (out string) {
	// TODO: escape keys and values?
	for k, v := range e {
		if v == nil {
			out += fmt.Sprintf("unset-env %s;", k)
		} else {
			out += fmt.Sprintf("set-env %s %s", k, v)
		}
	}
	return
}

const elvishHook = `
fn devbox-hook {
	devbox shellenv --config "{{ .ProjectDir}}" | slurp
}
set edit:before-readline = [ $@edit:before-readline $devbox-hook~ ]
`

// Hook implements Shell.
func (elvish) Hook() (string, error) {
	return elvishHook, nil
}
