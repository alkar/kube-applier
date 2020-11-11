package run

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/client"
	"github.com/utilitywarehouse/kube-applier/git"
	"github.com/utilitywarehouse/kube-applier/kubectl"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/metrics"
	"github.com/utilitywarehouse/kube-applier/sysutil"
)

const (
	defaultRunnerWorkerCount = 2
)

// Request defines an apply run request
type Request struct {
	Type Type
	Args interface{}
}

// ApplyOptions contains global configuration for Apply
type ApplyOptions struct {
	ClusterResources    []string
	NamespacedResources []string
}

type Runner struct {
	Clock          sysutil.ClockInterface
	DiffURLFormat  string
	DryRun         bool
	Errors         chan<- error
	KubeClient     *client.Client
	KubectlClient  *kubectl.Client
	Metrics        *metrics.Prometheus
	PruneBlacklist []string
	RepoPath       string
	RunQueue       <-chan Request
	RunResults     chan<- Result
	WorkerCount    int
}

// Start runs a continuous loop that starts a new run when a request comes into the queue channel.
func (r *Runner) Start() {
	r.RunResults <- r.initialiseResultFromKubernetes()
	for t := range r.RunQueue {
		newRun, err := r.run(t)
		if err != nil {
			r.Errors <- err
			return
		}
		if newRun != nil {
			r.RunResults <- *newRun
		}
	}
}

// Run executes the requested apply run, and returns a Result with data about
// the completed run (or nil if the run failed to complete).
func (r *Runner) run(t Request) (*Result, error) {
	start := r.Clock.Now()
	log.Logger.Info("Started apply run", "start-time", start)

	apps, err := r.KubeClient.ListApplications(context.TODO())
	if err != nil {
		log.Logger.Error("Could not list Applications: %v", err)
	}

	gitUtil, cleanupTemp, err := r.copyRepository(apps)
	if err != nil {
		return nil, err
	}
	defer cleanupTemp()

	var appList []kubeapplierv1alpha1.Application
	if t.Type == ScheduledFullRun || t.Type == ForcedFullRun {
		appList = apps
	} else if t.Type == FailedOnlyRun {
		for _, a := range apps {
			if a.Status.LastRun != nil && !a.Status.LastRun.Success {
				appList = append(appList, a)
			}
		}
	} else if t.Type == PartialRun {
		appList = r.pruneUnchangedDirs(gitUtil, apps)
	} else if t.Type == SingleDirectoryRun {
		valid := false
		for _, app := range apps {
			if app.Spec.RepositoryPath == t.Args.(string) {
				appList = []kubeapplierv1alpha1.Application{app}
				valid = true
				break
			}
		}
		if !valid {
			log.Logger.Error(fmt.Sprintf("Invalid path '%s' requested, ignoring", t.Args.(string)))
			return nil, nil
		}
	} else {
		log.Logger.Error(fmt.Sprintf("Run type '%s' is not properly handled", t.Type))
	}

	// TODO: BatchApplier performs the same check, is this necessary? (especially if we merge appList with apps below)
	if len(appList) == 0 {
		log.Logger.Info("No Applications eligible to apply")
		return nil, nil
	}

	hash, err := gitUtil.HeadHashForPaths(".")
	if err != nil {
		return nil, err
	}
	commitLog, err := gitUtil.HeadCommitLogForPaths(".")
	if err != nil {
		return nil, err
	}

	clusterResources, namespacedResources, err := r.KubeClient.PrunableResourceGVKs()
	if err != nil {
		return nil, err
	}
	applyOptions := &ApplyOptions{
		ClusterResources:    clusterResources,
		NamespacedResources: namespacedResources,
	}

	dirs := make([]string, len(appList))
	for i, app := range appList {
		dirs[i] = app.Spec.RepositoryPath
	}
	log.Logger.Debug(fmt.Sprintf("applying dirs: %v", dirs))
	r.Apply(gitUtil.RepoPath, appList, applyOptions)

	finish := r.Clock.Now()

	log.Logger.Info("Finished apply run", "stop-time", finish)

	success := true
	results := make(map[string]string)

	statusRunInfo := kubeapplierv1alpha1.ApplicationStatusRunInfo{
		Started:  metav1.NewTime(start),
		Finished: metav1.NewTime(finish),
		Commit:   hash,
		Type:     t.Type.String(),
	}
	for i := range appList {
		appList[i].Status.LastRun.Info = statusRunInfo
		if !appList[i].Status.LastRun.Success {
			success = false
		}

		// TODO: what does this do?
		results[appList[i].Spec.RepositoryPath] = appList[i].Status.LastRun.Output

		if err := r.KubeClient.UpdateApplicationStatus(context.TODO(), &appList[i]); err != nil {
			log.Logger.Warn(fmt.Sprintf("Could not update Application run info: %v\n", err))
		}
	}

	r.Metrics.UpdateResultSummary(results)

	r.Metrics.UpdateRunLatency(r.Clock.Since(start).Seconds(), success)
	r.Metrics.UpdateLastRunTimestamp(finish)

	// merge apps (all Applications) and appList (applied in this run)
	resultApps := appList
	for _, a1 := range apps {
		found := false
		for _, a2 := range appList {
			if a1.Name == a2.Name && a1.Namespace == a2.Namespace {
				found = true
			}
		}
		if !found {
			resultApps = append(resultApps, a1)
		}
	}
	newRun := Result{
		Applications:  resultApps,
		DiffURLFormat: r.DiffURLFormat,
		FullCommit:    commitLog,
		LastRun:       statusRunInfo,
		RootPath:      r.RepoPath,
	}
	return &newRun, nil
}

