package logger

import (
	"fmt"
	"sync"
	"testing"
)

func TestLogBuffer_AddAndGetAll(t *testing.T) {
	buf := NewLogBuffer(10)

	e1 := buf.Add(LevelInfo, "App started")
	e2 := buf.Add(LevelSuccess, "Connected to backend")
	e3 := buf.Add(LevelWarning, "Slow response")
	e4 := buf.Add(LevelError, "Failed to load")

	if e1.ID != 1 || e2.ID != 2 || e3.ID != 3 || e4.ID != 4 {
		t.Fatalf("expected incrementing IDs, got %d, %d, %d, %d", e1.ID, e2.ID, e3.ID, e4.ID)
	}

	entries := buf.GetAll()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Message != "App started" || entries[3].Message != "Failed to load" {
		t.Fatalf("unexpected entries content: %+v", entries)
	}
}

func TestLogBuffer_CapacityLimit(t *testing.T) {
	capacity := 5
	buf := NewLogBuffer(capacity)

	for i := 1; i <= 10; i++ {
		buf.Add(LevelInfo, fmt.Sprintf("message %d", i))
	}

	entries := buf.GetAll()
	if len(entries) != capacity {
		t.Fatalf("expected bounded length %d, got %d", capacity, len(entries))
	}

	// Should contain the last 5 messages: 6, 7, 8, 9, 10
	if entries[0].Message != "message 6" {
		t.Errorf("expected first retained message to be 'message 6', got '%s'", entries[0].Message)
	}
	if entries[4].Message != "message 10" {
		t.Errorf("expected last message to be 'message 10', got '%s'", entries[4].Message)
	}
}

func TestLogBuffer_Clear(t *testing.T) {
	buf := NewLogBuffer(10)
	buf.Add(LevelInfo, "test")
	if len(buf.GetAll()) != 1 {
		t.Fatalf("expected 1 entry")
	}

	buf.Clear()
	if len(buf.GetAll()) != 0 {
		t.Fatalf("expected 0 entries after Clear")
	}
}

func TestLogBuffer_ConcurrentSafety(t *testing.T) {
	buf := NewLogBuffer(100)
	var wg sync.WaitGroup

	numGoroutines := 20
	iterations := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf.Add(LevelInfo, fmt.Sprintf("goroutine %d iter %d", gid, j))
				if j%10 == 0 {
					_ = buf.GetAll()
				}
			}
		}(i)
	}

	wg.Wait()
	entries := buf.GetAll()
	if len(entries) != 100 {
		t.Fatalf("expected bounded 100 entries, got %d", len(entries))
	}
}
