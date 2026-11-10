package browser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"unicode"
)

const (
	maxLocalStatePatchBytes    = 16 << 20
	maxProfileDisplayNameRunes = 100
)

// LocalStateBackupSuffix 改账户名称前留的 Local State 原件，文件名是 "Local State" 加这个后缀
const LocalStateBackupSuffix = ".wytaof-backup"

var (
	localStateProfileNamePath      = []string{"profile", "info_cache", "Default", "name"}
	localStateUsingDefaultNamePath = []string{"profile", "info_cache", "Default", "is_using_default_name"}
)

// EnsureProfileDisplayName 把浏览器右上角账户菜单里显示的名称改成实例名称。
// 浏览器正在运行、文件结构不符合预期时一律不动；返回值表示是否改动了文件。
func EnsureProfileDisplayName(userDataDir string, displayName string) (bool, error) {
	name := sanitizeProfileDisplayName(displayName)
	if name == "" || strings.TrimSpace(userDataDir) == "" {
		return false, nil
	}
	if userDataDirInUse(userDataDir) {
		return false, nil
	}

	localStatePath := filepath.Join(userDataDir, "Local State")
	info, err := os.Stat(localStatePath)
	if os.IsNotExist(err) {
		return seedProfileDisplayName(userDataDir, name)
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() || info.Size() > maxLocalStatePatchBytes {
		return false, nil
	}

	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return false, err
	}
	patched, changed, err := patchLocalStateProfileName(data, name)
	if err != nil || !changed {
		return false, err
	}
	if err := backupLocalStateOnce(localStatePath, data); err != nil {
		return false, fmt.Errorf("备份 Local State 失败，已放弃修改: %w", err)
	}
	if err := writeFileAtomic(localStatePath, patched, info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// backupLocalStateOnce 第一次改 Local State 之前留一份原件。
// Local State 里存着解密 Cookie 用的密钥，原件在，登录状态就能找回来。
func backupLocalStateOnce(localStatePath string, original []byte) error {
	backupPath := localStatePath + LocalStateBackupSuffix
	if info, err := os.Stat(backupPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s 不是普通文件", backupPath)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomic(backupPath, original, 0o644)
}

func sanitizeProfileDisplayName(name string) string {
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	})
	cleaned := []rune(strings.Join(fields, " "))
	if len(cleaned) > maxProfileDisplayNameRunes {
		cleaned = cleaned[:maxProfileDisplayNameRunes]
	}
	return strings.TrimSpace(string(cleaned))
}

func userDataDirInUse(userDataDir string) bool {
	if goruntime.GOOS == "windows" {
		lockPath := filepath.Join(userDataDir, "lockfile")
		file, err := os.OpenFile(lockPath, os.O_RDWR, 0)
		if err != nil {
			return !os.IsNotExist(err)
		}
		_ = file.Close()
		return false
	}
	_, err := os.Lstat(filepath.Join(userDataDir, "SingletonLock"))
	return err == nil
}

// seedProfileDisplayName 只处理从没启动过的实例：预置账户名称，并标记“首次运行已完成”。
// 不标记的话，命令行里没有 --no-first-run 时，浏览器第一次运行会自己把账户名称改成 Work。
func seedProfileDisplayName(userDataDir string, name string) (bool, error) {
	prefsPath := filepath.Join(userDataDir, "Default", "Preferences")
	if _, err := os.Stat(prefsPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		return false, err
	}
	prefs, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"name":               name,
			"using_default_name": false,
		},
	})
	if err != nil {
		return false, err
	}
	if err := writeFileAtomic(prefsPath, prefs, 0o644); err != nil {
		return false, err
	}

	localStatePath := filepath.Join(userDataDir, "Local State")
	if _, err := os.Stat(localStatePath); !os.IsNotExist(err) {
		return true, nil
	}
	localState, err := json.Marshal(map[string]any{
		"browser": map[string]any{"first_run_finished": true},
	})
	if err != nil {
		return true, err
	}
	return true, writeFileAtomic(localStatePath, localState, 0o644)
}

func patchLocalStateProfileName(data []byte, name string) ([]byte, bool, error) {
	if !json.Valid(data) {
		return nil, false, fmt.Errorf("Local State 不是合法的 JSON，已跳过账户名称同步")
	}
	nameSpan, found, err := findJSONValueSpan(data, localStateProfileNamePath)
	if err != nil || !found {
		return nil, false, err
	}
	var current string
	if err := json.Unmarshal(data[nameSpan.start:nameSpan.end], &current); err != nil {
		return nil, false, nil
	}
	flagSpan, hasFlag, err := findJSONValueSpan(data, localStateUsingDefaultNamePath)
	if err != nil {
		return nil, false, err
	}
	flagIsTrue := hasFlag && string(bytes.TrimSpace(data[flagSpan.start:flagSpan.end])) == "true"
	if current == name && !flagIsTrue {
		return data, false, nil
	}

	encodedName, err := encodeJSONString(name)
	if err != nil {
		return nil, false, err
	}
	replacements := []jsonSpanReplacement{{span: nameSpan, value: encodedName}}
	if flagIsTrue {
		replacements = append(replacements, jsonSpanReplacement{span: flagSpan, value: []byte("false")})
	}
	patched := applyJSONSpanReplacements(data, replacements)

	if !json.Valid(patched) {
		return nil, false, fmt.Errorf("账户名称写入后 Local State 校验不通过，已放弃修改")
	}
	verifySpan, ok, err := findJSONValueSpan(patched, localStateProfileNamePath)
	if err != nil || !ok {
		return nil, false, fmt.Errorf("账户名称写入后回读失败，已放弃修改")
	}
	var verify string
	if err := json.Unmarshal(patched[verifySpan.start:verifySpan.end], &verify); err != nil || verify != name {
		return nil, false, fmt.Errorf("账户名称写入后回读不一致，已放弃修改")
	}
	return patched, true, nil
}

