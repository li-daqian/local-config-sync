package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/li-daqian/local-config-sync/internal/core"
)

const version = "0.1.0"

type parsedArguments struct {
	Options     map[string]string
	Flags       map[string]bool
	Positionals []string
}

func parseArguments(args []string, valueOptions, boolOptions map[string]bool) (parsedArguments, error) {
	result := parsedArguments{Options: map[string]string{}, Flags: map[string]bool{}, Positionals: []string{}}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if !strings.HasPrefix(argument, "--") {
			result.Positionals = append(result.Positionals, argument)
			continue
		}
		name := argument
		if equal := strings.Index(argument, "="); equal >= 0 {
			name, result.Options[argument[:equal]] = argument[:equal], argument[equal+1:]
			if !valueOptions[name] {
				return result, core.Invalidf("unknown option %s", name)
			}
			continue
		}
		if boolOptions[name] {
			result.Flags[name] = true
			continue
		}
		if !valueOptions[name] {
			return result, core.Invalidf("unknown option %s", name)
		}
		if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
			return result, core.Invalidf("option %s requires a value", name)
		}
		index++
		result.Options[name] = args[index]
	}
	return result, nil
}

func required(options map[string]string, name string) (string, error) {
	value := strings.TrimSpace(options[name])
	if value == "" {
		return "", core.Invalidf("required option %s not specified", name)
	}
	return value, nil
}

func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}
func removeJSON(args []string) []string {
	result := []string{}
	for _, arg := range args {
		if arg != "--json" {
			result = append(result, arg)
		}
	}
	return result
}

func success(command string, payload map[string]any, jsonOutput bool) error {
	response := map[string]any{"ok": true, "command": command}
	for key, value := range payload {
		response[key] = value
	}
	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(response)
	}
	fmt.Fprintf(os.Stdout, "%s completed successfully\n", command)
	if len(payload) > 0 {
		content, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(os.Stdout, string(content))
	}
	return nil
}

func outputFailure(command string, err error, jsonOutput bool) int {
	local := core.AsError(err)
	if jsonOutput {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "command": command, "error": local})
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", local.Code, local.Message)
	}
	if code, ok := core.ExitCodes[local.Code]; ok {
		return code
	}
	return 1
}

func usage() {
	fmt.Fprintln(os.Stdout, `Usage: local-config <command> [options]

Synchronize project-local overlay configuration through managed repositories.

Commands:
  init
  provider github <auth|repositories>
  repository add <git|local-folder>
  repository <list|show|files|doctor|auth|remove>
  preview
  link [--kind file|directory] [--initial-strategy auto|local|remote]
  pull | push | sync
  status
  doctor
  unlink`)
}

