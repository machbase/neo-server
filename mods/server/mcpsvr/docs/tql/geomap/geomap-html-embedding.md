# Machbase Neo Embed GeoMap in HTML

Save the code below as `basic_map.tql` and we will show you how to embed the result of this TQL into web page.

```js
SCRIPT({
    $.yield({
        type:"polygon",
        coordinates:[[37,-109.05],[41,-109.03],[41,-102.05],[37,-102.05]],
        properties: {
            fill: false
        }
    });
    $.yield({
        type: "marker",
        coordinates:[38.9934,-105.5018],
        properties: {
            popup:{content: Date()}
        }
    });
    $.yield({
        type: "circleMarker",
        coordinates:[38.935,-105.520],
        properties:{ radius: 40, fillOpacity:0.4, stroke: false }
    });
})
GEOMAP()
```

## IFRAME

```html
<html>
<body>
    <iframe src="basic_map.tql" width="600" height="600"/>
</body>
</html>
```

## JSON Response

Call `.tql` script file with a custom HTTP header `X-Tql-Output: json` to produce the result in JSON instead of full HTML document, so that caller can embed the chart into any place of the HTML DOM. The `X-Tql-Output: json` header is actually equivalent to the `GEOMAP()` SINK with `geoMapJson(true)` option like `GEOMAP( geoMapJson(true)))`.

When the response of `/db/tql/basic_map.tql` is JSON, it contains required addresses of the result javascript.

```json
{
    "geomapID":"MTcwMzE3NjYwMjA0Nzg1NjY0",
    "style": {
        "width": "600px",
        "height": "600px",
        "grayscale": 0
    },
    "jsAssets": ["/web/geomap/leaflet.js"],
    "cssAssets": ["/web/geomap/leaflet.css"],
	"jsCodeAssets": [
        "/web/api/tql-assets/MTcwMzE3NjYwMjA0Nzg1NjY0_opt.js",
        "/web/api/tql-assets/MTcwMzE3NjYwMjA0Nzg1NjY0.js"
    ]
}
```

- `geomapID` random generated ID of a map, a client can set a specific ID with `geomapID()` option.
- `jsAssets` server returns the addresses of lefletjs resources. The array may contains the main echarts (`leaflet.js`) and extra plugins javascript files.
- `cssAssets` contains css assets required by lefletjs.
- `jsCodeAssets` machbase-neo generates the javascript to properly render the lefletjs with the result data.

The HTML document below is an example to utilize the JSON response above to render maps.

```html
<html>
<head>
    <link rel="stylesheet" href="/web/geomap/leaflet.css">
</head>
<body>
    <script src="/web/geomap/leaflet.js"></script>
    <div id="map_is_here"></div>
    <script>
        function loadJS(url) {
            var scriptElement = document.createElement('script');
            scriptElement.src = url;
            document.getElementsByTagName('body')[0].appendChild(scriptElement);
        }
    </script>
    <script>
        fetch("basic_map.tql", {
            headers: { "X-Tql-Output": "json" }
        }).then(function(rsp){
            return rsp.json()
        }).then(function(obj) {
            const mapDiv = document.createElement('div')
            mapDiv.setAttribute("id", obj.geomapID)
            mapDiv.style.width = obj.style.width
            mapDiv.style.height = obj.style.height
            document.getElementById('map_is_here').appendChild(mapDiv)
            obj.jsCodeAssets.forEach((js) => loadJS(js))
        }).catch(function(err){
            console.log("geomap fetch error", err)
        })
    </script>
</body>
</html>
```

- Line 3, Pre-load lefletjs style sheet which is included in `cssAssets` fields in the above JSON response.
- Line 6, Pre-load lefletjs library which is included in `jsAssets` fields in above response example.
- Line 17, The HTTP header `X-Tql-Output: json`. So that machbase-neo TQL engine generates a JSON containing meta information of map instead of full HTML document. Because when a client requests a `*.tql` file with `GET` method, machbase-neo generates HTML document by default.
- Line 26, Load js files into the HTML DOM tree that are generated and replied in `jsCodeAssets`.

## Dynamic TQL

The api `/db/tql` can receive POSTed TQL script and produces the result in javascript. Caller side javascript can load the result javascript dynamically as the example below.

In this example, the `geomapID()` (line 20) is provided and the document has a `<div>` with the same `id`.

```html
<html>
<head>
    <link rel="stylesheet" href="/web/geomap/leaflet.css">
</head>
<body id="body">
    <script src="/web/geomap/leaflet.js"></script>
    <div id="map_is_here" style="width:100%; height:100%;"/>
    <script>
        function loadJS(url) {
            var scriptElement = document.createElement('script');
            scriptElement.src = url;
            document.getElementsByTagName('body')[0].appendChild(scriptElement);
        }
    </script>
    <script>
        fetch("/db/tql", {
            method:"POST", 
            body:`
            SCRIPT({
                $.yield({
                    type:"polygon",
                    coordinates:[[37,-109.05],[41,-109.03],[41,-102.05],[37,-102.05]],
                    properties: {
                        fill: false
                    }
                });
                $.yield({
                    type: "marker",
                    coordinates:[38.9934,-105.5018],
                    properties: {
                        popup:{content: Date()}
                    }
                });
                $.yield({
                    type: "circleMarker",
                    coordinates:[38.935,-105.520],
                    properties:{ radius: 40, fillOpacity:0.4, stroke: false }
                });
            })
            GEOMAP( geomapID("map_is_here") )`
        }).then(function(rsp){
            return rsp.json()
        }).then(function(obj) {
            const mapDiv = document.getElementById('map_is_here')
            obj.jsCodeAssets.forEach((js) => loadJS(js))
        }).catch(function(err){
            console.log("geomap fetch error", err)
        })
    </script>
</body>
</html>
```

## Loading Sequence Problem

In the examples above, if we tried to load the both of `jsAssets` and `jsCodeAssets` dynamically, like below code for example.

```js
const assets = obj.jsAssets.concat(obj.jsCodeAssets)
assets.forEach((js) => loadJS(js))
```

There must be some loading sequence issue, because the lefletjs library in `obj.jsAssets` might not be completely loaded before `obj.jsCodeAssets` are loaded. To avoid the problem of loading sequence, it can be fixed like below code.

Add `load` event listener to enable callback for load-completion.

```js
function loadJS(url, callback) {
    var scriptElement = document.createElement('script');
    scriptElement.src = url;
    document.getElementsByTagName('body')[0].appendChild(scriptElement);
    scriptElement.addEventListener("load", ()=>{
        if (callback !== undefined) {
            callback()
        }
    })
}
```

When the last `jsAssets` loaded, start to load `jsCodeAssets`.

```js
for (let i = 0; i < obj.jsAssets.length; i++ ){
    if (i < obj.jsAssets.length -1){ 
        loadJS(obj.jsAssets[i])
    } else { // when the last asset file is loaded, start to load jsCodeAssets
        loadJS(obj.jsAssets[i], () => {
            obj.jsCodeAssets.forEach(js => loadJS(js)) 
        })
    }
}
```