# Machbase Neo GEOMAP()

**Available since version 8.0.44**

**Syntax**: `GEOMAP( [geomapID()] [, tileTemplate()] [, size()] )` 

`GEOMAP` generates a map display and shows markers and geometric shapes based on provided coordinates.
It functions similarly to `CHART`, but it uses coordinates instead of scalar values. The supported coordinates system is [WGS84](https://en.wikipedia.org/wiki/World_Geodetic_System).

The `GEOMAP()` function processes input data in JavaScript Object format.
Each input object must include `type` and `coordinates` fields, with an optional `properties` field.

A layer in the `GEOMAP()` function is an object that is rendered on the map according to its specified type.
For example, a layer with the type `circle` will display a circle on the map based on the provided properties.

## tileTemplate()

**Syntax**: `tileTemplate(url_template)`

The map tile server url template.
The default is `https://tile.openstreetmap.org/{z}/{x}/{y}.png`.

**Important:** If the map clients (web browsers) cannot access the default tile server due to the firewall and organization's security policy, you will need to run your own tile server inside your organization and set the tile server URL using `tileTemplate()`. Instructions on how to run a tile server are beyond the scope of this document. Please refer to the following for more information about the tile server: https://wiki.openstreetmap.org/wiki/Tile_servers

## tileGrayscale()

**Syntax**: `tileGrayscale(scale)`

- `scale` *float* Set the gray scale of the tile image it should be 0 ≤ scale ≤ 1.0. (Default: `0`)

## geomapID()

**Syntax**: `geomapID(id)`

If you need to specify the map id (*string*) instead of auto-generated one.

## size()

**Syntax**: `size(width, height)`

- `width` *string* map width in HTML syntax ex) `'800px'`
- `height` *string* map height in HTML syntax ex) `'800px'`

## Layers

Layers are markers and geometric shapes that `GEOMAP` shows on the map.
The input data of `GEOMAP()` should be a dictionary structure represented as a JavaScript object.

The object must have `type` and `coordinates` fields, with an optional `properties` field.

**syntax**

```js
{
    type: "circle", // marker, circleMarker, polyline ...
    coordinates: [Lat, Lon],
    properties: {
        radius: Radius,
        color: "#FF0000",
        weight: 1
    }
}
```

