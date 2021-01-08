// Package run implements structs for scheduling and performing apply runs that
// apply manifest files from a git repository source based on configuration
// stored in Waybill CRDs and scheduling.
package run

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

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
	Type    Type
	Waybill *kubeapplierv1alpha1.Waybill
}

// ApplyOptions contains global configuration for Apply
type ApplyOptions struct {
	ClusterResources    []string
	NamespacedResources []string
}

func (o *ApplyOptions) pruneWhitelist(waybill *kubeapplierv1alpha1.Waybill, pruneBlacklist []string) []string {
	var pruneWhitelist []string
	if pointer.BoolPtrDerefOr(waybill.Spec.Prune, true) {
		pruneWhitelist = append(pruneWhitelist, o.NamespacedResources...)

		if waybill.Spec.PruneClusterResources {
			pruneWhitelist = append(pruneWhitelist, o.ClusterResources...)
		}

		// Trim blacklisted items out of the whitelist
		pruneBlacklist := uniqueStrings(append(pruneBlacklist, waybill.Spec.PruneBlacklist...))
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
	PruneBlacklist []string
	RepoPath       string
	WorkerCount    int
	workerGroup    *sync.WaitGroup
	workerQueue    chan Request
}

// Start runs a continuous loop that starts a new run when a request comes into the queue channel.
func (r *Runner) Start() chan<- Request {
	if r.workerGroup != nil {
		log.Logger("runner").Info("Runner is already started, will not do anything")
		return nil
	}
	if r.WorkerCount == 0 {
		r.WorkerCount = defaultRunnerWorkerCount
	}
	r.workerQueue = make(chan Request, defaultWorkerQueueSize)
	r.workerGroup = &sync.WaitGroup{}
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
		// wb := request.Waybill
		wbId := fmt.Sprintf("%s/%s", request.Waybill.Namespace, request.Waybill.Name)
		log.Logger("runner").Info("Started apply run", "waybill", wbId)
		metrics.UpdateRunRequest(request.Type.String(), request.Waybill, -1)

		clusterResources, namespacedResources, err := r.KubeClient.PrunableResourceGVKs()
		if err != nil {
			log.Logger("runner").Error("Could not compute list of prunable resources", "waybill", wbId, "error", err)
			r.setRequestFailure(request, err)
			continue
		}
		applyOptions := &ApplyOptions{
			ClusterResources:    clusterResources,
			NamespacedResources: namespacedResources,
		}
		delegateToken, err := r.getDelegateToken(request.Waybill)
		if err != nil {
			log.Logger("runner").Error("Could not fetch delegate token", "waybill", wbId, "error", err)
			r.setRequestFailure(request, fmt.Errorf("failed fetching delegate token: %w", err))
			continue
		}
		tmpRepoPath, err := r.setupRepositoryClone(request.Waybill)
		if err != nil {
			log.Logger("runner").Error("Could not setup apply environment", "waybill", wbId, "error", err)
			r.setRequestFailure(request, fmt.Errorf("failed setting up repository clone: %w", err))
			continue
		}
		hash, err := (&git.Util{RepoPath: tmpRepoPath}).HeadHashForPaths(*request.Waybill.Spec.RepositoryPath)
		if err != nil {
			log.Logger("runner").Error("Could not determine HEAD hash", "waybill", wbId, "error", err)
			r.setRequestFailure(request, err)
			os.RemoveAll(tmpRepoPath)
			continue
		}

		r.apply(tmpRepoPath, delegateToken, request.Waybill, applyOptions)

		// TODO: move these in apply()
		request.Waybill.Status.LastRun.Commit = hash
		request.Waybill.Status.LastRun.Type = request.Type.String()

		if err := r.KubeClient.UpdateWaybillStatus(context.TODO(), request.Waybill); err != nil {
			log.Logger("runner").Warn("Could not update Waybill run info", "waybill", wbId, "error", err)
		}

		if request.Waybill.Status.LastRun.Success {
			log.Logger("runner").Debug(fmt.Sprintf("Apply run output for %s:\n%s\n%s", wbId, request.Waybill.Status.LastRun.Command, request.Waybill.Status.LastRun.Output))
		} else {
			log.Logger("runner").Warn(fmt.Sprintf("Apply run for %s encountered errors:\n%s", wbId, request.Waybill.Status.LastRun.ErrorMessage))
		}

		metrics.UpdateFromLastRun(request.Waybill)

		log.Logger("runner").Info("Finished apply run", "waybill", wbId)
		os.RemoveAll(tmpRepoPath)
	}
}

// setRequestFailure is used to update the status of a Waybill when the request
// failed to setup and before attempting to apply.
// TODO: it might be preferrable to convert these to events
func (r *Runner) setRequestFailure(req Request, err error) {
	req.Waybill.Status.LastRun = &kubeapplierv1alpha1.WaybillStatusRun{
		Command:      "", // These fields are not available since the request
		Commit:       "", // failed before even attempting an apply
		Output:       "",
		ErrorMessage: err.Error(),
		// We don't need to provide accurate timestamps here, the request failed
		// during setup, before attempting to apply
		Finished: metav1.NewTime(r.Clock.Now()),
		Started:  metav1.NewTime(r.Clock.Now()),
		Success:  false,
		Type:     req.Type.String(),
	}
}

