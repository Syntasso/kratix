package integration_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 Run these tests using `make int-test` to ensure that the correct resources are applied
 to the k8s cluster under test.
*/
var (
	k8sClient client.Client
	err       error

	interval = "3s"
	timeout  = "60s"

	redis_gvk = schema.GroupVersionKind{
		Group:   "redis.redis.opstreelabs.in",
		Version: "v1beta1",
		Kind:    "Redis",
	}

	postgres_gvk = schema.GroupVersionKind{
		Group:   "postgresql.dev4devs.com",
		Version: "v1alpha1",
		Kind:    "Database",
	}

	work_gvk = schema.GroupVersionKind{
		Group:   "platform.kratix.io",
		Version: "v1alpha1",
		Kind:    "Work",
	}

	cluster_gvk = schema.GroupVersionKind{
		Group:   "platform.kratix.io",
		Version: "v1alpha1",
		Kind:    "Cluster",
	}
)

const (
	//Targets only cluster-worker-1
	REDIS_CRD                     = "../../config/samples/redis/redis-promise.yaml"
	REDIS_RESOURCE_REQUEST        = "../../config/samples/redis/redis-resource-request.yaml"
	REDIS_RESOURCE_UPDATE_REQUEST = "../../config/samples/redis/redis-resource-update-request.yaml"
	POSTGRES_CRD                  = "../../config/samples/postgres/postgres-promise.yaml"
	//Targets All clusters
	POSTGRES_RESOURCE_REQUEST = "../../config/samples/postgres/postgres-resource-request.yaml"
	// Targets the platform cluster
	PavedPathCRD             = "../../samples/paved-path-demo/paved-path-demo-promise.yaml"
	PavedPathResourceRequest = "../../samples/paved-path-demo/paved-path-demo-resource-request.yaml"

	//Clusters
	PLATFORM_WORKER_CLUSTER_1 = "./assets/platform_worker_cluster_1.yaml"
	DEV_WORKER_CLUSTER_1      = "./assets/worker_cluster_1.yaml"
	DEV_WORKER_CLUSTER_2      = "./assets/worker_cluster_2.yaml"
	PRODUCTION_WORKER_CLUSTER = "./assets/worker_cluster_3.yaml"

	// Flux related files
	GitOpsTKInstall         = "../../hack/worker/gitops-tk-install.yaml"
	PlatformGitOpsResources = "./assets/platform_worker_cluster_1_gitops-tk-resources.yaml"
)

