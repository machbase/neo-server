# Machbase Neo JavaScript Spatial Module

## haversine()

`haversine()` is for calculating the Haversine distance.
The Haversine formula is used to calculate the great-circle distance
between two points on a sphere, given their latitudes and longitudes.

**Usage example**

```js
m = require("@jsh/spatial");
latLon1 = [45.04, 7.42];  // Turin, Italy
latLon2 = [3.09, 101.42]; // Kuala Lumpur, Malaysia
distance = m.haversine({radius: 6371, coordinates:[latLon1, latLon2]})
console.log(distance.toFixed(0), "Km");

// 10078 Km
```