package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// migrateLegacyFingerprintSchemes 升级到指纹方案版本后的一次性迁移：
// 先把数据库整份备份到 data/backups/，再把所有未绑定方案的实例绑到默认方案（固定项统一为方案值，种子等设备项保留）。
func (a *App) migrateLegacyFingerprintSchemes() {
	if a.config == nil || a.config.Browser.FingerprintSchemeMigrated {
		return
	}
	log := logger.New("Browser")
	dbPath := a.resolveAppPath(a.config.Database.SQLite.Path)
	if backupDir, err := backupSQLiteFiles(dbPath, "fpscheme"); err != nil {
		log.Error("迁移前备份数据库失败，本次不执行指纹方案迁移", logger.F("error", err.Error()))
		return
	} else if backupDir != "" {
		log.Info("指纹方案迁移前已备份数据库", logger.F("dir", backupDir))
	}
	updated, err := a.browserMgr.MigrateLegacyProfilesToDefaultScheme()
	if err != nil {
		log.Error("实例绑定默认指纹方案失败", logger.F("error", err.Error()), logger.F("updated", updated))
		return
	}
	a.config.Browser.FingerprintSchemeMigrated = true
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("保存指纹方案迁移标记失败", logger.F("error", err.Error()))
		return
	}
	log.Info("已把全部实例绑定到默认指纹方案", logger.F("updated", updated))
}

// backupSQLiteFiles 把 app.db 及其 -wal / -shm 原样复制到 data/backups/<标签>-<时间>/
func backupSQLiteFiles(dbPath string, tag string) (string, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	dir := filepath.Join(filepath.Dir(dbPath), "backups", fmt.Sprintf("%s-%s", tag, time.Now().Format("20060102-150405")))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFileContents(src, filepath.Join(dir, filepath.Base(src))); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}