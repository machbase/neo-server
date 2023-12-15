package geomap

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
<div class="geomap_container">
    <div class="geomap_item" id="{{ .MapID }}" style="width:{{ .Width }};height:{{ .Height }};"></div>
</div>

<script type="text/javascript">
    "use strict";
    {{- range .JSCodes }}
    {{ . | safeJS }}
    {{- end }}
</script>
{{ end }}
`

var HtmlTemplate = `{{- define "geomap" }}<!DOCTYPE html>
<html>
    {{- template "header" . }}
<body>
    {{- template "base" . }}
<style>
    .geomap_container {margin-top:30px; display: flex;justify-content: center;align-items: center;}
    .geomap_item {margin: auto;}
	.leaflet-tile-pane{ -webkit-filter: grayscale({{ .TileGrayscale }}%); filter: grayscale({{ .TileGrayscale }}%);}
</style>
</body>
</html>
{{ end }}
`

var JsonTemplate = `
{{- define "geomap" }}
{
    "ID":"{{ .MapID }}",
    "style": {
        "width": "{{ .Width }}",
        "height": "{{ .Height }}",
        "grayscale": {{ .TileGrayscale }}
    },
    "jsAssets": {{ .JSAssetsNoEscaped }},
    "cssAssets": {{ .CSSAssetsNoEscaped }},
    "view": {
        "center": {{ .InitialLatLon }},
        "zoomLevel": {{ .InitialZoomLevel }}
    },
    "tile": {
        "template": "{{ .TileTemplate }}",
        "option": {{ .TileOptionNoEscaped }}
    },
    "icons": {{.IconsNoEscaped }},
    "layers": {{ .LayersNoEscaped }}
}
{{ end }}
`