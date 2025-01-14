(()=>{
var map = L.map("WejMYXCGcYNL", {crs: L.CRS.EPSG3857, attributionControl:false});
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
var __ptstyle = {color:"#2020F0",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
var rec = {color:"#ff0000",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
L.geoJSON({"features":[{"geometry":{"coordinates":[102,0.5],"type":"Point"},"properties":{"prop0":"value0"},"type":"Feature"},{"geometry":{"coordinates":[[102,0],[103,1],[104,0],[105,1]],"type":"LineString"},"properties":{"prop0":"value0","prop1":0},"type":"Feature"},{"geometry":{"coordinates":[[[100,0],[101,0],[101,1],[100,1],[100,0]]],"type":"Polygon"},"properties":{"prop0":"value0","prop1":{"this":"that"}},"type":"Feature"}],"type":"FeatureCollection"}).addTo(map);
L.geoJSON({"geometry":{"coordinates":[125.6,10.1],"type":"Point"},"properties":{"name":"Dinagat Islands"},"type":"Feature"}).addTo(map);
L.geoJSON({"coordinates":[135.7,20.1],"type":"Point"}).addTo(map);
map.fitBounds([[0,100],[20.1,135.7]]);
})();