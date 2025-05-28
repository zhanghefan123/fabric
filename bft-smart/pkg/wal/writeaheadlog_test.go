// Copyright IBM Corp. All Rights Reserved.
//
// SPDX-License-Identifier: Apache-2.0
//

package wal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger-labs/SmartBFT/pkg/api"
	"github.com/hyperledger-labs/SmartBFT/smartbftprotos"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestWriteAheadLogFile_Create(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("Good", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		if wal == nil {
			return
		}

		dataItems, err := wal.ReadAll()
		assert.EqualError(t, err, ErrWriteOnly.Error())
		assert.Nil(t, dataItems)

		crc := wal.CRC()
		err = wal.Close()
		assert.NoError(t, err)

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyFirstFileCreation(t, logger, dirPath, expectedFileName, crc)
	})

	t.Run("Good - with options", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good-w-options")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 100 * 1024, BufferSizeBytes: 1024})
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		var crc uint32
		if wal != nil {
			err = wal.Close()
			assert.NoError(t, err)
			crc = wal.CRC()
		}

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyFirstFileCreation(t, logger, dirPath, expectedFileName, crc)
	})

	t.Run("Bad - already exist", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "bad-exist")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)
		f, err := os.Create(filepath.Join(dirPath, "0000000000000008.wal"))
		assert.NoError(t, err)
		assert.NotNil(t, f)
		err = f.Close()
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, nil)
		assert.Error(t, err)
		assert.Equal(t, err, ErrWALAlreadyExists)
		assert.Nil(t, wal)
	})
}

func TestWriteAheadLogFile_Open(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("Good", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 4 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 20
		for m := 0; m < NumRec; m++ {
			data1 := make([]byte, NumBytes)
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(n % (m + 1))
			}
			err = wal.Append(data1, false)
			assert.NoError(t, err)
		}

		err = wal.Close()
		assert.NoError(t, err)

		wal, err = Open(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		err = wal.Append([]byte{1, 2, 3, 4}, false)
		assert.EqualError(t, err, "wal: in READ mode")
		err = wal.TruncateTo()
		assert.EqualError(t, err, "wal: in READ mode")

		err = wal.Close()
		assert.NoError(t, err)
	})

	t.Run("Bad - does not exist", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "bad-not-exist")

		wal, err := Open(logger, dirPath, nil)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "no such file or directory")
		}
		assert.Nil(t, wal)
	})

	t.Run("Bad - no files", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "bad-no-files")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Open(logger, dirPath, nil)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "file does not exist")
		}
		assert.Nil(t, wal)
	})
}

func TestWriteAheadLogFile_Close(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("Idempotent", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "idempotent")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		if wal == nil {
			return
		}

		crc := wal.CRC()
		err = wal.Close()
		assert.NoError(t, err)
		err = wal.Close()
		assert.NoError(t, err)
		assert.Equal(t, crc, wal.CRC())

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyFirstFileCreation(t, logger, dirPath, expectedFileName, crc)
	})

	t.Run("Cannot Append", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "cannot-append")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		if wal == nil {
			return
		}

		crc := wal.CRC()
		err = wal.Close()
		assert.NoError(t, err)
		err = wal.Append([]byte{1, 2, 3, 4}, true)
		assert.EqualError(t, err, os.ErrClosed.Error())

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyFirstFileCreation(t, logger, dirPath, expectedFileName, crc)
	})
}

