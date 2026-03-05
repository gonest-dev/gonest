package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

func main() {
	create := flag.Bool("create", false, "Create tags")
	push := flag.Bool("push", false, "Push tags to remote")
	delete := flag.Bool("delete", false, "Delete tags locally and remotely")
	purge := flag.Bool("purge", false, "Delete all tags EXCEPT the ones for the specified version")
	bump := flag.String("bump", "", "Bump version: 'minor' (2nd number) or 'patch' (3rd number)")
	flag.Parse()

	// If purge is specified
	if *purge {
		args := flag.Args()
		if len(args) < 1 || args[0] == "" {
			fmt.Println("Error: version to keep is required for --purge")
			os.Exit(1)
		}
		purgeTags(args[0])
		return
	}

	// If bump is specified, we need to infer the version from the latest git tag
	var version string
	var oldVersion string
	if *bump != "" {
		if *bump != "minor" && *bump != "patch" {
			fmt.Println("Error: --bump must be 'minor' or 'patch'")
			os.Exit(1)
		}

		latest := getLatestTag()
		if latest == "" {
			fmt.Println("Error: could not find any existing tags to bump from")
			os.Exit(1)
		}
		oldVersion = latest
		version = bumpVersion(latest, *bump)
		fmt.Printf("Bumping version form %s to %s (level: %s)\n", oldVersion, version, *bump)

		// When bumping, we implicitly assume we want to create and push the new tag
		// AND delete the old tag as per requirements ("costumo remover a tag anterior")
		// The requirements say: "criando uma nova tag (usando o tag-create e tag-push) e em seguida removendo a anterior (tag-delete)"
		*create = true
		*push = true
		*delete = true
	} else {
		args := flag.Args()
		if len(args) < 1 || args[0] == "" {
			if !*create && !*push && !*delete {
				fmt.Println("Usage: go run . [--bump minor|patch] | [--create|--push|--delete <version>]")
				os.Exit(1)
			}
			fmt.Println("Error: version is required unless --bump is used")
			os.Exit(1)
		}
		version = args[0]
	}

	modules := readModulesFromGoWork()

	if len(modules) == 0 {
		fmt.Println("Warning: no modules found in go.work")
	}

	// 1. Create new tags (if requested)
	if *create {
		createTags(version, modules)
	}

	// 2. Push new tags (if requested)
	if *push {
		pushTags(version, modules)
	}

	// 3. Delete old tags (if bumping, we delete the OLD version)
	// If NOT bumping, but --delete was passed, we delete the PASSED version.
	if *delete {
		targetDeleteVersion := version
		if *bump != "" {
			targetDeleteVersion = oldVersion
		}

		if targetDeleteVersion != "" {
			deleteTags(targetDeleteVersion, modules)
		}
	}
}

func getLatestTag() string {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		return "v0.0.0" // Fallback para iniciar o bump
	}
	return strings.TrimSpace(string(out))
}

func bumpVersion(ver, level string) string {
	// Remove v prefix if exists
	cleanVer := strings.TrimPrefix(ver, "v")

	parts := strings.Split(cleanVer, ".")
	if len(parts) < 3 {
		// Handle cases like "0.1" -> treat as "0.1.0"
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
	}

	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])

	switch level {
	case "minor": // bumping 2nd number
		minor++
		patch = 0
	case "patch": // bumping 3rd number
		patch++
	}

	newVer := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	return "v" + newVer
}

func readModulesFromGoWork() []string {
	file, err := os.Open("go.work")
	if err != nil {
		fmt.Printf("Error opening go.work: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var modules []string
	scanner := bufio.NewScanner(file)
	inUseBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}

		if inUseBlock && line == ")" {
			inUseBlock = false
			continue
		}

		if inUseBlock {
			if idx := strings.Index(line, "//"); idx != -1 {
				line = strings.TrimSpace(line[:idx])
			}

			line = strings.Trim(line, "\"")

			if line == "" {
				continue
			}

			module := strings.TrimPrefix(line, "./")

			if strings.HasPrefix(module, "_tools/") {
				continue
			}

			modules = append(modules, module)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading go.work: %v\n", err)
		os.Exit(1)
	}

	return modules
}

func createTags(version string, modules []string) {
	if tagExists(version) {
		fmt.Printf("Tag %s already exists for root, skipping creation.\n", version)
	} else {
		fmt.Printf("Creating tag %s for root...\n", version)
		mustRun("git", "tag", version)
	}

	runParallel(modules, 5, func(mod string) {
		tag := fmt.Sprintf("%s/%s", mod, version)
		if tagExists(tag) {
			fmt.Printf("Tag %s already exists, skipping creation.\n", tag)
			return
		}
		fmt.Printf("Creating tag %s...\n", tag)
		mustRun("git", "tag", tag)
	})

	fmt.Println("All tags creation processed.")
}

func tagExists(tag string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/tags/"+tag)
	return cmd.Run() == nil
}

func pushTags(version string, modules []string) {
	fmt.Printf("Pushing tag %s...\n", version)
	mustRun("git", "push", "origin", version)

	runParallel(modules, 5, func(mod string) {
		tag := fmt.Sprintf("%s/%s", mod, version)
		fmt.Printf("Pushing tag %s...\n", tag)
		mustRun("git", "push", "origin", tag)
	})

	fmt.Println("All tags pushed successfully.")
}

func deleteTags(version string, modules []string) {
	if tagExists(version) {
		fmt.Printf("Deleting tag %s...\n", version)
		run("git", "tag", "-d", version)
	} else {
		fmt.Printf("Tag %s not found locally, skipping delete.\n", version)
	}
	run("git", "push", "origin", ":refs/tags/"+version)

	runParallel(modules, 5, func(mod string) {
		tag := fmt.Sprintf("%s/%s", mod, version)
		if tagExists(tag) {
			fmt.Printf("Deleting tag %s...\n", tag)
			run("git", "tag", "-d", tag)
		} else {
			fmt.Printf("Tag %s not found locally, skipping delete.\n", tag)
		}
		run("git", "push", "origin", ":refs/tags/"+tag)
	})

	fmt.Println("Tag deletion completed (some tags may not have existed).")
}

func purgeTags(version string) {
	fmt.Printf("Purging all tags except version %s...\n", version)

	cmd := exec.Command("git", "tag")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error listing tags: %v\n", err)
		return
	}

	tags := strings.Split(string(out), "\n")
	suffix := version
	if !strings.HasPrefix(suffix, "v") {
		// If the user passed 0.1.0 instead of v0.1.0, we still want to match correctly if tags have 'v'.
		// But usually we match exactly what's passed at the end of the tag.
	}

	// Double check if we should match exactly at the end or just contain.
	// The pwsh was -notmatch 'v0.1.0$' -> which means "doesn't end with v0.1.0"

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		if strings.HasSuffix(tag, version) {
			fmt.Printf("Keeping tag: %s\n", tag)
			continue
		}

		fmt.Printf("Purging tag: %s\n", tag)
		// Delete locally
		run("git", "tag", "-d", tag)
		// Delete remotely
		run("git", "push", "origin", ":refs/tags/"+tag)
	}

	fmt.Println("Purge completed.")
}

func runParallel(modules []string, maxConcurrency int, fn func(string)) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrency)

	for _, mod := range modules {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release
			fn(m)
		}(mod)
	}

	wg.Wait()
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: command %s %v failed: %v\n", name, args, err)
	}
}

func mustRun(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running command %s %v: %v\n", name, args, err)
		os.Exit(1)
	}
}
