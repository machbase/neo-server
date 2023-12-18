package chart

var ChartJsonTemplate = `
{{- define "chart" }}
{
    "chartID":"{{ .ChartID }}",
    {{ $len := len .JSAssets }} {{ if gt $len 0 }}
    "jsAssets": {{ .JSAssetsNoEscaped }},
    {{ end }}
    {{ $len := len .CSSAssets }} {{ if gt $len 0 }}
    "cssAssets" : {{ .JSAssetsNoEscaped }},
    {{ end }}
	{{ $len := len .JSCodeAssets }} {{ if gt $len 0 }}
	"jsCodeAssets": {{ .JSCodeAssetsNoEscaped }},
	{{ end }}
    "style": {
        "width": "{{ .Width }}",
        "height": "{{ .Height }}"	
    },
    "theme": "{{ .Theme }}"
}
{{ end }}
`

var HeaderTemplate = `
{{ define "header" }}
<head>
    <meta charset="utf-8">
    <title>{{ .PageTitle }}</title>
{{- range .JSAssets }}
    <script src="{{ . }}"></script>
{{- end }}
{{- range .CSSAssets }}
    <link href="{{ . }}" rel="stylesheet">
{{- end }}
    <style>
        .chart_container {margin-top:30px; display: flex;justify-content: center;align-items: center;}
        .chart_item {margin: auto;}
    </style>
</head>
{{ end }}
`

var BaseTemplate = `
{{- define "base" }}
<div class="chart_container">
    <div class="chart_item" id="{{ .ChartID }}" style="width:{{ .Width }};height:{{ .Height }};"></div>
</div>
{{- range .JSCodeAssets }}
<script src="{{ . }}"></script>
{{- end }}
{{ end }}
`

var ChartTemplate = `{{- define "chart" }}<!DOCTYPE html>
<html>
    {{- template "header" . }}
<body>
    {{- template "base" . }}
</body>
</html>
{{ end }}
`
