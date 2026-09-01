package metric

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSeriesIDJSONAndValidation(t *testing.T) {
	series, err := NewSeriesID(" requests_1 ", "Requests", time.Second, 10)
	if err != nil {
		t.Fatalf("NewSeriesID() error: %v", err)
	}
	if series.ID() != "REQUESTS_1" || series.Title() != "Requests" || series.Period() != time.Second || series.MaxCount() != 10 {
		t.Fatalf("unexpected series: %+v", series)
	}
	if oldest := series.OldestTime(); oldest.After(time.Now().Add(-9*time.Second)) || oldest.Before(time.Now().Add(-11*time.Second)) {
		t.Fatalf("OldestTime() was %v", oldest)
	}

	data, err := json.Marshal(series)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	var decoded SeriesID
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if !reflect.DeepEqual(decoded, series) {
		t.Fatalf("decoded series was %+v, want %+v", decoded, series)
	}
	if err := json.Unmarshal([]byte("{"), &decoded); err == nil {
		t.Fatal("SeriesID.UnmarshalJSON() returned nil for invalid JSON")
	}

	for _, invalid := range []string{"", "1STARTS_WITH_NUMBER", "A"} {
		if _, err := NewSeriesID(invalid, "", time.Second, 1); err == nil {
			t.Fatalf("NewSeriesID(%q) returned nil error", invalid)
		}
	}
}

func TestNewFileStorage(t *testing.T) {
	if storage := NewFileStorage("", 1); storage != nil {
		t.Fatalf("NewFileStorage(\"\") was %v, want nil", storage)
	}
	storage := NewFileStorage(t.TempDir(), 0)
	if cap(storage.wChan) != 100 {
		t.Fatalf("default channel capacity was %d, want 100", cap(storage.wChan))
	}
	storage = NewFileStorage(t.TempDir(), 3)
	if cap(storage.wChan) != 3 {
		t.Fatalf("channel capacity was %d, want 3", cap(storage.wChan))
	}
}

func TestFileStorageOpenBackupAndClose(t *testing.T) {
	dir := t.TempDir()
	seriesPath := filepath.Join(dir, "SERIES.ts")
	requireMetricWriteFile(t, seriesPath, "series-data\n")
	requireMetricWriteFile(t, filepath.Join(dir, "ignored.txt"), "ignored\n")
	if err := os.Mkdir(filepath.Join(dir, "DIRECTORY.ts"), 0755); err != nil {
		t.Fatalf("os.Mkdir() error: %v", err)
	}

	storage := NewFileStorage(dir, 1)
	if err := storage.Open(); err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	assertMetricFileContent(t, seriesPath+".bak", "series-data\n")
	if err := storage.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	missingStorage := NewFileStorage(filepath.Join(dir, "missing"), 1)
	if err := missingStorage.Open(); err == nil {
		t.Fatal("Open() returned nil for a missing directory")
	}
}

