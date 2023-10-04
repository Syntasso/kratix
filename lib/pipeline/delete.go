package pipeline

import (
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const kratixActionDelete = "delete"

func NewDeletePipeline(rr *unstructured.Unstructured, pipelines []platformv1alpha1.Pipeline, resourceRequestIdentifier, promiseIdentifier string) batchv1.Job {

	args := NewPipelineArgs(promiseIdentifier, resourceRequestIdentifier, rr.GetNamespace())

	containers, pipelineVolumes := deletePipelineContainers(rr, pipelines)

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.DeletePipelineName(),
			Namespace: args.Namespace(),
			Labels:    args.DeletePipelinePodLabels(),
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: args.DeletePipelinePodLabels(),
				},
				Spec: v1.PodSpec{
					RestartPolicy:      v1.RestartPolicyOnFailure,
					ServiceAccountName: args.ServiceAccountName(),
					Containers:         []v1.Container{containers[len(containers)-1]},
					InitContainers:     containers[0 : len(containers)-1],
					Volumes:            pipelineVolumes,
				},
			},
		},
	}
}

func deletePipelineContainers(rr *unstructured.Unstructured, pipelines []platformv1alpha1.Pipeline) ([]v1.Container, []v1.Volume) {
	volumes, volumeMounts := pipelineVolumes()

	//TODO: Does this get called for promises too? If so, change the parameter name and dynamically set input below
	readerContainer := readerContainer(rr, platformv1alpha1.KratixWorkflowTypeResource, "shared-input")
	containers := []v1.Container{
		readerContainer,
	}

	if len(pipelines) > 0 {
		//TODO: We only support 1 workflow for now
		for _, c := range pipelines[0].Spec.Containers {
			containers = append(containers, v1.Container{
				Name:         c.Name,
				Image:        c.Image,
				VolumeMounts: volumeMounts,
				Env: []v1.EnvVar{
					{
						Name:  kratixActionEnvVar,
						Value: kratixActionDelete,
					},
				},
			})
		}
	}

	return containers, volumes
}
