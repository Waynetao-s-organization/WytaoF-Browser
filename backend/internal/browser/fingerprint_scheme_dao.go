package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FingerprintSchemeDAO 指纹方案持久化接口
type FingerprintSchemeDAO interface {
	List() ([]*FingerprintScheme, error)
	Upsert(scheme *FingerprintScheme) error
	Delete(schemeId string) error
	Count() (int, error)
}

type SQLiteFingerprintSchemeDAO struct {
	db *sql.DB
}

func NewSQLiteFingerprintSchemeDAO(db *sql.DB) *SQLiteFingerprintSchemeDAO {
	return &SQLiteFingerprintSchemeDAO{db: db}
}

func (d *SQLiteFingerprintSchemeDAO) List() ([]*FingerprintScheme, error) {
	rows, err := d.db.Query(`
		SELECT scheme_id, scheme_name, description, builtin, is_default, sort_order, fixed_json, device_json, created_at, updated_at
		FROM browser_fingerprint_schemes ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询指纹方案失败: %w", err)
	}
	defer rows.Close()
	var list []*FingerprintScheme
	for rows.Next() {
		var (
			s                     FingerprintScheme
			builtin, isDefault    int
			fixedJSON, deviceJSON string
		)
		if err := rows.Scan(&s.SchemeId, &s.SchemeName, &s.Description, &builtin, &isDefault, &s.SortOrder, &fixedJSON, &deviceJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取指纹方案失败: %w", err)
		}
		s.Builtin = builtin == 1
		s.IsDefault = isDefault == 1
		_ = json.Unmarshal([]byte(fixedJSON), &s.Fixed)
		_ = json.Unmarshal([]byte(deviceJSON), &s.Device)
		list = append(list, &s)
	}
	return list, rows.Err()
}

func (d *SQLiteFingerprintSchemeDAO) Upsert(scheme *FingerprintScheme) error {
	if scheme == nil {
		return errors.New("方案为空")
	}
	now := time.Now().Format(time.RFC3339)
	if scheme.CreatedAt == "" {
		scheme.CreatedAt = now
	}
	scheme.UpdatedAt = now
	fixedJSON, _ := json.Marshal(scheme.Fixed)
	deviceJSON, _ := json.Marshal(scheme.Device)
	builtin, isDefault := 0, 0
	if scheme.Builtin {
		builtin = 1
	}
	if scheme.IsDefault {
		isDefault = 1
	}
	_, err := d.db.Exec(`
		INSERT INTO browser_fingerprint_schemes
		  (scheme_id, scheme_name, description, builtin, is_default, sort_order, fixed_json, device_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scheme_id) DO UPDATE SET
		  scheme_name = excluded.scheme_name,
		  description = excluded.description,
		  builtin     = excluded.builtin,
		  is_default  = excluded.is_default,
		  sort_order  = excluded.sort_order,
		  fixed_json  = excluded.fixed_json,
		  device_json = excluded.device_json,
		  updated_at  = excluded.updated_at`,
		scheme.SchemeId, scheme.SchemeName, scheme.Description, builtin, isDefault, scheme.SortOrder,
		string(fixedJSON), string(deviceJSON), scheme.CreatedAt, scheme.UpdatedAt)
	if err != nil {
		return fmt.Errorf("保存指纹方案失败: %w", err)
	}
	return nil
}

func (d *SQLiteFingerprintSchemeDAO) Delete(schemeId string) error {
	if _, err := d.db.Exec(`DELETE FROM browser_fingerprint_schemes WHERE scheme_id = ?`, schemeId); err != nil {
		return fmt.Errorf("删除指纹方案失败: %w", err)
	}
	return nil
}

func (d *SQLiteFingerprintSchemeDAO) Count() (int, error) {
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM browser_fingerprint_schemes`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计指纹方案失败: %w", err)
	}
	return n, nil
}
