package pkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
)

type archiveEntry struct {
	name     string
	typeflag byte
	content  string
}

func makeArchive(t *testing.T, entries ...archiveEntry) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	assert.NoError(t, tw.WriteHeader(&tar.Header{Name: "exec-id/", Typeflag: tar.TypeDir, Mode: 0755}))
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Typeflag: entry.typeflag, Mode: 0644}
		if entry.typeflag == tar.TypeSymlink {
			header.Linkname = entry.content
		} else {
			header.Size = int64(len(entry.content))
		}
		assert.NoError(t, tw.WriteHeader(header))
		if entry.typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(entry.content))
			assert.NoError(t, err)
		}
	}
	assert.NoError(t, tw.Close())
	assert.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestReadEngineRequest(t *testing.T) {
	root := archiveEntry{"exec-id/engine-request.json", tar.TypeReg, `{"id":"exec-id"}`}
	nested := archiveEntry{"exec-id/charts/engine-request.json", tar.TypeReg, `{"nested":true}`}

	tests := []struct {
		name    string
		entries []archiveEntry
	}{
		{"root entry after a nested one", []archiveEntry{nested, root}},
		{"root entry before a nested one", []archiveEntry{root, nested}},
		{"root entry with a ./ prefix", []archiveEntry{{"./exec-id/engine-request.json", tar.TypeReg, `{"id":"exec-id"}`}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, err := readEngineRequest(makeArchive(t, tt.entries...))

			assert.NoError(t, err)
			assert.Equal(t, `{"id":"exec-id"}`, string(request))
		})
	}
}

func TestReadEngineRequestMissing(t *testing.T) {
	tests := []struct {
		name    string
		entries []archiveEntry
	}{
		{"only a nested entry", []archiveEntry{{"exec-id/charts/engine-request.json", tar.TypeReg, `{}`}}},
		{"symlink at the root", []archiveEntry{{"exec-id/engine-request.json", tar.TypeSymlink, "/etc/passwd"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readEngineRequest(makeArchive(t, tt.entries...))

			assert.ErrorIs(t, err, errEngineRequestNotInArchive)
		})
	}
}

func TestReadEngineRequestNotGzip(t *testing.T) {
	_, err := readEngineRequest([]byte("not an archive"))

	assert.Error(t, err)
}

func TestReadEngineRequestCorrupt(t *testing.T) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.NoCompression)
	assert.NoError(t, err)
	tw := tar.NewWriter(gz)
	content := `{"secret":"value"}`
	assert.NoError(t, tw.WriteHeader(&tar.Header{Name: "exec-id/engine-request.json", Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(content))}))
	_, err = tw.Write([]byte(content))
	assert.NoError(t, err)
	assert.NoError(t, tw.Close())
	assert.NoError(t, gz.Close())
	archive := buf.Bytes()
	at := bytes.Index(archive, []byte("secret"))
	assert.Positive(t, at)
	archive[at] = 'X'

	_, err = readEngineRequest(archive)

	assert.ErrorIs(t, err, gzip.ErrChecksum)
}

func TestReadEngineRequestTruncated(t *testing.T) {
	archive := makeArchive(t, archiveEntry{"exec-id/engine-request.json", tar.TypeReg, string(bytes.Repeat([]byte("x"), 300000))})

	_, err := readEngineRequest(archive[:len(archive)/2])

	assert.Error(t, err)
}
