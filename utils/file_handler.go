package utils

import (
	log "github.com/sirupsen/logrus"
	"os"
	"runtime"
)

func WriteInFile(clusterId string, fileName string, content []byte) string {
	fullPath := GetFullPath(clusterId)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		err := os.Mkdir(fullPath, 0777)
		if err != nil {
			log.Error("Couldn't create folder : " + err.Error())
			os.Exit(1)
		}
	}

	err := os.WriteFile(fullPath+fileName, content, 0777)
	if err != nil {
		log.Error("Couldn't write file : " + err.Error())
		return ""
	}

	return fullPath + fileName
}

func GetFullPath(clusterId string) string {
	if runtime.GOOS == "windows" {
		return "C:\\Users\\Default\\AppData\\Local\\Temp\\qovery_" + clusterId + "\\"
	}

	return "/tmp/qovery_" + clusterId + "/"
}

func DeleteFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Error(err)
	}
}

func DeleteFolder(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		log.Error(err)
	}
}