func TestWriteAheadLogFile_Append(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("Good", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		rec1 := &smartbftprotos.LogRecord{
			Type:       smartbftprotos.LogRecord_ENTRY,
			TruncateTo: true,
			Data:       []byte{1, 2, 3, 4},
		}
		err = wal.Append(rec1.Data, rec1.TruncateTo)
		assert.NoError(t, err)

		rec2 := &smartbftprotos.LogRecord{
			Type:       smartbftprotos.LogRecord_ENTRY,
			TruncateTo: false,
			Data:       []byte{5, 6, 7, 8, 9, 10, 11, 12},
		}
		err = wal.Append(rec2.Data, rec2.TruncateTo)
		assert.NoError(t, err)

		err = wal.Append(nil, false)
		assert.Error(t, err)

		err = wal.Append([]byte{}, false)
		assert.Error(t, err)

		crc := wal.CRC()
		err = wal.Close()
		assert.NoError(t, err)

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyAppend(t, logger, dirPath, expectedFileName, crc, rec1, rec2)
	})

	t.Run("File switch", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "switch")

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 20
		records := make([]*smartbftprotos.LogRecord, NumRec)
		var crc1, crc2 uint32
		for m := 0; m < NumRec; m++ {
			data1 := make([]byte, NumBytes)
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(n % (m + 1))
			}

			rec := &smartbftprotos.LogRecord{
				Type:       smartbftprotos.LogRecord_ENTRY,
				TruncateTo: false,
				Data:       data1,
			}
			if m == 0 {
				rec.TruncateTo = true
			}

			records[m] = rec

			err = wal.Append(rec.Data, rec.TruncateTo)
			assert.NoError(t, err)

			if m == 9 {
				crc1 = wal.CRC()
			}
		}
		crc2 = wal.CRC()

		err = wal.Close()
		assert.NoError(t, err)

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyAppend(t, logger, dirPath, expectedFileName, crc1, records[:10]...)
		expectedFileName = fmt.Sprintf(walFileTemplate, 2)
		verifyAppend(t, logger, dirPath, expectedFileName, crc2, records[10:]...)
	})

	t.Run("File recycle", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "recycle")

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 41
		records := make([]*smartbftprotos.LogRecord, NumRec)
		var crc1, crc2 uint32
		for m := 0; m < NumRec; m++ {
			data1 := make([]byte, NumBytes)
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(n % (m + 1))
			}

			rec := &smartbftprotos.LogRecord{
				Type:       smartbftprotos.LogRecord_ENTRY,
				TruncateTo: false,
				Data:       data1,
			}
			if m%3 == 0 {
				rec.TruncateTo = true
			}

			records[m] = rec

			err = wal.Append(rec.Data, rec.TruncateTo)
			assert.NoError(t, err)

			names, err := dirReadWalNames(dirPath)
			assert.NoError(t, err)
			assert.True(t, len(names) <= 2)

			if m == 39 {
				crc1 = wal.CRC()
			}
		}
		crc2 = wal.CRC()

		err = wal.Close()
		assert.NoError(t, err)

		expectedFileName := fmt.Sprintf(walFileTemplate, 4)
		verifyAppend(t, logger, dirPath, expectedFileName, crc1, records[30:40]...)
		expectedFileName = fmt.Sprintf(walFileTemplate, 5)
		verifyAppend(t, logger, dirPath, expectedFileName, crc2, records[40:]...)
	})

	t.Run("TruncateTo", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "TruncateTo")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		rec1 := &smartbftprotos.LogRecord{
			Type:       smartbftprotos.LogRecord_ENTRY,
			TruncateTo: false,
			Data:       []byte{1, 2, 3, 4},
		}
		err = wal.Append(rec1.Data, rec1.TruncateTo)
		assert.NoError(t, err)

		rec2 := &smartbftprotos.LogRecord{
			Type:       smartbftprotos.LogRecord_CONTROL,
			TruncateTo: true,
		}
		err = wal.TruncateTo()
		assert.NoError(t, err)

		rec3 := &smartbftprotos.LogRecord{
			Type:       smartbftprotos.LogRecord_ENTRY,
			TruncateTo: false,
			Data:       []byte{5, 6, 7, 8, 9, 10, 11, 12},
		}
		err = wal.Append(rec3.Data, rec3.TruncateTo)
		assert.NoError(t, err)

		err = wal.Append(nil, false)
		assert.Error(t, err)

		err = wal.Append([]byte{}, false)
		assert.Error(t, err)

		crc := wal.CRC()
		err = wal.Close()
		assert.NoError(t, err)

		expectedFileName := fmt.Sprintf(walFileTemplate, 1)
		verifyAppend(t, logger, dirPath, expectedFileName, crc, rec1, rec2, rec3)
	})
}

