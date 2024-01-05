(()=>{
    "use strict";
    const _column_0=[1692670838086.467,1692670839086.467,1692670840086.467];
    const _column_1=[0,1,2];
    const _columns=[_column_0,_column_1];
    function column(idx) { return _columns[idx]; }
    let _chart = echarts.init(document.getElementById('WejMYXCGcYNL'), "white");
    let _chartOption = {
                "xAxis": { "type": "time", "data": _column_0 },
                "yAxis": { "type": "value"},
                "series": [
                    { "type": "line", "data": _column_1 }
                ]
            };
    _chart.setOption(_chartOption);
    _chart.dispatchAction({"areas": {}, "type": ""});
})();