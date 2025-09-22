package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type yamlAgent struct {
	Name          string   `yaml:"name"`
	Persona       string   `yaml:"persona"`
	Role          string   `yaml:"role"`
	Expertise     []string `yaml:"expertise"`
	SystemMessage string   `yaml:"systemMessage"`
	Tools         []string `yaml:"tools"`
}

type agentSummary struct {
	Persona   string   `json:"persona"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Expertise []string `json:"expertise"`
}

func resolveAgentsDir() string {
	if v := os.Getenv("AGENTS_DIR"); v != "" {
		return v
	}
	// Default inside container image
	if _, err := os.Stat("/app/agents"); err == nil {
		return "/app/agents"
	}
	// Fallback to repo-relative path when running locally from backend directory
	return filepath.Clean("../runners/claude-code-runner/agents")
}

func readAllAgentYAMLs(dir string) ([]yamlAgent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []yamlAgent
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") || strings.HasPrefix(name, "agent-schema") || name == "README.yaml" {
			continue
		}
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var a yamlAgent
		if err := yaml.Unmarshal(b, &a); err != nil {
			continue
		}
		if a.Persona != "" {
			out = append(out, a)
		}
	}
	return out, nil
}

func listAgents(c *gin.Context) {
	dir := resolveAgentsDir()
	agents, err := readAllAgentYAMLs(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read agents: %v", err)})
		return
	}
	resp := make([]agentSummary, 0, len(agents))
	for _, a := range agents {
		resp = append(resp, agentSummary{
			Persona:   a.Persona,
			Name:      a.Name,
			Role:      a.Role,
			Expertise: a.Expertise,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func getAgentMarkdown(c *gin.Context) {
	persona := c.Param("persona")
	if persona == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "persona required"})
		return
	}
	md, err := renderAgentMarkdownContent(persona)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "persona not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to render agent markdown: %v", err)})
		return
	}
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
}

// renderAgentMarkdownContent builds the markdown content for a given agent persona
// by reading its YAML definition from the configured agents directory.
func renderAgentMarkdownContent(persona string) (string, error) {
	if strings.TrimSpace(persona) == "" {
		return "", fmt.Errorf("persona required")
	}
	dir := resolveAgentsDir()
	agents, err := readAllAgentYAMLs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read agents: %w", err)
	}
	var found *yamlAgent
	for i := range agents {
		if strings.EqualFold(agents[i].Persona, persona) {
			found = &agents[i]
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("persona not found")
	}
	var sb strings.Builder

	// --- YAML Front Matter ---
	prettyPersona := titleCaseFromSnakeOrUpper(found.Persona)
	displayName := found.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = prettyPersona
	}

	// Derive description: prefer expertise list, then role, then first line of systemMessage
	description := ""
	if len(found.Expertise) > 0 {
		description = fmt.Sprintf("%s Agent focused on %s.", displayName, strings.Join(found.Expertise, ", "))
	} else if strings.TrimSpace(found.Role) != "" {
		description = fmt.Sprintf("%s Agent focused on %s.", displayName, found.Role)
	} else if strings.TrimSpace(found.SystemMessage) != "" {
		firstLine := strings.SplitN(strings.TrimSpace(found.SystemMessage), "\n", 2)[0]
		description = firstLine
	} else {
		description = fmt.Sprintf("%s Agent.", displayName)
	}

	fmt.Fprintf(&sb, "---\n")
	fmt.Fprintf(&sb, "name: %s (%s)\n", displayName, prettyPersona)
	fmt.Fprintf(&sb, "description: %s\n", description)
	fmt.Fprintf(&sb, "tools: Read, Write, Edit, Bash, Glob, Grep, WebSearch\n")
	fmt.Fprintf(&sb, "---\n\n")

	// --- Existing Markdown Content ---
	fmt.Fprintf(&sb, "# %s (%s)\n\n", found.Name, found.Persona)
	if found.Role != "" {
		fmt.Fprintf(&sb, "- Role: %s\n", found.Role)
	}
	if len(found.Expertise) > 0 {
		fmt.Fprintf(&sb, "- Expertise:\n")
		for _, e := range found.Expertise {
			fmt.Fprintf(&sb, "  - %s\n", e)
		}
	}
	if found.SystemMessage != "" {
		fmt.Fprintf(&sb, "\n## System message\n\n%s\n", found.SystemMessage)
	}
	return sb.String(), nil
}

// titleCaseFromSnakeOrUpper converts strings like "ENGINEERING_MANAGER" or "engineering manager"
// into "Engineering Manager".
func titleCaseFromSnakeOrUpper(s string) string {
	s = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", " ")
	parts := strings.Fields(s)
	for i := range parts {
		p := parts[i]
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