func TestWriteAheadLogFile_ReadAll(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("Good - one empty file", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good-1-empty")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		err = wal.Close()
		assert.NoError(t, err)

		wal, err = Open(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		dataItems, err := wal.ReadAll()
		assert.NoError(t, err)
		assert.NotNil(t, dataItems)
		assert.Equal(t, 0, len(dataItems))

		dataItems, err = wal.ReadAll()
		assert.EqualError(t, err, ErrWriteOnly.Error())
		assert.Nil(t, dataItems)
	})

	t.Run("Good - 1 file", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good-1-file")

		wal, err := Create(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		data1 := []byte{1, 2, 3, 4}
		data2 := []byte{5, 6, 7, 8}
		err = wal.Append(data1, false)
		assert.NoError(t, err)
		err = wal.Append(data2, false)
		assert.NoError(t, err)

		err = wal.Close()
		assert.NoError(t, err)

		wal, err = Open(logger, dirPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, wal)

		dataItems, err := wal.ReadAll()
		assert.NoError(t, err)
		assert.NotNil(t, dataItems)
		assert.Equal(t, 2, len(dataItems))
		assert.True(t, bytes.Equal(data1, dataItems[0]))
		assert.True(t, bytes.Equal(data2, dataItems[1]))
	})

	t.Run("Good - many files", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "good-many")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 100
		data1 := make([]byte, NumBytes)
		for m := 0; m < NumRec; m++ {
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(m)
			}
			err = wal.Append(data1, false)
			assert.NoError(t, err)
		}

		err = wal.Close()
		assert.NoError(t, err)

		logger.Infof(">>> Open #1")

		wal, err = Open(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		logger.Infof(">>> ReadAll #1")

		dataItems, err := wal.ReadAll()
		assert.NoError(t, err)
		assert.NotNil(t, dataItems)
		assert.Equal(t, NumRec, len(dataItems))
		for i, data := range dataItems {
			assert.Equal(t, byte(i), data[0])
		}

		// continue to write
		logger.Infof(">>> Continue to write")

		for m := 0; m < NumRec; m++ {
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(m)
			}

			if m == NumRec/2 {
				err = wal.Append(data1, true)
			} else {
				err = wal.Append(data1, false)
			}
			assert.NoError(t, err)
		}

		err = wal.Close()
		assert.NoError(t, err)

		logger.Infof(">>> Open #2")

		wal, err = Open(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		logger.Infof(">>> ReadAll #2")

		dataItems, err = wal.ReadAll()
		assert.NoError(t, err)
		assert.NotNil(t, dataItems)
		assert.Equal(t, NumRec/2, len(dataItems))
		for i, data := range dataItems {
			assert.Equal(t, byte(i+NumRec/2), data[0])
		}

		err = wal.Close()
		assert.NoError(t, err)
	})
}

