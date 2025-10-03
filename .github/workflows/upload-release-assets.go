package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	log.SetFlags(0)

	flags := flag.NewFlagSet("upload-release-assets", flag.ContinueOnError)
	flags.SetOutput(log.Writer())

	var releaseId string
	flags.StringVar(&releaseId, "release-id", "", "ID of the Github release")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	if releaseId == "" {
		log.Fatal("please provide a release ID")
	}

	buildArtifacts, err := os.ReadDir("./build")
	if err != nil {
		log.Fatalf("failed to read build directory: %v", err)
	}

	for _, buildArtifact := range buildArtifacts {
		name := buildArtifact.Name()

		var contentType string
		switch {
		case strings.HasSuffix(name, "tar.gz"):
			contentType = "application/gzip"
		case strings.HasSuffix(name, "sha256"):
			contentType = "text/plain"
		case strings.HasSuffix(name, "yaml"):
			contentType = "text/yaml"
		case strings.HasSuffix(name, "json"):
			contentType = "application/json"
		default:
			log.Fatalf("file %s has an unsupported extension", name)
		}

		uploadReleaseAsset(releaseId, name, contentType)
	}
}

func uploadReleaseAsset(releaseId, name string, contentType string) {
	githubToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		log.Fatal("please set environment variable GITHUB_TOKEN")
	}

	cmd := exec.Command(
		"curl",
		"-L",
		"-X", "POST",
		"-H", "Accept: application/vnd.github+json",
		"-H", "Authorization: Bearer "+githubToken,
		"-H", "X-GitHub-Api-Version: 2022-11-28",
		"-H", "Content-Type: "+contentType,
		fmt.Sprintf("https://uploads.github.com/repos/gclaussn/go-bpmn/releases/%s/assets?name=%s", releaseId, name),
		"--data-binary", "@./build/"+name,
	)

	out, err := cmd.Output()
	if len(out) != 0 {
		log.Println(string(out))
	}
	if err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
}
