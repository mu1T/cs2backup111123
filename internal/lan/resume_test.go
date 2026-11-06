package lan

import (
	"testing"
	"time"
)

func TestDecideResume(t *testing.T) {
	mod := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		have      int64
		stored    ResumeMeta
		respTotal int64
		respMod   time.Time
		wantSkip  int64
		wantValid bool
	}{
		{
			name:      "没有断点文件：从头下",
			have:      0,
			stored:    ResumeMeta{},
			respTotal: 1000,
			respMod:   mod,
			wantSkip:  0,
			wantValid: false,
		},
		{
			name:      "包没变：从断点续",
			have:      400,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 1000,
			respMod:   mod,
			wantSkip:  400,
			wantValid: true,
		},
		{
			name:      "长度变了：断点作废",
			have:      400,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 2000,
			respMod:   mod,
			wantSkip:  0,
			wantValid: false,
		},
		{
			name:      "修改时间变了：断点作废",
			have:      400,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 1000,
			respMod:   mod.Add(time.Hour),
			wantSkip:  0,
			wantValid: false,
		},
		{
			name:      "服务端没给修改时间：长度相同就续",
			have:      400,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 1000,
			respMod:   time.Time{},
			wantSkip:  400,
			wantValid: true,
		},
		{
			name:      "断点里没存修改时间：长度相同就续",
			have:      400,
			stored:    ResumeMeta{Total: 1000},
			respTotal: 1000,
			respMod:   mod,
			wantSkip:  400,
			wantValid: true,
		},
		{
			name:      "断点已经等于包长：没什么可续的",
			have:      1000,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 1000,
			respMod:   mod,
			wantSkip:  0,
			wantValid: false,
		},
		{
			name:      "断点比包还大（文件被改过）：作废",
			have:      1500,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 1000,
			respMod:   mod,
			wantSkip:  0,
			wantValid: false,
		},
		{
			name:      "响应没给长度：无法判断，从头下",
			have:      400,
			stored:    ResumeMeta{Total: 1000, ModTime: mod},
			respTotal: 0,
			respMod:   mod,
			wantSkip:  0,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideResume(tt.have, tt.stored, tt.respTotal, tt.respMod)
			if got.Skip != tt.wantSkip || got.Valid != tt.wantValid {
				t.Errorf("DecideResume = {%d %v}, want {%d %v}", got.Skip, got.Valid, tt.wantSkip, tt.wantValid)
			}
		})
	}
}
