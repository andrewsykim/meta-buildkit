package main

import (
	"context"
	"fmt"
	"os"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/system"
)

const workDir = "/meta-buildkit"

func main() {
	buildImage := llb.Image("docker.io/library/golang:1.17-alpine").
		AddEnv("PATH", "/usr/local/go/bin:"+system.DefaultPathEnvUnix).
		File(llb.Mkdir(workDir, os.ModeDir)).
		Dir(workDir).
		File(llb.Copy(llb.Local("src"), "go.mod", ".")).
		File(llb.Copy(llb.Local("src"), "go.sum", ".")).
		File(llb.Copy(llb.Local("src"), "main.go", ".")).
		Run(llb.Shlex("go mod download")).Root().
		Run(llb.Shlex("go build -o meta-buildkit main.go")).Root()

	meta := llb.Scratch().
		AddEnv("PATH", "/bin").
		File(llb.Mkdir("/bin", os.ModeDir)).
		File(llb.Copy(buildImage, "/meta-buildkit/meta-buildkit", "/bin/"))

	bkClient, err := client.New(context.TODO(), "tcp://127.0.0.1:8372")
	if err != nil {
		fmt.Printf("failed to connect to buildkit: %v\n", err)
		os.Exit(1)
	}
	defer bkClient.Close()

	llbDef, err := meta.Marshal(context.TODO())
	if err != nil {
		fmt.Printf("failed to marshall llb: %v\n", err)
		os.Exit(1)
	}

	ch := make(chan *client.SolveStatus)
	go func() {
		for {
			status := <-ch
			if status == nil {
				break
			}
			fmt.Printf("status is %v\n", status)
			for _, v := range status.Vertexes {
				fmt.Printf("====vertex: %+v\n", v)
			}
			for _, s := range status.Statuses {
				fmt.Printf("====status: %+v\n", s)
			}
			for _, l := range status.Logs {
				fmt.Printf("====log: %s\n", string(l.Data))
			}
		}
	}()

	localPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("failed to current path: %v\n", err)
		os.Exit(1)
	}

	solveOpt := client.SolveOpt{
		LocalDirs: map[string]string{
			"src": localPath,
		},
	}

	if err = llb.WriteTo(llbDef, os.Stdout); err != nil {
		fmt.Printf("failed to write LLB defintion to stdout: %v\n", err)
		os.Exit(1)
	}

	_, err = bkClient.Solve(context.TODO(), llbDef, solveOpt, ch)
	if err != nil {
		fmt.Printf("failed to solve llb: %v\n", err)
		os.Exit(1)
	}
}
