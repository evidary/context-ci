package capture

// The two built-in groups. A coding agent reads these files as instructions or as configuration
// whether or not anybody listed them in .context-ci.yaml, so they are found by name. The group
// names carry a dot, which no group in capture.files may, so the two can never collide.
const (
	GroupAgentInstructions = "agent.instructions"
	GroupAgentConfig       = "agent.config"
)

// AgentInstructionPatterns are files a coding agent reads as instructions: what to do, how to
// behave, which commands and skills exist.
//
// The list is matched by name only. A match says "a tool that follows this convention would read
// this file", not "a tool in this repository does"; and a tool that keeps its instructions
// somewhere else is not covered until the path is added to capture.files.
var AgentInstructionPatterns = []string{
	"**/AGENTS.md",
	"**/CLAUDE.md",
	"**/GEMINI.md",
	"**/SKILL.md",
	".agents/skills/**",
	".claude/agents/**",
	".claude/commands/**",
	".claude/rules/**",
	".claude/skills/**",
	".clinerules",
	".clinerules/**",
	".continue/rules/**",
	".cursor/rules/**",
	".cursorrules",
	".github/agents/**",
	".github/chatmodes/**",
	".github/copilot-instructions.md",
	".github/instructions/**",
	".github/prompts/**",
	".junie/guidelines.md",
	".kiro/steering/**",
	".roo/rules/**",
	".roorules",
	".windsurf/rules/**",
	".windsurfrules",
}

// AgentConfigPatterns are files that decide what a coding agent may run or reach: MCP server
// lists, hooks, permissions, plugin manifests. A change here adds a capability, not a sentence,
// which is why the class fails by default while an instruction edit only warns.
var AgentConfigPatterns = []string{
	".aider.conf.yml",
	".claude-plugin/**",
	".claude/hooks/**",
	".claude/settings.json",
	".codex/config.toml",
	".cursor/hooks.json",
	".cursor/mcp.json",
	".gemini/settings.json",
	".kiro/settings/mcp.json",
	".mcp.json",
	".roo/mcp.json",
	".vscode/mcp.json",
}
