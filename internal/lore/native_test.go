package lore_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/solessfir/lazylore/internal/lore"
)

func TestNativeWorkflow(t *testing.T) {
	binary, serverBinary := os.Getenv("LORE_TEST_BINARY"), os.Getenv("LORE_TEST_SERVER")
	if binary == "" || serverBinary == "" {
		t.Skip("set LORE_TEST_BINARY and LORE_TEST_SERVER to run the native Lore workflow")
	}
	for _, path := range []string{binary, serverBinary} {
		if !filepath.IsAbs(path) {
			t.Fatalf("native executable path must be absolute: %q", path)
		}
	}
	root := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, root)
	}
	serverDir, globalDir := filepath.Join(root, "server"), filepath.Join(root, "global")
	for _, dir := range []string{serverDir, globalDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("LORE_GLOBAL_PATH", globalDir)
	t.Setenv("LORE_AUTH_PATH", globalDir)
	t.Setenv("LORE_USE_SERVICE", "0")
	t.Setenv("LORE_ENV", "native-test")
	t.Setenv("LORE_CONFIG_PATH", serverDir)

	quic, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer quic.Close()
	grpc, err := net.Listen("tcp", quic.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer grpc.Close()
	health, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer health.Close()
	quicPort := quic.LocalAddr().(*net.UDPAddr).Port
	grpcPort := grpc.Addr().(*net.TCPAddr).Port
	healthPort := health.Addr().(*net.TCPAddr).Port
	config := fmt.Sprintf(`[server.quic]
host = "127.0.0.1"
port = %d
[server.grpc]
host = "127.0.0.1"
port = %d
[server.http]
host = "127.0.0.1"
port = %d
[immutable_store.local]
path = %s
[mutable_store.local]
path = %s
`, quicPort, grpcPort, healthPort, strconv.Quote(filepath.Join(serverDir, "immutable")), strconv.Quote(filepath.Join(serverDir, "mutable")))
	if err := os.WriteFile(filepath.Join(serverDir, "local.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(serverDir, "server.log")
	serverLog, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	server := exec.Command(serverBinary)
	server.Dir, server.Stdout, server.Stderr = serverDir, serverLog, serverLog
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "LORE__") {
			server.Env = append(server.Env, entry)
		}
	}
	quic.Close()
	grpc.Close()
	health.Close()
	if err := server.Start(); err != nil {
		serverLog.Close()
		t.Fatal(err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Wait() }()
	t.Cleanup(func() {
		server.Process.Kill()
		<-serverDone
		serverLog.Close()
	})
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health_check", healthPort))
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
	}
	if !ready {
		output, _ := os.ReadFile(logPath)
		t.Fatalf("native server did not become ready:\n%s", output)
	}

	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	cli := func(dir string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, args...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("lore %q: %v\n%s", args, err, output)
		}
	}
	url := fmt.Sprintf("lore://127.0.0.1:%d/native-test", quicPort)
	author := `native-test "quoted" \name 名字`
	cli(repo, "--identity="+author, "repository", "create", url)
	setIdentity := func(dir string) {
		t.Helper()
		path := filepath.Join(dir, ".lore", "config.toml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "identity =") {
			if err := os.WriteFile(path, append([]byte("identity = "+strconv.Quote(author)+"\n"), data...), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	setIdentity(repo)
	runner := lore.NewExecRunner(binary, repo)
	t.Cleanup(runner.Shutdown)
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(dir, name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	check := func(_ lore.Result, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	status := func() lore.Status {
		t.Helper()
		value, err := lore.GetStatus(runner)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	history := func() []lore.Revision {
		t.Helper()
		value, err := lore.History(runner, 10)
		if err != nil || len(value) == 0 {
			t.Fatalf("history = %#v, error = %v", value, err)
		}
		return value
	}
	symbols := "#@%[]?*.txt"
	if runtime.GOOS == "windows" {
		symbols = "#@%[].txt"
	}
	paths := []string{"-notes.txt", "--force", "move", "merge", "spaces 'quoted'.txt", symbols, "normal.txt", ".loreignore"}
	for _, path := range paths {
		write(repo, path, "initial\n")
	}
	write(repo, ".loreignore", "ignored.txt\n")
	write(repo, "ignored.txt", "ignored\n")
	initialStatus := status()
	visible := make(map[string]bool)
	for _, change := range append(initialStatus.Staged, initialStatus.Unstaged...) {
		visible[change.Path] = true
		if change.Path == "ignored.txt" {
			t.Fatal("status reported an ignored file")
		}
	}
	for _, path := range paths {
		if !visible[path] {
			t.Fatalf("status did not discover %q", path)
		}
	}
	check(lore.Stage(runner, paths[0]))
	staged := status().Staged
	if len(staged) != 1 || staged[0].Path != paths[0] {
		t.Fatalf("single-file stage touched other paths: %#v", staged)
	}
	check(lore.Unstage(runner, paths[0]))
	if staged := status().Staged; len(staged) != 0 {
		t.Fatalf("unstage left staged files: %#v", staged)
	}
	check(lore.Stage(runner, paths...))
	if staged := status().Staged; len(staged) != len(paths) {
		t.Fatalf("literal paths were not all staged: %#v", staged)
	}
	message := "--initial 'quoted' \"double quoted\"\nsecond line"
	check(lore.Commit(runner, message))
	first := history()[0]
	if first.Message != message {
		t.Fatalf("commit message = %q, want %q", first.Message, message)
	}
	if first.Author != author {
		t.Fatalf("commit author = %q, want %q", first.Author, author)
	}
	write(repo, paths[0], "initial\nmodified\n")
	status()
	patch, err := lore.Diff(runner, paths[0])
	if err != nil || !strings.Contains(patch, "+modified") {
		t.Fatalf("file diff = %q, error = %v", patch, err)
	}
	check(lore.Stage(runner, paths[0]))
	check(lore.Commit(runner, "second"))
	second := history()[0]
	patch, err = lore.DiffRevision(runner, first.Hash, second.Hash)
	if err != nil || !strings.Contains(patch, "+modified") {
		t.Fatalf("revision diff = %q, error = %v", patch, err)
	}
	main := status().Branch
	check(lore.CreateBranch(runner, "-feature"))
	check(lore.SwitchBranch(runner, "-feature"))
	if branch := status().Branch; branch != "-feature" {
		t.Fatalf("current branch = %q", branch)
	}
	if revisions, err := lore.HistoryForBranch(runner, "-feature", 10); err != nil || len(revisions) == 0 {
		t.Fatalf("branch history = %#v, error = %v", revisions, err)
	}
	check(lore.SwitchBranch(runner, main))
	check(lore.PushStream(runner, nil))

	peer := filepath.Join(root, "peer")
	cli(root, "--identity=native-test", "clone", url, peer)
	setIdentity(peer)
	peerRunner := lore.NewExecRunner(binary, peer)
	t.Cleanup(peerRunner.Shutdown)
	write(peer, "normal.txt", "peer update\n")
	check(lore.Stage(peerRunner, "normal.txt"))
	check(lore.Commit(peerRunner, "peer update"))
	check(lore.PushStream(peerRunner, nil))
	write(repo, "normal.txt", "local update\n")
	status()
	patch, err = lore.Diff(runner, "normal.txt")
	if err != nil || !strings.Contains(patch, "-initial") || !strings.Contains(patch, "+local update") || strings.Contains(patch, "-peer update") {
		t.Fatalf("diff used the remote head instead of the workspace revision: %q, error = %v", patch, err)
	}
	check(lore.Reset(runner, "normal.txt"))
	check(lore.Pull(runner))
	if content := read(repo, "normal.txt"); content != "peer update\n" {
		t.Fatalf("pull content = %q", content)
	}
	if latest := history()[0]; latest.Message != "peer update" {
		t.Fatalf("history did not refresh after pull: %#v", latest)
	}
	if err := os.Remove(filepath.Join(repo, paths[0])); err != nil {
		t.Fatal(err)
	}
	missing := false
	for _, change := range status().Unstaged {
		missing = missing || change.Path == paths[0] && change.Status == 'D'
	}
	if !missing {
		t.Fatal("status omitted a missing tracked file")
	}
	check(lore.Reset(runner, paths[0]))
	check(lore.LockAcquire(runner, "normal.txt"))
	locks, err := lore.LockStatus(runner, "normal.txt")
	if err != nil || len(locks) != 1 || locks[0].Path != "normal.txt" || locks[0].Owner == "" {
		t.Fatalf("acquired lock status = %#v, error = %v", locks, err)
	}
	userID, err := lore.CurrentUserID(runner)
	if err != nil || userID != locks[0].Owner {
		t.Fatalf("current identity = %q, lock owner = %q, error = %v", userID, locks[0].Owner, err)
	}
	check(lore.LockRelease(runner, "normal.txt"))
	if locks, err := lore.LockStatus(runner, "normal.txt"); err != nil || len(locks) != 0 {
		t.Fatalf("released lock status = %#v, error = %v", locks, err)
	}
	write(repo, paths[0], "discard this\n")
	write(repo, "move", "keep this\n")
	write(repo, "selected-new.txt", "discard this\n")
	write(repo, "untouched-new.txt", "keep this\n")
	status()
	check(lore.DiscardAllChanges(runner, []string{paths[0], "selected-new.txt"}))
	if content := read(repo, paths[0]); content != "initial\nmodified\n" {
		t.Fatalf("discarded tracked content = %q", content)
	}
	if _, err := os.Stat(filepath.Join(repo, "selected-new.txt")); !os.IsNotExist(err) {
		t.Fatalf("selected new file survived discard: %v", err)
	}
	if read(repo, "move") != "keep this\n" || read(repo, "untouched-new.txt") != "keep this\n" {
		t.Fatal("discard changed an unselected file")
	}
	// An external stage can race confirmation before the TUI receives a refresh.
	check(lore.Stage(runner, "move"))
	if _, err := lore.DiscardUnstagedChanges(runner, []string{"move"}); err == nil {
		t.Fatal("unstaged discard accepted a newly staged file")
	}
	if read(repo, "move") != "keep this\n" {
		t.Fatal("unstaged discard changed a newly staged file")
	}
	found := false
	for _, change := range status().Staged {
		found = found || change.Path == "move"
	}
	if !found {
		t.Fatal("unstaged discard removed the file's staging")
	}
}
