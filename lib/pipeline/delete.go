package pipeline

import (
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const kratixDeleteOperation = "delete"

func NewDeletePipeline(rr *unstructured.Unstructured, pipelines []platformv1alpha1.Pipeline, resourceRequestIdentifier, promiseIdentifier string) v1.Pod {

	args := newPipelineArgs(promiseIdentifier, resourceRequestIdentifier, rr.GetNamespace())

	containers, pipelineVolumes := deletePipelineContainers(rr, pipelines)

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.DeletePipelineName(),
			Namespace: args.Namespace(),
			Labels:    args.DeletePipelinePodLabels(),
		},
		Spec: v1.PodSpec{
			RestartPolicy:      v1.RestartPolicyOnFailure,
			ServiceAccountName: args.ServiceAccountName(),
			Containers:         []v1.Container{containers[len(containers)-1]},
			InitContainers:     containers[0 : len(containers)-1],
			Volumes:            pipelineVolumes,
		},
	}

	return pod
}

func deletePipelineContainers(rr *unstructured.Unstructured, pipelines []platformv1alpha1.Pipeline) ([]v1.Container, []v1.Volume) {
	volumes := []v1.Volume{
		{Name: "shared-input", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
		{Name: "shared-output", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
		{Name: "shared-metadata", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
	}
	volumeMounts := []v1.VolumeMount{
		{MountPath: "/kratix/input", Name: "shared-input", ReadOnly: true},
		{MountPath: "/kratix/output", Name: "shared-output"},
		{MountPath: "/kratix/metadata", Name: "shared-metadata"},
	}

	readerContainer := readerContainer(rr, "shared-input")
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
						Name:  kratixOperationEnvVar,
						Value: kratixDeleteOperation,
					},
				},
			})
		}
	}

	return containers, volumes
}
