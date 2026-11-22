package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type auditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`
	SourcePath string    `json:"source_path"`
	NewName    string    `json:"new_name"`
	Status     string    `json:"status"`
	PlanID     string    `json:"plan_id,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type auditLogger struct {
	file    *os.File
	encoder *json.Encoder
}

func openAuditLogger(filename string) (*auditLogger, error) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开审计日志失败: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("设置审计日志权限失败: %w", err)
	}
	return &auditLogger{file: file, encoder: json.NewEncoder(file)}, nil
}

func (logger *auditLogger) write(entry auditEntry) error {
	entry.Timestamp = time.Now().UTC()
	if err := logger.encoder.Encode(entry); err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}
	if err := logger.file.Sync(); err != nil {
		return fmt.Errorf("同步审计日志失败: %w", err)
	}
	return nil
}

func (logger *auditLogger) close() error {
	return logger.file.Close()
}
