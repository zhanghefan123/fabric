package file

import (
	"fmt"
	"os"
)

// WriteStringIntoFile 将 strings 写入到文件之中
func WriteStringIntoFile(filePath string, content string) (err error) {
	fileHandle, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("WriteStringsIntoFile: %w", err)
	}
	defer func() {
		closeErr := fileHandle.Close()
		if err == nil {
			err = closeErr
		}
	}()
	// 进行实际的内容的写入
	_, err = fileHandle.WriteString(content)
	if err != nil {
		return fmt.Errorf("WriteStringsIntoFile: %w", err)
	}
	return nil
}