// Stop gracefully shuts down the Runner.
func (r *Runner) Stop() {
	if r.workerGroup == nil {
		return
	}
	close(r.workerQueue)
	r.workerGroup.Wait()
	r.workerGroup = nil
}

func (r *Runner) getDelegateToken(waybill *kubeapplierv1alpha1.Waybill) (string, error) {
	secret, err := r.KubeClient.GetSecret(context.TODO(), waybill.Namespace, *waybill.Spec.DelegateServiceAccountSecretRef)
	if err != nil {
		return "", err
	}
	if secret.Type != corev1.SecretTypeServiceAccountToken {
		return "", fmt.Errorf("Secret %s/%s is not of type %s", secret.Namespace, secret.Name, corev1.SecretTypeServiceAccountToken)
	}
	delegateToken, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("Secret %s/%s does not contain key 'token'", secret.Namespace, secret.Name)
	}
	return string(delegateToken), nil
}

func (r *Runner) setupRepositoryClone(waybill *kubeapplierv1alpha1.Waybill) (string, error) {
	var env []string
	// strongbox integration
	if waybill.Spec.StrongboxKeyringSecretRef != "" {
		secret, err := r.KubeClient.GetSecret(context.TODO(), waybill.Namespace, waybill.Spec.StrongboxKeyringSecretRef)
		if err != nil {
			return "", err
		}
		strongboxData, ok := secret.Data[".strongbox_keyring"]
		if !ok {
			return "", fmt.Errorf("Secret %s/%s does not contain key '.strongbox_keyring'", secret.Namespace, secret.Name)
		}
		tmpHomeDir, err := ioutil.TempDir("", fmt.Sprintf("run_%s_%s_%d_home", waybill.Namespace, waybill.Name, r.Clock.Now().Unix()))
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpHomeDir)
		if err := ioutil.WriteFile(filepath.Join(tmpHomeDir, ".strongbox_keyring"), strongboxData, 0400); err != nil {
			return "", err
		}
		env = []string{fmt.Sprintf("STRONGBOX_HOME=%s", tmpHomeDir)}
	}
	// repository clone
	tmpRepoDir, err := ioutil.TempDir("", fmt.Sprintf("run_%s_%s_%d_home", waybill.Namespace, waybill.Name, r.Clock.Now().Unix()))
	if err != nil {
		return "", err
	}
	root, sub, err := (&git.Util{RepoPath: r.RepoPath}).SplitPath()
	if err != nil {
		os.RemoveAll(tmpRepoDir)
		return "", err
	}
	subpath := filepath.Join(sub, *waybill.Spec.RepositoryPath)
	if err := git.CloneRepository(root, tmpRepoDir, subpath, env); err != nil {
		os.RemoveAll(tmpRepoDir)
		return "", err
	}
	return filepath.Join(tmpRepoDir, sub), nil
}

// Apply takes a list of files and attempts an apply command on each.
func (r *Runner) apply(rootPath, token string, waybill *kubeapplierv1alpha1.Waybill, options *ApplyOptions) {
	start := r.Clock.Now()
	path := filepath.Join(rootPath, *waybill.Spec.RepositoryPath)
	log.Logger("runner").Info("Applying files", "path", path)

	dryRunStrategy := "none"
	if r.DryRun || waybill.Spec.DryRun {
		dryRunStrategy = "server"
	}

	cmd, output, err := r.KubectlClient.Apply(
		path,
		kubectl.ApplyFlags{
			Namespace:      waybill.Namespace,
			DryRunStrategy: dryRunStrategy,
			PruneWhitelist: options.pruneWhitelist(waybill, r.PruneBlacklist),
			ServerSide:     waybill.Spec.ServerSideApply,
			Token:          token,
		},
	)
	finish := r.Clock.Now()

	waybill.Status.LastRun = &kubeapplierv1alpha1.WaybillStatusRun{
		Command:      cmd,
		Output:       output,
		ErrorMessage: "",
		Finished:     metav1.NewTime(finish),
		Started:      metav1.NewTime(start),
	}
	if err != nil {
		waybill.Status.LastRun.ErrorMessage = err.Error()
	} else {
		waybill.Status.LastRun.Success = true
	}
}

// Enqueue attempts to add a run request to the queue, timing out after 5
// seconds.
func Enqueue(queue chan<- Request, t Type, waybill *kubeapplierv1alpha1.Waybill) {
	wbId := fmt.Sprintf("%s/%s", waybill.Namespace, waybill.Name)
	if t != ForcedRun && !pointer.BoolPtrDerefOr(waybill.Spec.AutoApply, true) {
		log.Logger("runner").Debug("Run ignored, waybill autoApply is disabled", "waybill", wbId, "type", t)
		return
	}
	select {
	case queue <- Request{Type: t, Waybill: waybill}:
		log.Logger("runner").Debug("Run queued", "waybill", wbId, "type", t)
		metrics.UpdateRunRequest(t.String(), waybill, 1)
	case <-time.After(5 * time.Second):
		log.Logger("runner").Error("Timed out trying to queue a run, run queue is full", "waybill", wbId, "type", t)
		metrics.AddRunRequestQueueFailure(t.String(), waybill)
	}
}