func TestFileStorageWriteShrinkAndLoad(t *testing.T) {
	dir := t.TempDir()
	series, err := NewSeriesID("SERIES", "Series", time.Hour, 2)
	if err != nil {
		t.Fatalf("NewSeriesID() error: %v", err)
	}
	storage := NewFileStorage(dir, 1)
	storage.shrinkThresholdDuration = 0

	oldProduct := Product{Name: "requests", Time: time.Now().Add(-4 * time.Hour), Value: &CounterValue{Value: 1}, Type: "counter"}
	recentProduct := Product{Name: "requests", Time: time.Now(), Value: &CounterValue{Value: 2}, Type: "counter"}
	if err := storage.write(series, oldProduct, false); err != nil {
		t.Fatalf("write(old) error: %v", err)
	}
	if err := storage.write(series, recentProduct, false); err != nil {
		t.Fatalf("write(recent) error: %v", err)
	}
	if len(storage.files) != 0 {
		t.Fatalf("write after shrink retained %d open files", len(storage.files))
	}

	seriesPath := filepath.Join(dir, "SERIES.ts")
	data, err := os.ReadFile(seriesPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	if strings.Contains(string(data), oldProduct.Time.Format(time.RFC3339Nano)) {
		t.Fatalf("shrunk file retained old product: %s", data)
	}
	if !strings.Contains(string(data), recentProduct.Time.Format(time.RFC3339Nano)) {
		t.Fatalf("shrunk file omitted recent product: %s", data)
	}

	malformedLine := "not-json\n"
	otherProduct := Product{Name: "other", Time: time.Now(), Value: &CounterValue{Value: 3}, Type: "counter"}
	loadData := malformedLine + marshalMetricProduct(t, oldProduct) + "\n" + marshalMetricProduct(t, otherProduct) + "\n" + marshalMetricProduct(t, recentProduct) + "\n"
	requireMetricWriteFile(t, seriesPath+".bak", loadData)
	products, err := storage.Load(series, "requests")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(products) != 1 || products[0].Name != "requests" || products[0].Value == nil {
		t.Fatalf("Load() returned unexpected products: %+v", products)
	}

	missingSeries, _ := NewSeriesID("MISSING", "", time.Hour, 2)
	products, err = storage.Load(missingSeries, "requests")
	if err != nil || products != nil {
		t.Fatalf("Load(missing) returned products=%v err=%v", products, err)
	}

	closingProduct := Product{Name: "requests", Time: time.Now(), Value: &CounterValue{Value: 4}, Type: "counter"}
	if err := storage.write(missingSeries, closingProduct, true); err != nil {
		t.Fatalf("write(closing) error: %v", err)
	}
	if _, exists := storage.files[missingSeries.ID()]; exists {
		t.Fatal("closing write retained an open file")
	}
}

func TestFileStorageWriteErrors(t *testing.T) {
	series, _ := NewSeriesID("SERIES", "", time.Second, 1)
	storage := NewFileStorage(filepath.Join(t.TempDir(), "missing"), 1)
	if err := storage.write(series, Product{Name: "requests", Time: time.Now(), Type: "counter", Value: &CounterValue{}}, false); err == nil {
		t.Fatal("write() returned nil for a missing storage directory")
	}
}

func TestParseProductTypes(t *testing.T) {
	tests := []struct {
		productType string
		wantType    any
	}{
		{productType: "counter", wantType: &CounterValue{}},
		{productType: "gauge", wantType: &GaugeValue{}},
		{productType: "timer", wantType: &TimerValue{}},
		{productType: "meter", wantType: &MeterValue{}},
		{productType: "odometer", wantType: &OdometerValue{}},
		{productType: "histogram", wantType: &HistogramValue{}},
	}
	for _, tc := range tests {
		t.Run(tc.productType, func(t *testing.T) {
			line := `{"name":"metric","ts":"2026-01-02T03:04:05Z","value":{},"type":"` + tc.productType + `"}`
			var product Product
			if err := parseProduct(&product, line, true); err != nil {
				t.Fatalf("parseProduct() error: %v", err)
			}
			if reflect.TypeOf(product.Value) != reflect.TypeOf(tc.wantType) {
				t.Fatalf("value type was %T, want %T", product.Value, tc.wantType)
			}
			if err := parseProduct(&product, line, false); err != nil {
				t.Fatalf("parseProduct(includeValue=false) error: %v", err)
			}
			if product.Value != nil {
				t.Fatalf("includeValue=false returned value %v", product.Value)
			}
		})
	}

	var product Product
	if err := parseProduct(&product, "not-json", true); err == nil {
		t.Fatal("parseProduct() returned nil for invalid JSON")
	}
	if err := parseProduct(&product, `{"type":"unknown","value":{}}`, true); err == nil {
		t.Fatal("parseProduct() returned nil for an unknown type")
	}
}

func marshalMetricProduct(t *testing.T, product Product) string {
	t.Helper()
	data, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	return string(data)
}

func requireMetricWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("os.WriteFile(%q) error: %v", path, err)
	}
}

func assertMetricFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error: %v", path, err)
	}
	if got := string(data); got != want {
		t.Fatalf("file content was %q, want %q", got, want)
	}
}