var _ = Describe("kratix Platform Integration Test", func() {
	BeforeSuite(func() {
		initK8sClient()

		By("kratix is running")
		Eventually(func() bool {
			pod := getKratixControllerPod()
			return isPodRunning(pod)
		}, timeout, interval).Should(BeTrue())

		By("A Cluster labelled as dev is registered")
		registerWorkerCluster("worker-cluster-1", DEV_WORKER_CLUSTER_1)

		By("A Cluster labelled as dev && cache is registered")
		registerWorkerCluster("worker-cluster-2", DEV_WORKER_CLUSTER_2)

		By("A Cluster labelled as production is registered")
		registerWorkerCluster("worker-cluster-3", PRODUCTION_WORKER_CLUSTER)

		By("registering the platform cluster")
		registerWorkerCluster("platform-cluster-worker-1", PLATFORM_WORKER_CLUSTER_1)

		By("installing flux on the platform")
		installFlux("platform-cluster-worker-1", PlatformGitOpsResources)
	})

	Describe("Redis Promise lifecycle", func() {
		Describe("Applying Redis Promise", func() {
			It("Applying a Promise CRD manifests a Redis api-resource", func() {
				applyPromiseCRD(REDIS_CRD)

				Eventually(func() bool {
					return isAPIResourcePresent(redis_gvk)
				}, timeout, interval).Should(BeTrue())
			})

			It("places the resources to Workers as defined in the Promise", func() {
				workloadNamespacedName := types.NamespacedName{
					Name:      "redis-promise-default",
					Namespace: "default",
				}
				Eventually(func(g Gomega) {
					resourceName := "redis.redis.redis.opstreelabs.in"
					resourceKind := "CustomResourceDefinition"

					devClusterHasCrd, _ := workerHasCRD(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
					g.Expect(devClusterHasCrd).To(BeTrue(), "dev cluster 1 should have the crds")

					devClusterHasResources, _ := workerHasResource(workloadNamespacedName, "a-non-crd-resource", "Namespace", DEV_WORKER_CLUSTER_1)
					g.Expect(devClusterHasResources).To(BeTrue(), "dev cluster 1 should have the resources")

					devCacheClusterHasCrd, _ := workerHasCRD(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_2)
					g.Expect(devCacheClusterHasCrd).To(BeTrue(), "dev cluster 2 should have the crds")

					devCacheClusterHasResources, _ := workerHasResource(workloadNamespacedName, "a-non-crd-resource", "Namespace", DEV_WORKER_CLUSTER_2)
					g.Expect(devCacheClusterHasResources).To(BeTrue(), "dev cluster 2 should have the resources")

					prodClusterHasCrd, _ := workerHasCRD(workloadNamespacedName, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)
					g.Expect(prodClusterHasCrd).To(BeFalse(), "production cluster should not have the crds")

					prodClusterHasResources, _ := workerHasResource(workloadNamespacedName, "a-non-crd-resource", "Namespace", PRODUCTION_WORKER_CLUSTER)
					g.Expect(prodClusterHasResources).To(BeFalse(), "production cluster should not have the resources")
				}, timeout, interval).Should(Succeed(), "has the Redis CRD in the expected cluster")
			})
		})

		Describe("Applying Redis resource triggers the TransformationPipeline™", func() {
			It("Applying Redis resource triggers the TransformationPipeline™", func() {
				applyResourceRequest(REDIS_RESOURCE_REQUEST)

				expectedName := types.NamespacedName{
					Name:      "redis-promise-default-default-opstree-redis",
					Namespace: "default",
				}
				Eventually(func() bool {
					return hasResourceBeenApplied(work_gvk, expectedName)
				}, timeout, interval).Should(BeTrue())
			})

			It("Should place a Redis resource request to one Worker`", func() {
				Eventually(func(g Gomega) {
					workloadNamespacedName := types.NamespacedName{
						Name:      "redis-promise-default-default-opstree-redis",
						Namespace: "default",
					}

					//Assert that the Redis resource is present
					resourceName := "opstree-redis"
					resourceKind := "Redis"

					By("asserting the created work has the right cluster selectors", func() {
						var work platformv1alpha1.Work
						k8sClient.Get(context.Background(), workloadNamespacedName, &work)
						g.Expect(work.Spec.ClusterSelector).To(Equal(
							map[string]string{
								"environment": "dev",
								"data":        "cache",
							},
						))
					})

					By("asserting the resource definitions are in a matching cluster", func() {
						devClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
						g.Expect(devClusterHasResources).To(BeFalse(), "dev cluster should not have the resources")

						devCacheClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_2)
						g.Expect(devCacheClusterHasResources).To(BeTrue(), "dev cache cluster should have the resources")

						productionClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)
						g.Expect(productionClusterHasResources).To(BeFalse(), "production cluster should not have the resources")
					})
				}, timeout, interval).Should(Succeed())
			})

			PIt("Updates an existing Redis resource on the Worker", func() {
				updateResourceRequest(REDIS_RESOURCE_UPDATE_REQUEST)

				Eventually(func() bool {
					workloadNamespacedName := types.NamespacedName{
						Name:      "redis-promise-default-default-opstree-redis",
						Namespace: "default",
					}

					//Read from Minio
					//Assert that the Redis resource is present
					resourceName := "opstree-redis"
					resourceKind := "Redis"

					foundCluster1, obj1 := workerHasResource(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
					foundCluster2, obj2 := workerHasResource(workloadNamespacedName, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)

					if foundCluster1 && foundCluster2 {
						return false
					}

					if !foundCluster1 && !foundCluster2 {
						return false
					}

					//make it work, make it pretty (it works needs to be made pretty)
					var obj unstructured.Unstructured
					if foundCluster1 {
						obj = obj1
					} else if foundCluster2 {
						obj = obj2
					} else {
						return false
					}
					//

					spec := obj.Object["spec"]
					global := spec.(map[string]interface{})["global"]
					password := global.(map[string]interface{})["password"]
					return password == "Opstree@12345"

				}, timeout, interval).Should(BeTrue())
			})
		})
	})

	Describe("Postgres Promise lifecycle", func() {
		Describe("Applying Postgres Promise", func() {
			It("Applying a Promise CRD manifests a Postgres api-resource", func() {
				applyPromiseCRD(POSTGRES_CRD)

				Eventually(func() bool {
					return isAPIResourcePresent(postgres_gvk)
				}, timeout, interval).Should(BeTrue())
			})
		})

		Describe("Applying Postgres resource request triggers the TransformationPipeline™", func() {
			It("Applying Postgres resource triggers the TransformationPipeline™", func() {
				applyResourceRequest(POSTGRES_RESOURCE_REQUEST)

				expectedName := types.NamespacedName{
					Name:      "postgres-promise-default-default-database",
					Namespace: "default",
				}
				Eventually(func() bool {
					return hasResourceBeenApplied(work_gvk, expectedName)
				}, timeout, interval).Should(BeTrue())
			})

			PIt("Places a CRD that is defined in the resource request to only ONE Worker", func() {

			})

			It("Places a Postgres resources to one worker", func() {
				Eventually(func(g Gomega) {
					workloadNamespacedName := types.NamespacedName{
						Name:      "postgres-promise-default-default-database",
						Namespace: "default",
					}

					resourceName := "database"
					resourceKind := "Database"

					devClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
					devCacheClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, DEV_WORKER_CLUSTER_2)
					prodClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)
					platformClusterHasResources, _ := workerHasResource(workloadNamespacedName, resourceName, resourceKind, PLATFORM_WORKER_CLUSTER_1)

					g.Expect([]bool{devClusterHasResources, devCacheClusterHasResources, prodClusterHasResources, platformClusterHasResources}).To(
						ContainElements(false, false, false, true),
					)

				}, timeout, interval).Should(Succeed(), "Postgres should only be placed in only one worker")
			})
		})
	})

	Describe("paved path promise lifecycle", func() {
		Describe("applying the promise", func() {
			var ppd_gvk = schema.GroupVersionKind{
				Group:   "example.promise.syntasso.io",
				Version: "v1",
				Kind:    "paved-path-demo",
			}

			It("places the resources and crds on the right clusters", func() {
				applyPromiseCRD(PavedPathCRD)

				By("creates the a paved-path-demo api resource", func() {
					Eventually(func() bool {
						return isAPIResourcePresent(ppd_gvk)
					}, timeout, interval).Should(BeTrue())
				})

				By("creating the paved-path-demo resources on the platform cluster", func() {
					ppdWorkload := types.NamespacedName{
						Name:      "paved-path-demo-promise-default",
						Namespace: "default",
					}
					resourceKind := "Promise"

					var testCases = []struct {
						cluster string
						exists  bool
					}{
						{cluster: PLATFORM_WORKER_CLUSTER_1, exists: true},
						{cluster: DEV_WORKER_CLUSTER_1, exists: false},
						{cluster: DEV_WORKER_CLUSTER_2, exists: false},
						{cluster: PRODUCTION_WORKER_CLUSTER, exists: false},
					}

					Eventually(func(g Gomega) {
						for _, testCase := range testCases {
							knativeResource, _ := workerHasResource(ppdWorkload, "knative-serving-promise", resourceKind, testCase.cluster)
							postgresResource, _ := workerHasResource(ppdWorkload, "ha-postgres-promise", resourceKind, testCase.cluster)
							g.Expect(knativeResource).To(Equal(testCase.exists), testCase.cluster)
							g.Expect(postgresResource).To(Equal(testCase.exists), testCase.cluster)
						}
					}, timeout, interval).Should(Succeed())
				})

				By("creating the knative crds on the dev clusters", func() {
					resourceName := "services.serving.knative.dev"
					resourceKind := "CustomResourceDefinition"
					knativeWorkload := types.NamespacedName{
						Name:      "knative-serving-promise-default",
						Namespace: "default",
					}
					Eventually(func(g Gomega) {
						platformHasCrd, _ := workerHasCRD(knativeWorkload, resourceName, resourceKind, PLATFORM_WORKER_CLUSTER_1)
						prodClusterHasCrd, _ := workerHasCRD(knativeWorkload, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)
						devClusterHasCrd, _ := workerHasCRD(knativeWorkload, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
						devCluster2HasCrd, _ := workerHasCRD(knativeWorkload, resourceName, resourceKind, DEV_WORKER_CLUSTER_2)

						g.Expect(platformHasCrd).To(BeFalse(), "platform cluster should not have the crds")
						g.Expect(prodClusterHasCrd).To(BeFalse(), "prod cluster should not have the crds")
						g.Expect(devClusterHasCrd).To(BeTrue(), "dev cluster 1 should have the crds")
						g.Expect(devCluster2HasCrd).To(BeTrue(), "dev cluster 2 should have the crds")
					}, timeout, interval).Should(Succeed())
				})

				By("creating the postgres resources on the dev clusters", func() {
					resourceName := "postgres-operator"
					resourceKind := "ConfigMap"
					postgresWorkload := types.NamespacedName{
						Name:      "ha-postgres-promise-default",
						Namespace: "default",
					}
					Eventually(func(g Gomega) {
						platformHasResource, _ := workerHasResource(postgresWorkload, resourceName, resourceKind, PLATFORM_WORKER_CLUSTER_1)
						prodClusterHasCrd, _ := workerHasResource(postgresWorkload, resourceName, resourceKind, PRODUCTION_WORKER_CLUSTER)
						devClusterHasResource, _ := workerHasResource(postgresWorkload, resourceName, resourceKind, DEV_WORKER_CLUSTER_1)
						devCluster2HasResource, _ := workerHasResource(postgresWorkload, resourceName, resourceKind, DEV_WORKER_CLUSTER_2)

						g.Expect(platformHasResource).To(BeFalse(), "platform cluster should not have the crds")
						g.Expect(prodClusterHasCrd).To(BeFalse(), "prod cluster should not have the crds")
						g.Expect(devClusterHasResource).To(BeTrue(), "dev cluster 1 should have the crds")
						g.Expect(devCluster2HasResource).To(BeTrue(), "dev cluster 2 should have the crds")
					}, timeout, interval).Should(Succeed())
				})
			})
		})

		Describe("applying a paved-path-demo resource request", func() {
			var ppdPromiseWork = types.NamespacedName{
				Name:      "paved-path-demo-promise-default",
				Namespace: "default",
			}

			It("creates the instances on the dev clusters", func() {
				applyResourceRequest(PavedPathResourceRequest)

				By("triggering the request pipeline", func() {
					Eventually(func() bool {
						return hasResourceBeenApplied(work_gvk, ppdPromiseWork)
					}, timeout, interval).Should(BeTrue(), "paved path demo pipeline has not been triggered")
				})

				By("creating a work for the platform cluster", func() {
					Eventually(func(g Gomega) {
						var work platformv1alpha1.Work
						k8sClient.Get(context.Background(), ppdPromiseWork, &work)
						g.Expect(work.Spec.ClusterSelector).To(Equal(
							map[string]string{
								"environment": "platform",
							},
						))
					}, timeout, interval).Should(Succeed(), "paved path promise work does not have the right cluster selectors")
				})

				By("creating two works for the dev clusters", func() {
					testCases := []struct {
						name string
					}{
						{name: "knative-serving-promise-default-default-knative-serving"},
						{name: "ha-postgres-promise-default-default-acid-minimal-cluster"},
					}

					for _, testCase := range testCases {
						Eventually(func(g Gomega) {
							expectedWork := types.NamespacedName{
								Name:      testCase.name,
								Namespace: "default",
							}
							var work platformv1alpha1.Work
							k8sClient.Get(context.Background(), expectedWork, &work)
							g.Expect(work.Spec.ClusterSelector).To(Equal(
								map[string]string{
									"environment": "dev",
								},
							))
						}, timeout, interval).Should(Succeed())
					}
				})

				By("placing the resource yamls at one of the dev cluster buckets", func() {
					testCases := []struct {
						name         string
						kind         string
						metadataName string
					}{
						{
							name:         "ha-postgres-promise-default-default-acid-minimal-cluster",
							kind:         "postgresql",
							metadataName: "acid-minimal-cluster",
						},
						{
							name:         "knative-serving-promise-default-default-knative-serving",
							kind:         "Namespace",
							metadataName: "kourier-system",
						},
					}

					for _, testCase := range testCases {
						Eventually(func(g Gomega) {
							workloadNamespacedName := types.NamespacedName{
								Name:      testCase.name,
								Namespace: "default",
							}
							devClusterHasResources, _ := workerHasResource(workloadNamespacedName, testCase.metadataName, testCase.kind, DEV_WORKER_CLUSTER_1)
							devCluster2HasResources, _ := workerHasResource(workloadNamespacedName, testCase.metadataName, testCase.kind, DEV_WORKER_CLUSTER_2)
							platformClusterHasResources, _ := workerHasResource(workloadNamespacedName, testCase.metadataName, testCase.kind, PLATFORM_WORKER_CLUSTER_1)
							productionClusterHasResources, _ := workerHasResource(workloadNamespacedName, testCase.metadataName, testCase.kind, PRODUCTION_WORKER_CLUSTER)

							g.Expect(devClusterHasResources || devCluster2HasResources).To(BeTrue(), "one of the dev cluster should have the resources")
							g.Expect(platformClusterHasResources && productionClusterHasResources).To(BeFalse(), "neither prod nor platform cluster should have the resources")
						}, timeout, interval).Should(Succeed())
					}
				})
			})
		})
	})
})

