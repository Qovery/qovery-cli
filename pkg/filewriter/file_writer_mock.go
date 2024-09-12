//go:build testing

package filewriter

import (
	"io/fs"
)

type FileWriterServiceMock struct {
	FileContentWritten string
}

func (service *FileWriterServiceMock) WriteFile(name string, data []byte, perm fs.FileMode) error {
	service.FileContentWritten = string(data)

	return nil
}