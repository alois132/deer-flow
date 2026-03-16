package skills

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type fakeClient struct {
	uploads   []uploadCall
	downloads []downloadCall
	commands  []string
	findOut   string
}

type uploadCall struct {
	local  string
	remote string
}

type downloadCall struct {
	remote string
	local  string
}

func (f *fakeClient) UploadFile(ctx context.Context, localPath string, remotePath string) error {
	f.uploads = append(f.uploads, uploadCall{local: localPath, remote: remotePath})
	return nil
}

func (f *fakeClient) DownloadFile(ctx context.Context, remotePath string, localPath string) error {
	f.downloads = append(f.downloads, downloadCall{remote: remotePath, local: localPath})
	return os.WriteFile(localPath, []byte("data"), 0644)
}

func (f *fakeClient) ExecuteCommand(ctx context.Context, command string) (string, error) {
	f.commands = append(f.commands, command)
	if strings.HasPrefix(command, "find ") {
		return f.findOut, nil
	}
	return "", nil
}

func TestLocalStoreLoadUploadsFiles(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skill-a")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "ref.md"), []byte("ref"), 0644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	store := NewLocalStore(root)
	client := &fakeClient{}
	if err := store.Load(context.Background(), client, "/mnt/skills"); err != nil {
		t.Fatalf("load: %v", err)
	}

	var got []string
	for _, call := range client.uploads {
		got = append(got, filepath.ToSlash(call.remote))
	}
	sort.Strings(got)
	want := []string{
		"/mnt/skills/skill-a/SKILL.md",
		"/mnt/skills/skill-a/references/ref.md",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uploads mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestLocalStoreSaveDownloadsFiles(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStore(root)
	client := &fakeClient{
		findOut: "/mnt/skills/skill-a/SKILL.md\n/mnt/skills/skill-a/scripts/run.sh\n",
	}
	if err := store.Save(context.Background(), client, "/mnt/skills"); err != nil {
		t.Fatalf("save: %v", err)
	}

	if len(client.downloads) != 2 {
		t.Fatalf("expected 2 downloads, got %d", len(client.downloads))
	}
	expected := []string{
		filepath.Join(root, "skill-a", "SKILL.md"),
		filepath.Join(root, "skill-a", "scripts", "run.sh"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}
}

func TestLocalStoreLoadCleansRemoteExtras(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skill-a")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	store := NewLocalStore(root, WithCleanRemote(true))
	client := &fakeClient{
		findOut: "/mnt/skills/skill-a/SKILL.md\n/mnt/skills/skill-b/extra.md\n",
	}
	if err := store.Load(context.Background(), client, "/mnt/skills"); err != nil {
		t.Fatalf("load: %v", err)
	}

	var hasRemove bool
	for _, cmd := range client.commands {
		if strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "skill-b/extra.md") {
			hasRemove = true
			break
		}
	}
	if !hasRemove {
		t.Fatalf("expected cleanup command for extra file, got: %#v", client.commands)
	}
}