func registerWorkerCluster(clusterName, clusterConfig string) {
	applyResourceRequest(clusterConfig)

	//defined in config/samples/platform_v1alpha1_worker_*_cluster.yaml
	expectedName := types.NamespacedName{
		Name:      clusterName,
		Namespace: "default",
	}
	Eventually(func() bool {
		return hasResourceBeenApplied(cluster_gvk, expectedName)
	}, timeout, interval).Should(BeTrue())
}

func installFlux(clusterName string, gitopsResourcePath string) {
	kubeCreate(GitOpsTKInstall)
	kubeCreate(gitopsResourcePath)

	Eventually(func(g Gomega) {
		namespace := &v1.Namespace{}
		resource := types.NamespacedName{
			Name: "kratix-worker-system",
		}
		k8sClient.Get(context.Background(), resource, namespace)
		g.Expect(err).ToNot(HaveOccurred())
	}, "120s", interval).Should(Succeed(), "timed out waiting for `kratix-worker-system` namespace (on "+clusterName+")")
}

func getClusterConfigPath(clusterConfig string) string {
	yamlFile, err := ioutil.ReadFile(clusterConfig)
	Expect(err).ToNot(HaveOccurred())

	cluster := &platformv1alpha1.Cluster{}
	err = yaml.Unmarshal(yamlFile, cluster)
	Expect(err).ToNot(HaveOccurred())
	return cluster.Spec.BucketPath
}

