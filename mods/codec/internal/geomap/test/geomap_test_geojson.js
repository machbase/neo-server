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
map.fitBounds([[0.5,102],[20.1,135.7]]);
var obj0 = L.geoJSON({features:[{geometry:{coordinates:[102,0.5],type:"Point"},properties:{prop0:"value0"},type:"Feature"},{geometry:{coordinates:[[102,0],[103,1],[104,0],[105,1]],type:"LineString"},properties:{prop0:"value0",prop1:0},type:"Feature"},{geometry:{coordinates:[[[100,0],[101,0],[101,1],[100,1],[100,0]]],type:"Polygon"},properties:{prop0:"value0",prop1:{this:"that"}},type:"Feature"}],popup:{content:"<b>GeoJSON</b>",open:0},type:"FeatureCollection"},opt.geojson).addTo(map);
var obj1 = L.geoJSON({geometry:{coordinates:[125.6,10.1],type:"Point"},properties:{name:"Dinagat Islands",popup:{content:"<b>Dinagat Islands</b>",open:true}},type:"Feature"},opt.geojson).addTo(map);
var obj2 = L.geoJSON({coordinates:[135.7,20.1],type:"Point"},opt.geojson).addTo(map);
})(WejMYXCGcYNL);