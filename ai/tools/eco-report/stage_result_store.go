package ecoreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const resultFileLimit = 4 << 20

func (r *run) needsReportLock() bool {
	switch r.arg(0) {
	case "record", "merge-slot", "init", "invalidate", "stage-result", "result-context", "decisions-reviewed", "scope", "stamp", "gate", "carry", "state", "list", "close", "discard", "finalize", "promote":
		return true
	}
	return false
}

func resultDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (r *run) canonicalReportPath() string {
	parent, err := filepath.EvalSymlinks(filepath.Dir(r.report))
	if err == nil {
		return filepath.Join(parent, filepath.Base(r.report))
	}
	return filepath.Clean(r.report)
}

func (r *run) resultManifestPath() string {
	return r.gitCommonPath("idsd-stage-results/" + resultDigest([]byte(r.canonicalReportPath())) + ".json")
}

func (r *run) readResultManifest() *resultManifest {
	if protocol := fieldValue(r.report, "result-protocol"); protocol != "" && protocol != "1" {
		r.refuse("error: unsupported report result-protocol; restore a supported report before re-qualifying")
	}
	path := r.resultManifestPath()
	if info, err := os.Lstat(filepath.Dir(path)); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		r.refuse("error: stage result manifest directory is not a real directory")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		r.refuse("error: could not inspect stage result manifest directory: " + err.Error())
	}
	body, err := readResultFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if fieldValue(r.report, "result-protocol") != "" {
			r.refuse("error: typed report is missing its stage result manifest; restore the manifest before re-qualifying")
		}
		return nil
	}
	if err != nil {
		r.refuse("error: could not read stage result manifest: " + err.Error())
	}
	var manifest resultManifest
	if err := decodeResultJson(body, &manifest); err != nil {
		r.refuse("error: invalid stage result manifest: " + err.Error())
	}
	if manifest.Version != 1 || manifest.Report != r.canonicalReportPath() || manifest.Context.Version != 1 || manifest.Context.Attempt == "" || manifest.Results == nil {
		r.refuse("error: invalid stage result manifest identity")
	}
	ids, items := map[string]bool{}, map[string]bool{}
	for _, saved := range manifest.Results {
		if err := validateStageResult(saved.Result); err != nil {
			r.refuse("error: invalid stored stage result: " + err.Error())
		}
		if ids[saved.Result.Id] || len(saved.Before) != 64 {
			r.refuse("error: invalid duplicate stage result or recovery identity in manifest")
		}
		ids[saved.Result.Id] = true
		for _, item := range saved.Result.Items {
			if items[item.Id] {
				r.refuse("error: duplicate finding in stage result manifest")
			}
			items[item.Id] = true
		}
	}
	return &manifest
}

func (r *run) requireResultManifest() resultManifest {
	manifest := r.readResultManifest()
	if manifest == nil {
		r.refuse("error: no typed qualification attempt — run report.sh invalidate")
	}
	return *manifest
}

func (r *run) writeResultManifest(manifest resultManifest) {
	body, err := json.Marshal(manifest)
	body = append(body, '\n')
	if err == nil && len(body) > resultFileLimit {
		err = fmt.Errorf("result manifest exceeds %d bytes", resultFileLimit)
	}
	if err == nil {
		err = writePrivateResultFile(r.resultManifestPath(), body)
	}
	if err != nil {
		r.refuse("error: could not persist stage result manifest: " + err.Error())
	}
}

func (r *run) clearResultManifest() {
	if err := os.Remove(r.resultManifestPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.refuse("error: could not clear stage result manifest: " + err.Error())
	}
}

// Copy obligations before the directory moves. A crash can leave an unused copy, never a report
// whose findings lost their manifest. A failed promotion keeps the original authoritative copy.
func (r *run) preparePromotedResults(target string) []string {
	var originals []string
	priorReport := r.report
	defer func() { r.report = priorReport }()
	for _, name := range r.reportNames() {
		r.setReportPaths(name)
		manifest := r.readResultManifest()
		if manifest == nil {
			continue
		}
		r.assertResultProjection(*manifest)
		originals = append(originals, r.resultManifestPath())
		r.report = filepath.Join(target, "intents", name, agentsDirName, reportName)
		manifest.Report = r.canonicalReportPath()
		r.writeResultManifest(*manifest)
	}
	return originals
}

func readResultFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	handle, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	body, err := io.ReadAll(io.LimitReader(handle, resultFileLimit+1))
	if err == nil && len(body) > resultFileLimit {
		err = fmt.Errorf("result file exceeds %d bytes", resultFileLimit)
	}
	return body, err
}

func privateResultDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a real directory", path)
	}
	return os.Chmod(path, 0o700)
}

func writePrivateResultFile(path string, body []byte) error {
	parent := filepath.Dir(path)
	if err := privateResultDirectory(parent); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(parent, ".result.")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(body); err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		return err
	}
	return syncResultDirectory(parent)
}

func syncResultDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func syncResultProjection(path string) error {
	handle, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	err = handle.Sync()
	if closeErr := handle.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncResultDirectory(filepath.Dir(path))
}

// All report operations share this stable inode, including reads that combine the report and receipt.
// Locking the report itself would lose exclusion when atomic projection replaces its inode.
func (r *run) lockReports() *os.File {
	path := r.gitCommonPath("idsd-report.lock")
	handle, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		r.refuse("error: could not open report lock: " + err.Error())
	}
	info, err := handle.Stat()
	if err != nil || !info.Mode().IsRegular() {
		handle.Close()
		r.refuse("error: report lock is not a regular file")
	}
	if err := handle.Chmod(0o600); err != nil {
		handle.Close()
		r.refuse("error: could not protect report lock: " + err.Error())
	}
	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_EX); err != nil {
		handle.Close()
		r.refuse("error: could not lock reports: " + err.Error())
	}
	return handle
}