func workerHasCRD(workloadNamespacedName types.NamespacedName, resourceName, resourceKind, clusterConfig string) (bool, unstructured.Unstructured) {
	objectName := "00-" + workloadNamespacedName.Namespace + "-" + workloadNamespacedName.Name + "-crds.yaml"
	bucketName := getClusterConfigPath(clusterConfig) + "-kratix-crds"
	return minioHasWorkloadWithResourceWithNameAndKind(bucketName, objectName, resourceName, resourceKind)
}

func workerHasResource(workloadNamespacedName types.NamespacedName, resourceName, resourceKind, clusterConfig string) (bool, unstructured.Unstructured) {
	objectName := "01-" + workloadNamespacedName.Namespace + "-" + workloadNamespacedName.Name + "-resources.yaml"
	bucketName := getClusterConfigPath(clusterConfig) + "-kratix-resources"
	return minioHasWorkloadWithResourceWithNameAndKind(bucketName, objectName, resourceName, resourceKind)
}

func minioHasWorkloadWithResourceWithNameAndKind(bucketName string, objectName string, resourceName string, resourceKind string) (bool, unstructured.Unstructured) {
	endpoint := "localhost:31337"
	secretAccessKey := "minioadmin"
	accessKeyID := "minioadmin"
	useSSL := false

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	Expect(err).ToNot(HaveOccurred())

	minioObject, err := minioClient.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	Expect(err).ToNot(HaveOccurred())

	decoder := yaml.NewYAMLOrJSONDecoder(minioObject, 2048)

	ul := []unstructured.Unstructured{}
	for {
		us := unstructured.Unstructured{}
		err = decoder.Decode(&us)
		if err == io.EOF {
			//We reached the end of the file, move on to looking for the resource
			break
		} else if err != nil {
			/* There has been an error reading from Minio. It's likely that the
			   document has not been created in Minio yet, therefore we return
			   control to the ginkgo.Eventually to re-execute the assertions */
			return false, unstructured.Unstructured{}
		} else {
			//append the first resource to the resource slice, and go back through the loop
			ul = append(ul, us)
		}
	}

	for _, us := range ul {
		if us.GetKind() == resourceKind && us.GetName() == resourceName {
			//Hooray! we found the resource we're looking for!
			return true, us
		}
	}

	//We cannot find the resource and kind we are looking for
	return false, unstructured.Unstructured{}
}

