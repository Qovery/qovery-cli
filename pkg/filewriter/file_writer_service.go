package filewriter

import (
	"io/fs"
	"os"
)

type FileWriterService interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type FileWriterServiceImpl struct{}

func NewFileWriterService() *FileWriterServiceImpl {
	return &FileWriterServiceImpl{}
}

func (service *FileWriterServiceImpl) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
