var MTY3NzQ2MDY4NzQyNTc4MTc2 = {
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
var map = L.map("MTY3NzQ2MDY4NzQyNTc4MTc2", {crs: L.CRS.EPSG3857, attributionControl:false});
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
map.setView([37.49785,127.027756],13);
var obj0 = L.geoJSON({geometry:{coordinates:[127.027756,37.49785],type:"Point"},type:"Feature"},opt.geojson).addTo(map);
})(MTY3NzQ2MDY4NzQyNTc4MTc2);