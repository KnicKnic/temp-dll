package tempdll

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

type fileHolder struct {
	handle *os.File
}

func (holder *fileHolder) safeClose() {
	if holder.handle != nil {
		holder.handle.Close()
		holder.handle = nil
	}
}

func safeWriteFile(fileName string, contents []byte, retryCount int, retryDelay time.Duration) (*os.File, error) {

	var file, wFile fileHolder // allows us to be a more easily use defers & closes
	var err, wErr error
	defer file.safeClose()
	defer wFile.safeClose()

	// since we always read, write, reread we need at least 1 loop
	retryCount += 1
	for i := 0; i <= retryCount; i += 1 {
		file.safeClose()
		wFile.safeClose()

		file.handle, err = os.OpenFile(fileName, os.O_RDONLY, 0600)
		if err == nil {
			fileOnDisk := bytes.NewBuffer(nil)
			io.Copy(fileOnDisk, file.handle)

			if bytes.Equal(fileOnDisk.Bytes(), contents) {
				toReturn := file.handle
				file.handle = nil
				return toReturn, nil
			} else if i == retryCount {
				// files shouldn't be the same on our last go around
				return nil, fmt.Errorf("file contents differed last lastWriterError %w", wErr)
			}
		}

		// ensure we close file
		if err != nil {
			file.safeClose()
		}
		// there was an error reading or files differed
		wFile.handle, wErr = os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)

		if wErr != nil {
			if i == retryCount {
				// files shouldn't be the same on our last go around
				return nil, fmt.Errorf("Error writting file readerError %w, lastWriterError %w", err, wErr)
			}
		}

		toWrite := bytes.NewReader(contents)
		io.Copy(wFile.handle, toWrite)

		if i+1 < retryCount {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("file contents differed last readerError %w, lastWriterError %w", err, wErr)

}
