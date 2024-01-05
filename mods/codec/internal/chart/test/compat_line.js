(()=>{
    "use strict";
    let _chartID = 'WejMYXCGcYNL';
    let _chart = echarts.init(document.getElementById(_chartID), "westeros");
    let _chartOption = {
    "title":{"text":"Title", "subtext":"subtitle"},
    "animation":true, "color":["#80FFA5", "#00DDFF", "#37A2FF"],
    "legend":{"show":true,"data":["column[1]"]},
    "dataZoom":[{"type":"slider", "start":0, "end":100}],
    "visualMap":[{"type":"continuous", "calculable":true, "min":-2, "max":2, "inRange":{"color":["#a50026","#d73027","#f46d43","#fdae61","#e0f3f8","#abd9e9","#74add1","#4575b4","#313695","#313695","#4575b4","#74add1","#abd9e9","#e0f3f8","#fdae61","#f46d43","#d73027","#a50026"]}}],
    "toolbox":{ "feature":{
        "saveAsImage":{"show":true, "type":"png", "name":"test.png", "title":"save"},
        "dataZoom":{"show":true, "title":{"zoom":"zoom", "back":"back"}},
        "dataView":{"show":true, "title":"view", "lang":["Data", "Close", "Refresh"]}
    }},
    "tooltip":{"show":true, "trigger":"axis"},
    "xAxis":{"name":"time","type":"time"},
    "yAxis":{"name":"y","type":"value"},
    "series":[
        {
        "type":"line", "name":"column[1]", "data":[[1692670838086.467,-2],[1692670839086.467,-1],[1692670840086.467,0],[1692670841086.467,1],[1692670842086.467,2]],
        "markArea":{"data":[
        [{"name":"Area1", "itemStyle":{"color":"#ff000033", "opacity":0.3}, "xAxis":1692670838586.467}, {"xAxis":1692670839086.467}],[{"name":"Area2", "itemStyle":{"color":"#ff000033", "opacity":0.3}, "xAxis":1692670838686.467}, {"xAxis":1692670839286.467}]
        ]},"markLine":{"symbol":["none","none"], "data":[
        {"name":"line-X", "xAxis":1692670838286.467, "label":{"formatter":"line-X"}},{"name":"half", "yAxis":0.5, "label":{"formatter":"half"}}
        ]}
        }
    ]};
    _chart.setOption(_chartOption);
    _chart.dispatchAction({"areas": {}, "type": ""});
})();