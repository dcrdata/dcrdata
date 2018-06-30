package dcrsqlite

import (
	"testing"

	"github.com/decred/dcrdata/testutil"
)

// TestMissingParentFolder ensures InitDB() is able to create a new DB-file parent directory if necessary
// See https://github.com/decred/dcrdata/issues/515
func TestMissingParentFolder(t *testing.T) {
	testutil.ResetTempFolder()
	targetDBFile := testutil.FilePathInsideTempDir("x/y/z/" + testutil.DefaultDBFileName)
	dbInfo := &DBInfo{FileName: targetDBFile}
	db, err := InitDB(dbInfo)

	if err != nil {
		t.Fatalf("InitDB() failed: %v", err)
	}

	if db == nil {
		t.Fatalf("InitDB() failed")
	}
}
