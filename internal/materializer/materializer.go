package materializer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wayne930242/agent-workspace-engine/internal/authcheck"
	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
)

type Options struct {
	StrictAuth bool
}

func Materialize(outDir string, m *manifest.WorkspaceManifest) error {
	return MaterializeWithOptions(outDir, m, Options{})
}

func MaterializeWithOptions(outDir string, m *manifest.WorkspaceManifest, opts Options) error {
	workspaceDir := filepath.Join(outDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}

	baseRepoPath := resolvePath(m.SourceDir, m.BaseRepo.Path)
	switch m.BaseRepo.Kind {
	case "repo":
		skipPaths := buildBaseSkipPaths(m, baseRepoPath)
		includes := m.BaseRepo.Includes
		if err := copyDirFiltered(baseRepoPath, workspaceDir, func(path string) bool {
			base := filepath.Base(path)
			if base == ".git" || base == "build" || isUnderSkippedPath(path, skipPaths) {
				return true
			}
			if len(includes) > 0 {
				rel, err := filepath.Rel(baseRepoPath, path)
				if err != nil {
					return false
				}
				return !shouldInclude(rel, includes)
			}
			return false
		}); err != nil {
			return fmt.Errorf("copy base repo: %w", err)
		}
	case "git":
		if err := cloneGitRepo(m.BaseRepo, workspaceDir, opts.StrictAuth); err != nil {
			return fmt.Errorf("clone base git repo: %w", err)
		}
	default:
		return fmt.Errorf("unsupported base repo kind %q", m.BaseRepo.Kind)
	}

	attachedRoot := filepath.Join(workspaceDir, "_attached")
	for i, repo := range m.AttachedRepos {
		alias := repo.Alias
		if alias == "" {
			alias = fmt.Sprintf("repo-%d", i+1)
		}
		target := filepath.Join(attachedRoot, alias)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create attached repo root: %w", err)
		}
		switch repo.Kind {
		case "repo":
			if err := copyDirFiltered(resolvePath(m.SourceDir, repo.Path), target, func(path string) bool {
				return filepath.Base(path) == ".git"
			}); err != nil {
				return fmt.Errorf("copy attached repo %q: %w", alias, err)
			}
		case "git":
			if err := cloneGitRepo(repo, target, opts.StrictAuth); err != nil {
				return fmt.Errorf("clone attached git repo %q: %w", alias, err)
			}
		default:
			return fmt.Errorf("unsupported attached repo kind %q", repo.Kind)
		}
	}

	for _, overlay := range m.ResolvedOverlays {
		if err := copyDirFiltered(overlay.SourceDir, workspaceDir, func(string) bool { return false }); err != nil {
			return fmt.Errorf("apply overlay %q: %w", overlay.Namespace, err)
		}
	}

	if err := pruneEmptyDirs(workspaceDir, workspaceDir); err != nil {
		return fmt.Errorf("prune empty directories: %w", err)
	}

	return nil
}

func resolvePath(sourceDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(sourceDir, path))
}

func buildBaseSkipPaths(m *manifest.WorkspaceManifest, baseRepoPath string) []string {
	var skip []string

	for _, repo := range m.AttachedRepos {
		path := resolvePath(m.SourceDir, repo.Path)
		if isSubPath(baseRepoPath, path) {
			skip = append(skip, path)
		}
	}
	for _, overlay := range m.ResolvedOverlays {
		if isSubPath(baseRepoPath, overlay.SourceDir) {
			skip = append(skip, overlay.SourceDir)
		}
	}
	if m.Prompt != nil && m.Prompt.Kind == "file" {
		path := resolvePath(m.SourceDir, m.Prompt.Path)
		if isSubPath(baseRepoPath, path) {
			skip = append(skip, path)
		}
	}

	return skip
}

func shouldInclude(relPath string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	for _, inc := range includes {
		// exact match or file is under this include path
		if relPath == inc || strings.HasPrefix(relPath, inc+string(filepath.Separator)) {
			return true
		}
		// relPath is a parent directory of inc (e.g. relPath="src", inc="src/api")
		if strings.HasPrefix(inc, relPath+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isSubPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isUnderSkippedPath(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		rel, err := filepath.Rel(skip, path)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
			return true
		}
	}
	return false
}

func copyDirFiltered(src, dst string, skip func(path string) bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		if skip(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}

func cloneGitRepo(repo manifest.RepoRef, target string, strictAuth bool) error {
	if repo.URL == "" {
		return fmt.Errorf("git source requires URL")
	}
	if repo.AuthStrategy != "" {
		check := authcheck.Check(repo.AuthStrategy)
		if strictAuth && !check.Available {
			return fmt.Errorf("auth strategy %q is not available: %s", repo.AuthStrategy, check.Detail)
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	args := []string{"clone", "--depth", "1"}
	if repo.Ref != "" {
		args = append(args, "--branch", repo.Ref)
	}
	args = append(args, repo.URL, target)

	cmd := exec.Command("git", args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if repo.AuthStrategy != "" {
			return fmt.Errorf("git clone failed using auth strategy %q: %w: %s", repo.AuthStrategy, err, strings.TrimSpace(string(output)))
		}
		return fmt.Errorf("git clone failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func pruneEmptyDirs(root, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(dir, entry.Name())
		if err := pruneEmptyDirs(root, child); err != nil {
			return err
		}
	}

	if dir == root {
		return nil
	}

	entries, err = os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(dir)
	}
	return nil
}
