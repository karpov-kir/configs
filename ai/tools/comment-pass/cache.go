package commentpass

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// A rerun calls the model only for a file whose input changed. The page, the file as the change left
// it and the reviewer's notes on it are hashed together. A file whose hash matches its last call takes
// that call's reply again, so a comment Kirill accepted is not sampled afresh. The cache holds one
// reply per file and nothing else.

// cacheEntry is the last call of one file.
type cacheEntry struct {
	Key   string `json:"key"`
	Reply string `json:"reply"`
}

// inputKey is the hash of everything one file's call reads.
func inputKey(page, file string, notes []string) string {
	h := sha256.New()
	for _, part := range []string{page, file, strings.Join(notes, "\n")} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// cachePath is where one file's last call is kept: the state directory, the repository and the path.
func cachePath(stateHome, repoKey, path string) string {
	return filepath.Join(stateHome, "kk-flavor", "comment-pass", repoKey, strings.ReplaceAll(path, "/", "_")+".json")
}

// cachedReply is the last reply for the file where its key matches, or empty.
func cachedReply(file, key string) string {
	body, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var entry cacheEntry
	if json.Unmarshal(body, &entry) != nil || entry.Key != key {
		return ""
	}
	return entry.Reply
}

// keepReply records the file's reply under its key.
func keepReply(file, key, reply string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(cacheEntry{Key: key, Reply: reply}, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(body, '\n'), 0o644)
}