func encodeJSONString(value string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

type jsonSpan struct {
	start int
	end   int
}

type jsonSpanReplacement struct {
	span  jsonSpan
	value []byte
}

func applyJSONSpanReplacements(data []byte, replacements []jsonSpanReplacement) []byte {
	ordered := append([]jsonSpanReplacement{}, replacements...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].span.start < ordered[j].span.start })

	var out bytes.Buffer
	out.Grow(len(data) + 64)
	cursor := 0
	for _, item := range ordered {
		out.Write(data[cursor:item.span.start])
		out.Write(item.value)
		cursor = item.span.end
	}
	out.Write(data[cursor:])
	return out.Bytes()
}

// findJSONValueSpan 返回 path 指向的值在原始 JSON 里的字节范围，其余内容一个字节都不解析改写。
func findJSONValueSpan(data []byte, path []string) (jsonSpan, bool, error) {
	scanner := &jsonSpanScanner{data: data}
	scanner.skipSpace()
	return scanner.findValue(path)
}

type jsonSpanScanner struct {
	data []byte
	pos  int
}

func (s *jsonSpanScanner) peek() byte {
	if s.pos >= len(s.data) {
		return 0
	}
	return s.data[s.pos]
}

func (s *jsonSpanScanner) skipSpace() {
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

func (s *jsonSpanScanner) findValue(path []string) (jsonSpan, bool, error) {
	if len(path) == 0 {
		start := s.pos
		if err := s.skipValue(); err != nil {
			return jsonSpan{}, false, err
		}
		return jsonSpan{start: start, end: s.pos}, true, nil
	}
	if s.peek() != '{' {
		return jsonSpan{}, false, s.skipValue()
	}

	s.pos++
	s.skipSpace()
	if s.peek() == '}' {
		s.pos++
		return jsonSpan{}, false, nil
	}

	var result jsonSpan
	found := false
	seenKey := false
	for {
		s.skipSpace()
		key, err := s.readString()
		if err != nil {
			return jsonSpan{}, false, err
		}
		s.skipSpace()
		if s.peek() != ':' {
			return jsonSpan{}, false, fmt.Errorf("JSON 结构异常：位置 %d 缺少冒号", s.pos)
		}
		s.pos++
		s.skipSpace()

		if key == path[0] {
			if seenKey {
				return jsonSpan{}, false, fmt.Errorf("JSON 结构异常：键 %q 重复出现", key)
			}
			seenKey = true
			span, ok, err := s.findValue(path[1:])
			if err != nil {
				return jsonSpan{}, false, err
			}
			result, found = span, ok
		} else if err := s.skipValue(); err != nil {
			return jsonSpan{}, false, err
		}

		s.skipSpace()
		switch s.peek() {
		case ',':
			s.pos++
		case '}':
			s.pos++
			return result, found, nil
		default:
			return jsonSpan{}, false, fmt.Errorf("JSON 结构异常：位置 %d 不是逗号或右花括号", s.pos)
		}
	}
}

func (s *jsonSpanScanner) readString() (string, error) {
	start := s.pos
	if err := s.skipString(); err != nil {
		return "", err
	}
	var value string
	if err := json.Unmarshal(s.data[start:s.pos], &value); err != nil {
		return "", err
	}
	return value, nil
}

func (s *jsonSpanScanner) skipString() error {
	if s.peek() != '"' {
		return fmt.Errorf("JSON 结构异常：位置 %d 不是字符串", s.pos)
	}
	s.pos++
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return nil
		default:
			s.pos++
		}
	}
	return fmt.Errorf("JSON 结构异常：字符串没有结束")
}

func (s *jsonSpanScanner) skipValue() error {
	switch s.peek() {
	case '"':
		return s.skipString()
	case '{':
		return s.skipContainer('}', true)
	case '[':
		return s.skipContainer(']', false)
	case 0:
		return fmt.Errorf("JSON 结构异常：内容提前结束")
	}
	start := s.pos
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			if s.pos == start {
				return fmt.Errorf("JSON 结构异常：位置 %d 缺少值", s.pos)
			}
			return nil
		}
		s.pos++
	}
	return nil
}

func (s *jsonSpanScanner) skipContainer(closing byte, isObject bool) error {
	s.pos++
	s.skipSpace()
	if s.peek() == closing {
		s.pos++
		return nil
	}
	for {
		s.skipSpace()
		if isObject {
			if err := s.skipString(); err != nil {
				return err
			}
			s.skipSpace()
			if s.peek() != ':' {
				return fmt.Errorf("JSON 结构异常：位置 %d 缺少冒号", s.pos)
			}
			s.pos++
			s.skipSpace()
		}
		if err := s.skipValue(); err != nil {
			return err
		}
		s.skipSpace()
		switch s.peek() {
		case ',':
			s.pos++
		case closing:
			s.pos++
			return nil
		default:
			return fmt.Errorf("JSON 结构异常：位置 %d 不是逗号或结束符", s.pos)
		}
	}
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if perm == 0 {
		perm = 0o644
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wytaof-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	_ = os.Chmod(tmpPath, perm)
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