func run(rawArgs []string) (activeCommand string, err error) {
	jsonOutput := jsonRequested(rawArgs)
	args := removeJSON(rawArgs)
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		usage()
		return "help", nil
	}
	if args[0] == "--version" || args[0] == "-V" {
		fmt.Fprintln(os.Stdout, version)
		return "version", nil
	}
	service := core.NewService("")
	activeCommand = args[0]
	switch args[0] {
	case "init":
		parsed, err := parseArguments(args[1:], map[string]bool{"--default-link-mode": true}, map[string]bool{})
		if err != nil {
			return activeCommand, err
		}
		mode := core.LinkMode(parsed.Options["--default-link-mode"])
		if mode == "" {
			mode = core.LinkModeSymlink
		}
		if mode != core.LinkModeSymlink && mode != core.LinkModeCopy {
			return activeCommand, core.Invalidf("--default-link-mode must be symlink or copy")
		}
		config, err := service.Init(mode)
		if err != nil {
			return activeCommand, err
		}
		return activeCommand, success(activeCommand, map[string]any{"home": service.Paths.Home, "config": config}, jsonOutput)
	case "repository":
		return runRepository(service, args[1:], jsonOutput)
	case "provider":
		return runProvider(args[1:], jsonOutput)
	case "preview":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true, "--repository": true, "--source-path": true, "--target": true, "--kind": true}, map[string]bool{})
		if err != nil {
			return activeCommand, err
		}
		repositoryID, err := required(parsed.Options, "--repository")
		if err != nil {
			return activeCommand, err
		}
		sourcePath, err := required(parsed.Options, "--source-path")
		if err != nil {
			return activeCommand, err
		}
		targetPath, err := required(parsed.Options, "--target")
		if err != nil {
			return activeCommand, err
		}
		project := parsed.Options["--project"]
		if project == "" {
			project = "."
		}
		preview, err := service.PreviewLink(core.LinkInput{Project: project, RepositoryID: repositoryID, SourcePath: sourcePath, TargetPath: targetPath, Kind: core.MappingKind(parsed.Options["--kind"])})
		if err != nil {
			return activeCommand, err
		}
		content, _ := json.Marshal(preview)
		payload := map[string]any{}
		_ = json.Unmarshal(content, &payload)
		return activeCommand, success(activeCommand, payload, jsonOutput)
	case "link":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true, "--repository": true, "--source-path": true, "--target": true, "--mode": true, "--kind": true, "--initial-strategy": true, "--id": true}, map[string]bool{"--allow-sensitive": true})
		if err != nil {
			return activeCommand, err
		}
		repositoryID, err := required(parsed.Options, "--repository")
		if err != nil {
			return activeCommand, err
		}
		sourcePath, err := required(parsed.Options, "--source-path")
		if err != nil {
			return activeCommand, err
		}
		targetPath, err := required(parsed.Options, "--target")
		if err != nil {
			return activeCommand, err
		}
		project := parsed.Options["--project"]
		if project == "" {
			project = "."
		}
		mode := core.LinkMode(parsed.Options["--mode"])
		if mode != "" && mode != core.LinkModeSymlink && mode != core.LinkModeCopy {
			return activeCommand, core.Invalidf("--mode must be symlink or copy")
		}
		kind := core.MappingKind(parsed.Options["--kind"])
		strategy := core.InitialStrategy(parsed.Options["--initial-strategy"])
		mapping, err := service.Link(core.LinkInput{Project: project, RepositoryID: repositoryID, SourcePath: sourcePath, TargetPath: targetPath, Mode: mode, Kind: kind, InitialStrategy: strategy, ID: parsed.Options["--id"], AllowSensitive: parsed.Flags["--allow-sensitive"]})
		if err != nil {
			return activeCommand, err
		}
		return activeCommand, success(activeCommand, map[string]any{"projectPath": mapping.ProjectPath, "mapping": mapping}, jsonOutput)
	case "pull", "push", "sync":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true, "--repository": true}, map[string]bool{"--allow-sensitive": true})
		if err != nil {
			return activeCommand, err
		}
		project, repositoryID := parsed.Options["--project"], parsed.Options["--repository"]
		if project == "" && repositoryID == "" {
			project = "."
		}
		results, err := service.Sync(core.SyncOptions{Project: project, RepositoryID: repositoryID, AllowSensitive: parsed.Flags["--allow-sensitive"]}, args[0])
		if err != nil {
			return activeCommand, err
		}
		payload := map[string]any{"repositories": results}
		if project != "" {
			payload["projectPath"] = project
		}
		return activeCommand, success(activeCommand, payload, jsonOutput)
	case "status":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true}, map[string]bool{})
		if err != nil {
			return activeCommand, err
		}
		project := parsed.Options["--project"]
		if project == "" {
			project = "."
		}
		status, err := service.Status(project)
		if err != nil {
			return activeCommand, err
		}
		content, _ := json.Marshal(status)
		payload := map[string]any{}
		_ = json.Unmarshal(content, &payload)
		return activeCommand, success(activeCommand, payload, jsonOutput)
	case "doctor":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true}, map[string]bool{})
		if err != nil {
			return activeCommand, err
		}
		result, err := service.Doctor(parsed.Options["--project"], "")
		if err != nil {
			return activeCommand, err
		}
		payload := map[string]any{"ok": result.OK, "checks": result.Checks}
		if parsed.Options["--project"] != "" {
			payload["projectPath"] = parsed.Options["--project"]
		}
		return activeCommand, success(activeCommand, payload, jsonOutput)
	case "unlink":
		parsed, err := parseArguments(args[1:], map[string]bool{"--project": true}, map[string]bool{"--keep-files": true, "--keep-exclude": true})
		if err != nil {
			return activeCommand, err
		}
		project := parsed.Options["--project"]
		if project == "" {
			project = "."
		}
		removed, err := service.Unlink(project, parsed.Flags["--keep-files"], parsed.Flags["--keep-exclude"])
		if err != nil {
			return activeCommand, err
		}
		return activeCommand, success(activeCommand, map[string]any{"projectPath": project, "removedMappings": removed}, jsonOutput)
	default:
		return activeCommand, core.Invalidf("unknown command %s", args[0])
	}
}

