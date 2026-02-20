'use strict';

const _spatial = require('@jsh/spatial');

// Haversine formula to calculate the great-circle distance between two points
// on the Earth's surface given their latitude and longitude in decimal degrees.
// Default radius is Earth's radius in meters (6371000 meters)
// coord1 and coord2 can be arrays [lat, lon] or objects {lat: ..., lon: ...}
function haversine(coord1, coord2, radius = 6371000) {
    let lat1, lon1, lat2, lon2;
    if (Array.isArray(coord1)) {
        [lat1, lon1] = coord1;
    } else {
        lat1 = coord1.lat;
        lon1 = coord1.lon;
    }
    if (Array.isArray(coord2)) {
        [lat2, lon2] = coord2;
    } else {
        lat2 = coord2.lat;
        lon2 = coord2.lon;
    }
    return _spatial.haversine({
        radius: radius,
        coord: [[lat1, lon1], [lat2, lon2]],
    });
}

module.exports = {
    haversine,
    parseGeoJSON: _spatial.parseGeoJSON,
    simplify: _spatial.simplify,
}