package main

import (
	"log"
	"os"
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/utilitywarehouse/kube-applier/applylist"
	"github.com/utilitywarehouse/kube-applier/git"
	"github.com/utilitywarehouse/kube-applier/kube"
	"github.com/utilitywarehouse/kube-applier/metrics"
	"github.com/utilitywarehouse/kube-applier/run"
	"github.com/utilitywarehouse/kube-applier/sysutil"
	"github.com/utilitywarehouse/kube-applier/webserver"
)

const (
	// Default number of seconds to wait before checking the Git repo for new commits.
	defaultPollIntervalSeconds = 5

	// Default number of seconds to wait in between apply runs (if no new commits to the repo have been made).
	defaultFullRunIntervalSeconds = 5 * 60

	// Number of seconds to wait in between attempts to locate the repo at the specified path.
	// Git-sync atomically places the repo at the specified path once it is finished pulling, so it will not be present immediately.
	waitForRepoInterval = 1 * time.Second
)

func main() {

	app := cli.App("kube-applier", "Kube applier")
	repoPath := *app.String(cli.StringOpt{
		Name:   "repoPath",
		Desc:   "Git repo path",
		EnvVar: "REPO_PATH",
	})
	listenPort := *app.Int(cli.IntOpt{
		Name:   "listenPort",
		Value:  8080,
		Desc:   "Listen port",
		EnvVar: "LISTEN_PORT",
	})
	server := *app.String(cli.StringOpt{
		Name:   "server",
		Value:  "",
		Desc:   "K8s server. Mainly for local testing.",
		EnvVar: "SERVER",
	})
	diffURLFormat := *app.String(cli.StringOpt{
		Name:   "diffURLFormat",
		Value:  "https://github.com/utilitywarehouse/kubernetes-manifests/commit/%s",
		Desc:   "Github commit diff url",
		EnvVar: "DIFF_URL_FORMAT",
	})
	blacklistPath := *app.String(cli.StringOpt{
		Name:   "blacklistPath",
		Value:  "",
		Desc:   "Path ",
		EnvVar: "BLACKLIST_PATH",
	})
	pollInterval := *app.Int(cli.IntOpt{
		Name:   "pollIntervalSeconds",
		Value:  5,
		Desc:   "Poll interval",
		EnvVar: "POLL_INTERVAL_SECONDS",
	})
	fullRunInterval := *app.Int(cli.IntOpt{
		Name:   "fullRunIntervalSeconds",
		Value:  60,
		Desc:   "Full run interval",
		EnvVar: "FULL_RUN_INTERVAL_SECONDS",
	})
	if diffURLFormat != "" && !strings.Contains(diffURLFormat, "%s") {
		log.Fatalf("Invalid DIFF_URL_FORMAT, must contain %q: %v", "%s", diffURLFormat)
	}
	app.Action = func() {
		metrics := &metrics.Prometheus{}
		metrics.Init()

		clock := &sysutil.Clock{}

		if err := sysutil.WaitForDir(repoPath, clock, waitForRepoInterval); err != nil {
			log.Fatal(err)
		}

		kubeClient := &kube.Client{server}
		kubeClient.Configure()

		batchApplier := &run.BatchApplier{kubeClient, metrics}
		gitUtil := &git.GitUtil{repoPath}
		fileSystem := &sysutil.FileSystem{}
		listFactory := &applylist.Factory{repoPath, blacklistPath, fileSystem}

		// Webserver and scheduler send run requests to runQueue channel, runner receives the requests and initiates runs.
		// Only 1 pending request may sit in the queue at a time.
		runQueue := make(chan bool, 1)

		// Runner sends run results to runResults channel, webserver receives the results and displays them.
		// Limit of 5 is arbitrary - there is significant delay between sends, and receives are handled near instantaneously.
		runResults := make(chan run.Result, 5)

		// Runner, webserver, and scheduler all send fatal errors to errors channel, and main() exits upon receiving an error.
		// No limit needed, as a single fatal error will exit the program anyway.
		errors := make(chan error)

		runner := &run.Runner{
			batchApplier,
			listFactory,
			gitUtil,
			clock,
			metrics,
			diffURLFormat,
			runQueue,
			runResults,
			errors,
		}
		scheduler := &run.Scheduler{gitUtil, time.Duration(pollInterval) * time.Second, time.Duration(fullRunInterval) * time.Second, runQueue, errors}
		webserver := &webserver.WebServer{listenPort, clock, metrics.GetHandler(), runQueue, runResults, errors}

		go scheduler.Start()
		go runner.Start()
		go webserver.Start()

		for err := range errors {
			log.Fatal(err)
		}
	}
	app.Run(os.Args)
}