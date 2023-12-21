(()=>{
    var map = L.map("WejMYXCGcYNL", {crs: L.CRS.EPSG3857, attributionControl:false});
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png").addTo(map);
    var __ptstyle = {color:"#2020F0",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
    var rec = {color:"#ff0000",fillOpacity:0.5,opacity:0.5,radius:4,stroke:false};
    var obj0 = L.marker([37.49785,127.027756], {}).addTo(map);
    obj0.bindPopup("<b>Gangname</b><br/>Hello World?");
    obj0.openPopup();
    var obj1 = L.circleMarker([37.503058,127.018666], {radius:100}).addTo(map);
    obj1.bindPopup("<b>circle1</b>");
    var obj2 = L.circleMarker([37.496727,127.026612], __ptstyle).addTo(map);
    obj2.bindPopup("<b>point1</b>");
    map.fitBounds([[37.496727,127.018666],[37.503058,127.027756]]);
})();