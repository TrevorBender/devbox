{{- /*

This template defines the shellrc file that the devbox shell will run at
startup when using the elvish shell.

*/ -}}

{{- if .OriginalInitPath -}}
try {
    eval (cat "{{ .OriginalInitPath }}" | slurp)
} catch e {
    echo "error sourcing original path: " $e
}
{{ end -}}

# Begin Devbox Post-init Hook

# TODO: convert export env to elvish syntax
{{ with .ExportEnv }}
{{ . }}
{{- end }}

{{- /*
TODO: elvish history file
*/ -}}
{{- if .HistoryFile }}
{{- end }}

# Prepend to the prompt to make it clear we're in a devbox shell.
# Not needed with starship
# TODO: general support

{{- if .ShellStartTime }}
# log that the shell is ready now!
devbox log shell-ready {{ .ShellStartTime }}
{{ end }}

# End Devbox Post-init Hook

# Switch to the directory where devbox.json config is
var workingDir = (pwd)
cd "{{ .ProjectDir }}"

# Source the hooks file, which contains the project's init hooks and plugin hooks.
eval (cat "{{ .HooksFilePath }}" | slurp)

cd $workingDir

{{- if .ShellStartTime }}
# log that the shell is interactive now!
devbox log shell-interactive {{ .ShellStartTime }}
{{ end }}

# TODO: Add refresh alias (only if it doesn't already exist)
# if not type refresh >/dev/null 2>&1
#     alias refresh='eval (devbox shellenv | string collect)'
#     export DEVBOX_REFRESH_ALIAS="refresh"
# end
