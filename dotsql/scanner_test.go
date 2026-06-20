package dotsql

import (
	"bufio"
	"strings"
	"testing"
)

func TestGetTag(t *testing.T) {
	var tests = []struct {
		line string
		want string
	}{
		{"SELECT 1+1", ""},
		{"-- Some Comment", ""},
		{"-- name:  ", ""},
		{"-- name: find-users-by-name", "find-users-by-name"},
		{"  --  name:  save-user ", "save-user"},
	}

	for _, c := range tests {
		got := getTag(c.line)
		if got != c.want {
			t.Errorf("isTag('%s') == %s, expect %v", c.line, got, c.want)
		}
	}
}

func TestScannerRun(t *testing.T) {
	sqlFile := `
	-- name: all-users
	-- Finds all users
	SELECT * from USER

	-- name: empty-query-should-not-be-stored
	-- name: save-user
	INSERT INTO users (?, ?, ?)
	`

	scanner := &Scanner{}
	queries := scanner.Run(bufio.NewScanner(strings.NewReader(sqlFile)))

	numberOfQueries := len(queries)
	expectedQueries := 3
	if numberOfQueries != expectedQueries {
		t.Errorf("Scanner/Run() has %d queries instead of %d",
			numberOfQueries, expectedQueries)
	}
	// empty-query-should-not-be-stored should be marked as disabled
	if info, ok := queries["empty-query-should-not-be-stored"]; !ok {
		t.Errorf("Scanner/Run() missing empty-query-should-not-be-stored")
	} else if !info.Disabled {
		t.Errorf("Scanner/Run() empty-query-should-not-be-stored should be disabled")
	}
}
