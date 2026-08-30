package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCheckArgsJSON(t *testing.T) {
	path, jsonOut, err := parseCheckArgs([]string{"check", "--json", "/tmp/repo"})
	if err != nil || path != "/tmp/repo" || !jsonOut {
		t.Fatalf("got path=%q json=%v err=%v", path, jsonOut, err)
	}
	path, jsonOut, err = parseCheckArgs([]string{"check", "/tmp/repo"})
	if err != nil || path != "/tmp/repo" || jsonOut {
		t.Fatalf("got path=%q json=%v err=%v", path, jsonOut, err)
	}
}

func TestBuildCheckJSONResultClearAndAttention(t *testing.T) {
	rows := []AggregateRow{{
		Machine: "laptop",
		Local:   true,
		Repo: RepoSnapshot{
			Path:    "/code/app",
			Origin:  "github.com/acme/app",
			Branch:  "main",
			IsClean: true,
		},
	}}
	clear := buildCheckJSONResult("app", rows, nil)
	if clear.SchemaVersion != 1 || !clear.OK || clear.Attention || len(clear.Situations) != 0 {
		t.Fatalf("clear: %+v", clear)
	}
	if len(clear.Machines) != 1 || !clear.Machines[0].Local || clear.Machines[0].ID != "laptop" {
		t.Fatalf("machines: %+v", clear.Machines)
	}

	sit := []Situation{{
		Kind:     SituationLocalDirty,
		Nudge:    "commit or stash local changes",
		Machines: []string{"laptop"},
	}}
	att := buildCheckJSONResult("app", rows, sit)
	if att.OK || !att.Attention || len(att.Situations) != 1 || att.Situations[0].Kind != string(SituationLocalDirty) {
		t.Fatalf("attention: %+v", att)
	}
}

func TestBuildCheckJSONDistinguishesLocalRemote(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "laptop", Local: true, Repo: RepoSnapshot{Origin: "github.com/acme/app", Branch: "main", IsClean: true}},
		{Machine: "desktop", Local: false, Repo: RepoSnapshot{Origin: "github.com/acme/app", Branch: "main", IsDirty: true, HasUnstaged: true}},
	}
	got := buildCheckJSONResult("app", rows, nil)
	if len(got.Machines) != 2 {
		t.Fatalf("want 2 machines: %+v", got.Machines)
	}
	var sawLocal, sawRemote bool
	for _, m := range got.Machines {
		if m.Local {
			sawLocal = true
		} else {
			sawRemote = true
		}
	}
	if !sawLocal || !sawRemote {
		t.Fatalf("expected local and remote markers: %+v", got.Machines)
	}
}

func TestOrderCheckRowsLocalFirst(t *testing.T) {
	rows := []AggregateRow{
		{Machine: "z-remote", Local: false},
		{Machine: "a-remote", Local: false},
		{Machine: "laptop", Local: true},
	}
	got := orderCheckRows(rows)
	if !got[0].Local || got[0].Machine != "laptop" {
		t.Fatalf("local first: %+v", got)
	}
	if got[1].Machine != "a-remote" || got[2].Machine != "z-remote" {
		t.Fatalf("remotes by id: %+v", got)
	}
}

func TestFormatCheckMachineCell(t *testing.T) {
	got := formatCheckMachineCell(RepoSnapshot{
		Branch:      "main",
		IsDirty:     true,
		HasUnstaged: true,
		HasStaged:   true,
	})
	want := "Dirty on main (unstaged, staged)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	clean := formatCheckMachineCell(RepoSnapshot{Branch: "main", IsClean: true})
	if clean != "Clean on main" {
		t.Fatalf("clean: %q", clean)
	}
}

func TestRunCheckModeJSONClear(t *testing.T) {
	dir := initEmptyCommitRepo(t)
	stdout, code := captureCheckJSON(t, dir, "m1", true)
	if code != exitCheckOK {
		t.Fatalf("exit=%d want %d; out=%s", code, exitCheckOK, stdout)
	}
	var got CheckJSONResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v\nout=%q", err, stdout)
	}
	if got.SchemaVersion != 1 || !got.OK || got.Attention || got.Error != "" {
		t.Fatalf("result: %+v", got)
	}
	if len(got.Machines) == 0 {
		t.Fatal("expected machines")
	}
}

func TestRunCheckModeJSONAttention(t *testing.T) {
	dir := initEmptyCommitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, code := captureCheckJSON(t, dir, "m1", true)
	if code != exitCheckAttention {
		t.Fatalf("exit=%d want %d; out=%s", code, exitCheckAttention, stdout)
	}
	var got CheckJSONResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v\nout=%q", err, stdout)
	}
	if got.OK || !got.Attention || len(got.Situations) == 0 {
		t.Fatalf("result: %+v", got)
	}
}

func TestRunCheckModeJSONErrorNotFalseClear(t *testing.T) {
	dir := t.TempDir() // not a git repo
	stdout, code := captureCheckJSON(t, dir, "m1", true)
	if code != exitCheckError {
		t.Fatalf("exit=%d want %d", code, exitCheckError)
	}
	var got CheckJSONResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json: %v\nout=%q", err, stdout)
	}
	if got.OK || got.Attention || got.Error == "" {
		t.Fatalf("error payload must not look clear: %+v", got)
	}
}

func TestRunCheckModeHumanStillText(t *testing.T) {
	dir := initEmptyCommitRepo(t)
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := runCheckMode(context.Background(), dir, "m1", "", true, 0, false)
	w.Close()
	os.Stdout = old
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if code != exitCheckOK {
		t.Fatalf("exit=%d", code)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("human mode should not be JSON: %q", out)
	}
	if !strings.Contains(out, "→ ok") {
		t.Fatalf("missing ok nudge: %q", out)
	}
}

func initEmptyCommitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "t@example.com"},
		{"git", "config", "user.name", "t"},
		{"git", "commit", "--allow-empty", "-m", "init"},
		{"git", "remote", "add", "origin", bare},
		{"git", "push", "-u", "origin", "main"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	return dir
}

func captureCheckJSON(t *testing.T, path, machine string, jsonOut bool) (stdout string, code int) {
	t.Helper()
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code = runCheckMode(context.Background(), path, machine, "", true, 0, jsonOut)
	w.Close()
	os.Stdout = old
	_, _ = buf.ReadFrom(r)
	return buf.String(), code
}
