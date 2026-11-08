package migration

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Detect fingerprints an uploaded backup without parsing its rows. It is used
// by the restore endpoint to reject foreign databases before replacing the
// live database.
func Detect(r io.Reader) (Source, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return SourceUnknown, fmt.Errorf("read upload: %w", err)
	}
	if len(raw) > maxUploadBytes {
		return SourceUnknown, errors.New("backup exceeds 128 MiB limit")
	}
	return DetectBytes(raw)
}

// DetectBytes is Detect for callers that already hold the bytes.
func DetectBytes(raw []byte) (Source, error) {
	ex, err := extractSQLite(raw)
	if err != nil {
		return SourceUnknown, err
	}
	db, cleanup, err := openSnapshot(ex.DB)
	if err != nil {
		return SourceUnknown, err
	}
	defer cleanup()
	sch, err := readSchema(db)
	if err != nil {
		return SourceUnknown, err
	}
	return detectSource(sch), nil
}

// Parse inspects an uploaded backup and returns a vendor-neutral snapshot.
// It is read-only: nothing is written to the panel database.
func Parse(r io.Reader) (*Snapshot, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(raw) > maxUploadBytes {
		return nil, errors.New("backup exceeds 128 MiB limit")
	}
	ex, err := extractSQLite(raw)
	if err != nil {
		return nil, err
	}
	db, cleanup, err := openSnapshot(ex.DB)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	sch, err := readSchema(db)
	if err != nil {
		return nil, err
	}
	source := detectSource(sch)
	switch source {
	case SourceSelf:
		return nil, errors.New("this is a backup exported by this panel; use Restore instead of migration")
	case SourceXUI:
		return parseXUI(db, sch)
	case SourceSUI:
		return parseSUI(db, sch)
	default:
		return nil, fmt.Errorf("unrecognised backup: tables found: %v", sch.tableNames())
	}
}

// ParseFile is a convenience wrapper for local tooling and tests.
func ParseFile(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}
