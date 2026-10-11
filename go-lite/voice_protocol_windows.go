//go:build windows

package main

import (
	"bufio"
	"errors"
	"io"
)

const maxVoiceEventBytes = 1024 * 1024

// Read one bounded JSONL event at a time off the UI thread. An oversized or
// truncated event is a protocol failure: never parse its suffix as a new final.
func readVoiceProtocolLines(r io.Reader, handleLine func([]byte) bool) error {
	reader := bufio.NewReaderSize(r, 64*1024)
	line := make([]byte, 0, 4096)
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > maxVoiceEventBytes {
			return errors.New("voice event exceeds protocol size limit")
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 && !handleLine(line) {
				return nil
			}
		}
		line = line[:0]
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