func runRepository(service *core.Service, args []string, jsonOutput bool) (string, error) {
	if len(args) == 0 {
		return "repository", core.Invalidf("repository subcommand is required")
	}
	active := "repository." + args[0]
	switch args[0] {
	case "add":
		active = "repository.add"
		if len(args) < 2 {
			return active, core.Invalidf("repository type is required")
		}
		switch args[1] {
		case "git":
			parsed, err := parseArguments(args[2:], map[string]bool{"--id": true, "--name": true, "--url": true, "--branch": true}, map[string]bool{})
			if err != nil {
				return active, err
			}
			id, err := required(parsed.Options, "--id")
			if err != nil {
				return active, err
			}
			remoteURL, err := required(parsed.Options, "--url")
			if err != nil {
				return active, err
			}
			branch := parsed.Options["--branch"]
			if branch == "" {
				branch = "main"
			}
			if _, err := service.Init(core.LinkModeSymlink); err != nil {
				return active, err
			}
			repository, err := service.Repositories.AddGit(id, parsed.Options["--name"], remoteURL, branch)
			if err != nil {
				return active, err
			}
			return active, success(active, map[string]any{"repository": core.SanitizeRepository(repository)}, jsonOutput)
		case "local-folder":
			parsed, err := parseArguments(args[2:], map[string]bool{"--id": true, "--name": true, "--path": true}, map[string]bool{})
			if err != nil {
				return active, err
			}
			id, err := required(parsed.Options, "--id")
			if err != nil {
				return active, err
			}
			path, err := required(parsed.Options, "--path")
			if err != nil {
				return active, err
			}
			if _, err := service.Init(core.LinkModeSymlink); err != nil {
				return active, err
			}
			repository, err := service.Repositories.AddLocalFolder(id, parsed.Options["--name"], path)
			if err != nil {
				return active, err
			}
			return active, success(active, map[string]any{"repository": core.SanitizeRepository(repository)}, jsonOutput)
		default:
			return active, core.Invalidf("unsupported repository type %s", args[1])
		}
	case "list":
		if _, err := parseArguments(args[1:], map[string]bool{}, map[string]bool{}); err != nil {
			return active, err
		}
		repositories, err := service.Repositories.List()
		if err != nil {
			return active, err
		}
		safe := make([]core.SafeRepository, 0, len(repositories))
		for _, repository := range repositories {
			safe = append(safe, core.SanitizeRepository(repository))
		}
		return active, success(active, map[string]any{"repositories": safe}, jsonOutput)
	case "show", "files", "doctor", "remove":
		parsed, err := parseArguments(args[1:], map[string]bool{}, map[string]bool{})
		if err != nil {
			return active, err
		}
		if len(parsed.Positionals) != 1 {
			return active, core.Invalidf("repository id is required")
		}
		id := parsed.Positionals[0]
		if args[0] == "show" {
			repository, err := service.Repositories.Get(id)
			if err != nil {
				return active, err
			}
			return active, success(active, map[string]any{"repository": core.SanitizeRepository(repository)}, jsonOutput)
		}
		if args[0] == "files" {
			files, err := service.RepositoryFiles(id)
			if err != nil {
				return active, err
			}
			return active, success(active, map[string]any{"repositoryId": files.RepositoryID, "files": files.Files}, jsonOutput)
		}
		if args[0] == "doctor" {
			result, err := service.Doctor("", id)
			if err != nil {
				return active, err
			}
			return active, success(active, map[string]any{"repositoryId": id, "ok": result.OK, "checks": result.Checks}, jsonOutput)
		}
		removed, err := service.Repositories.Remove(id)
		if err != nil {
			return active, err
		}
		return active, success(active, map[string]any{"repository": core.SanitizeRepository(removed), "workspacePreserved": true}, jsonOutput)
	case "auth":
		parsed, err := parseArguments(args[1:], map[string]bool{"--url": true, "--method": true}, map[string]bool{})
		if err != nil {
			return active, err
		}
		method := parsed.Options["--method"]
		if method == "" {
			method = "auto"
		}
		valid := map[string]bool{"auto": true, "ssh": true, "credential": true, "gh": true}
		if !valid[method] {
			return active, core.Invalidf("unsupported authentication method %s", method)
		}
		id, remoteURL := "", parsed.Options["--url"]
		if len(parsed.Positionals) > 1 {
			return active, core.Invalidf("too many repository ids")
		}
		if len(parsed.Positionals) == 1 {
			id = parsed.Positionals[0]
		}
		if (id == "") == (remoteURL == "") {
			return active, core.Invalidf("Specify exactly one repository id or --url")
		}
		var checks []core.DiagnosticCheck
		if id != "" {
			checks, err = service.Authenticate(id, method)
		} else {
			checks, err = service.AuthenticateURL(remoteURL, method)
		}
		if err != nil {
			return active, err
		}
		payload := map[string]any{"method": method, "checks": checks}
		if id != "" {
			payload["repositoryId"] = id
		} else {
			payload["remoteUrl"] = remoteURL
		}
		return active, success(active, payload, jsonOutput)
	default:
		return active, core.Invalidf("unknown repository subcommand %s", args[0])
	}
}

func runProvider(args []string, jsonOutput bool) (string, error) {
	if len(args) != 2 || args[0] != "github" {
		return "provider", core.Invalidf("usage: provider github <auth|repositories>")
	}
	active := "provider.github." + args[1]
	switch args[1] {
	case "auth":
		checks, err := core.AuthenticateGitHubProvider()
		if err != nil {
			return active, err
		}
		return active, success(active, map[string]any{"provider": "github", "authenticated": true, "checks": checks}, jsonOutput)
	case "repositories":
		repositories, err := core.ListGitHubRepositories()
		if err != nil {
			return active, err
		}
		return active, success(active, map[string]any{"provider": "github", "repositories": repositories}, jsonOutput)
	default:
		return active, core.Invalidf("unknown GitHub provider subcommand %s", args[1])
	}
}

func main() {
	jsonOutput := jsonRequested(os.Args[1:])
	active, err := run(os.Args[1:])
	if err != nil {
		os.Exit(outputFailure(active, err, jsonOutput))
	}
}
