var MTY3NzQ2MDY4NzQyNTc4MTc2 = {
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
var map;
if (opt && opt.map) {
  map = opt.map;
  opt.map.eachLayer(function (layer) {
    if (!(layer instanceof L.TileLayer)) {
      opt.map.removeLayer(layer);
    }
  });
} else {
  map = L.map("MTY3NzQ2MDY4NzQyNTc4MTc2", {crs: L.CRS.EPSG3857, attributionControl:false});
  L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
  opt.map = map;
}
opt.initBounds = [[48.855746990243716,2.288226120523035],[48.862234983151495,2.2968403487010107]];
map.fitBounds(opt.initBounds);
var obj0 = L.geoJSON({geometry:{coordinates:[[[[2.291863239086439,48.8577137262115],[2.293452085617105,48.856693553273885],[2.2968403487010107,48.85892279314069],[2.2951175030651143,48.86006886087142],[2.291863239086439,48.8577137262115]]],[[[2.288226120523035,48.86156752523257],[2.2899681088877344,48.86042149181674],[2.290810388976098,48.86063558796482],[2.2909826735397587,48.8611015587675],[2.28947039792655,48.862234983151495],[2.288226120523035,48.86156752523257]]],[[[2.2912927602678224,48.85709062155263],[2.2905402133688426,48.85661663833349],[2.291917551492446,48.855746990243716],[2.2926328654095016,48.85624492205244],[2.2912927602678224,48.85709062155263]]]],type:"MultiPolygon"},type:"Feature"},opt.geojson).addTo(map);
})(MTY3NzQ2MDY4NzQyNTc4MTc2);