| Name            | Type          | Description   |
|:--------------- |:--------------|:--------------|
| `type`          | `String`      | Type of the layer. <br/> e.g., `marker`, `circle`, `circleMarker`, etc. |
| `coordinates`   | `[]Float`,<br/> `[][]Float`, ... | Coordinates for the `type` in [latitude, longitude] order |
| `properties`    | `Dictionary` | Various options depending on the `type`.<br/>See [Properties](#properties) |

## marker

```js
FAKE(json({
    [38.9934, -105.5018]
}))

SCRIPT({
    var lat = $.values[0];
    var lon = $.values[1];
    $.yield({
        type: "marker",
        coordinates: [lat, lon]
    });
})

GEOMAP()
```

## circleMarker

**Properties**

| Property        | Default          | Description   |
|:--------------- |:-----------------|:--------------|
| `radius`        | 10               | Radius of the circle marker, in pixels. |

```js
FAKE(json({
    [38.935, -105.520]
}))

SCRIPT({
    var lat = $.values[0];
    var lon = $.values[1];
    $.yield({
        type: "circleMarker",
        coordinates: [lat, lon],
        properties:{
            radius: 40
        }
    });
})

GEOMAP()
```

## circle

**Properties**

| Property        | Default          | Description   |
|:--------------- |:-----------------|:--------------|
| `radius`        | 10               | Radius of the circle, in meters. |

```js
FAKE(json({
    [38.935, -105.520]
}))

SCRIPT({
    var lat = $.values[0];
    var lon = $.values[1];
    $.yield({
        type: "circle",
        coordinates: [lat, lon],
        properties:{
            radius: 400
        }
    });
})

GEOMAP()
```

## polyline

```js
FAKE(json({
    [45.51, -122.68],
    [37.77, -122.43],
    [34.04, -118.2]
}))

SCRIPT({
    var points = [];
    function finalize() {
        $.yield({
            type: "polyline",
            coordinates: points
        });
    }
},{
    var lat = $.values[0];
    var lon = $.values[1];
    points.push( [lat, lon] );
})

GEOMAP()
```

## polygon

```js
FAKE(json({
    [37, -109.05],
    [41, -109.03],
    [41, -102.05],
    [37, -102.05]
}))

SCRIPT({
    var points = [];
    function finalize() {
        $.yield({
            type: "polygon",
            coordinates: points
        });
    }
},{
    var lat = $.values[0];
    var lon = $.values[1];
    points.push( [lat, lon] );
})

GEOMAP()
```

## Properties

## Layer Properties

| Property        | Type   | Default   | Description   |
|:--------------- |:-------|:----------|:--------------|
| `stroke`        | Boolean| `true`    | Whether to draw stroke along the path. Set it to false to disable borders on polygons or circles. |
| `color`         | String | `'#3388ff'` | Stroke color  |
| `weight`        | Number | `3`       | Stroke width in pixels |
| `opacity`       | Number | `1.0`     | The opacity of the marker.|
| `fillColor`     | String |           | Fill color. Defaults to the value of the color property. |
| `fillOpacity`   | Number | `0.2`     | Fill opacity. |
| `popup`         | Object | `null`    | See [Popup](#popup). |
| `tooltip`       | Object | `null`    | See [Tooltip](#tooltip). |

## Popup

If layer properties has `popup` object it displays popup message when user click the layer.

| Property        | Type   | Default    | Description   |
|:--------------- |:-------|:-----------|:--------------|
| `content`       | String |            | The content of the popup in Text/HTML. |
| `open`          | Boolean| `false`    | Set initial open state |
| `maxWidth`      | Number | `300`      | Max width of the popup, in pixels. |
| `minWidth`      | Number | `50`       | Min width of the popup, in pixels. |

```js
FAKE(json({
    ["Stoll Mountain", 38.9934, -105.5018],
    ["Pulver Mountain", 39.0115, -105.5173]
}))

SCRIPT({
    var name = $.values[0];
    var lat  = $.values[1];
    var lon  = $.values[2];
    $.yield({
        type: "marker",
        coordinates: [lat, lon],
        properties: {
            popup: {
                content: '<b>'+name+'</b>'
            }
        }
    });
})

GEOMAP()
```

## Tooltip

**Available since version 8.0.44**

Used to display small texts on top of map layers.

| Property         | Type   | Default    | Description   |
|:--------------- |:-------|:-----------|:--------------|
| `content`       | String |            | The content of the popup in Text/HTML. |
| `open`          | Boolean| `false`    | Set initial open state |
| `direction`     | String | `auto`     | Direction where to open the tooltip. `right,left,top,bottom,center,auto` |
| `permanent`     | Boolean| `false`    | Whether to open the tooltip permanently or only on mouseover |
| `opacity`       | Number | `0.9`      | Tooltip container opacity |

```js
FAKE(json({
    ["Stoll Mountain", 38.9934, -105.5018],
    ["Pulver Mountain", 39.0115, -105.5173]
}))
SCRIPT({
    var name = $.values[0];
    var lat  = $.values[1];
    var lon  = $.values[2];
    $.yield({
        type: "marker",
        coordinates: [lat, lon],
        properties: {
            tooltip: {
                content: '<b>'+name+'</b>',
                direction: "auto",
                permanent: true
            }
        }
    });
})
GEOMAP()
```

## Examples

Load test data from a CSV file and insert it into the "TRIP" table.
This TQL downloads the CSV file from the given URL, 
converts the CSV strings into the appropriate data types,
and inserts the records into the TRIP table.

```js
// CSV Format: TIME, LAT, LON
CSV(file("https://docs.machbase.com/assets/example/data-trajectory-firenze.csv"))
DROP(1) // skip header
SCRIPT({
    // create trip table, if not exists
    $.db().exec("CREATE TAG TABLE IF NOT EXISTS TRIP ("+
        "name varchar(100) primary key, "+
        "time datetime basetime, "+
        "value double summarized, "+
        "lat double, "+
        "lon double "+
    ")")
    // parse time form csv string '23-04-21 16:53:21:123000'
    function parseTime(str) { 
        y = "20"+str.substr(0,2);
        m = str.substr(3,2) - 1;
        d = str.substr(6,2);
        hours = str.substr(9, 2);
        mins = str.substr(12,2);
        secs = str.substr(15, 2);
        milli = str.substr(18, 3)
        var D = new Date(y, m, d, hours, mins, secs, milli);
        return (D.getFullYear() == y && D.getMonth() == m && D.getDate() == d) ? D : 'invalid date';
    }
}, {
    var ts = parseTime($.values[0]).getTime(); // epoch mills
    var lat = parseFloat($.values[1]);
    var lon = parseFloat($.values[2]);
    // yield name, time, value, lat, lon
    $.yield("firenze", ts, 0, lat, lon)
})
// epoch from milli to nano and to datetime type
MAPVALUE(1, time(value(1)*1000000))
// insert into trip table
INSERT("name", "time", "value", "lat", "lon", table("TRIP"))
```

## Trajectory

### SQL

```js
SQL(`SELECT time, lat, lon FROM TRIP
     WHERE name = 'firenze' ORDER BY time`)
SCRIPT({
    // time to epoch nanos to Date (javascript)
    var timestamp = new Date($.values[0].UnixNano()/1000000); 
    // coordinate [lat, lon]
    var coord = [$.values[1], $.values[2]]; 
    $.yield({
        type:"circle",
        coordinates: coord,
        properties: {
            radius: 15,
            tooltip: {
                content: ""+timestamp
            }
        }
    });
})
GEOMAP()
```

### CSV

```js
// CSV Format: TIME, LAT, LON
CSV(file("https://docs.machbase.com/assets/example/data-trajectory-firenze.csv"))

DROP(1) // skip header

SCRIPT({
    var timestamp = $.values[0];
    var coord = [
        parseFloat($.values[1]), 
        parseFloat($.values[2])
    ];
    $.yield({
        type:"circle",
        coordinates: coord,
        properties: {
            radius: 15,
            tooltip: {
                content: timestamp
            }
        }
    });
})

GEOMAP()
```

## Distance and Speed

Using the Haversine formula to calculate the distance moved in meters between two points,
then computing the moving speed in kilometers per hour (Km/H) based on the time difference between these points.

### SQL

```js
SQL(`SELECT time, lat, lon FROM TRIP
     WHERE name = 'firenze' ORDER BY time`)
// calculate the distance and speed
SCRIPT({
    var EarthRadius = 6378137.0; // meters
    function degreesToRadians(d) { return d * Math.PI / 180; }
    function distance(p1, p2) {  // haversine distance
        lat1 = degreesToRadians(p1[0]);
        lon1 = degreesToRadians(p1[1]);
        lat2 = degreesToRadians(p2[0]);
        lon2 = degreesToRadians(p2[1]);
        diffLat = lat2 - lat1;
        diffLon = lon2 - lon1;
        a = Math.pow(Math.sin(diffLat/2), 2) + Math.cos(lat1)*Math.cos(lat2)*Math.pow(Math.sin(diffLon/2), 2);
        c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
        return c * EarthRadius;
    }
    var prevLoc, prevTs, dist;
},{
    var ts = $.values[0].Unix(); // unix epoch sec.
    var coord = [$.values[1], $.values[2]];
    dist = prevLoc === undefined ? 0 : distance(prevLoc, coord);
    speed = prevTs === undefined ? 0 : dist*3.600 / (ts - prevTs);
    prevLoc = coord;
    prevTs = ts;
    $.yield({
        type:"circleMarker",
        coordinates: coord,
        properties: {
            radius: 4,
            tooltip: {
                content: "speed: "+speed.toFixed(0)+" KM/H<br/>"+
                         "dist: "+dist.toFixed(0)+" m",
            }
        }
    });
})
GEOMAP()
```

### CSV

```js
// CSV Format: TIME("23-04-21 16:53:21:568000"), LAT, LON
CSV(file("https://docs.machbase.com/assets/example/data-trajectory-firenze.csv"))

// skip header, the first line
DROP(1) 

// parse time, and coordinates from strings
SCRIPT({
    function parseTime(str) { // parse '23-04-21 16:53:21'
        y = str.substr(0,2)+2000;
        m = str.substr(3,2) - 1;
        d = str.substr(6,2);
        hours = str.substr(9, 2);
        mins = str.substr(12,2);
        secs = str.substr(15, 2);
        var D = new Date(y, m, d,hours, mins, secs);
        return (D.getFullYear() == y && D.getMonth() == m && D.getDate() == d) ? D : 'invalid date';
    }
},{ 
    var ts = parseTime($.values[0]).getTime()/1000; // epoch seconds
    var lat = parseFloat($.values[1]);
    var lon = parseFloat($.values[2]);
    $.yield(ts, lat, lon);
})

// calculate the distance and speed
SCRIPT({
    var EarthRadius = 6378137.0; // meters
    function degreesToRadians(d) { return d * Math.PI / 180; }
    function distance(p1, p2) {  // haversine distance
        lat1 = degreesToRadians(p1[0]);
        lon1 = degreesToRadians(p1[1]);
        lat2 = degreesToRadians(p2[0]);
        lon2 = degreesToRadians(p2[1]);
        diffLat = lat2 - lat1;
        diffLon = lon2 - lon1;
        a = Math.pow(Math.sin(diffLat/2), 2) + Math.cos(lat1)*Math.cos(lat2)*Math.pow(Math.sin(diffLon/2), 2);
        c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
        return c * EarthRadius;
    }
    var prevLoc, prevTs, dist;
},{
    var ts = $.values[0];
    var coord = [$.values[1], $.values[2]];
    dist = prevLoc === undefined ? 0 : distance(prevLoc, coord);
    speed = prevTs === undefined ? 0 : dist*3.600 / (ts - prevTs);
    prevLoc = coord;
    prevTs = ts;
    $.yield({
        type:"circleMarker",
        coordinates: coord,
        properties: {
            radius: 4,
            tooltip: {
                content: "speed: "+speed.toFixed(0)+" KM/H<br/>"+
                         "dist: "+dist.toFixed(0)+" m",
            }
        }
    });
})
GEOMAP()
```