(()=>{
    "use strict";
    let chart = echarts.init(document.getElementById('WejMYXCGcYNL'), "westeros");
    chart.setOption({
            "xAxis": { "type": "time", "data": [1692670838086,1692670839086,1692670840086,1692670841086,1692670842086] },
            "yAxis": { "type": "value"},
            "series": [
                { "type": "line", "data": [-2,-1,0,1,2] }
            ]
        });
    chart.dispatchAction({"areas": {}, "type": ""});
})();