package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const spoolJournalVersion = 1

type spoolFailure struct {
	Class failureClass
	Err   error
}

func newSpoolFailure(class failureClass, format string, arguments ...interface{}) *spoolFailure {
	return &spoolFailure{Class: class, Err: fmt.Errorf(format, arguments...)}
}

type journalRecord struct {
	Kind        string `json:"kind"`
	Version     int    `json:"version,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Start       int64  `json:"start,omitempty"`
	End         int64  `json:"end,omitempty"`
	Records     int64  `json:"records,omitempty"`
	Callbacks   int64  `json:"callbacks,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

type durableSpool struct {
	path              string
	journalPath       string
	fingerprint       string
	data              *os.File
	journal           *os.File
	journalEnd        int64
	acceptedBytes     int64
	acceptedRecords   int64
	acceptedCallbacks int64
	retirementMarked  bool
	closed            bool
}

func openDurableSpool(config pluginConfig) (*durableSpool, error) {
	fingerprint, err := spoolFingerprint(config)
	if err != nil {
		return nil, err
	}
	data, err := openSpoolFile(config.SpoolPath, true)
	if err != nil {
		return nil, fmt.Errorf("could not exclusively open spool %q: %w", config.SpoolPath, err)
	}
	spool := &durableSpool{
		path:        config.SpoolPath,
		journalPath: config.SpoolPath + ".commit",
		fingerprint: fingerprint,
		data:        data,
	}
	journal, err := openSpoolFile(spool.journalPath, false)
	if err != nil {
		spool.close()
		return nil, fmt.Errorf("could not open acceptance journal %q: %w", spool.journalPath, err)
	}
	spool.journal = journal
	if err := syncParentDirectory(config.SpoolPath); err != nil {
		spool.close()
		return nil, fmt.Errorf("could not sync spool directory: %w", err)
	}
	if err := spool.recover(); err != nil {
		spool.close()
		return nil, err
	}
	return spool, nil
}

func openSpoolFile(path string, lock bool) (*os.File, error) {
	flags := syscall.O_CREAT | syscall.O_RDWR | syscall.O_CLOEXEC | syscall.O_NOFOLLOW
	fd, err := syscall.Open(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("could not create file handle")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("path is not a regular file")
	}
	if lock {
		if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func spoolFingerprint(config pluginConfig) (string, error) {
	identity := struct {
		Version  int    `json:"version"`
		Endpoint string `json:"endpoint"`
		Table    string `json:"table"`
		IDKey    string `json:"id_key"`
		Action   string `json:"action"`
	}{
		Version:  spoolJournalVersion,
		Endpoint: config.Endpoint.String(),
		Table:    config.Table,
		IDKey:    config.IDKey,
		Action:   config.Action,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func (s *durableSpool) recover() error {
	dataInfo, err := s.data.Stat()
	if err != nil {
		return fmt.Errorf("could not inspect spool %q: %w", s.path, err)
	}
	journalInfo, err := s.journal.Stat()
	if err != nil {
		return fmt.Errorf("could not inspect acceptance journal %q: %w", s.journalPath, err)
	}
	if journalInfo.Size() == 0 {
		if dataInfo.Size() != 0 {
			return fmt.Errorf("spool %q has data without acceptance metadata", s.path)
		}
		return s.writeFreshJournal()
	}
	if _, err := s.journal.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReader(s.journal)
	line, readErr := reader.ReadBytes('\n')
	if readErr != nil {
		if dataInfo.Size() == 0 {
			return s.writeFreshJournal()
		}
		return fmt.Errorf("acceptance journal %q has an incomplete header", s.journalPath)
	}
	header := journalRecord{}
	if json.Unmarshal(line, &header) != nil || header.Kind != "header" ||
		header.Version != spoolJournalVersion || header.Fingerprint == "" {
		if dataInfo.Size() == 0 {
			return s.writeFreshJournal()
		}
		return fmt.Errorf("acceptance journal %q has an invalid header", s.journalPath)
	}

	journalEnd := int64(len(line))
	records := make([]journalRecord, 0)
	for {
		line, readErr = reader.ReadBytes('\n')
		if readErr == io.EOF {
			if len(line) > 0 {
				if err := truncateAndSync(s.journal, journalEnd); err != nil {
					return fmt.Errorf("could not discard torn acceptance metadata: %w", err)
				}
			}
			break
		}
		if readErr != nil {
			return fmt.Errorf("could not read acceptance journal %q: %w", s.journalPath, readErr)
		}
		entry := journalRecord{}
		if json.Unmarshal(line, &entry) != nil || (entry.Kind != "accept" && entry.Kind != "retire") {
			return fmt.Errorf("acceptance journal %q is corrupt", s.journalPath)
		}
		records = append(records, entry)
		journalEnd += int64(len(line))
	}
	s.journalEnd = journalEnd

	acceptedEnd := int64(0)
	acceptedRecords := int64(0)
	acceptedCallbacks := int64(0)
	retired := false
	for index, entry := range records {
		switch entry.Kind {
		case "accept":
			if retired || entry.Start != acceptedEnd || entry.End <= entry.Start ||
				entry.Records <= 0 || len(entry.SHA256) != sha256.Size*2 {
				return fmt.Errorf("acceptance journal %q is corrupt at record %d", s.journalPath, index+1)
			}
			if _, err := hex.DecodeString(entry.SHA256); err != nil {
				return fmt.Errorf("acceptance journal %q has an invalid checksum", s.journalPath)
			}
			acceptedEnd = entry.End
			acceptedRecords += entry.Records
			acceptedCallbacks++
		case "retire":
			if retired || acceptedEnd == 0 || entry.End != acceptedEnd ||
				entry.Records != acceptedRecords || entry.Callbacks != acceptedCallbacks ||
				index != len(records)-1 {
				return fmt.Errorf("acceptance journal %q has an invalid retirement marker", s.journalPath)
			}
			retired = true
		}
	}

	s.acceptedBytes = acceptedEnd
	s.acceptedRecords = acceptedRecords
	s.acceptedCallbacks = acceptedCallbacks
	if retired {
		s.retirementMarked = true
		return s.finishRetirement()
	}
	if header.Fingerprint != s.fingerprint {
		if acceptedEnd != 0 {
			return fmt.Errorf("accepted spool %q does not match this output configuration", s.path)
		}
		if dataInfo.Size() > 0 {
			if err := truncateAndSync(s.data, 0); err != nil {
				return err
			}
		}
		return s.writeFreshJournal()
	}
	if dataInfo.Size() < acceptedEnd {
		return fmt.Errorf("spool %q is shorter than its accepted end", s.path)
	}
	for index, entry := range records {
		if entry.Kind != "accept" {
			continue
		}
		hash, err := hashFileRange(s.data, entry.Start, entry.End)
		if err != nil || hash != entry.SHA256 {
			return fmt.Errorf("spool %q checksum mismatch at accepted callback %d", s.path, index+1)
		}
		boundary := []byte{0}
		if _, err := s.data.ReadAt(boundary, entry.End-1); err != nil || boundary[0] != '\n' {
			return fmt.Errorf("spool %q lacks an NDJSON boundary at accepted callback %d", s.path, index+1)
		}
	}
	if dataInfo.Size() > acceptedEnd {
		if err := truncateAndSync(s.data, acceptedEnd); err != nil {
			return fmt.Errorf("could not discard unaccepted spool tail: %w", err)
		}
	}
	if _, err := s.data.Seek(acceptedEnd, io.SeekStart); err != nil {
		return err
	}
	if _, err := s.journal.Seek(s.journalEnd, io.SeekStart); err != nil {
		return err
	}
	return nil
}

func (s *durableSpool) writeFreshJournal() error {
	header := journalRecord{
		Kind:        "header",
		Version:     spoolJournalVersion,
		Fingerprint: s.fingerprint,
	}
	if err := s.journal.Truncate(0); err != nil {
		return err
	}
	if _, err := s.journal.Seek(0, io.SeekStart); err != nil {
		return err
	}
	line, err := marshalJournalRecord(header)
	if err != nil {
		return err
	}
	if err := writeAllFile(s.journal, line); err != nil || s.journal.Sync() != nil {
		if err != nil {
			return err
		}
		return fmt.Errorf("could not sync acceptance journal")
	}
	s.journalEnd = int64(len(line))
	_, err = s.journal.Seek(s.journalEnd, io.SeekStart)
	return err
}

type hashedSpoolWriter struct {
	file   *os.File
	hasher io.Writer
	err    error
	bytes  int64
}

func (w *hashedSpoolWriter) Write(data []byte) (int, error) {
	written, err := w.file.Write(data)
	if written > 0 {
		_, _ = w.hasher.Write(data[:written])
		w.bytes += int64(written)
	}
	if err != nil && w.err == nil {
		w.err = err
	}
	return written, err
}

func (s *durableSpool) appendCallback(data unsafe.Pointer, length int, config pluginConfig) (encodeStats, *spoolFailure) {
	if s.retirementMarked {
		return encodeStats{}, newSpoolFailure(failurePermanent, "cannot append while spool retirement is pending")
	}
	start := s.acceptedBytes
	journalStart := s.journalEnd
	if _, err := s.data.Seek(start, io.SeekStart); err != nil {
		return encodeStats{}, newSpoolFailure(failureRetryable, "could not seek spool: %v", err)
	}
	hasher := sha256.New()
	writer := &hashedSpoolWriter{file: s.data, hasher: hasher}
	stats, encodeErr := encodeCallback(data, length, config, writer)
	if encodeErr != nil {
		rollbackErr := truncateAndSync(s.data, start)
		if rollbackErr != nil {
			return encodeStats{}, newSpoolFailure(failurePermanent, "could not roll back rejected callback: %v", rollbackErr)
		}
		if writer.err != nil {
			return encodeStats{}, newSpoolFailure(failureRetryable, "could not append callback to spool: %v", writer.err)
		}
		return encodeStats{}, newSpoolFailure(failurePermanent, "%v", encodeErr)
	}
	if stats.Records == 0 {
		if writer.bytes != 0 {
			_ = truncateAndSync(s.data, start)
			return encodeStats{}, newSpoolFailure(failurePermanent, "empty callback wrote unexpected spool bytes")
		}
		return stats, nil
	}
	end := start + writer.bytes
	stats.Bytes = writer.bytes
	if err := s.data.Sync(); err != nil {
		if rollbackErr := truncateAndSync(s.data, start); rollbackErr != nil {
			return encodeStats{}, newSpoolFailure(failurePermanent, "could not roll back unsynced callback: %v", rollbackErr)
		}
		return encodeStats{}, newSpoolFailure(failureRetryable, "could not sync spool: %v", err)
	}
	entry := journalRecord{
		Kind:    "accept",
		Start:   start,
		End:     end,
		Records: stats.Records,
		SHA256:  hex.EncodeToString(hasher.Sum(nil)),
	}
	line, err := marshalJournalRecord(entry)
	if err != nil {
		_ = truncateAndSync(s.data, start)
		return encodeStats{}, newSpoolFailure(failurePermanent, "could not encode acceptance metadata: %v", err)
	}
	if _, err := s.journal.Seek(journalStart, io.SeekStart); err != nil ||
		writeAllFile(s.journal, line) != nil || s.journal.Sync() != nil {
		journalRollback := truncateAndSync(s.journal, journalStart)
		dataRollback := truncateAndSync(s.data, start)
		if journalRollback != nil || dataRollback != nil {
			return encodeStats{}, newSpoolFailure(failurePermanent, "could not roll back failed acceptance metadata")
		}
		return encodeStats{}, newSpoolFailure(failureRetryable, "could not persist callback acceptance metadata")
	}
	s.acceptedBytes = end
	s.acceptedRecords += stats.Records
	s.acceptedCallbacks++
	s.journalEnd = journalStart + int64(len(line))
	return stats, nil
}

func (s *durableSpool) retire() *spoolFailure {
	if s.acceptedBytes == 0 && !s.retirementMarked {
		return nil
	}
	if !s.retirementMarked {
		entry := journalRecord{
			Kind:      "retire",
			End:       s.acceptedBytes,
			Records:   s.acceptedRecords,
			Callbacks: s.acceptedCallbacks,
		}
		line, err := marshalJournalRecord(entry)
		if err != nil {
			return newSpoolFailure(failurePermanent, "could not encode retirement metadata: %v", err)
		}
		start := s.journalEnd
		if _, err := s.journal.Seek(start, io.SeekStart); err != nil ||
			writeAllFile(s.journal, line) != nil || s.journal.Sync() != nil {
			if rollbackErr := truncateAndSync(s.journal, start); rollbackErr != nil {
				return newSpoolFailure(failurePermanent, "could not roll back failed retirement marker: %v", rollbackErr)
			}
			return newSpoolFailure(failureRetryable, "could not persist spool retirement marker")
		}
		s.journalEnd += int64(len(line))
		s.retirementMarked = true
	}
	if err := s.finishRetirement(); err != nil {
		return newSpoolFailure(failureRetryable, "could not finish durable spool retirement: %v", err)
	}
	return nil
}

func (s *durableSpool) finishRetirement() error {
	if err := truncateAndSync(s.data, 0); err != nil {
		return err
	}
	if err := s.writeFreshJournal(); err != nil {
		return err
	}
	s.acceptedBytes = 0
	s.acceptedRecords = 0
	s.acceptedCallbacks = 0
	s.retirementMarked = false
	_, err := s.data.Seek(0, io.SeekStart)
	return err
}

func (s *durableSpool) reader() io.Reader {
	return io.NewSectionReader(s.data, 0, s.acceptedBytes)
}

func (s *durableSpool) close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	var first error
	if s.journal != nil {
		if err := s.journal.Close(); err != nil {
			first = err
		}
	}
	if s.data != nil {
		if err := syscall.Flock(int(s.data.Fd()), syscall.LOCK_UN); err != nil && first == nil {
			first = err
		}
		if err := s.data.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func marshalJournalRecord(record journalRecord) ([]byte, error) {
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func hashFileRange(file *os.File, start, end int64) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, io.NewSectionReader(file, start, end-start)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func truncateAndSync(file *os.File, size int64) error {
	if err := file.Truncate(size); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	_, err := file.Seek(size, io.SeekStart)
	return err
}

func writeAllFile(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func syncParentDirectory(path string) error {
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
