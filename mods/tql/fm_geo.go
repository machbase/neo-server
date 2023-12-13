package tql

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	Alt float64 `json:"alt,omitempty"`
}

func NewLatLng(lat, lng float64) *LatLng {
	return &LatLng{Lat: lat, Lng: lng}
}

func (ll *LatLng) Array() []float64 {
	return []float64{ll.Lat, ll.Lng}
}

func (ll *LatLng) DistanceTo(pt *LatLng) float64 {
	return 0
}
