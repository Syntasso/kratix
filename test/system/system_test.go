package system_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type cluster struct {
	context string
}

var (
	promisePath               = "./assets/bash-promise/promise.yaml"
	promiseWithSchedulingPath = "./assets/bash-promise/promise-with-scheduling.yaml"

	workerCtx = "--context=kind-worker"
	platCtx   = "--context=kind-platform"

	timeout  = time.Second * 90
	interval = time.Second * 2

	platform = cluster{context: "--context=kind-platform"}
	worker   = cluster{context: "--context=kind-worker"}
)

const pipelineTimeout = "--timeout=89s"

// This test uses a unique Bash Promise which allows us to easily test behaviours
// in the pipeline.
//
// # The promise dependencies has a single resource, the `bash-wcr-namespace` Namespace
//
// Below is the template for a RR to this Promise. It provides a hook to run an
// arbitrary Bash command in each of the two Pipeline containers. An example use
// case may be wanting to test status works which requires a written to a
// specific location. To do this you can write a RR that has the following:
//
// container0Cmd: echo "statusTest: pass" > /metadata/status.yaml
//
// The commands will be run in the pipeline container that is named in the spec.
// The Promise pipeline will always have a set number of containers, though
// a command is not required for every container.
// e.g. `container0Cmd` is run in the first container of the pipeline.
var baseRequestYAML = `apiVersion: test.kratix.io/v1alpha1
kind: bash
metadata:
  name: %s
spec:
  container0Cmd: |
    %s
  container1Cmd: |
    %s`

var _ = Describe("Kratix", func() {
	BeforeSuite(func() {
		initK8sClient()
	})

	Describe("Promise lifecycle", func() {
		It("successfully manages the promise lifecycle", func() {
			By("installing the promise", func() {
				platform.kubectl("apply", "-f", promisePath)

				platform.eventuallyKubectl("get", "crd", "bash.test.kratix.io")
				worker.eventuallyKubectl("get", "namespace", "bash-wcr-namespace")
			})

			By("deleting a promise", func() {
				platform.kubectl("delete", "promise", "bash")

				Eventually(func(g Gomega) {
					g.Expect(worker.kubectl("get", "namespace")).NotTo(ContainSubstring("bash-wcr-namespace"))
					g.Expect(platform.kubectl("get", "promise")).ShouldNot(ContainSubstring("bash"))
					g.Expect(platform.kubectl("get", "crd")).ShouldNot(ContainSubstring("bash"))
				}, timeout, interval).Should(Succeed())
			})
		})

		Describe("Resource requests", func() {
			BeforeEach(func() {
				platform.kubectl("apply", "-f", promisePath)
				platform.eventuallyKubectl("get", "crd", "bash.test.kratix.io")
			})

			It("executes the pipelines and schedules the work", func() {
				rrName := "rr-test"
				c1Command := `kubectl create namespace rr-ns --dry-run=client -oyaml > /output/ns.yaml
							echo "message: My awesome status message" > /metadata/status.yaml
							echo "key: value" >> /metadata/status.yaml`
				c2Command := `kubectl create configmap multi-container-config --namespace rr-ns --dry-run=client -oyaml > /output/configmap.yaml`

				commands := []string{c1Command, c2Command}

				platform.kubectl("apply", "-f", requestWithNameAndCommand(rrName, commands...))

				By("executing the pipeline pod", func() {
					platform.kubectl("wait", "--for=condition=PipelineCompleted", "bash", rrName, pipelineTimeout)
				})

				By("deploying the contents of /output to the worker cluster", func() {
					worker.eventuallyKubectl("get", "namespace", "rr-ns")
					worker.eventuallyKubectl("get", "configmap", "multi-container-config", "--namespace", "rr-ns")
				})

				By("updating the resource status", func() {
					Expect(platform.kubectl("get", "bash", rrName)).To(ContainSubstring("My awesome status message"))
					Expect(platform.kubectl("get", "bash", rrName, "-o", "jsonpath='{.status.key}'")).To(ContainSubstring("value"))
				})

				By("deleting the resource request", func() {
					platform.kubectl("delete", "bash", rrName)

					Eventually(func(g Gomega) {
						g.Expect(platform.kubectl("get", "bash")).NotTo(ContainSubstring(rrName))
						g.Expect(worker.kubectl("get", "namespace")).NotTo(ContainSubstring("mcns"))
					}, timeout, interval).Should(Succeed())
				})

			})

			AfterEach(func() {
				platform.kubectl("delete", "promise", "bash")
				Eventually(platform.kubectl("get", "promise")).ShouldNot(ContainSubstring("bash"))
			})
		})
	})

	Describe("Scheduling", func() {
		// Worker cluster (BucketStateStore):
		// - environment: dev
		// - security: high

		// Platform cluster (GitStateStore):
		// - environment: platform

		// PromiseScheduling:
		// - security: high
		BeforeEach(func() {
			platform.kubectl("label", "cluster", "worker-cluster-1", "security=high")
			platform.kubectl("apply", "-f", "./assets/platform_gitops-tk-resources.yaml")
			platform.kubectl("apply", "-f", "./assets/platform_gitstatestore.yaml")
			platform.kubectl("apply", "-f", "./assets/platform_kratix_cluster.yaml")
			platform.kubectl("apply", "-f", promiseWithSchedulingPath)
			platform.eventuallyKubectl("get", "crd", "bash.test.kratix.io")
		})

		AfterEach(func() {
			platform.kubectl("label", "cluster", "worker-cluster-1", "security-", "pci-")
			platform.kubectl("delete", "-f", promiseWithSchedulingPath)
			platform.kubectl("delete", "-f", "./assets/platform_kratix_cluster.yaml")
			platform.kubectl("delete", "-f", "./assets/platform_gitstatestore.yaml")
		})

		It("schedules resources to the correct clusters", func() {
			By("reconciling on new clusters", func() {
				By("only the worker cluster getting the dependency", func() {
					worker.eventuallyKubectl("get", "namespace", "bash-wcr-namespace")
					Expect(platform.kubectl("get", "namespace")).NotTo(ContainSubstring("bash-wcr-namespace"))
				})

				By("labeling the platform cluster, it gets the dependencies assigned", func() {
					platform.kubectl("label", "cluster", "platform-cluster-worker-1", "security=high")
					platform.eventuallyKubectl("get", "namespace", "bash-wcr-namespace")
				})
			})

			By("respecting the pipeline's scheduling", func() {
				pipelineCmd := `echo "[{\"target\":{\"matchLabels\":{\"pci\":\"true\"}}}]" > /metadata/scheduling.yaml
				kubectl create namespace rr-2-namespace --dry-run=client -oyaml > /output/ns.yaml`
				platform.kubectl("apply", "-f", requestWithNameAndCommand("rr-2", pipelineCmd))

				platform.kubectl("wait", "--for=condition=PipelineCompleted", "bash", "rr-2", pipelineTimeout)

				By("only scheduling the work when a cluster label matches", func() {
					Consistently(func() string {
						return platform.kubectl("get", "namespace") + "\n" + worker.kubectl("get", "namespace")
					}, "10s").ShouldNot(ContainSubstring("rr-2-namespace"))

					platform.kubectl("label", "cluster", "worker-cluster-1", "pci=true")

					worker.eventuallyKubectl("get", "namespace", "rr-2-namespace")
				})
			})
		})
	})
})

