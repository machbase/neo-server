(()=>{
    "use strict";
    let _chart = echarts.init(document.getElementById('MjYwMjY0NTY1OTY2MTUxNjg_'), "white");
    let _chartOption = {
    "legend":{"show":true,"data":["test-data"]},
    "dataZoom":[{"type":"slider", "start":0, "end":100}],
    "tooltip":{"show":true, "trigger":"axis"},
    "xAxis":{"name":"time","type":"time"},
    "yAxis":{"name":"demo","type":"value"},
    "series":[
        {"type":"scatter", "name":"test-data", "data":[[1692670838086.467,0],[1692670839086.467,1],[1692670840086.467,2]]}
    ]};
    _chart.setOption(_chartOption);
    _chart.dispatchAction({"areas": {}, "type": ""});
})();