func (r *Runner) pruneUnchangedDirs(gitUtil *git.Util, apps []kubeapplierv1alpha1.Application) []kubeapplierv1alpha1.Application {
	var prunedApps []kubeapplierv1alpha1.Application
	for _, app := range apps {
		path := path.Join(gitUtil.RepoPath, app.Spec.RepositoryPath)
		if app.Status.LastRun != nil && app.Status.LastRun.Info.Commit != "" {
			changed, err := gitUtil.HasChangesForPath(path, app.Status.LastRun.Info.Commit)
			if err != nil {
				log.Logger.Warn(fmt.Sprintf("Could not check dir '%s' for changes, forcing apply: %v", path, err))
				changed = true
			}
			if !changed {
				continue
			}
		} else {
			log.Logger.Info(fmt.Sprintf("No previous apply recorded for Application '%s/%s', forcing apply", app.Namespace, app.Name))
		}
		prunedApps = append(prunedApps, app)
	}
	return prunedApps
}

func (r *Runner) copyRepository(apps []kubeapplierv1alpha1.Application) (*git.Util, func(), error) {
	root, sub, err := (&git.Util{RepoPath: r.RepoPath}).SplitPath()
	if err != nil {
		return nil, nil, err
	}
	tmpDir, err := ioutil.TempDir("", fmt.Sprintf("run-%d-", r.Clock.Now().Unix()))
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	for _, a := range apps {
		paths = append(paths, fmt.Sprintf("%s/%s", sub, a.Spec.RepositoryPath))
	}
	if err := git.CloneRepository(root, tmpDir, paths...); err != nil {
		return nil, nil, err
	}
	return &git.Util{RepoPath: path.Join(tmpDir, sub)}, func() { os.RemoveAll(tmpDir) }, nil
}

func (r *Runner) initialiseResultFromKubernetes() Result {
	gitUtil := &git.Util{RepoPath: r.RepoPath}
	res := Result{
		DiffURLFormat: r.DiffURLFormat,
		RootPath:      r.RepoPath,
	}
	apps, err := r.KubeClient.ListApplications(context.TODO())
	if err != nil {
		log.Logger.Warn(fmt.Sprintf("Could not list Application resources: %v", err))
		return res
	}
	for _, app := range apps {
		// TODO: what do we do with these?
		if app.Status.LastRun != nil {
			res.Applications = append(res.Applications, app)
			if app.Status.LastRun.Info.Started.After(res.LastRun.Started.Time) {
				res.LastRun = app.Status.LastRun.Info
			}
		}
	}
	commitLog, err := gitUtil.CommitLog(res.LastRun.Commit)
	if err != nil {
		log.Logger.Warn(fmt.Sprintf("Could not get commit message for commit %s: %v", res.LastRun.Commit, err))
	}
	res.FullCommit = commitLog
	return res
}

