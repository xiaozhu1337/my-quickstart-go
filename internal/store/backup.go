package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
	"my-quickstart-go/internal/model"
)

// MaxBackupSize bounds each backup, including SQLite WAL data and embedded icons.
const MaxBackupSize int64 = 64 << 20

func ReadBackup(path string) (model.State, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".db":
		return readSQLiteBackup(path)
	case ".json", ".yaml", ".yml":
	default:
		return model.State{}, errors.New("backup extension must be .yaml, .yml, .json or .db")
	}
	data, err := readLimited(path, MaxBackupSize)
	if err != nil {
		return model.State{}, err
	}
	state := model.State{Settings: model.Default().Settings}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		var wire struct {
			Version       int             `json:"version"`
			ActiveGroupID string          `json:"activeGroupId"`
			Groups        []model.Group   `json:"groups"`
			Settings      json.RawMessage `json:"settings"`
		}
		if err = decodeJSON(data, &wire); err != nil {
			return state, err
		}
		state.Version, state.ActiveGroupID, state.Groups = wire.Version, wire.ActiveGroupID, wire.Groups
		if len(wire.Settings) > 0 {
			if err = decodeSettings(wire.Settings, &state.Settings); err != nil {
				return state, err
			}
		}
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err = decoder.Decode(&state); err != nil {
			return state, err
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			return state, errors.New("backup must contain exactly one YAML document")
		}
	}
	if state.Group(state.ActiveGroupID) == nil && len(state.Groups) > 0 {
		state.ActiveGroupID = state.Groups[0].ID
	}
	return state, validate(state)
}

func decodeJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("backup must contain exactly one JSON value")
	}
	return nil
}

func decodeSettings(data []byte, settings *model.Settings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if size, ok := raw["cardSize"]; ok {
		if _, present := raw["cardWidth"]; !present {
			raw["cardWidth"] = size
		}
		delete(raw, "cardSize")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return decodeJSON(data, settings)
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("backup must be a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("backup exceeds %d bytes", MaxBackupSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("backup exceeds %d bytes", MaxBackupSize)
	}
	return data, nil
}

func readSQLiteBackup(path string) (model.State, error) {
	// SQLite may create SHM files even in read-only mode. Open a private copy to leave the source untouched.
	dir, err := os.MkdirTemp("", "launcher-import-*")
	if err != nil {
		return model.State{}, err
	}
	defer os.RemoveAll(dir)
	copyPath := filepath.Join(dir, "launcher.db")
	remaining := MaxBackupSize
	for _, suffix := range []string{"", "-wal", "-journal"} {
		data, readErr := readLimited(path+suffix, remaining)
		if suffix != "" && errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return model.State{}, readErr
		}
		remaining -= int64(len(data))
		if err = os.WriteFile(copyPath+suffix, data, 0600); err != nil {
			return model.State{}, err
		}
	}
	db, err := connect(copyPath, "ro")
	if err != nil {
		return model.State{}, err
	}
	defer db.Close()
	var integrity string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil {
		return model.State{}, err
	}
	if integrity != "ok" {
		return model.State{}, fmt.Errorf("invalid SQLite backup: %s", integrity)
	}
	s := &Store{db: db, path: copyPath}
	return s.Load()
}

// WriteBackup replaces the destination only after the entire backup has been written and closed.
func WriteBackup(path string, state model.State) error {
	if err := validate(state); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var data []byte
	var err error
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(state, "", "  ")
	case ".yaml", ".yml":
		data, err = marshalYAML(state)
	case ".db":
	default:
		return errors.New("backup extension must be .yaml, .yml, .json or .db")
	}
	if err != nil {
		return err
	}
	if int64(len(data)) > MaxBackupSize {
		return fmt.Errorf("backup exceeds %d bytes", MaxBackupSize)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".launcher-backup-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if ext == ".db" {
		if err = file.Close(); err != nil {
			return err
		}
		db, openErr := connect(tempPath, "rw")
		if openErr != nil {
			return openErr
		}
		s := &Store{db: db, path: tempPath}
		err = s.Save(state)
		closeErr := s.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		info, statErr := os.Stat(tempPath)
		if statErr != nil {
			return statErr
		}
		if info.Size() > MaxBackupSize {
			return fmt.Errorf("backup exceeds %d bytes", MaxBackupSize)
		}
	} else {
		_, err = file.Write(data)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return os.Rename(tempPath, path)
}

var fieldComments = map[string]string{
	"version":              "配置格式版本，目前仅支持 1。",
	"activeGroupId":        "当前选中的分组 ID。",
	"groups":               "按显示顺序排列的分组。",
	"id":                   "唯一 ID；分组与启动项之间也不能重复。",
	"name":                 "界面显示名称。",
	"apps":                 "按显示顺序排列的启动项。",
	"path":                 "启动路径，或 command: / uwp: 类型的启动目标。",
	"args":                 "启动参数文本。",
	"envVars":              "启动环境变量，每项格式为 KEY=VALUE。",
	"icon":                 "保存的启动项图标。",
	"addedAt":              "添加时间，Unix 毫秒时间戳。",
	"settings":             "界面与行为设置；省略字段使用应用默认值。",
	"language":             "界面语言：zh-CN 或 en。",
	"cardWidth":            "卡片宽度（像素）。",
	"cardHeight":           "卡片高度（像素）。",
	"toggleHotkey":         "显示/隐藏窗口的全局快捷键；空字符串表示禁用。",
	"theme":                "主题：dark 或 light。",
	"sidebarWidth":         "侧边栏宽度（像素）。",
	"fontFamily":           "界面字体名称。",
	"fontSize":             "界面字号。",
	"cardFontSize":         "卡片名称字号。",
	"cardIconScale":        "卡片图标大小（像素）。",
	"dblClickBlankToHide":  "双击空白区域隐藏窗口。",
	"alwaysOnTop":          "窗口置顶。",
	"hideOnStartup":        "启动时隐藏窗口。",
	"useRelativePath":      "使用相对路径。",
	"rememberWindowBounds": "记住窗口位置和大小。",
	"windowBounds":         "保存的窗口范围；null 表示尚未保存。",
	"x":                    "窗口水平位置（像素）。",
	"y":                    "窗口垂直位置（像素）。",
	"width":                "窗口宽度（像素）。",
	"height":               "窗口高度（像素）。",
}

func marshalYAML(state model.State) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(state); err != nil {
		return nil, err
	}
	var comment func(*yaml.Node)
	comment = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i < len(n.Content); i += 2 {
				n.Content[i].HeadComment = fieldComments[n.Content[i].Value]
			}
		}
		for _, child := range n.Content {
			comment(child)
		}
	}
	comment(&node)
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
