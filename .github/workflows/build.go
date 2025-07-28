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

	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(log.Writer())

	var tagName string
	flags.StringVar(&tagName, "tag-name", "", "name of the tag to build")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	if tagName == "" {
		log.Fatal("please provide a tag name")
	}

	if err := os.RemoveAll("./build"); err != nil {
		log.Fatalf("failed to delete build directory: %v", err)
	}
	if err := os.MkdirAll("./build", 0700); err != nil {
		log.Fatalf("failed to create build directory: %v", err)
	}

	generateOpenApi(tagName)

	builds := []osArch{
		{os: "linux", arch: "amd64"},
		{os: "windows", arch: "amd64"},
	}

	for _, build := range builds {
		goBuild(build, "-ldflags", "-X main.version="+tagName, "-o", "./go-bpmn", "./cmd/go-bpmn")
		goBuild(build, "-ldflags", "-X github.com/gclaussn/go-bpmn/daemon.version="+tagName, "-o", "./go-bpmn-memd", "./cmd/go-bpmn-memd")
		goBuild(build, "-ldflags", "-X github.com/gclaussn/go-bpmn/daemon.version="+tagName, "-o", "./go-bpmn-pgd", "./cmd/go-bpmn-pgd")

		createTarGz(build)
		createChecksum(build)
	}
}

type osArch struct {
	os   string
	arch string
}

func generateOpenApi(tagName string) {
	cmd := exec.Command("go", "run", "cmd/openapi/main.go", "-output-path", "./build/go-bpmn-openapi.yaml", "-version", tagName)

	log.Print(strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
	if len(out) != 0 {
		log.Println(string(out))
	}
}

func goBuild(build osArch, args ...string) {
	cmd := exec.Command("go")
	cmd.Args = append(cmd.Args, "build")
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	cmd.Env = append(cmd.Env, "GOOS="+build.os)
	cmd.Env = append(cmd.Env, "GOARCH="+build.arch)

	log.Printf("%s-%s: %s", build.os, build.arch, strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
	if len(out) != 0 {
		log.Println(string(out))
	}
}

func createTarGz(build osArch) {
	cmd := exec.Command("tar", "cfz", fmt.Sprintf("./build/go-bpmn-%s-%s.tar.gz", build.os, build.arch), "go-bpmn", "go-bpmn-memd", "go-bpmn-pgd")

	log.Printf("%s-%s: %s", build.os, build.arch, strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to run command: %v", err)
	}
	if len(out) != 0 {
		log.Println(string(out))
	}
}

func createChecksum(build osArch) {
	cmd := exec.Command("sha256sum", fmt.Sprintf("go-bpmn-%s-%s.tar.gz", build.os, build.arch))
	cmd.Dir = "./build"

	log.Printf("%s-%s: %s", build.os, build.arch, strings.Join(cmd.Args, " "))

	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to run command: %v", err)
	}

	checksumFile, err := os.OpenFile(fmt.Sprintf("./build/go-bpmn-%s-%s.sha256", build.os, build.arch), os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		log.Fatalf("failed to open checksum file: %v", err)
	}

	defer checksumFile.Close()

	_, err = checksumFile.Write(out)
	if err != nil {
		log.Fatalf("failed to write checksum file: %v", err)
	}
}
