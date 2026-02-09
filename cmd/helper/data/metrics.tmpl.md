# Metrics

This document lists all the metrics exported by Akvorado. You can access them
through the API for each component: `/api/v0/inlet/metrics`,
`/api/v0/oulet/metrics`, etc.

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics
​# HELP akvorado_cmd_info Akvorado build information
​# TYPE akvorado_cmd_info gauge
akvorado_cmd_info{compiler="go1.24.4",version="v1.11.5-134-gaf3869cd701c"} 1
[...]
```

{{ range $s := . }}
## {{ $s.Prefix }}

| Name | Type | Help |
|------|------|------|
{{ range .Metrics -}}
| {{ formatMetricName .Name $s.Prefix }} | {{ .Type }} | {{ .Help }} |
{{ end -}}
{{ end -}}
