package chart

var ChartJsonTemplate = `
{{- define "chart" }}
{
    "chartID":"{{ .ChartID }}",
    "style": {
        "width": "{{ .Width }}",
        "height": "{{ .Height }}"	
    },
    "theme": "{{ .Theme }}",
    {{ $len := len .JSAssets }} {{ if gt $len 0 }}
    "jsAssets": {{ .JSAssetsNoEscaped }},
    {{ end }}
    {{ $len := len .CSSAssets }} {{ if gt $len 0 }}
    "cssAssets" : {{ .JSAssetsNoEscaped }},
    {{ end }}
	{{ $len := len .JSCodeAssets }} {{ if gt $len 0 }}
	"jsCodeAssets": {{ .JSCodeAssetsNoEscaped }},
	{{ end }}
    "chartOption": {{ .ChartOptionNoEscaped }},
	"chartAction": {{ .ChartDispatchActionNoEscaped }}
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
</head>
{{ end }}
`

var BaseTemplate = `
{{- define "base" }}
<div class="chart_container">
    <div class="chart_item" id="{{ .ChartID }}" style="width:{{ .Width }};height:{{ .Height }};"></div>
</div>

<script type="text/javascript">
    "use strict";
    let {{ .ChartID | safeJS }} = echarts.init(document.getElementById('{{ .ChartID | safeJS }}'), "{{ .Theme }}");
    let option_{{ .ChartID | safeJS }} = {{ .ChartOptionNoEscaped | safeJS }};
    let action_{{ .ChartID | safeJS }} = {{ .ChartDispatchActionNoEscaped | safeJS }};
    {{ .ChartID | safeJS }}.setOption(option_{{ .ChartID | safeJS }});
    {{ .ChartID | safeJS }}.dispatchAction(action_{{ .ChartID | safeJS }});

    {{- range .JSCodes }}
    {{ . | safeJS }}
    {{- end }}
</script>
{{ end }}
`

var ChartTemplate = `{{- define "chart" }}<!DOCTYPE html>
<html>
    {{- template "header" . }}
<body>
    {{- template "base" . }}
<style>
    .chart_container {margin-top:30px; display: flex;justify-content: center;align-items: center;}
    .chart_item {margin: auto;}
</style>
</body>
</html>
{{ end }}
`
