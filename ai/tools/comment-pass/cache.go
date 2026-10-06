package commentpass

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// A rerun calls the model only for a file whose input changed. The page, the prompt and the model are
// hashed together, and the prompt holds the file, its changed lines and the reviewer's notes. A file with its last call's hash reuses that
// call's reply, and a comment the reviewer accepted stays as it was. The cache holds one reply per
// file.

// cacheEntry is the last call of one file.
type cacheEntry struct {
	Key   string `json:"key"`
	Reply string `json:"reply"`
}

// inputKey is the hash of everything one file's call reads: the page, the prompt, which holds the file,
// its changed lines and the reviewer's notes, and the model.
func inputKey(page, prompt, model string) string {
	h := sha256.New()
	for _, part := range []string{page, prompt, model} {
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