func TestWriteAheadLogFile_Repair(t *testing.T) {
	testDir, err := os.MkdirTemp("", "unittest")
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewDevelopment()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	t.Run("with a tail", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "tail")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 102
		data1 := make([]byte, NumBytes)
		for m := 0; m < NumRec; m++ {
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(m)
			}
			err = wal.Append(data1, false)
			assert.NoError(t, err)
		}

		err = wal.Close()
		assert.NoError(t, err)

		// add tail to last file
		names, err := dirReadWalNames(dirPath)
		assert.NoError(t, err)
		lastFile := filepath.Join(dirPath, names[len(names)-1])
		f, err := os.OpenFile(lastFile, os.O_RDWR, walFilePermPrivateRW)
		assert.NoError(t, err)
		_, err = f.Seek(0, io.SeekEnd)
		assert.NoError(t, err)
		_, err = f.Write(make([]byte, 64))
		assert.NoError(t, err)
		err = f.Close()
		assert.NoError(t, err)

		assertTestRepair(t, logger, dirPath, NumRec)
	})

	t.Run("broken record", func(t *testing.T) {
		dirPath := filepath.Join(testDir, "broken")
		err := os.MkdirAll(dirPath, walDirPermPrivateRWX)
		assert.NoError(t, err)

		wal, err := Create(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
		assert.NoError(t, err)
		assert.NotNil(t, wal)
		if wal == nil {
			return
		}

		const NumBytes = 1024
		const NumRec = 102
		data1 := make([]byte, NumBytes)
		for m := 0; m < NumRec; m++ {
			for n := 0; n < NumBytes; n++ {
				data1[n] = byte(m)
			}
			err = wal.Append(data1, false)
			assert.NoError(t, err)
		}

		err = wal.Close()
		assert.NoError(t, err)

		// truncate last file
		names, err := dirReadWalNames(dirPath)
		assert.NoError(t, err)
		lastFile := filepath.Join(dirPath, names[len(names)-1])
		f, err := os.OpenFile(lastFile, os.O_RDWR, walFilePermPrivateRW)
		assert.NoError(t, err)
		offset, err := f.Seek(-1, io.SeekEnd)
		assert.NoError(t, err)
		err = f.Truncate(offset)
		assert.NoError(t, err)
		err = f.Close()
		assert.NoError(t, err)

		assertTestRepair(t, logger, dirPath, NumRec-1)
	})
}

func TestWriteAheadLogFile_InitializeAndReadAll(t *testing.T) {
	testDir, err := os.MkdirTemp("", t.Name())
	assert.NoErrorf(t, err, "generate temporary test dir")

	defer os.RemoveAll(testDir)

	basicLog, err := zap.NewProduction()
	assert.NoError(t, err)

	logger := basicLog.Sugar()

	// Create
	wal, entries, err := InitializeAndReadAll(logger, testDir, DefaultOptions())
	assert.NoError(t, err)
	assert.NotNil(t, wal)
	assert.Equal(t, 0, len(entries))

	crc := wal.CRC()
	err = wal.Close()
	assert.NoError(t, err)

	expectedFileName := fmt.Sprintf(walFileTemplate, 1)
	verifyFirstFileCreation(t, logger, testDir, expectedFileName, crc)

	// Open
	options := &Options{FileSizeBytes: 4 * 1024, BufferSizeBytes: 2048}
	wal, entries, err = InitializeAndReadAll(logger, testDir, options)
	assert.NoError(t, err)
	assert.NotNil(t, wal)
	assert.Equal(t, 0, len(entries))

	const (
		NumBytes = 1024
		NumRec   = 20
	)

	for m := 0; m < NumRec; m++ {
		data1 := make([]byte, NumBytes)
		for n := 0; n < NumBytes; n++ {
			data1[n] = byte(n % (m + 1))
		}

		err = wal.Append(data1, false)
		assert.NoError(t, err)
	}

	err = wal.Close()
	assert.NoError(t, err)

	wal, entries, err = InitializeAndReadAll(logger, testDir, options)
	assert.NoError(t, err)
	assert.NotNil(t, wal)
	assert.NotNil(t, entries)
	assert.Equal(t, NumRec, len(entries))

	rec1 := []byte{1, 2, 3, 4}
	err = wal.Append(rec1, true)
	assert.NoError(t, err)
	err = wal.Close()
	assert.NoError(t, err)

	// Corrupt, add tail to last file
	names, err := dirReadWalNames(testDir)
	assert.NoError(t, err)

	lastFile := filepath.Join(testDir, names[len(names)-1])
	f, err := os.OpenFile(lastFile, os.O_RDWR, walFilePermPrivateRW)
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekEnd)
	assert.NoError(t, err)
	_, err = f.Write(make([]byte, 64))
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)

	// Repair
	wal, entries, err = InitializeAndReadAll(logger, testDir, options)
	assert.NoError(t, err)
	assert.NotNil(t, wal)
	assert.NotNil(t, entries)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, rec1, entries[0])

	rec2 := []byte{5, 6, 7, 8}
	err = wal.Append(rec2, false)
	assert.NoError(t, err)
	err = wal.Close()
	assert.NoError(t, err)

	// One last time
	wal, entries, err = InitializeAndReadAll(logger, testDir, options)
	assert.NoError(t, err)
	assert.NotNil(t, wal)
	assert.NotNil(t, entries)
	assert.Equal(t, 2, len(entries))
	assert.Equal(t, rec1, entries[0])
	assert.Equal(t, rec2, entries[1])

	err = wal.Close()
	assert.NoError(t, err)
}

