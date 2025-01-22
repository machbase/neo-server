var WejMYXCGcYNL = {
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
((opt)=>{
var map = L.map("WejMYXCGcYNL", {crs: L.CRS.EPSG3857, attributionControl:false});
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
map.fitBounds([[37.49785,127.018666],[37.503058,127.027756]]);
var obj0 = L.marker([37.49785,127.027756],{}).addTo(map);
var popup0 = obj0.bindPopup("<b>Gangname</b><br/>Hello World?", {}).openPopup();
var obj1 = L.circleMarker([37.503058,127.018666],{radius:100}).addTo(map);
var popup1 = obj1.bindPopup("<b>circle1</b>", {});
})(WejMYXCGcYNL);