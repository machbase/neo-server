var WejMYXCGcYNL = {
    defaultPointStyle: {radius: 4, stroke: false, color: "#FF0000", opacity: 0.7, fillOpacity: 0.7},
    geojson: {
        pointToLayer: function (feature, latlng) {
            if (feature.properties && feature.properties.icon) {
                return L.marker(latlng, {icon: feature.properties.icon});
            }
            return L.circleMarker(latlng, {radius: 4, stroke: false, color: "#FF0000", opacity: 0.7, fillOpacity: 0.7});
        },
        style: function (feature) {
            if (feature.properties && feature.properties.style) {
                return feature.properties.style;
            }
            return {radius: 4, stroke: false, color: "#FF0000", opacity: 0.7, fillOpacity: 0.7};
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
obj0.bindPopup("<b>Gangname</b><br/>Hello World?").openPopup();
var obj1 = L.circleMarker([37.503058,127.018666],{radius:100}).addTo(map);
obj1.bindPopup("<b>circle1</b>");
})(WejMYXCGcYNL);