func requestWithNameAndCommand(name string, containerCmds ...string) string {
	normalisedCmds := make([]string, 2)
	for i := range normalisedCmds {
		cmd := "cp /input/* /output;"
		if len(containerCmds) > i {
			cmd += " " + containerCmds[i]
		}
		normalisedCmds[i] = strings.ReplaceAll(cmd, "\n", ";")
	}

	lci := len(normalisedCmds) - 1
	lastCommand := normalisedCmds[lci]
	if strings.HasSuffix(normalisedCmds[lci], ";") {
		lastCommand = lastCommand[:len(lastCommand)-1]
	}
	normalisedCmds[lci] = lastCommand + "; rm /output/object.yaml"

	file, err := ioutil.TempFile("", "kratix-test")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	args := []interface{}{name}
	for _, cmd := range normalisedCmds {
		args = append(args, cmd)
	}

	contents := fmt.Sprintf(baseRequestYAML, args...)
	fmt.Fprintln(GinkgoWriter, "Resource Request:")
	fmt.Fprintln(GinkgoWriter, contents)

	ExpectWithOffset(1, ioutil.WriteFile(file.Name(), []byte(contents), 644)).NotTo(HaveOccurred())

	return file.Name()
}

// run a command until it exits 0
func (c cluster) eventuallyKubectl(args ...string) string {
	args = append(args, c.context)
	var content string
	EventuallyWithOffset(1, func(g Gomega) {
		command := exec.Command("kubectl", args...)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		g.ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
		g.EventuallyWithOffset(1, session).Should(gexec.Exit(0))
		content = string(session.Out.Contents())
	}, timeout, interval).Should(Succeed(), strings.Join(args, " "))
	return content
}

// run command and return stdout. Errors if exit code non-zero
func (c cluster) kubectl(args ...string) string {
	args = append(args, c.context)
	command := exec.Command("kubectl", args...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	EventuallyWithOffset(1, session, timeout, interval).Should(gexec.Exit(0))
	return string(session.Out.Contents())
}