func assertTestRepair(t *testing.T, logger api.Logger, dirPath string, numRec int) {
	logger.Infof(">>> Open #1")
	wal, err := Open(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
	assert.NoError(t, err)
	assert.NotNil(t, wal)

	if wal == nil {
		return
	}

	logger.Infof(">>> ReadAll #1 - will fail")

	_, err = wal.ReadAll()
	assert.Error(t, err)
	err = wal.Close()
	assert.NoError(t, err)

	names, err := dirReadWalNames(dirPath)
	assert.NoError(t, err)

	logger.Infof(">>> Repair")
	err = Repair(logger, dirPath)
	assert.NoError(t, err)

	logger.Infof(">>> Open #2")
	wal, err = Open(logger, dirPath, &Options{FileSizeBytes: 10 * 1024, BufferSizeBytes: 2048})
	assert.NoError(t, err)
	assert.NotNil(t, wal)

	if wal == nil {
		return
	}

	logger.Infof(">>> ReadAll #2 OK")

	dataItems, err := wal.ReadAll()
	assert.NoError(t, err)
	assert.NotNil(t, dataItems)
	assert.Equal(t, numRec, len(dataItems))

	for i, data := range dataItems {
		assert.Equal(t, byte(i), data[0])
	}

	err = wal.Close()
	assert.NoError(t, err)

	cp, err := os.Open(filepath.Join(dirPath, names[len(names)-1]) + ".copy")
	assert.NoError(t, err)

	_ = cp.Close()
}

func verifyFirstFileCreation(
	t *testing.T,
	logger api.Logger,
	dirPath string,
	expectedFileName string,
	expectedCRC uint32,
) {
	names, err := dirReadWalNames(dirPath)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(names))
	assert.Equal(t, expectedFileName, names[0])

	r, err := NewLogRecordReader(logger, filepath.Join(dirPath, expectedFileName))
	assert.NoError(t, err)
	assert.NotNil(t, r)

	if r != nil {
		defer r.Close()
		record, err := r.Read()
		assert.Error(t, err, "no more records")
		assert.Nil(t, record)
		assert.Equal(t, expectedCRC, r.CRC())
	}
}

func verifyAppend(
	t *testing.T,
	logger api.Logger,
	dirPath string,
	expectedFileName string,
	expectedCRC uint32,
	records ...*smartbftprotos.LogRecord,
) {
	r, err := NewLogRecordReader(logger, filepath.Join(dirPath, expectedFileName))
	assert.NoError(t, err)
	assert.NotNil(t, r)

	if r == nil {
		return
	}
	defer r.Close()

	for _, expectedRecord := range records {
		record, err := r.Read()
		assert.NoError(t, err)
		assert.NotNil(t, record)
		assert.Equal(t, expectedRecord.Data, record.Data)
		assert.Equal(t, expectedRecord.Type, record.Type)
		assert.Equal(t, expectedRecord.TruncateTo, record.TruncateTo)
	}

	record, err := r.Read()
	assert.Error(t, err, "no more records")
	assert.Nil(t, record)
	assert.Equal(t, expectedCRC, r.CRC())
}
