(()=>{
    "use strict";
    let _chartID = 'zmsXewYeZOqW';
    let _chart = echarts.init(document.getElementById(_chartID), "westerose");
    let _chartOption = {
    "xAxis3D":{"name":"time","type":"time","show":true},
    "yAxis3D":{"name":"demo","type":"value","show":true},
    "zAxis3D":{"name":"z","type":"value","show":true},
    "grid3D":{"boxWidth":100, "boxHeight":100, "boxDepth":100, "viewControl":{"projection": "orthographic", "autoRotate":false,"speed":0}},
    "title":{"text":"Title", "subtext":"subtitle"},
    "tooltip":{"show":true, "trigger":"axis"},
    "series":[
    {"type":"line3D","coordinateSystem":"cartesian3D","data":[[1692670838086.467,0,0],[1692670839086.467,1,1],[1692670840086.467,2,2]],"shading":"lambert","lineStyle":{"opacity":1,"width":1}}
    ]};
    _chart.setOption(_chartOption);
    _chart.dispatchAction({"areas": {}, "type": ""});
})();