package boot

import (
	"crypto/sha256"
	"encoding/hex"
	"my-quickstart-go/internal/model"
	"path/filepath"
	"runtime"
	"strings"
)

const Flag = "startup-tasks"

func Profile(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:8])
}

func Enabled(tasks []model.ScheduledTask) bool {
	for _, task := range tasks {
		if task.AtBoot && task.Enabled {
			return true
		}
	}
	return false
}
