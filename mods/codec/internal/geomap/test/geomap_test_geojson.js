(()=>{
var map = L.map("WejMYXCGcYNL", {crs: L.CRS.EPSG3857, attributionControl:false});
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
map.fitBounds([[0,100],[20.1,135.7]]);
var __ptstyle = {color:"#2020F0",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
var rec = {color:"#ff0000",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
var obj0 = L.geoJSON({"features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[102,0.5]},"properties":{"prop0":"value0"}},{"type":"Feature","geometry":{"type":"LineString","coordinates":[[102,0],[103,1],[104,0],[105,1]]},"properties":{"prop0":"value0","prop1":0}},{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[100,0],[101,0],[101,1],[100,1],[100,0]]]},"properties":{"prop0":"value0","prop1":{"this":"that"}}}],"type":"FeatureCollection"}, {}).addTo(map);
obj0.bindPopup("<b>GeoJSON</b>");
var obj1 = L.geoJSON({"type":"Feature","geometry":{"type":"Point","coordinates":[125.6,10.1]},"properties":{"name":"Dinagat Islands"}}, {}).addTo(map);
obj1.bindPopup("<b>Dinagat Islands</b>").openPopup();
var obj2 = L.geoJSON({"coordinates":[135.7,20.1],"type":"Point"}, {}).addTo(map);
})();