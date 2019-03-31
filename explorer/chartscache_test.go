package explorer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/decred/dcrdata/v4/db/dbtypes"
)

var tempDir string

// TestMain setups the tempDir and cleans it up after tests.
func TestMain(m *testing.M) {
	var err error
	tempDir, err = ioutil.TempDir(os.TempDir(), "cache")
	if err != nil {
		log.Error(err)
		return
	}

	code := m.Run()

	// clean up
	os.RemoveAll(tempDir)

	os.Exit(code)
}

// TestChartsCache tests the reading and writing of the charts cache.
func TestChartsCache(t *testing.T) {
	gobPath := filepath.Join(tempDir, "log.gob")

	// chartsData needs to contain entries defined chartsCount.
	chartsData := map[string]*dbtypes.ChartsData{
		dbtypes.AvgBlockSize: {
			ValueF: []float64{1.2, 2.0, 5.9},
			SizeF:  []float64{1.2, 1.3, 1.6},
		},
	}
	var defHeight int64 = 300000

	// Write to the cache once and reuse the cache contents.
	cacheChartsData.Update(defHeight, chartsData)

	t.Run("Read_a_non-existent_gob_dump", func(t *testing.T) {
		err := ReadCacheFile(filepath.Join(tempDir, "log1.gob"), defHeight)
		if err == nil {
			t.Fatal("expected an error but found none")
		}
	})

	t.Run("Read_a_non-gob_file_encoding_dump", func(t *testing.T) {
		path := filepath.Join(tempDir, "log2.txt")

		err := ioutil.WriteFile(path, []byte(`Who let the dogs bark?`), 0644)
		if err != nil {
			t.Fatalf("expected no error but found: %v", err)
		}

		err = ReadCacheFile(path, defHeight)
		if err == nil {
			t.Fatal("expected an error but found non")
		}
	})

	t.Run("Write_to_existing_non-GOB_file", func(t *testing.T) {
		path := filepath.Join(tempDir, "log3.txt")

		err := ioutil.WriteFile(path, []byte(`Who let the dogs bark?`), 0644)
		if err != nil {
			t.Fatalf("expected no error but found: %v", err)
		}

		err = WriteCacheFile(path)
		if err == nil {
			t.Fatal("expected an error but found non")
		}
	})

	t.Run("Write_to_an_non_existent_file", func(t *testing.T) {
		err := WriteCacheFile(gobPath)
		if err != nil {
			t.Fatalf("expected no error but found: %v", err)
		}

		// check if the new dump file path exists
		if !isfileExists(gobPath) {
			t.Fatalf("expected to find the newly created file but its missing")
		}
	})

	t.Run("Read_from_an_existing_gob_encoded_file", func(t *testing.T) {
		// delete the cache contents by setting an empty array.
		cacheChartsData.Update(defHeight, map[string]*dbtypes.ChartsData{})

		// fetch the updated cache contents.
		contents := cacheChartsData.get()

		// returned cache contents should not be equal to chartsData.
		if reflect.DeepEqual(contents, chartsData) {
			t.Fatalf("expected new cache contents not to match the old onces but they did")
		}

		err := ReadCacheFile(gobPath, defHeight)
		if err != nil {
			t.Fatalf("expected no error but found: %v", err)
		}

		// fetch the cache contents again.
		contents = cacheChartsData.get()

		// returned cache contents should be equal to chartsData.
		if !reflect.DeepEqual(contents, chartsData) {
			t.Fatalf("expected new cache contents to match the old onces but they did not")
		}
	})
}
