package geomapjs

import "fmt"

const mapOptionsPopupOnly = `var %s = {
    geojson: {
        pointToLayer: function (feature, latlng) {
            if (feature.properties && feature.properties.icon) {
                return L.marker(latlng, {icon: feature.properties.icon});
            }
            return L.circleMarker(latlng, {
                radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 10,
                stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
                color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
                opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
                fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
            });
        },
        style: function (feature) {
            return {
                radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 4,
                stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
                weight: (feature.properties && feature.properties.weight) ? feature.properties.weight : 3,
                color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
                opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
                fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
            };
        },
        onEachFeature: function (feature, layer) {
            if (feature.properties && feature.properties.popup && feature.properties.popup.content) {
                if (feature.properties.popup.open) {
                    layer.bindPopup(feature.properties.popup.content).openPopup();
                } else {
                    layer.bindPopup(feature.properties.popup.content);
                }
            }
        },
    },
};
`

const geoJSONOptionsWithTooltip = `{
    pointToLayer: function (feature, latlng) {
        if (feature && feature.properties && feature.properties.icon) {
            return L.marker(latlng, { icon: feature.properties.icon });
        }
        return L.circleMarker(latlng, {
            radius: (feature && feature.properties && feature.properties.radius) ? feature.properties.radius : 10,
            stroke: (feature && feature.properties && feature.properties.stroke !== undefined) ? feature.properties.stroke : true,
            color: (feature && feature.properties && feature.properties.color) ? feature.properties.color : '#3388ff',
            opacity: (feature && feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
            fillOpacity: (feature && feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
        });
    },
    style: function (feature) {
        return {
            radius: (feature && feature.properties && feature.properties.radius) ? feature.properties.radius : 4,
            stroke: (feature && feature.properties && feature.properties.stroke !== undefined) ? feature.properties.stroke : true,
            weight: (feature && feature.properties && feature.properties.weight) ? feature.properties.weight : 3,
            color: (feature && feature.properties && feature.properties.color) ? feature.properties.color : '#3388ff',
            opacity: (feature && feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
            fillOpacity: (feature && feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
        };
    },
    onEachFeature: function (feature, layer) {
        if (feature && feature.properties && feature.properties.popup && feature.properties.popup.content) {
            if (feature.properties.popup.open) {
                layer.bindPopup(feature.properties.popup.content).openPopup();
            } else {
                layer.bindPopup(feature.properties.popup.content);
            }
        }
        if (feature && feature.properties && feature.properties.tooltip && feature.properties.tooltip.content) {
            if (feature.properties.tooltip.open) {
                layer.bindTooltip(feature.properties.tooltip.content).openTooltip();
            } else {
                layer.bindTooltip(feature.properties.tooltip.content);
            }
        }
    }
}`

func MapOptionsVarScript(varName string, includeTooltip bool) string {
	if !includeTooltip {
		return fmt.Sprintf(mapOptionsPopupOnly, varName)
	}
	return fmt.Sprintf("var %s = {\n    geojson: %s,\n};\n", varName, geoJSONOptionsWithTooltip)
}

func GeoJSONOptionsObjectLiteral(includeTooltip bool) string {
	if includeTooltip {
		return geoJSONOptionsWithTooltip
	}
	return `{
    pointToLayer: function (feature, latlng) {
        if (feature.properties && feature.properties.icon) {
            return L.marker(latlng, {icon: feature.properties.icon});
        }
        return L.circleMarker(latlng, {
            radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 10,
            stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
            color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
            opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
            fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
        });
    },
    style: function (feature) {
        return {
            radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 4,
            stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
            weight: (feature.properties && feature.properties.weight) ? feature.properties.weight : 3,
            color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
            opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
            fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
        };
    },
    onEachFeature: function (feature, layer) {
        if (feature.properties && feature.properties.popup && feature.properties.popup.content) {
            if (feature.properties.popup.open) {
                layer.bindPopup(feature.properties.popup.content).openPopup();
            } else {
                layer.bindPopup(feature.properties.popup.content);
            }
        }
    }
}`
}
