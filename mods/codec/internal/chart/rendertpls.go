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
	"chartOption": {{ .ChartOptionNoEscaped }}
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
    let tqlecharts_{{ .ChartID | safeJS }} = echarts.init(document.getElementById('{{ .ChartID | safeJS }}'), "{{ .Theme }}");
    let option_{{ .ChartID | safeJS }} = {{ .ChartOptionNoEscaped | safeJS }};
    {{ if isSet  "ChartActions" . }}
	let action_{{ .ChartID | safeJS }} = {{ .ChartActionsNoEscaped | safeJS }};
    {{ end }}
    tqlecharts_{{ .ChartID | safeJS }}.setOption(option_{{ .ChartID | safeJS }});
 	tqlecharts_{{ .ChartID | safeJS }}.dispatchAction(action_{{ .ChartID | safeJS }});

    {{- range .JSFunctions }}
    {{ . | safeJS }}
    {{- end }}
</script>
{{ end }}
`

var ChartTemplate = `
{{- define "chart" }}
<!DOCTYPE html>
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

var HeaderTpl = `
{{ define "header" }}
<head>
    <meta charset="utf-8">
    <title>{{ .PageTitle }}</title>
{{- range .JSAssets.Values }}
    <script src="{{ . }}"></script>
{{- end }}
{{- range .CustomizedJSAssets.Values }}
    <script src="{{ . }}"></script>
{{- end }}
{{- range .CSSAssets.Values }}
    <link href="{{ . }}" rel="stylesheet">
{{- end }}
{{- range .CustomizedCSSAssets.Values }}
    <link href="{{ . }}" rel="stylesheet">
{{- end }}
</head>
{{ end }}
`

var BaseTpl = `
{{- define "base" }}
<div class="chart_container">
    <div class="chart_item" id="{{ .ChartID }}" style="width:{{ .Initialization.Width }};height:{{ .Initialization.Height }};"></div>
</div>

<script type="text/javascript">
    "use strict";
    let goecharts_{{ .ChartID | safeJS }} = echarts.init(document.getElementById('{{ .ChartID | safeJS }}'), "{{ .Theme }}");
    let option_{{ .ChartID | safeJS }} = {{ .JSONNotEscaped | safeJS }};
    {{ if isSet  "BaseActions" . }}
	let action_{{ .ChartID | safeJS }} = {{ .JSONNotEscapedAction | safeJS }};
    {{ end }}
    goecharts_{{ .ChartID | safeJS }}.setOption(option_{{ .ChartID | safeJS }});
 	goecharts_{{ .ChartID | safeJS }}.dispatchAction(action_{{ .ChartID | safeJS }});

    {{- range .JSFunctions.Fns }}
    {{ . | safeJS }}
    {{- end }}
</script>
{{ end }}
`

var ChartTpl = `
{{- define "chart" }}
<!DOCTYPE html>
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

var ChartJsonTpl = `
{{- define "chart" }}
{
	"chartID":"{{ .ChartID }}",
	"style": {
		"width": "{{ .Initialization.Width }}",
		"height": "{{ .Initialization.Height }}"	
	},
	"theme": "{{ .Theme }}",
	"chartOption": {{ .JSONNotEscaped }}
}
{{ end }}
`

var PageTpl = `
{{- define "page" }}
<!DOCTYPE html>
<html>
    {{- template "header" . }}
<body>
{{ if eq .Layout "none" }}
    {{- range .Charts }} {{ template "base" . }} {{- end }}
{{ end }}

{{ if eq .Layout "center" }}
    <style> .container {display: flex;justify-content: center;align-items: center;} .item {margin: auto;} </style>
    {{- range .Charts }} {{ template "base" . }} {{- end }}
{{ end }}

{{ if eq .Layout "flex" }}
    <style> .box { justify-content:center; display:flex; flex-wrap:wrap } </style>
    <div class="box"> {{- range .Charts }} {{ template "base" . }} {{- end }} </div>
{{ end }}
</body>
</html>
{{ end }}
`
