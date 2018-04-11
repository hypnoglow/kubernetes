/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/kubectl/util/i18n"
)

var (
	jobLong = templates.LongDesc(i18n.T(`
		Create a job with the specified name.`))

	jobExample = templates.Examples(i18n.T(`
		# Create a job from a CronJob named "a-cronjob"
		kubectl create job test-job --from=cronjob/a-cronjob`))
)

type CreateJobOptions struct {
	Name string
	From string

	Namespace    string
	OutputFormat string
	Client       clientbatchv1.BatchV1Interface
	Out          io.Writer
	DryRun       bool
	Builder      *resource.Builder
	Cmd          *cobra.Command
}

// NewCmdCreateJob is a command to ease creating Jobs from CronJobs.
func NewCmdCreateJob(f cmdutil.Factory, cmdOut io.Writer) *cobra.Command {
	c := &CreateJobOptions{
		Out: cmdOut,
	}
	cmd := &cobra.Command{
		Use:     "job NAME [--from=CRONJOB]",
		Short:   jobLong,
		Long:    jobLong,
		Example: jobExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(c.Complete(f, cmd, args))
			cmdutil.CheckErr(c.RunCreateJob())
		},
	}
	cmdutil.AddApplyAnnotationFlags(cmd)
	cmdutil.AddValidateFlags(cmd)
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().String("from", "", "The name of the resource to create a Job from (only cronjob is supported).")

	return cmd
}

func (c *CreateJobOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return cmdutil.UsageErrorf(cmd, "NAME is required")
	}
	c.Name = args[0]

	c.From = cmdutil.GetFlagString(cmd, "from")
	c.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	c.Client = clientset.BatchV1()
	c.Builder = f.NewBuilder()
	c.DryRun = cmdutil.GetDryRunFlag(cmd)
	c.Cmd = cmd
	c.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	return nil
}

func (c *CreateJobOptions) RunCreateJob() error {
	infos, err := c.Builder.
		Unstructured().
		NamespaceParam(c.Namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(false, c.From).
		Flatten().
		Latest().
		Do().
		Infos()
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return fmt.Errorf("from must be an existing cronjob")
	}

	switch cronJob := infos[0].AsVersioned().(type) {
	case *batchv1beta1.CronJob:
		return c.createJobFromCronJobv1beta1(cronJob)
	case *batchv2alpha1.CronJob:
		return c.createJobFromCronJobv2alpha1(cronJob)
	default:
		return fmt.Errorf(
			"from must be %q or %q, but got %q",
			batchv1beta1.SchemeGroupVersion.WithKind("CronJob").String(),
			batchv2alpha1.SchemeGroupVersion.WithKind("CronJob").String(),
			cronJob.GetObjectKind().GroupVersionKind().String(),
		)
	}
}

func (c *CreateJobOptions) createJob(jobAnnotations, jobLabels map[string]string, jobSpec batchv1.JobSpec) error {
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	for k, v := range jobAnnotations {
		annotations[k] = v
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.Name,
			Namespace:   c.Namespace,
			Annotations: annotations,
			Labels:      jobLabels,
		},
		Spec: jobSpec,
	}

	if !c.DryRun {
		var err error
		job, err = c.Client.Jobs(c.Namespace).Create(job)
		if err != nil {
			return fmt.Errorf("failed to create job: %v", err)
		}
	}

	if useShortOutput := c.OutputFormat == "name"; useShortOutput || len(c.OutputFormat) == 0 {
		cmdutil.PrintSuccess(useShortOutput, c.Out, job, c.DryRun, "created")
		return nil
	}

	return cmdutil.PrintObject(c.Cmd, job, c.Out)
}

func (c *CreateJobOptions) createJobFromCronJobv1beta1(cronJob *batchv1beta1.CronJob) error {
	return c.createJob(
		cronJob.Spec.JobTemplate.Annotations,
		cronJob.Spec.JobTemplate.Labels,
		cronJob.Spec.JobTemplate.Spec,
	)
}

func (c *CreateJobOptions) createJobFromCronJobv2alpha1(cronJob *batchv2alpha1.CronJob) error {
	return c.createJob(
		cronJob.Spec.JobTemplate.Annotations,
		cronJob.Spec.JobTemplate.Labels,
		cronJob.Spec.JobTemplate.Spec,
	)
}
