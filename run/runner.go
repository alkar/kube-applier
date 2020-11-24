package run

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

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
	defaultWorkerQueueSize   = 512
)

// Request defines an apply run request
type Request struct {
	Type        Type
	Application *kubeapplierv1alpha1.Application
}

// ApplyOptions contains global configuration for Apply
type ApplyOptions struct {
	ClusterResources    []string
	NamespacedResources []string
}

func (o *ApplyOptions) PruneWhitelist(app *kubeapplierv1alpha1.Application, pruneBlacklist []string) []string {
	var pruneWhitelist []string
	if app.Spec.Prune {
		pruneWhitelist = append(pruneWhitelist, o.NamespacedResources...)

		if app.Spec.PruneClusterResources {
			pruneWhitelist = append(pruneWhitelist, o.ClusterResources...)
		}

		// Trim blacklisted items out of the whitelist
		pruneBlacklist := uniqueStrings(append(pruneBlacklist, app.Spec.PruneBlacklist...))
		for _, b := range pruneBlacklist {
			for i, w := range pruneWhitelist {
				if b == w {
					pruneWhitelist = append(pruneWhitelist[:i], pruneWhitelist[i+1:]...)
					break
				}
			}
		}
	}
	return pruneWhitelist
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

// Runner manages the full process of an apply run, including getting the
// appropriate files, running apply commands on them, and handling the results.
type Runner struct {
	Clock          sysutil.ClockInterface
	DiffURLFormat  string
	DryRun         bool
	KubeClient     *client.Client
	KubectlClient  *kubectl.Client
	Metrics        *metrics.Prometheus
	PruneBlacklist []string
	RepoPath       string
	WorkerCount    int
	workerGroup    sync.WaitGroup
	workerQueue    chan Request
	metricsMutex   sync.Mutex
}

// Start runs a continuous loop that starts a new run when a request comes into the queue channel.
func (r *Runner) Start() chan<- Request {
	if r.workerQueue != nil {
		log.Logger.Info("Runner is already started, will not do anything")
		return nil
	}

	r.metricsMutex = sync.Mutex{}

	if r.WorkerCount == 0 {
		r.WorkerCount = defaultRunnerWorkerCount
	}
	r.workerQueue = make(chan Request, defaultWorkerQueueSize)
	r.workerGroup = sync.WaitGroup{}
	r.workerGroup.Add(r.WorkerCount)
	for i := 0; i < r.WorkerCount; i++ {
		go r.applyWorker()
	}
	return r.workerQueue
}

func (r *Runner) applyWorker() {
	defer r.workerGroup.Done()
	for request := range r.workerQueue {
		// TODO: for brevity, we could do:
		// app := request.Application
		log.Logger.Info("Started apply run", "app", fmt.Sprintf("%s/%s", request.Application.Namespace, request.Application.Name))

		gitUtil, cleanupTemp, err := r.copyRepository(request.Application)
		if err != nil {
			log.Logger.Error("Could not create a repository copy", "error", err)
			continue
		}
		hash, err := gitUtil.HeadHashForPaths(request.Application.Spec.RepositoryPath)
		if err != nil {
			log.Logger.Error("Could not determine HEAD hash", "error", err)
			cleanupTemp()
			continue
		}
		clusterResources, namespacedResources, err := r.KubeClient.PrunableResourceGVKs()
		if err != nil {
			log.Logger.Error("Could not compute list of prunable resources", "error", err)
			cleanupTemp()
			continue
		}
		applyOptions := &ApplyOptions{
			ClusterResources:    clusterResources,
			NamespacedResources: namespacedResources,
		}

		r.apply(gitUtil.RepoPath, request.Application, applyOptions)

		// TODO: move these in apply()
		request.Application.Status.LastRun.Commit = hash
		request.Application.Status.LastRun.Type = request.Type.String()

		if err := r.KubeClient.UpdateApplicationStatus(context.TODO(), request.Application); err != nil {
			log.Logger.Warn(fmt.Sprintf("Could not update Application run info: %v\n", err))
		}

		if request.Application.Status.LastRun.Success {
			log.Logger.Info(fmt.Sprintf("%v\n%v", request.Application.Status.LastRun.Command, request.Application.Status.LastRun.Output))
		} else {
			log.Logger.Warn(fmt.Sprintf("%v\n%v", request.Application.Status.LastRun.Command, request.Application.Status.LastRun.ErrorMessage))
			// TODO: queue a retry here, with backoff, or better, have scheduler do it
		}

		// TODO: should we move the mutex to the metrics package?
		r.metricsMutex.Lock()
		// TODO: these should be redesigned, since we no longer have batch runs
		r.Metrics.UpdateNamespaceSuccess(request.Application.Namespace, request.Application.Status.LastRun.Success)
		r.Metrics.UpdateResultSummary(map[string]string{
			request.Application.Spec.RepositoryPath: request.Application.Status.LastRun.Output,
		})
		r.Metrics.UpdateRunLatency(r.Clock.Since(request.Application.Status.LastRun.Started.Time).Seconds(), request.Application.Status.LastRun.Success)
		r.Metrics.UpdateLastRunTimestamp(request.Application.Status.LastRun.Finished.Time)
		r.metricsMutex.Unlock()

		log.Logger.Info("Finished apply run", "app", fmt.Sprintf("%s/%s", request.Application.Namespace, request.Application.Name))
		cleanupTemp()
	}
}

func (r *Runner) Stop() {
	if r.workerQueue == nil {
		return
	}
	close(r.workerQueue)
	r.workerGroup.Wait()
}

func (r *Runner) copyRepository(app *kubeapplierv1alpha1.Application) (*git.Util, func(), error) {
	var env []string
	root, sub, err := (&git.Util{RepoPath: r.RepoPath}).SplitPath()
	if err != nil {
		return nil, nil, err
	}
	tmpDir, err := ioutil.TempDir("", fmt.Sprintf("run_%s_%s_%d", app.Namespace, app.Name, r.Clock.Now().Unix()))
	if err != nil {
		return nil, nil, err
	}
	cleanupDirs := []string{tmpDir}
	if app.Spec.StrongboxKeyringSecretRef != "" {
		sbHome, err := r.setupStrongboxKeyring(app)
		if err != nil {
			return nil, nil, err
		}
		env = []string{fmt.Sprintf("STRONGBOX_HOME=%s", sbHome)}
		cleanupDirs = append(cleanupDirs, sbHome)
	}
	cleanup := func() {
		for _, v := range cleanupDirs {
			os.RemoveAll(v)
		}
	}
	path := filepath.Join(sub, app.Spec.RepositoryPath)
	if err := git.CloneRepository(root, tmpDir, path, env); err != nil {
		cleanup()
		return nil, nil, err
	}
	return &git.Util{RepoPath: filepath.Join(tmpDir, sub)}, cleanup, nil
}

func (r *Runner) setupStrongboxKeyring(app *kubeapplierv1alpha1.Application) (string, error) {
	secret, err := r.KubeClient.GetSecret(context.TODO(), app.Namespace, app.Spec.StrongboxKeyringSecretRef)
	if err != nil {
		return "", err
	}
	data, ok := secret.Data[".strongbox_keyring"]
	if !ok {
		return "", fmt.Errorf("Secret %s/%s does not contain key '.strongbox_keyring'", secret.Namespace, secret.Name)
	}
	tmpDir, err := ioutil.TempDir("", fmt.Sprintf("run_%s_%s_%d_strongbox", app.Namespace, app.Name, r.Clock.Now().Unix()))
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(filepath.Join(tmpDir, ".strongbox_keyring"), data, 0400); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpDir, nil
}

// Apply takes a list of files and attempts an apply command on each.
func (r *Runner) apply(rootPath string, app *kubeapplierv1alpha1.Application, options *ApplyOptions) {
	start := r.Clock.Now()
	path := filepath.Join(rootPath, app.Spec.RepositoryPath)
	log.Logger.Info(fmt.Sprintf("Applying dir %v", path))

	dryRunStrategy := "none"
	if r.DryRun || app.Spec.DryRun {
		dryRunStrategy = "server"
	}

	cmd, output, err := r.KubectlClient.Apply(path, kubectl.ApplyFlags{
		Namespace:      app.Namespace,
		DryRunStrategy: dryRunStrategy,
		PruneWhitelist: options.PruneWhitelist(app, r.PruneBlacklist),
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

// Enqueue attempts to add a run request to the queue, timing out after 5
// seconds.
func Enqueue(queue chan<- Request, t Type, app *kubeapplierv1alpha1.Application) {
	select {
	case queue <- Request{Type: t, Application: app}:
		log.Logger.Debug(fmt.Sprintf("%s queued for %s/%s", t, app.Namespace, app.Name))
	case <-time.After(5 * time.Second):
		log.Logger.Error("Timed out trying to queue a %s for %s/%s, run queue is full", t, app.Namespace, app.Name)
	}
}
