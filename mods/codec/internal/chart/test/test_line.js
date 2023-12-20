(()=>{
    "use strict";
    let chart = echarts.init(document.getElementById('WejMYXCGcYNL'), "white");
    chart.setOption({
                "xAxis": { "type": "time", "data": [1692670838086.467,1692670839086.467,1692670840086.467] },
                "yAxis": { "type": "value"},
                "series": [
                    { "type": "line", "data": [0,1,2] }
                ]
            });
    chart.dispatchAction({"areas": {}, "type": ""});
})();