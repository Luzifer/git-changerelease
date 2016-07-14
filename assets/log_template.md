# {{ .NextVersion }} / {{ .Now.Format "2006-01-02" }}
{{ range $line := .LogLines }}
  * {{ $line.Subject }}
{{- end }}

{{ .OldLog }}