// Apply takes a list of files and attempts an apply command on each.
// It returns two lists of ApplyAttempts - one for files that succeeded, and one for files that failed.
func (r *Runner) Apply(rootPath string, appList []kubeapplierv1alpha1.Application, options *ApplyOptions) {
	if len(appList) == 0 {
		return
	}

	if r.WorkerCount == 0 {
		r.WorkerCount = defaultRunnerWorkerCount
	}

	wg := sync.WaitGroup{}
	mutex := sync.Mutex{}

	apps := make(chan *kubeapplierv1alpha1.Application, len(appList))

	for i := 0; i < r.WorkerCount; i++ {
		wg.Add(1)
		go func(root string, apps <-chan *kubeapplierv1alpha1.Application, opts *ApplyOptions) {
			defer wg.Done()
			for app := range apps {
				r.apply(root, app, opts)

				mutex.Lock()
				if app.Status.LastRun.Success {
					log.Logger.Info(fmt.Sprintf("%v\n%v", app.Status.LastRun.Command, app.Status.LastRun.Output))
				} else {
					log.Logger.Warn(fmt.Sprintf("%v\n%v", app.Status.LastRun.Command, app.Status.LastRun.ErrorMessage))
				}
				r.Metrics.UpdateNamespaceSuccess(app.Namespace, app.Status.LastRun.Success)
				mutex.Unlock()
			}
		}(rootPath, apps, options)
	}

	for i := range appList {
		apps <- &appList[i]
	}

	close(apps)
	wg.Wait()

	sort.Slice(appList, func(i, j int) bool {
		return appList[i].Spec.RepositoryPath < appList[j].Spec.RepositoryPath
	})
}

func (r *Runner) apply(rootPath string, app *kubeapplierv1alpha1.Application, options *ApplyOptions) {
	start := r.Clock.Now()
	path := filepath.Join(rootPath, app.Spec.RepositoryPath)
	log.Logger.Info(fmt.Sprintf("Applying dir %v", path))

	var pruneWhitelist []string
	if app.Spec.Prune {
		pruneWhitelist = append(pruneWhitelist, options.NamespacedResources...)

		if app.Spec.PruneClusterResources {
			pruneWhitelist = append(pruneWhitelist, options.ClusterResources...)
		}

		// Trim blacklisted items out of the whitelist
		pruneBlacklist := uniqueStrings(append(r.PruneBlacklist, app.Spec.PruneBlacklist...))
		for _, b := range pruneBlacklist {
			for i, w := range pruneWhitelist {
				if b == w {
					pruneWhitelist = append(pruneWhitelist[:i], pruneWhitelist[i+1:]...)
					break
				}
			}
		}
	}

	dryRunStrategy := "none"
	if r.DryRun || app.Spec.DryRun {
		dryRunStrategy = "server"
	}

	cmd, output, err := r.KubectlClient.Apply(path, kubectl.ApplyFlags{
		Namespace:      app.Namespace,
		DryRunStrategy: dryRunStrategy,
		PruneWhitelist: pruneWhitelist,
		ServerSide:     app.Spec.ServerSideApply,
	})
	finish := r.Clock.Now()

	app.Status.LastRun = &kubeapplierv1alpha1.ApplicationStatusRun{
		Command:      cmd,
		Output:       output,
		ErrorMessage: "",
		Finished:     metav1.NewTime(finish),
		Started:      metav1.NewTime(start),
	}
	if err != nil {
		app.Status.LastRun.ErrorMessage = err.Error()
	} else {
		app.Status.LastRun.Success = true
	}
}

func uniqueStrings(in []string) []string {
	m := make(map[string]bool)
	for _, i := range in {
		m[i] = true
	}
	out := make([]string, len(m))
	i := 0
	for v := range m {
		out[i] = v
		i++
	}
	return out
}
