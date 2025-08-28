package metric

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Storage interface {
	Store(measure string, field string, series string, ts *TimeSeries) error
	Load(measure string, field string, series string) (*TimeSeries, error)
}

func NewFileStorage(dir string) *FileStorage {
	return &FileStorage{dir: dir}
}

type FileStorage struct {
	dir string
}

var _ Storage = (*FileStorage)(nil)

func (ds *FileStorage) Store(measure string, field string, seriesName string, ts *TimeSeries) error {
	filename := fmt.Sprintf("%s_%s_%s.metric", measure, field, seriesName)
	path := filepath.Join(ds.dir, cleanPath(filename))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ") // Optional: for pretty printing
	err = enc.Encode(ts)
	if err != nil {
		return fmt.Errorf("failed to marshal time series: %w", err)
	}
	return nil
}

func (ds *FileStorage) Load(measure string, field string, seriesName string) (*TimeSeries, error) {
	filename := fmt.Sprintf("%s_%s_%s.metric", measure, field, seriesName)
	path := filepath.Join(ds.dir, cleanPath(filename))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File does not exist, return nil
		}
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var ts = &TimeSeries{}
	if err := dec.Decode(ts); err != nil {
		return nil, err
	}

	if len(ts.data) > 0 {
		now := nowFunc().Round(ts.interval)
		starts := now.Add(-ts.interval * time.Duration(ts.maxCount))
		for i := len(ts.data) - 1; i >= 0; i-- {
			if ts.data[i].Time.Compare(starts) <= 0 {
				if i == len(ts.data)-1 {
					ts.data = ts.data[:0] // Clear the data if all are older than the start time
				} else {
					ts.data = ts.data[i+1:] // Trim the data to only keep recent entries
				}
				break
			}
		}
	}
	return ts, err
}

func cleanPath(path string) string {
	invalidChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", " ", "\t", "\n"}
	cleanedPath := path
	for _, char := range invalidChars {
		cleanedPath = strings.ReplaceAll(cleanedPath, char, "_") // Replace with underscore
	}
	return cleanedPath
}