//TODO Refactor this lot into own function. We can reuse this logic in controllers/suite_test.go
func hasResourceBeenApplied(gvk schema.GroupVersionKind, expectedName types.NamespacedName) bool {
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)

	err := k8sClient.Get(context.Background(), expectedName, resource)
	return err == nil
}

func isAPIResourcePresent(gvk schema.GroupVersionKind) bool {
	_, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	return err == nil
}

func applyResourceRequest(filepath string) {
	kubeCreate(filepath, "default")
}

func kubeCreate(filepath string, opts ...string) {
	yamlFile, err := os.Open(filepath)
	Expect(err).ToNot(HaveOccurred())

	resources := []*unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(yamlFile, 2048)
	for {
		us := unstructured.Unstructured{}

		err := decoder.Decode(&us)
		if err != nil {
			if err == io.EOF {
				break
			}
			Fail(err.Error())
		}
		if len(us.Object) == 0 {
			continue
		}
		resources = append(resources, &us)
	}

	for _, resource := range resources {
		if len(opts) != 0 {
			resource.SetNamespace(opts[0])
		}
		err = k8sClient.Create(context.Background(), resource)
		if err != nil && !errors.IsAlreadyExists(err) {
			Fail(err.Error())
		}
	}
}

func updateResourceRequest(filepath string) {
	yamlFile, err := ioutil.ReadFile(filepath)
	Expect(err).ToNot(HaveOccurred())

	request := &unstructured.Unstructured{}
	err = yaml.Unmarshal(yamlFile, request)
	Expect(err).ToNot(HaveOccurred())

	request.SetNamespace("default")

	currentResource := unstructured.Unstructured{}
	key := types.NamespacedName{
		Name:      request.GetName(),
		Namespace: request.GetNamespace(),
	}
	currentResource.SetGroupVersionKind(redis_gvk)

	err = k8sClient.Get(context.Background(), key, &currentResource)
	Expect(err).ToNot(HaveOccurred())

	//casting and stuff here
	currentResource.Object["spec"] = request.Object["spec"]
	err = k8sClient.Update(context.Background(), &currentResource)
	Expect(err).ToNot(HaveOccurred())
}

func applyPromiseCRD(filepath string) {
	promiseCR := &platformv1alpha1.Promise{}
	yamlFile, err := ioutil.ReadFile(filepath)
	Expect(err).NotTo(HaveOccurred())

	err = yaml.Unmarshal(yamlFile, promiseCR)
	Expect(err).ToNot(HaveOccurred())

	promiseCR.Namespace = "default"
	err = k8sClient.Create(context.Background(), promiseCR)
	if !errors.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred())
	}
}

func isPodRunning(pod v1.Pod) bool {
	switch pod.Status.Phase {
	case v1.PodRunning:
		return true
	default:
		return false
	}
}

func getKratixControllerPod() v1.Pod {
	isController, _ := labels.NewRequirement("control-plane", selection.Equals, []string{"controller-manager"})
	selector := labels.NewSelector().
		Add(*isController)

	listOps := &client.ListOptions{
		Namespace:     "kratix-platform-system",
		LabelSelector: selector,
	}

	pods := &v1.PodList{}
	k8sClient.List(context.Background(), pods, listOps)
	if len(pods.Items) == 1 {
		return pods.Items[0]
	}
	return v1.Pod{}
}

func initK8sClient() {
	cfg := ctrl.GetConfigOrDie()

	err = platformv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
}
