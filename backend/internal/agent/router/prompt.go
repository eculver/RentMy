package router

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"text/template"
)

// promptCache loads and caches prompt templates from the filesystem.
// Prompts live at {promptsDir}/{agent_name}/v{N}.txt.
// The latest version is resolved by scanning the directory.
type promptCache struct {
	dir   string
	mu    sync.RWMutex
	cache map[string]*template.Template // key: "agent_name/v1"
}

func newPromptCache(dir string) *promptCache {
	return &promptCache{
		dir:   dir,
		cache: make(map[string]*template.Template),
	}
}

var versionRegexp = regexp.MustCompile(`^v(\d+)\.txt$`)

// LatestVersion returns the latest prompt version string (e.g., "v3") for the
// given agent name, by scanning the agent's prompt directory.
func (c *promptCache) LatestVersion(agentName string) (string, error) {
	dir := filepath.Join(c.dir, agentName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("prompt: reading dir %s: %w", dir, err)
	}

	var versions []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := versionRegexp.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		versions = append(versions, n)
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("prompt: no versioned prompts found in %s", dir)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))
	return fmt.Sprintf("v%d", versions[0]), nil
}

// Load returns a parsed template for the given agent name and version string.
// Results are cached in memory after the first load.
func (c *promptCache) Load(agentName, version string) (*template.Template, error) {
	key := agentName + "/" + version
	c.mu.RLock()
	t, ok := c.cache[key]
	c.mu.RUnlock()
	if ok {
		return t, nil
	}

	path := filepath.Join(c.dir, agentName, version+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("prompt: reading %s: %w", path, err)
	}
	t, err = template.New(key).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("prompt: parsing %s: %w", path, err)
	}

	c.mu.Lock()
	c.cache[key] = t
	c.mu.Unlock()
	return t, nil
}

// Render loads the latest version for the agent, renders it with data, and
// returns the rendered string plus the version identifier.
func (c *promptCache) Render(agentName string, data any) (rendered, version string, err error) {
	version, err = c.LatestVersion(agentName)
	if err != nil {
		return "", "", err
	}
	t, err := c.Load(agentName, version)
	if err != nil {
		return "", "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("prompt: rendering %s/%s: %w", agentName, version, err)
	}
	return buf.String(), version, nil
}

// UnknownTaskError is returned when a task is not in the tier matrix.
type UnknownTaskError struct {
	Task AgentTask
}

func (e *UnknownTaskError) Error() string {
	return fmt.Sprintf("router: unknown agent task %q — add it to tier_matrix.go", e.Task)
}
